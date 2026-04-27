package models

type Bucket struct {
	BaseModel
	Id      uint     `gorm:"primaryKey"`
	Name    string   `gorm:"uniqueIndex;not null"`
	Objects []Object `gorm:"foreignKey:BucketId"`
}
