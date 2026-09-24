package dto

import "time"

type CreateSatelliteAssetRequest struct {
	SatelliteCode     string   `json:"satellite_code" validate:"required,min=3,max=32"`
	Name              string   `json:"name" validate:"required,min=2,max=120"`
	OrbitClass        string   `json:"orbit_class" validate:"required,oneof=LEO MEO GEO HEO"`
	SupportedBands    []string `json:"supported_bands" validate:"required,min=1,dive,oneof=S X Ka Ku"`
	PriorityWeight    float64  `json:"priority_weight" validate:"gte=0,lte=100"`
	MinimumContactSec int      `json:"minimum_contact_sec" validate:"required,gte=30,lte=86400"`
	AssetStatus       string   `json:"asset_status" validate:"required,oneof=active standby retired"`
}

type UpdateSatelliteAssetRequest struct {
	Name              string   `json:"name" validate:"required,min=2,max=120"`
	OrbitClass        string   `json:"orbit_class" validate:"required,oneof=LEO MEO GEO HEO"`
	SupportedBands    []string `json:"supported_bands" validate:"required,min=1,dive,oneof=S X Ka Ku"`
	PriorityWeight    float64  `json:"priority_weight" validate:"gte=0,lte=100"`
	MinimumContactSec int      `json:"minimum_contact_sec" validate:"required,gte=30,lte=86400"`
	AssetStatus       string   `json:"asset_status" validate:"required,oneof=active standby retired"`
	ExpectedVersion   uint     `json:"expected_version" validate:"required,gte=1"`
}

type SatelliteAssetResponse struct {
	ID                uint      `json:"id"`
	SatelliteCode     string    `json:"satellite_code"`
	Name              string    `json:"name"`
	OrbitClass        string    `json:"orbit_class"`
	SupportedBands    []string  `json:"supported_bands"`
	PriorityWeight    float64   `json:"priority_weight"`
	MinimumContactSec int       `json:"minimum_contact_sec"`
	AssetStatus       string    `json:"asset_status"`
	Version           uint      `json:"version"`
	UpdatedAt         time.Time `json:"updated_at"`
}
