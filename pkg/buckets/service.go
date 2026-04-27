package buckets

import (
	"bucket-brigade/models"
	"errors"

	"gorm.io/gorm"
)

type BucketService struct {
	repo BucketRepository
}

func NewBucketService(repo BucketRepository) *BucketService {
	return &BucketService{repo: repo}
}

func (s *BucketService) WithTx(tx *gorm.DB) *BucketService {
	return &BucketService{
		repo: s.repo.WithTx(tx),
	}
}

func (s *BucketService) GetByName(name string) (*models.Bucket, error) {
	return s.repo.GetByName(name)
}

func (s *BucketService) GetOrCreateBucket(name string) (*models.Bucket, error) {
	bucket, err := s.GetByName(name)
	if err == nil {
		return bucket, nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		bucket = &models.Bucket{Name: name}
		return s.repo.Create(bucket)
	}
	return nil, err
}
