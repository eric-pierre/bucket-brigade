package objects

import (
	"bucket-brigade/models"

	"gorm.io/gorm"
)

type ObjectRepository interface {
	GetByKeyAndBucket(key string, bucket *models.Bucket, preloadContent bool) (*models.Object, error)
	Create(object *models.Object) error
	Save(object *models.Object) error
	Delete(object *models.Object, unscoped bool) error
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

func (r *objectRepository) GetByKeyAndBucket(key string, bucket *models.Bucket, preloadContent bool) (*models.Object, error) {
	var object models.Object
	db := r.db
	if preloadContent {
		db = db.Preload("Content")
	}
	err := db.Where("key = ? AND bucket_id = ?", key, bucket.Id).First(&object).Error
	if err != nil {
		return nil, err
	}
	return &object, nil
}

func (r *objectRepository) Create(object *models.Object) error {
	return r.db.Create(object).Error
}

func (r *objectRepository) Save(object *models.Object) error {
	return r.db.Save(object).Error
}

func (r *objectRepository) Delete(object *models.Object, unscoped bool) error {
	db := r.db
	if unscoped {
		db = db.Unscoped()
	}
	return db.Delete(object).Error
}
