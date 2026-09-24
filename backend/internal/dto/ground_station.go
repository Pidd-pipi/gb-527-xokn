package dto

import "time"

type CreateGroundStationRequest struct {
	StationCode    string   `json:"station_code" validate:"required,min=3,max=32"`
	Name           string   `json:"name" validate:"required,min=2,max=120"`
	Latitude       float64  `json:"latitude" validate:"gte=-90,lte=90"`
	Longitude      float64  `json:"longitude" validate:"gte=-180,lte=180"`
	AntennaCount   int      `json:"antenna_count" validate:"required,gte=1,lte=32"`
	SupportedBands []string `json:"supported_bands" validate:"required,min=1,dive,oneof=S X Ka Ku"`
	SlewBufferSec  int      `json:"slew_buffer_sec" validate:"gte=0,lte=3600"`
	StationStatus  string   `json:"station_status" validate:"required,oneof=active maintenance retired"`
}

type UpdateGroundStationRequest struct {
	Name            string   `json:"name" validate:"required,min=2,max=120"`
	Latitude        float64  `json:"latitude" validate:"gte=-90,lte=90"`
	Longitude       float64  `json:"longitude" validate:"gte=-180,lte=180"`
	AntennaCount    int      `json:"antenna_count" validate:"required,gte=1,lte=32"`
	SupportedBands  []string `json:"supported_bands" validate:"required,min=1,dive,oneof=S X Ka Ku"`
	SlewBufferSec   int      `json:"slew_buffer_sec" validate:"gte=0,lte=3600"`
	StationStatus   string   `json:"station_status" validate:"required,oneof=active maintenance retired"`
	ExpectedVersion uint     `json:"expected_version" validate:"required,gte=1"`
}

type GroundStationResponse struct {
	ID             uint      `json:"id"`
	StationCode    string    `json:"station_code"`
	Name           string    `json:"name"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	AntennaCount   int       `json:"antenna_count"`
	SupportedBands []string  `json:"supported_bands"`
	SlewBufferSec  int       `json:"slew_buffer_sec"`
	StationStatus  string    `json:"station_status"`
	Version        uint      `json:"version"`
	UpdatedAt      time.Time `json:"updated_at"`
}
