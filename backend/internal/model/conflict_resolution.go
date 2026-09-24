package model

import "time"

type ConflictResolution struct {
	ID                 uint   `gorm:"primaryKey"`
	ConflictKey        string `gorm:"size:80;uniqueIndex;not null"`
	WindowIDsJSON      string `gorm:"type:text;not null"`
	WindowVersionsJSON string `gorm:"type:text;not null"`
	ConflictType       string `gorm:"size:32;index;not null"`
	EvidenceJSON       string `gorm:"type:text;not null"`
	SuggestionsJSON    string `gorm:"type:text;not null"`
	WeightsJSON        string `gorm:"type:text;not null"`
	SelectedAction     string `gorm:"type:text;not null"`
	ResolutionStatus   string `gorm:"size:24;index;not null"`
	ResolvedBy         string `gorm:"size:80;not null"`
	ReviewNote         string `gorm:"size:500;not null"`
	Version            uint   `gorm:"not null;default:1"`
	ResolvedAt         *time.Time
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

func (ConflictResolution) TableName() string { return "conflict_resolutions" }
