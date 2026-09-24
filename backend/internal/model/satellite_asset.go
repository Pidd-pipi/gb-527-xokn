package model

import "time"

type SatelliteAsset struct {
	ID                 uint      `gorm:"primaryKey"`
	SatelliteCode      string    `gorm:"size:32;uniqueIndex;not null"`
	Name               string    `gorm:"size:120;not null"`
	OrbitClass         string    `gorm:"size:24;not null"`
	SupportedBandsJSON string    `gorm:"type:text;not null"`
	PriorityWeight     float64   `gorm:"not null"`
	MinimumContactSec  int       `gorm:"not null"`
	AssetStatus        string    `gorm:"size:24;index;not null"`
	Version            uint      `gorm:"not null;default:1"`
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (SatelliteAsset) TableName() string { return "satellite_assets" }
