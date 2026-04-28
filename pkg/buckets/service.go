package buckets

import (
	"bucket-brigade/dbs"
	"bucket-brigade/models"
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Service interface {
	GetByName(ctx context.Context, name string) (*models.Bucket, error)
	GetOrCreateBucket(ctx context.Context, name string) (*models.Bucket, error)
	WithTx(tx *gorm.DB) Service
}

type BucketService struct {
	repo   BucketRepository
	logger *logrus.Entry
}

func NewBucketService(repo BucketRepository, logger *logrus.Entry) Service {
	return &BucketService{
		repo:   repo,
		logger: logger,
	}
}

func (s *BucketService) WithTx(tx *gorm.DB) Service {
	return &BucketService{
		repo:   s.repo.WithTx(tx),
		logger: s.logger,
	}
}

func (s *BucketService) GetByName(ctx context.Context, name string) (*models.Bucket, error) {
	return s.repo.GetByName(ctx, name)
}

func (s *BucketService) GetOrCreateBucket(ctx context.Context, name string) (*models.Bucket, error) {
	bucket, err := s.GetByName(ctx, name)
	if err == nil {
		return bucket, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	bucket = &models.Bucket{Name: name}
	if err = s.repo.Create(ctx, bucket); err != nil {
		if dbs.IsUniqueConstraintError(err) {
			return s.repo.GetByName(ctx, name)
		}
		return nil, err
	}
	return bucket, nil
}
