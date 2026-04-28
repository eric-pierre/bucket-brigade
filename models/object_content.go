package models

type ObjectContent struct {
	BaseModel
	ID       uint     `gorm:"primaryKey"`
	BucketID uint     `gorm:"uniqueIndex:idx_sha256_bucket;not null"`
	Bucket   Bucket   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	Sha256   string   `gorm:"uniqueIndex:idx_sha256_bucket;not null"`
	Size     int64    `gorm:"not null"`
	Path     string   `gorm:"not null"`
	RefCount int64    `gorm:"not null;default:0"`
	Objects  []Object `gorm:"foreignKey:ContentID"`
}
