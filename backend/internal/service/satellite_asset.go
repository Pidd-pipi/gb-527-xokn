package service

import (
	"strings"

	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/model"
	"satellite-contact-window-deconfliction/backend/internal/repository"
)

type SatelliteAssetService struct {
	repository *repository.SatelliteAssetRepository
	audit      *AuditService
}

func NewSatelliteAssetService(repository *repository.SatelliteAssetRepository, audit *AuditService) *SatelliteAssetService {
	return &SatelliteAssetService{repository: repository, audit: audit}
}

func (service *SatelliteAssetService) List(page, pageSize int, status, search string) ([]dto.SatelliteAssetResponse, dto.PageMeta, error) {
	assets, total, err := service.repository.List(page, pageSize, status, search)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list satellite assets", err)
	}
	responses := make([]dto.SatelliteAssetResponse, 0, len(assets))
	for _, asset := range assets {
		responses = append(responses, satelliteResponse(asset))
	}
	return responses, pageMeta(page, pageSize, total), nil
}

func (service *SatelliteAssetService) Get(id uint) (dto.SatelliteAssetResponse, error) {
	asset, err := service.repository.Get(id)
	if err != nil {
		return dto.SatelliteAssetResponse{}, MapRepositoryError("satellite asset", err)
	}
	return satelliteResponse(asset), nil
}

func (service *SatelliteAssetService) Create(request dto.CreateSatelliteAssetRequest, actor dto.Actor, requestID string) (dto.SatelliteAssetResponse, error) {
	asset := model.SatelliteAsset{
		SatelliteCode: strings.ToUpper(strings.TrimSpace(request.SatelliteCode)), Name: strings.TrimSpace(request.Name), OrbitClass: request.OrbitClass,
		SupportedBandsJSON: encodeStrings(uniqueStrings(request.SupportedBands)), PriorityWeight: request.PriorityWeight,
		MinimumContactSec: request.MinimumContactSec, AssetStatus: request.AssetStatus, Version: 1,
	}
	if err := service.repository.Create(&asset); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return dto.SatelliteAssetResponse{}, Conflict("duplicate_satellite_code", "satellite_code already exists", err)
		}
		return dto.SatelliteAssetResponse{}, Internal("could not create satellite asset", err)
	}
	if err := service.audit.Record(actor, requestID, "satellite.created", "satellite_asset", auditID(asset.ID), map[string]any{"satellite_code": asset.SatelliteCode}, nil, satelliteSummary(asset)); err != nil {
		return dto.SatelliteAssetResponse{}, err
	}
	return satelliteResponse(asset), nil
}

func (service *SatelliteAssetService) Update(id uint, request dto.UpdateSatelliteAssetRequest, actor dto.Actor, requestID string) (dto.SatelliteAssetResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.SatelliteAssetResponse{}, MapRepositoryError("satellite asset", err)
	}
	values := map[string]any{
		"name": strings.TrimSpace(request.Name), "orbit_class": request.OrbitClass, "supported_bands_json": encodeStrings(uniqueStrings(request.SupportedBands)),
		"priority_weight": request.PriorityWeight, "minimum_contact_sec": request.MinimumContactSec, "asset_status": request.AssetStatus,
	}
	updated, err := service.repository.Update(id, request.ExpectedVersion, values)
	if err != nil {
		return dto.SatelliteAssetResponse{}, Internal("could not update satellite asset", err)
	}
	if !updated {
		return dto.SatelliteAssetResponse{}, Conflict("version_conflict", "satellite asset changed; reload before updating", nil)
	}
	after, err := service.repository.Get(id)
	if err != nil {
		return dto.SatelliteAssetResponse{}, MapRepositoryError("satellite asset", err)
	}
	if err := service.audit.Record(actor, requestID, "satellite.updated", "satellite_asset", auditID(id), map[string]any{"expected_version": request.ExpectedVersion}, satelliteSummary(before), satelliteSummary(after)); err != nil {
		return dto.SatelliteAssetResponse{}, err
	}
	return satelliteResponse(after), nil
}

func satelliteResponse(asset model.SatelliteAsset) dto.SatelliteAssetResponse {
	return dto.SatelliteAssetResponse{
		ID: asset.ID, SatelliteCode: asset.SatelliteCode, Name: asset.Name, OrbitClass: asset.OrbitClass,
		SupportedBands: decodeStrings(asset.SupportedBandsJSON), PriorityWeight: asset.PriorityWeight, MinimumContactSec: asset.MinimumContactSec,
		AssetStatus: asset.AssetStatus, Version: asset.Version, UpdatedAt: asset.UpdatedAt,
	}
}

func satelliteSummary(asset model.SatelliteAsset) map[string]any {
	return map[string]any{"satellite_code": asset.SatelliteCode, "orbit_class": asset.OrbitClass, "bands": decodeStrings(asset.SupportedBandsJSON), "priority_weight": asset.PriorityWeight, "minimum_contact_sec": asset.MinimumContactSec, "status": asset.AssetStatus, "version": asset.Version}
}
