package models

type ObjectContent struct {
	BaseModel
	Id       uint     `gorm:"primaryKey"`
	Sha256   string   `gorm:"uniqueIndex;not null"`
	Size     int64    `gorm:"not null"`
	Path     string   `gorm:"not null"`
	RefCount int64    `gorm:"not null;default:0"`
	Objects  []Object `gorm:"foreignKey:ContentId"`
}
