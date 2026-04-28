package contents

import (
	"bucket-brigade/config"
	"bucket-brigade/dbs"
	"bucket-brigade/models"
	"bucket-brigade/pkg/observability"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
)

type ContentResult struct {
	Content *models.ObjectContent
	Created bool // true if a new content record and backing file were created
}

type PendingContentCleanup struct {
	Content *models.ObjectContent
}

type Service interface {
	GetOrCreateContent(ctx context.Context, body io.Reader, bucket *models.Bucket) (*ContentResult, error)
	IncrementRefCount(ctx context.Context, content *models.ObjectContent) error
	DecrementRefCount(ctx context.Context, content *models.ObjectContent) (*PendingContentCleanup, error)
	CleanupNewContent(path string)
	FinalizeUnreferencedContent(ctx context.Context, cleanup *PendingContentCleanup)
	CleanupZeroRefContents(ctx context.Context)
	WithTx(tx *gorm.DB) Service
}

type ObjectContentService struct {
	repo        ObjectContentRepository
	contentsDir string
	logger      *logrus.Entry
}

func NewObjectContentService(repo ObjectContentRepository, cfg *config.Config, logger *logrus.Entry) (Service, error) {
	contentsDir := filepath.Join(cfg.Storage.BasePath, "contents")
	if err := os.MkdirAll(contentsDir, 0755); err != nil {
		return nil, err
	}
	return &ObjectContentService{
		repo:        repo,
		contentsDir: contentsDir,
		logger:      logger,
	}, nil
}

func (s *ObjectContentService) WithTx(tx *gorm.DB) Service {
	return &ObjectContentService{
		repo:        s.repo.WithTx(tx),
		contentsDir: s.contentsDir,
		logger:      s.logger,
	}
}

func (s *ObjectContentService) GetOrCreateContent(ctx context.Context, body io.Reader, bucket *models.Bucket) (*ContentResult, error) {
	tempFile, err := os.CreateTemp(s.contentsDir, "bb-content-*")
	if err != nil {
		return nil, err
	}
	removeTempFile := true
	defer func() {
		if removeTempFile {
			_ = os.Remove(tempFile.Name())
		}
	}()
	defer tempFile.Close()

	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	size, err := io.Copy(multiWriter, body)
	if err != nil {
		return nil, err
	}

	sha256Sum := hex.EncodeToString(hasher.Sum(nil))

	content, err := s.repo.GetBySha256AndBucket(ctx, sha256Sum, bucket.ID)
	if err == nil {
		return &ContentResult{Content: content, Created: false}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	content = &models.ObjectContent{
		BucketID: bucket.ID,
		Sha256:   sha256Sum,
		Size:     size,
		Path:     tempFile.Name(),
		RefCount: 0,
	}
	if err := s.repo.Create(ctx, content); err != nil {
		// Concurrent uploads of the same bytes into the same bucket can both miss the
		// initial lookup, then race on the unique (sha256, bucket_id) index. Treat the
		// duplicate-key loser as success by re-loading the row that the winner created.
		if dbs.IsUniqueConstraintError(err) {
			content, err := s.repo.GetBySha256AndBucket(ctx, sha256Sum, bucket.ID)
			if err != nil {
				return nil, err
			}
			return &ContentResult{Content: content, Created: false}, nil
		}
		return nil, err
	}
	removeTempFile = false
	return &ContentResult{Content: content, Created: true}, nil
}

func (s *ObjectContentService) IncrementRefCount(ctx context.Context, content *models.ObjectContent) error {
	content, err := s.repo.Lock(ctx, content)
	if err != nil {
		return err
	}
	content.RefCount++
	return s.repo.Update(ctx, content)
}

func (s *ObjectContentService) DecrementRefCount(ctx context.Context, content *models.ObjectContent) (*PendingContentCleanup, error) {
	content, err := s.repo.Lock(ctx, content)
	if err != nil {
		return nil, err
	}

	content.RefCount--
	s.logger.WithFields(logrus.Fields{
		"content_id": content.ID,
		"ref_count":  content.RefCount,
	}).Debug("decremented ref count")

	if content.RefCount <= 0 {
		content.RefCount = 0
		if err := s.repo.Update(ctx, content); err != nil {
			return nil, err
		}
		return &PendingContentCleanup{Content: content}, nil
	}

	return nil, s.repo.Update(ctx, content)
}

func (s *ObjectContentService) CleanupNewContent(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		s.logger.WithFields(logrus.Fields{
			"path":  path,
			"error": err,
		}).Error("failed to clean up rolled back content file")
	}
}

func (s *ObjectContentService) FinalizeUnreferencedContent(ctx context.Context, cleanup *PendingContentCleanup) {
	if cleanup == nil {
		return
	}

	ctx, span := otel.Tracer(observability.ServiceName).Start(ctx, "FinalizeUnreferencedContent")
	defer span.End()

	// Re-fetch to guard against a concurrent transaction that incremented ref count
	// between our decrement and this post-commit cleanup.
	content, err := s.repo.GetOne(ctx, cleanup.Content.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"content_id": cleanup.Content.ID,
			"error":      err,
		}).Error("failed to load unreferenced content for cleanup")
		return
	}
	if content.RefCount > 0 {
		return
	}

	if err := os.Remove(cleanup.Content.Path); err != nil && !os.IsNotExist(err) {
		s.logger.WithFields(logrus.Fields{
			"path":  cleanup.Content.Path,
			"error": err,
		}).Error("failed to remove unreferenced content file")
		return
	}
	if err := s.repo.Delete(ctx, content); err != nil {
		s.logger.WithFields(logrus.Fields{
			"content_id": content.ID,
			"error":      err,
		}).Error("failed to delete unreferenced content row")
	}
}

// CleanupZeroRefContents is a startup crash-recovery sweep. In the normal delete
// flow, ref_count is set to 0 inside a transaction and the backing file and DB
// row are removed post-commit. A crash in that window leaves ref_count=0 rows
// that will never be cleaned up otherwise.
func (s *ObjectContentService) CleanupZeroRefContents(ctx context.Context) {
	ctx, span := otel.Tracer(observability.ServiceName).Start(ctx, "CleanupZeroRefContents")
	defer span.End()

	contents, err := s.repo.ListByRefCount(ctx, 0)
	if err != nil {
		s.logger.WithError(err).Error("failed to list zero-ref contents for cleanup")
		return
	}

	for i := range contents {
		content := contents[i]
		if err := os.Remove(content.Path); err != nil && !os.IsNotExist(err) {
			s.logger.WithFields(logrus.Fields{
				"path":  content.Path,
				"error": err,
			}).Error("failed to remove zero-ref content file")
			continue
		}
		if err := s.repo.Delete(ctx, &content); err != nil {
			s.logger.WithFields(logrus.Fields{
				"content_id": content.ID,
				"error":      err,
			}).Error("failed to delete zero-ref content row")
		}
	}
}
