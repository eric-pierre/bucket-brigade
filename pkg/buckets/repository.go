package buckets

import (
	"bucket-brigade/models"
	"context"

	"gorm.io/gorm"
)

type BucketRepository interface {
	GetByName(ctx context.Context, name string) (*models.Bucket, error)
	Create(ctx context.Context, bucket *models.Bucket) error
	WithTx(tx *gorm.DB) BucketRepository
}

type bucketRepository struct {
	db *gorm.DB
}

func NewBucketRepository(db *gorm.DB) BucketRepository {
	return &bucketRepository{db: db}
}

func (r *bucketRepository) WithTx(tx *gorm.DB) BucketRepository {
	return &bucketRepository{db: tx}
}

func (r *bucketRepository) GetByName(ctx context.Context, name string) (*models.Bucket, error) {
	var bucket models.Bucket
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&bucket).Error
	if err != nil {
		return nil, err
	}
	return &bucket, nil
}

func (r *bucketRepository) Create(ctx context.Context, bucket *models.Bucket) error {
	return r.db.WithContext(ctx).Create(bucket).Error
}
