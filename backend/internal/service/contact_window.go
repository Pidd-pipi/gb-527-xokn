package service

import (
	"fmt"
	"strings"
	"time"

	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/model"
	"satellite-contact-window-deconfliction/backend/internal/repository"
)

type ContactWindowService struct {
	repository *repository.ContactWindowRepository
	stations   *repository.GroundStationRepository
	assets     *repository.SatelliteAssetRepository
	audit      *AuditService
}

func NewContactWindowService(windowRepository *repository.ContactWindowRepository, stations *repository.GroundStationRepository, assets *repository.SatelliteAssetRepository, audit *AuditService) *ContactWindowService {
	return &ContactWindowService{repository: windowRepository, stations: stations, assets: assets, audit: audit}
}

func (service *ContactWindowService) List(filter dto.ContactWindowFilter) ([]dto.ContactWindowResponse, dto.PageMeta, error) {
	windows, total, err := service.repository.List(filter)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list contact windows", err)
	}
	responses, err := service.responses(windows)
	if err != nil {
		return nil, dto.PageMeta{}, err
	}
	return responses, pageMeta(filter.Page, filter.PageSize, total), nil
}

func (service *ContactWindowService) Get(id uint) (dto.ContactWindowResponse, error) {
	window, err := service.repository.Get(id)
	if err != nil {
		return dto.ContactWindowResponse{}, MapRepositoryError("contact window", err)
	}
	responses, err := service.responses([]model.ContactWindow{window})
	if err != nil {
		return dto.ContactWindowResponse{}, err
	}
	return responses[0], nil
}

func (service *ContactWindowService) Create(request dto.CreateContactWindowRequest, actor dto.Actor, requestID string) (dto.ContactWindowResponse, error) {
	start, end, err := parseRange(request.StartAt, request.EndAt)
	if err != nil {
		return dto.ContactWindowResponse{}, err
	}
	station, asset, err := service.resources(request.StationID, request.SatelliteID)
	if err != nil {
		return dto.ContactWindowResponse{}, err
	}
	if station.StationStatus == "retired" || asset.AssetStatus == "retired" {
		return dto.ContactWindowResponse{}, Conflict("inactive_resource", "retired resources cannot receive contact windows", nil)
	}
	window := model.ContactWindow{
		StationID: request.StationID, SatelliteID: request.SatelliteID, StartAt: start, EndAt: end, Band: request.Band,
		ElevationPeakDeg: request.ElevationPeakDeg, WindowStatus: constants.WindowStatusCandidate, Priority: request.Priority,
		Locked: false, SourceVersion: strings.TrimSpace(request.SourceVersion), Version: 1,
	}
	if err := service.repository.Create(&window); err != nil {
		return dto.ContactWindowResponse{}, Internal("could not create contact window", err)
	}
	if err := service.audit.Record(actor, requestID, "window.created", "contact_window", auditID(window.ID), map[string]any{"source_version": window.SourceVersion}, nil, windowSummary(window)); err != nil {
		return dto.ContactWindowResponse{}, err
	}
	return service.Get(window.ID)
}

func (service *ContactWindowService) Update(id uint, request dto.UpdateContactWindowRequest, actor dto.Actor, requestID string) (dto.ContactWindowResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.ContactWindowResponse{}, MapRepositoryError("contact window", err)
	}
	if before.Locked {
		return dto.ContactWindowResponse{}, Conflict("locked_window", "locked windows cannot be moved or edited", nil)
	}
	start, end, err := parseRange(request.StartAt, request.EndAt)
	if err != nil {
		return dto.ContactWindowResponse{}, err
	}
	if _, _, err := service.resources(request.StationID, request.SatelliteID); err != nil {
		return dto.ContactWindowResponse{}, err
	}
	if request.WindowStatus != before.WindowStatus && !constants.CanTransitionWindow(before.WindowStatus, request.WindowStatus) {
		return dto.ContactWindowResponse{}, Conflict("invalid_state", fmt.Sprintf("cannot transition window from %s to %s", before.WindowStatus, request.WindowStatus), nil)
	}
	if request.WindowStatus == constants.WindowStatusLocked {
		return dto.ContactWindowResponse{}, BadRequest("use_lock_action", "use the dedicated lock action for locked status")
	}
	values := map[string]any{
		"station_id": request.StationID, "satellite_id": request.SatelliteID, "start_at": start, "end_at": end, "band": request.Band,
		"elevation_peak_deg": request.ElevationPeakDeg, "window_status": request.WindowStatus, "priority": request.Priority,
		"source_version": strings.TrimSpace(request.SourceVersion),
	}
	updated, err := service.repository.Update(id, request.ExpectedVersion, values)
	if err != nil {
		return dto.ContactWindowResponse{}, Internal("could not update contact window", err)
	}
	if !updated {
		return dto.ContactWindowResponse{}, Conflict("version_conflict", "contact window changed; reload before updating", nil)
	}
	after, err := service.repository.Get(id)
	if err != nil {
		return dto.ContactWindowResponse{}, MapRepositoryError("contact window", err)
	}
	if err := service.audit.Record(actor, requestID, "window.updated", "contact_window", auditID(id), map[string]any{"expected_version": request.ExpectedVersion}, windowSummary(before), windowSummary(after)); err != nil {
		return dto.ContactWindowResponse{}, err
	}
	return service.Get(id)
}

func (service *ContactWindowService) Submit(id uint, request dto.WindowActionRequest, actor dto.Actor, requestID string) (dto.ContactWindowResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.ContactWindowResponse{}, MapRepositoryError("contact window", err)
	}
	if before.Locked {
		return dto.ContactWindowResponse{}, Conflict("locked_window", "locked windows cannot be submitted again", nil)
	}
	if !constants.CanTransitionWindow(before.WindowStatus, constants.WindowStatusSubmitted) {
		return dto.ContactWindowResponse{}, Conflict("invalid_state", "only candidate windows can be submitted", nil)
	}
	station, asset, err := service.resources(before.StationID, before.SatelliteID)
	if err != nil {
		return dto.ContactWindowResponse{}, err
	}
	if err := validateOperational(before, station, asset); err != nil {
		return dto.ContactWindowResponse{}, err
	}
	updated, err := service.repository.Update(id, request.ExpectedVersion, map[string]any{"window_status": constants.WindowStatusSubmitted})
	if err != nil {
		return dto.ContactWindowResponse{}, Internal("could not submit contact window", err)
	}
	if !updated {
		return dto.ContactWindowResponse{}, Conflict("version_conflict", "contact window changed; reload before submitting", nil)
	}
	after, _ := service.repository.Get(id)
	if err := service.audit.Record(actor, requestID, "window.submitted", "contact_window", auditID(id), map[string]any{"compatibility_checked": true, "expected_version": request.ExpectedVersion}, windowSummary(before), windowSummary(after)); err != nil {
		return dto.ContactWindowResponse{}, err
	}
	return service.Get(id)
}

func (service *ContactWindowService) Lock(id uint, request dto.WindowActionRequest, actor dto.Actor, requestID string) (dto.ContactWindowResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.ContactWindowResponse{}, MapRepositoryError("contact window", err)
	}
	if before.Locked || !constants.CanTransitionWindow(before.WindowStatus, constants.WindowStatusLocked) {
		return dto.ContactWindowResponse{}, Conflict("invalid_state", "only unlocked candidate or submitted windows can be locked", nil)
	}
	station, asset, err := service.resources(before.StationID, before.SatelliteID)
	if err != nil {
		return dto.ContactWindowResponse{}, err
	}
	if err := validateOperational(before, station, asset); err != nil {
		return dto.ContactWindowResponse{}, err
	}
	updated, err := service.repository.Update(id, request.ExpectedVersion, map[string]any{"locked": true, "window_status": constants.WindowStatusLocked})
	if err != nil {
		return dto.ContactWindowResponse{}, Internal("could not lock contact window", err)
	}
	if !updated {
		return dto.ContactWindowResponse{}, Conflict("version_conflict", "contact window changed; reload before locking", nil)
	}
	after, _ := service.repository.Get(id)
	if err := service.audit.Record(actor, requestID, "window.locked", "contact_window", auditID(id), map[string]any{"expected_version": request.ExpectedVersion}, windowSummary(before), windowSummary(after)); err != nil {
		return dto.ContactWindowResponse{}, err
	}
	return service.Get(id)
}

func (service *ContactWindowService) resources(stationID, satelliteID uint) (model.GroundStation, model.SatelliteAsset, error) {
	station, err := service.stations.Get(stationID)
	if err != nil {
		return station, model.SatelliteAsset{}, MapRepositoryError("ground station", err)
	}
	asset, err := service.assets.Get(satelliteID)
	if err != nil {
		return station, asset, MapRepositoryError("satellite asset", err)
	}
	return station, asset, nil
}

func (service *ContactWindowService) responses(windows []model.ContactWindow) ([]dto.ContactWindowResponse, error) {
	stations, err := service.stations.ListAll()
	if err != nil {
		return nil, Internal("could not load station references", err)
	}
	assets, err := service.assets.ListAll()
	if err != nil {
		return nil, Internal("could not load satellite references", err)
	}
	stationCodes := map[uint]string{}
	assetCodes := map[uint]string{}
	for _, station := range stations {
		stationCodes[station.ID] = station.StationCode
	}
	for _, asset := range assets {
		assetCodes[asset.ID] = asset.SatelliteCode
	}
	result := make([]dto.ContactWindowResponse, 0, len(windows))
	for _, window := range windows {
		result = append(result, dto.ContactWindowResponse{
			ID: window.ID, StationID: window.StationID, StationCode: stationCodes[window.StationID], SatelliteID: window.SatelliteID,
			SatelliteCode: assetCodes[window.SatelliteID], StartAt: window.StartAt, EndAt: window.EndAt, DurationSec: window.DurationSec(),
			Band: window.Band, ElevationPeakDeg: window.ElevationPeakDeg, WindowStatus: window.WindowStatus, Priority: window.Priority,
			Locked: window.Locked, SourceVersion: window.SourceVersion, Version: window.Version, UpdatedAt: window.UpdatedAt,
		})
	}
	return result, nil
}

func parseRange(startValue, endValue string) (time.Time, time.Time, error) {
	start, err := time.Parse(time.RFC3339, startValue)
	if err != nil {
		return time.Time{}, time.Time{}, BadRequest("invalid_time_range", "start_at must be RFC3339", err)
	}
	end, err := time.Parse(time.RFC3339, endValue)
	if err != nil {
		return time.Time{}, time.Time{}, BadRequest("invalid_time_range", "end_at must be RFC3339", err)
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, BadRequest("invalid_time_range", "end_at must be later than start_at")
	}
	if end.Sub(start) > 24*time.Hour {
		return time.Time{}, time.Time{}, BadRequest("invalid_time_range", "a contact window cannot exceed 24 hours")
	}
	return start.UTC(), end.UTC(), nil
}

func validateOperational(window model.ContactWindow, station model.GroundStation, asset model.SatelliteAsset) error {
	if station.StationStatus != "active" || asset.AssetStatus != "active" {
		return Conflict("inactive_resource", "only active station and satellite resources can be submitted or locked", nil)
	}
	if !containsString(decodeStrings(station.SupportedBandsJSON), window.Band) || !containsString(decodeStrings(asset.SupportedBandsJSON), window.Band) {
		return Conflict("band_incompatible", "selected band is not supported by both resources", nil)
	}
	if window.DurationSec() < asset.MinimumContactSec {
		return Conflict("duration_shortfall", "contact duration is below the satellite minimum", nil)
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func windowSummary(window model.ContactWindow) map[string]any {
	return map[string]any{"station_id": window.StationID, "satellite_id": window.SatelliteID, "start_at": window.StartAt, "end_at": window.EndAt, "band": window.Band, "status": window.WindowStatus, "priority": window.Priority, "locked": window.Locked, "source_version": window.SourceVersion, "version": window.Version}
}
