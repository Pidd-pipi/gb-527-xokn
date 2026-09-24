package model

import "time"

type ContactWindow struct {
	ID               uint      `gorm:"primaryKey"`
	StationID        uint      `gorm:"index;not null"`
	SatelliteID      uint      `gorm:"index;not null"`
	StartAt          time.Time `gorm:"index;not null"`
	EndAt            time.Time `gorm:"index;not null"`
	Band             string    `gorm:"size:12;not null"`
	ElevationPeakDeg float64   `gorm:"not null"`
	WindowStatus     string    `gorm:"size:24;index;not null"`
	Priority         int       `gorm:"not null"`
	Locked           bool      `gorm:"index;not null"`
	SourceVersion    string    `gorm:"size:64;not null"`
	Version          uint      `gorm:"not null;default:1"`
	CreatedAt        time.Time `gorm:"not null"`
	UpdatedAt        time.Time `gorm:"not null"`
}

func (ContactWindow) TableName() string { return "contact_windows" }

func (window ContactWindow) DurationSec() int {
	return int(window.EndAt.Sub(window.StartAt).Seconds())
}
