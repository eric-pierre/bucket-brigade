package objects

import (
	"bucket-brigade/config"
	"bucket-brigade/dbs"
	"bucket-brigade/models"
	"bucket-brigade/pkg/buckets"
	"bucket-brigade/pkg/contents"
	"bucket-brigade/pkg/observability"
	"context"
	"errors"
	"io"
	"os"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"
)

type ObjectService struct {
	repo           ObjectRepository
	cfg            *config.Config
	bucketService  buckets.Service
	contentService contents.Service
	logger         *logrus.Entry
}

func NewObjectService(repo ObjectRepository, cfg *config.Config, bucketService buckets.Service, contentService contents.Service, logger *logrus.Entry) *ObjectService {
	return &ObjectService{
		repo:           repo,
		cfg:            cfg,
		bucketService:  bucketService,
		contentService: contentService,
		logger:         logger,
	}
}

func (s *ObjectService) WithTx(tx *gorm.DB) *ObjectService {
	return &ObjectService{
		repo:           s.repo.WithTx(tx),
		cfg:            s.cfg,
		bucketService:  s.bucketService.WithTx(tx),
		contentService: s.contentService.WithTx(tx),
		logger:         s.logger,
	}
}

func (s *ObjectService) UploadObject(ctx context.Context, bucketName, key string, body io.Reader) error {
	ctx, span := otel.Tracer(observability.ServiceName).Start(ctx, "UploadObject")
	defer span.End()

	var pendingCleanup *contents.PendingContentCleanup

	err := s.repo.Transaction(func(tx *gorm.DB) error {
		var err error
		pendingCleanup, err = s.WithTx(tx).uploadObjectTx(ctx, bucketName, key, body)
		return err
	})

	if err != nil {
		return err
	}

	s.contentService.FinalizeUnreferencedContent(ctx, pendingCleanup)
	s.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"key":    key,
	}).Debug("uploaded object")
	return nil
}

func (s *ObjectService) uploadObjectTx(ctx context.Context, bucketName, key string, body io.Reader) (pendingCleanup *contents.PendingContentCleanup, err error) {
	bucket, err := s.bucketService.GetOrCreateBucket(ctx, bucketName)
	if err != nil {
		return nil, err
	}

	contentResult, err := s.contentService.GetOrCreateContent(ctx, body, bucket)
	if err != nil {
		return nil, err
	}
	newContent := contentResult.Content
	if contentResult.Created {
		defer func() {
			if err != nil {
				s.contentService.CleanupNewContent(newContent.Path)
			}
		}()
	}

	oldObject, err := s.repo.GetByKeyAndBucket(ctx, key, bucket)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		object := &models.Object{Key: key, BucketID: bucket.ID, ContentID: newContent.ID}
		if err = s.repo.Create(ctx, object); err == nil {
			err = s.contentService.IncrementRefCount(ctx, newContent)
			return nil, err
		}
		if !dbs.IsUniqueConstraintError(err) {
			return nil, err
		}

		// Another concurrent PUT created the same (bucket, key) after our lookup but
		// before our INSERT. Treat the duplicate-key loser as an overwrite against the
		// now-existing row so callers do not see a spurious 500 under normal contention.
		oldObject, err = s.repo.GetByKeyAndBucket(ctx, key, bucket)
		if err != nil {
			return nil, err
		}
	}

	if oldObject.ContentID == newContent.ID {
		return nil, nil
	}

	oldContent := oldObject.Content
	oldObject.Content = models.ObjectContent{}
	oldObject.ContentID = newContent.ID
	if err = s.repo.Save(ctx, oldObject); err != nil {
		return nil, err
	}
	if err = s.contentService.IncrementRefCount(ctx, newContent); err != nil {
		return nil, err
	}
	pendingCleanup, err = s.contentService.DecrementRefCount(ctx, &oldContent)
	return pendingCleanup, err
}

func (s *ObjectService) DownloadObject(ctx context.Context, bucketName, key string) (*models.Object, io.ReadSeekCloser, error) {
	ctx, span := otel.Tracer(observability.ServiceName).Start(ctx, "DownloadObject")
	defer span.End()

	bucket, err := s.bucketService.GetByName(ctx, bucketName)
	if err != nil {
		return nil, nil, err
	}

	object, err := s.repo.GetByKeyAndBucket(ctx, key, bucket)
	if err != nil {
		return nil, nil, err
	}

	f, err := os.Open(object.Content.Path)
	if err != nil {
		return nil, nil, err
	}

	return object, f, nil
}

func (s *ObjectService) DeleteObject(ctx context.Context, bucketName, key string) error {
	ctx, span := otel.Tracer(observability.ServiceName).Start(ctx, "DeleteObject")
	defer span.End()

	var pendingCleanup *contents.PendingContentCleanup

	err := s.repo.Transaction(func(tx *gorm.DB) error {
		txService := s.WithTx(tx)

		bucket, err := txService.bucketService.GetByName(ctx, bucketName)
		if err != nil {
			return err
		}

		object, err := txService.repo.GetByKeyAndBucket(ctx, key, bucket)
		if err != nil {
			return err
		}

		content := object.Content

		if err := txService.repo.Delete(ctx, object); err != nil {
			return err
		}

		pendingCleanup, err = txService.contentService.DecrementRefCount(ctx, &content)
		return err
	})

	if err != nil {
		return err
	}

	s.contentService.FinalizeUnreferencedContent(ctx, pendingCleanup)
	s.logger.WithFields(logrus.Fields{
		"bucket": bucketName,
		"key":    key,
	}).Debug("deleted object")
	return nil
}
