package models

type Object struct {
	BaseModel
	ID        uint          `gorm:"primaryKey"`
	Key       string        `gorm:"uniqueIndex:idx_bucket_key;not null"`
	BucketID  uint          `gorm:"uniqueIndex:idx_bucket_key;not null"`
	Bucket    Bucket        `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ContentID uint          `gorm:"index;not null"`
	Content   ObjectContent `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
