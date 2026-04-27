package objects

import (
	"bucket-brigade/config"
	"bucket-brigade/models"
	"bucket-brigade/pkg/buckets"
	"bucket-brigade/pkg/contents"
	"errors"
	"fmt"
	"io"
	"os"

	"gorm.io/gorm"
)

type ObjectService struct {
	repo           ObjectRepository
	cfg            *config.Config
	bucketService  BucketService
	contentService ContentService
}

type BucketService interface {
	GetByName(name string) (*models.Bucket, error)
	GetOrCreateBucket(name string) (*models.Bucket, error)
}

type bucketServiceTx interface {
	BucketService
	WithTx(tx *gorm.DB) *buckets.BucketService
}

type ContentService interface {
	GetOrCreateContent(body io.Reader) (*models.ObjectContent, bool, error)
	IncrementRefCount(content *models.ObjectContent) error
	DecrementRefCount(content *models.ObjectContent) (*contents.PendingContentCleanup, error)
	CleanupNewContent(path string)
	FinalizeUnreferencedContent(cleanup *contents.PendingContentCleanup)
	CleanupZeroRefContents()
}

type contentServiceTx interface {
	ContentService
	WithTx(tx *gorm.DB) *contents.ObjectContentService
}

func NewObjectService(repo ObjectRepository, cfg *config.Config, bucketService BucketService, contentService ContentService) *ObjectService {
	return &ObjectService{
		repo:           repo,
		cfg:            cfg,
		bucketService:  bucketService,
		contentService: contentService,
	}
}

func (s *ObjectService) WithTx(tx *gorm.DB) *ObjectService {
	txBucketService := s.bucketService.(bucketServiceTx)
	txContentService := s.contentService.(contentServiceTx)

	return &ObjectService{
		repo:           s.repo.WithTx(tx),
		cfg:            s.cfg,
		bucketService:  txBucketService.WithTx(tx),
		contentService: txContentService.WithTx(tx),
	}
}

func (s *ObjectService) UploadObject(bucketName, objectId string, body io.Reader) error {
	var rollbackNewContentPath string
	var pendingCleanup *contents.PendingContentCleanup

	err := s.repo.Transaction(func(tx *gorm.DB) error {
		txService := s.WithTx(tx)

		bucket, err := txService.bucketService.GetOrCreateBucket(bucketName)
		if err != nil {
			return err
		}

		newContent, createdNewContent, err := txService.contentService.GetOrCreateContent(body)
		if err != nil {
			return err
		}
		if createdNewContent {
			rollbackNewContentPath = newContent.Path
		}

		// Check if object already exists
		oldObject, err := txService.repo.GetByKeyAndBucket(objectId, bucket, true)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// New object
			object := &models.Object{
				Key:       objectId,
				BucketId:  bucket.Id,
				ContentId: newContent.Id,
			}
			if err := txService.repo.Create(object); err != nil {
				return err
			}
			// Increment refcount for new content
			return txService.contentService.IncrementRefCount(newContent)
		} else if err != nil {
			return err
		}

		// Existing object
		if oldObject.ContentId != newContent.Id {
			oldContent := oldObject.Content

			// Update object to point to new content
			oldObject.Content = models.ObjectContent{}
			oldObject.ContentId = newContent.Id
			if err := txService.repo.Save(oldObject); err != nil {
				return err
			}

			// Increment refcount for new content
			if err := txService.contentService.IncrementRefCount(newContent); err != nil {
				return err
			}

			// Decrement refcount for old content
			pendingCleanup, err = txService.contentService.DecrementRefCount(&oldContent)
			return err
		}

		// Same content, do nothing
		return nil
	})

	if err != nil {
		s.contentService.CleanupNewContent(rollbackNewContentPath)
		return err
	}

	s.contentService.FinalizeUnreferencedContent(pendingCleanup)
	return nil
}

func (s *ObjectService) DownloadObject(bucketName, objectId string) (*models.Object, io.ReadSeekCloser, error) {
	bucket, err := s.bucketService.GetByName(bucketName)
	if err != nil {
		return nil, nil, err
	}

	object, err := s.repo.GetByKeyAndBucket(objectId, bucket, true)
	if err != nil {
		return nil, nil, err
	}

	if object.Content.Path == "" {
		return nil, nil, fmt.Errorf("object has no content")
	}

	f, err := os.Open(object.Content.Path)
	if err != nil {
		return nil, nil, err
	}

	return object, f, nil
}

func (s *ObjectService) DeleteObject(bucketName, objectId string) error {
	var pendingCleanup *contents.PendingContentCleanup

	err := s.repo.Transaction(func(tx *gorm.DB) error {
		txService := s.WithTx(tx)

		bucket, err := txService.bucketService.GetByName(bucketName)
		if err != nil {
			return err
		}

		object, err := txService.repo.GetByKeyAndBucket(objectId, bucket, true)
		if err != nil {
			return err
		}

		content := object.Content

		// Delete from DB (unscoped)
		if err := txService.repo.Delete(object, true); err != nil {
			return err
		}

		// Decrement refcount for content
		pendingCleanup, err = txService.contentService.DecrementRefCount(&content)
		return err
	})

	if err != nil {
		return err
	}

	s.contentService.FinalizeUnreferencedContent(pendingCleanup)
	return nil
}
