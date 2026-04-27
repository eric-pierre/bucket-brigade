package models

type Object struct {
	BaseModel
	Id        uint          `gorm:"primaryKey"`
	Key       string        `gorm:"uniqueIndex:idx_bucket_key;not null"`
	BucketId  uint          `gorm:"uniqueIndex:idx_bucket_key;not null"`
	Bucket    Bucket        `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	ContentId uint          `gorm:"index;not null"`
	Content   ObjectContent `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
}
