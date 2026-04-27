package buckets

import (
	"bucket-brigade/models"

	"gorm.io/gorm"
)

type BucketRepository interface {
	GetByName(name string) (*models.Bucket, error)
	Create(bucket *models.Bucket) (*models.Bucket, error)
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

func (r *bucketRepository) GetByName(name string) (*models.Bucket, error) {
	var bucket models.Bucket
	err := r.db.Where("name = ?", name).First(&bucket).Error
	if err != nil {
		return nil, err
	}
	return &bucket, nil
}

func (r *bucketRepository) Create(bucket *models.Bucket) (*models.Bucket, error) {
	if err := r.db.Create(bucket).Error; err != nil {
		return nil, err
	}
	return bucket, nil
}
