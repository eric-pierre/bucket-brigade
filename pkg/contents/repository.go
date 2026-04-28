package contents

import (
	"bucket-brigade/models"
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ObjectContentRepository interface {
	GetBySha256AndBucket(ctx context.Context, sha256 string, bucketID uint) (*models.ObjectContent, error)
	GetOne(ctx context.Context, contentId uint) (*models.ObjectContent, error)
	ListByRefCount(ctx context.Context, refCount int64) ([]models.ObjectContent, error)
	Lock(ctx context.Context, content *models.ObjectContent) (*models.ObjectContent, error)
	Create(ctx context.Context, content *models.ObjectContent) error
	Update(ctx context.Context, content *models.ObjectContent) error
	Delete(ctx context.Context, content *models.ObjectContent) error
	WithTx(tx *gorm.DB) ObjectContentRepository
}

type objectContentRepository struct {
	db *gorm.DB
}

func NewObjectContentRepository(db *gorm.DB) ObjectContentRepository {
	return &objectContentRepository{db: db}
}

func (r *objectContentRepository) WithTx(tx *gorm.DB) ObjectContentRepository {
	return &objectContentRepository{db: tx}
}

func (r *objectContentRepository) GetBySha256AndBucket(ctx context.Context, sha256 string, bucketID uint) (*models.ObjectContent, error) {
	var content models.ObjectContent
	err := r.db.WithContext(ctx).Where("sha256 = ? AND bucket_id = ?", sha256, bucketID).First(&content).Error
	if err != nil {
		return nil, err
	}
	return &content, nil
}

func (r *objectContentRepository) GetOne(ctx context.Context, contentId uint) (*models.ObjectContent, error) {
	var content models.ObjectContent
	err := r.db.WithContext(ctx).First(&content, contentId).Error
	if err != nil {
		return nil, err
	}
	return &content, nil
}

func (r *objectContentRepository) ListByRefCount(ctx context.Context, refCount int64) ([]models.ObjectContent, error) {
	var contents []models.ObjectContent
	err := r.db.WithContext(ctx).Where("ref_count = ?", refCount).Find(&contents).Error
	return contents, err
}

func (r *objectContentRepository) Lock(ctx context.Context, content *models.ObjectContent) (*models.ObjectContent, error) {
	var locked models.ObjectContent
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, content.ID).Error
	if err != nil {
		return nil, err
	}
	return &locked, nil
}

func (r *objectContentRepository) Create(ctx context.Context, content *models.ObjectContent) error {
	return r.db.WithContext(ctx).Create(content).Error
}

func (r *objectContentRepository) Update(ctx context.Context, content *models.ObjectContent) error {
	return r.db.WithContext(ctx).Save(content).Error
}

func (r *objectContentRepository) Delete(ctx context.Context, content *models.ObjectContent) error {
	return r.db.WithContext(ctx).Delete(content).Error
}
