package model

import "time"

type GroundStation struct {
	ID                 uint      `gorm:"primaryKey"`
	StationCode        string    `gorm:"size:32;uniqueIndex;not null"`
	Name               string    `gorm:"size:120;not null"`
	Latitude           float64   `gorm:"not null"`
	Longitude          float64   `gorm:"not null"`
	AntennaCount       int       `gorm:"not null"`
	SupportedBandsJSON string    `gorm:"type:text;not null"`
	SlewBufferSec      int       `gorm:"not null"`
	StationStatus      string    `gorm:"size:24;index;not null"`
	Version            uint      `gorm:"not null;default:1"`
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (GroundStation) TableName() string { return "ground_stations" }
