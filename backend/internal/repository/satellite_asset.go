package repository

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"satellite-contact-window-deconfliction/backend/internal/model"
)

type SatelliteAssetRepository struct{ db *gorm.DB }

func NewSatelliteAssetRepository(db *gorm.DB) *SatelliteAssetRepository {
	return &SatelliteAssetRepository{db: db}
}

func (repository *SatelliteAssetRepository) WithDB(db *gorm.DB) *SatelliteAssetRepository {
	return &SatelliteAssetRepository{db: db}
}

func (repository *SatelliteAssetRepository) List(page, pageSize int, status, search string) ([]model.SatelliteAsset, int64, error) {
	query := repository.db.Model(&model.SatelliteAsset{})
	if status != "" {
		query = query.Where("asset_status = ?", status)
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		pattern := "%" + strings.ToLower(trimmed) + "%"
		query = query.Where("LOWER(satellite_code) LIKE ? OR LOWER(name) LIKE ?", pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count satellite assets: %w", err)
	}
	var assets []model.SatelliteAsset
	if err := query.Order("priority_weight DESC, satellite_code ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&assets).Error; err != nil {
		return nil, 0, fmt.Errorf("list satellite assets: %w", err)
	}
	return assets, total, nil
}

func (repository *SatelliteAssetRepository) ListAll() ([]model.SatelliteAsset, error) {
	var assets []model.SatelliteAsset
	if err := repository.db.Order("satellite_code ASC").Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("list all satellite assets: %w", err)
	}
	return assets, nil
}

func (repository *SatelliteAssetRepository) Get(id uint) (model.SatelliteAsset, error) {
	var asset model.SatelliteAsset
	if err := repository.db.First(&asset, id).Error; err != nil {
		return asset, fmt.Errorf("get satellite asset %d: %w", id, err)
	}
	return asset, nil
}

func (repository *SatelliteAssetRepository) Create(asset *model.SatelliteAsset) error {
	if err := repository.db.Create(asset).Error; err != nil {
		return fmt.Errorf("create satellite asset: %w", err)
	}
	return nil
}

func (repository *SatelliteAssetRepository) Update(id, expectedVersion uint, values map[string]any) (bool, error) {
	values["version"] = gorm.Expr("version + 1")
	result := repository.db.Model(&model.SatelliteAsset{}).Where("id = ? AND version = ?", id, expectedVersion).Updates(values)
	if result.Error != nil {
		return false, fmt.Errorf("update satellite asset: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}
