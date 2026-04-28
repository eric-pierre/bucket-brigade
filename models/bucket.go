package models

type Bucket struct {
	BaseModel
	ID      uint     `gorm:"primaryKey"`
	Name    string   `gorm:"uniqueIndex;not null"`
	Objects []Object `gorm:"foreignKey:BucketID"`
}
