package contents

import (
	"bucket-brigade/config"
	"bucket-brigade/dbs"
	"bucket-brigade/models"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type PendingContentCleanup struct {
	Content *models.ObjectContent
}

type ObjectContentService struct {
	repo ObjectContentRepository
	cfg  *config.Config
}

func NewObjectContentService(repo ObjectContentRepository, cfg *config.Config) *ObjectContentService {
	return &ObjectContentService{
		repo: repo,
		cfg:  cfg,
	}
}

func (s *ObjectContentService) WithTx(tx *gorm.DB) *ObjectContentService {
	return &ObjectContentService{
		repo: s.repo.WithTx(tx),
		cfg:  s.cfg,
	}
}

// GetOrCreateContent creates a content record if it doesn't exist and returns whether
// a new backing file was created during this call.
func (s *ObjectContentService) GetOrCreateContent(body io.Reader) (*models.ObjectContent, bool, error) {
	contentsDir := filepath.Join(s.cfg.Storage.BasePath, "contents")
	if err := os.MkdirAll(contentsDir, 0755); err != nil {
		return nil, false, err
	}

	tempFile, err := os.CreateTemp(contentsDir, "bb-content-*")
	if err != nil {
		return nil, false, err
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
		return nil, false, err
	}

	sha256Sum := hex.EncodeToString(hasher.Sum(nil))

	// Check if content already exists
	content, err := s.repo.GetBySha256(sha256Sum)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		content = &models.ObjectContent{
			Sha256:   sha256Sum,
			Size:     size,
			Path:     tempFile.Name(),
			RefCount: 0,
		}
		if err := s.repo.Create(content); err != nil {
			if dbs.IsUniqueConstraintError(err) {
				content, err := s.repo.GetBySha256(sha256Sum)
				return content, false, err
			}
			return nil, false, err
		}
		removeTempFile = false
		return content, true, nil
	} else if err != nil {
		return nil, false, err
	}

	return content, false, nil
}

func (s *ObjectContentService) IncrementRefCount(content *models.ObjectContent) error {
	content, err := s.repo.Lock(content)
	if err != nil {
		return err
	}
	content.RefCount++
	return s.repo.Update(content)
}

func (s *ObjectContentService) DecrementRefCount(content *models.ObjectContent) (*PendingContentCleanup, error) {
	content, err := s.repo.Lock(content)
	if err != nil {
		return nil, err
	}

	content.RefCount--

	if content.RefCount <= 0 {
		content.RefCount = 0
		if err := s.repo.Update(content); err != nil {
			return nil, err
		}
		return &PendingContentCleanup{Content: content}, nil
	}

	return nil, s.repo.Update(content)
}

func (s *ObjectContentService) CleanupNewContent(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logrus.Errorf("failed to clean up rolled back content file %s: %v", path, err)
	}
}

func (s *ObjectContentService) FinalizeUnreferencedContent(cleanup *PendingContentCleanup) {
	if cleanup == nil {
		return
	}

	content, err := s.repo.Get(cleanup.Content)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		logrus.Errorf("failed to load unreferenced content %d for cleanup: %v", cleanup.Content.Id, err)
		return
	}
	if content.RefCount > 0 || content.Path != cleanup.Content.Path {
		return
	}

	if err := os.Remove(cleanup.Content.Path); err != nil && !os.IsNotExist(err) {
		logrus.Errorf("failed to remove unreferenced content file %s: %v", cleanup.Content.Path, err)
		return
	}
	if err := s.repo.Delete(content); err != nil {
		logrus.Errorf("failed to delete unreferenced content row %d: %v", cleanup.Content.Id, err)
	}
}

func (s *ObjectContentService) CleanupZeroRefContents() {
	contents, err := s.repo.ListByRefCount(0)
	if err != nil {
		logrus.Errorf("failed to list zero-ref contents for cleanup: %v", err)
		return
	}

	for i := range contents {
		content := contents[i]
		if err := os.Remove(content.Path); err != nil && !os.IsNotExist(err) {
			logrus.Errorf("failed to remove zero-ref content file %s: %v", content.Path, err)
			continue
		}
		if err := s.repo.Delete(&content); err != nil {
			logrus.Errorf("failed to delete zero-ref content row %d: %v", content.Id, err)
		}
	}
}
