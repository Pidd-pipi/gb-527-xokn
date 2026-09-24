package service

import (
	"encoding/json"
	"strings"

	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/model"
	"satellite-contact-window-deconfliction/backend/internal/repository"
)

type GroundStationService struct {
	repository *repository.GroundStationRepository
	audit      *AuditService
}

func NewGroundStationService(repository *repository.GroundStationRepository, audit *AuditService) *GroundStationService {
	return &GroundStationService{repository: repository, audit: audit}
}

func (service *GroundStationService) List(page, pageSize int, status, search string) ([]dto.GroundStationResponse, dto.PageMeta, error) {
	stations, total, err := service.repository.List(page, pageSize, status, search)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list ground stations", err)
	}
	responses := make([]dto.GroundStationResponse, 0, len(stations))
	for _, station := range stations {
		responses = append(responses, stationResponse(station))
	}
	return responses, pageMeta(page, pageSize, total), nil
}

func (service *GroundStationService) Get(id uint) (dto.GroundStationResponse, error) {
	station, err := service.repository.Get(id)
	if err != nil {
		return dto.GroundStationResponse{}, MapRepositoryError("ground station", err)
	}
	return stationResponse(station), nil
}

func (service *GroundStationService) Create(request dto.CreateGroundStationRequest, actor dto.Actor, requestID string) (dto.GroundStationResponse, error) {
	station := model.GroundStation{
		StationCode: strings.ToUpper(strings.TrimSpace(request.StationCode)), Name: strings.TrimSpace(request.Name), Latitude: request.Latitude,
		Longitude: request.Longitude, AntennaCount: request.AntennaCount, SupportedBandsJSON: encodeStrings(uniqueStrings(request.SupportedBands)),
		SlewBufferSec: request.SlewBufferSec, StationStatus: request.StationStatus, Version: 1,
	}
	if err := service.repository.Create(&station); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return dto.GroundStationResponse{}, Conflict("duplicate_station_code", "station_code already exists", err)
		}
		return dto.GroundStationResponse{}, Internal("could not create ground station", err)
	}
	after := stationSummary(station)
	if err := service.audit.Record(actor, requestID, "station.created", "ground_station", auditID(station.ID), map[string]any{"station_code": station.StationCode}, nil, after); err != nil {
		return dto.GroundStationResponse{}, err
	}
	return stationResponse(station), nil
}

func (service *GroundStationService) Update(id uint, request dto.UpdateGroundStationRequest, actor dto.Actor, requestID string) (dto.GroundStationResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.GroundStationResponse{}, MapRepositoryError("ground station", err)
	}
	values := map[string]any{
		"name": strings.TrimSpace(request.Name), "latitude": request.Latitude, "longitude": request.Longitude,
		"antenna_count": request.AntennaCount, "supported_bands_json": encodeStrings(uniqueStrings(request.SupportedBands)),
		"slew_buffer_sec": request.SlewBufferSec, "station_status": request.StationStatus,
	}
	updated, err := service.repository.Update(id, request.ExpectedVersion, values)
	if err != nil {
		return dto.GroundStationResponse{}, Internal("could not update ground station", err)
	}
	if !updated {
		return dto.GroundStationResponse{}, Conflict("version_conflict", "ground station changed; reload before updating", nil)
	}
	after, err := service.repository.Get(id)
	if err != nil {
		return dto.GroundStationResponse{}, MapRepositoryError("ground station", err)
	}
	if err := service.audit.Record(actor, requestID, "station.updated", "ground_station", auditID(id), map[string]any{"expected_version": request.ExpectedVersion}, stationSummary(before), stationSummary(after)); err != nil {
		return dto.GroundStationResponse{}, err
	}
	return stationResponse(after), nil
}

func stationResponse(station model.GroundStation) dto.GroundStationResponse {
	return dto.GroundStationResponse{
		ID: station.ID, StationCode: station.StationCode, Name: station.Name, Latitude: station.Latitude, Longitude: station.Longitude,
		AntennaCount: station.AntennaCount, SupportedBands: decodeStrings(station.SupportedBandsJSON), SlewBufferSec: station.SlewBufferSec,
		StationStatus: station.StationStatus, Version: station.Version, UpdatedAt: station.UpdatedAt,
	}
}

func stationSummary(station model.GroundStation) map[string]any {
	return map[string]any{"station_code": station.StationCode, "antenna_count": station.AntennaCount, "bands": decodeStrings(station.SupportedBandsJSON), "slew_buffer_sec": station.SlewBufferSec, "status": station.StationStatus, "version": station.Version}
}

func encodeStrings(values []string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}
func decodeStrings(encoded string) []string {
	values := []string{}
	_ = json.Unmarshal([]byte(encoded), &values)
	return values
}
func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			result = append(result, trimmed)
		}
	}
	return result
}
