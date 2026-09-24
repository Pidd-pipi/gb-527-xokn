package dto

import "time"

type CreateContactWindowRequest struct {
	StationID        uint    `json:"station_id" validate:"required,gte=1"`
	SatelliteID      uint    `json:"satellite_id" validate:"required,gte=1"`
	StartAt          string  `json:"start_at" validate:"required"`
	EndAt            string  `json:"end_at" validate:"required"`
	Band             string  `json:"band" validate:"required,oneof=S X Ka Ku"`
	ElevationPeakDeg float64 `json:"elevation_peak_deg" validate:"gte=0,lte=90"`
	Priority         int     `json:"priority" validate:"gte=0,lte=10"`
	SourceVersion    string  `json:"source_version" validate:"required,min=3,max=64"`
}

type UpdateContactWindowRequest struct {
	StationID        uint    `json:"station_id" validate:"required,gte=1"`
	SatelliteID      uint    `json:"satellite_id" validate:"required,gte=1"`
	StartAt          string  `json:"start_at" validate:"required"`
	EndAt            string  `json:"end_at" validate:"required"`
	Band             string  `json:"band" validate:"required,oneof=S X Ka Ku"`
	ElevationPeakDeg float64 `json:"elevation_peak_deg" validate:"gte=0,lte=90"`
	WindowStatus     string  `json:"window_status" validate:"required,oneof=candidate submitted locked allocated cancelled"`
	Priority         int     `json:"priority" validate:"gte=0,lte=10"`
	SourceVersion    string  `json:"source_version" validate:"required,min=3,max=64"`
	ExpectedVersion  uint    `json:"expected_version" validate:"required,gte=1"`
}

type WindowActionRequest struct {
	ExpectedVersion uint `json:"expected_version" validate:"required,gte=1"`
}

type ContactWindowFilter struct {
	StationID   uint
	SatelliteID uint
	Status      string
	From        *time.Time
	To          *time.Time
	Page        int
	PageSize    int
}

type ContactWindowResponse struct {
	ID               uint      `json:"id"`
	StationID        uint      `json:"station_id"`
	StationCode      string    `json:"station_code"`
	SatelliteID      uint      `json:"satellite_id"`
	SatelliteCode    string    `json:"satellite_code"`
	StartAt          time.Time `json:"start_at"`
	EndAt            time.Time `json:"end_at"`
	DurationSec      int       `json:"duration_sec"`
	Band             string    `json:"band"`
	ElevationPeakDeg float64   `json:"elevation_peak_deg"`
	WindowStatus     string    `json:"window_status"`
	Priority         int       `json:"priority"`
	Locked           bool      `json:"locked"`
	SourceVersion    string    `json:"source_version"`
	Version          uint      `json:"version"`
	UpdatedAt        time.Time `json:"updated_at"`
}
