package objects

import (
	"bucket-brigade/models"
	"context"

	"gorm.io/gorm"
)

type ObjectRepository interface {
	GetByKeyAndBucket(ctx context.Context, key string, bucket *models.Bucket) (*models.Object, error)
	Create(ctx context.Context, object *models.Object) error
	Save(ctx context.Context, object *models.Object) error
	Delete(ctx context.Context, object *models.Object) error
	WithTx(tx *gorm.DB) ObjectRepository
	Transaction(fn func(tx *gorm.DB) error) error
}

type objectRepository struct {
	db *gorm.DB
}

func NewObjectRepository(db *gorm.DB) ObjectRepository {
	return &objectRepository{db: db}
}

func (r *objectRepository) WithTx(tx *gorm.DB) ObjectRepository {
	return &objectRepository{db: tx}
}

func (r *objectRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *objectRepository) GetByKeyAndBucket(ctx context.Context, key string, bucket *models.Bucket) (*models.Object, error) {
	var object models.Object
	err := r.db.WithContext(ctx).Preload("Content").Where("key = ? AND bucket_id = ?", key, bucket.ID).First(&object).Error
	if err != nil {
		return nil, err
	}
	return &object, nil
}

func (r *objectRepository) Create(ctx context.Context, object *models.Object) error {
	return r.db.WithContext(ctx).Create(object).Error
}

func (r *objectRepository) Save(ctx context.Context, object *models.Object) error {
	return r.db.WithContext(ctx).Save(object).Error
}

func (r *objectRepository) Delete(ctx context.Context, object *models.Object) error {
	return r.db.WithContext(ctx).Delete(object).Error
}
