package contents

import (
	"bucket-brigade/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ObjectContentRepository interface {
	GetBySha256(sha256 string) (*models.ObjectContent, error)
	Get(content *models.ObjectContent) (*models.ObjectContent, error)
	ListByRefCount(refCount int64) ([]models.ObjectContent, error)
	Lock(content *models.ObjectContent) (*models.ObjectContent, error)
	Create(content *models.ObjectContent) error
	Update(content *models.ObjectContent) error
	Delete(content *models.ObjectContent) error
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

func (r *objectContentRepository) GetBySha256(sha256 string) (*models.ObjectContent, error) {
	var content models.ObjectContent
	err := r.db.Where("sha256 = ?", sha256).First(&content).Error
	if err != nil {
		return nil, err
	}
	return &content, nil
}

func (r *objectContentRepository) Get(content *models.ObjectContent) (*models.ObjectContent, error) {
	var loaded models.ObjectContent
	err := r.db.First(&loaded, content.Id).Error
	if err != nil {
		return nil, err
	}
	return &loaded, nil
}

func (r *objectContentRepository) ListByRefCount(refCount int64) ([]models.ObjectContent, error) {
	var contents []models.ObjectContent
	err := r.db.Where("ref_count = ?", refCount).Find(&contents).Error
	return contents, err
}

func (r *objectContentRepository) Lock(content *models.ObjectContent) (*models.ObjectContent, error) {
	var locked models.ObjectContent
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, content.Id).Error
	if err != nil {
		return nil, err
	}
	return &locked, nil
}

func (r *objectContentRepository) Create(content *models.ObjectContent) error {
	return r.db.Create(content).Error
}

func (r *objectContentRepository) Update(content *models.ObjectContent) error {
	return r.db.Save(content).Error
}

func (r *objectContentRepository) Delete(content *models.ObjectContent) error {
	return r.db.Unscoped().Delete(content).Error
}
