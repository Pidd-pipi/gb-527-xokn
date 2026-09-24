package repository

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"satellite-contact-window-deconfliction/backend/internal/model"
)

type GroundStationRepository struct{ db *gorm.DB }

func NewGroundStationRepository(db *gorm.DB) *GroundStationRepository {
	return &GroundStationRepository{db: db}
}

func (repository *GroundStationRepository) WithDB(db *gorm.DB) *GroundStationRepository {
	return &GroundStationRepository{db: db}
}

func (repository *GroundStationRepository) List(page, pageSize int, status, search string) ([]model.GroundStation, int64, error) {
	query := repository.db.Model(&model.GroundStation{})
	if status != "" {
		query = query.Where("station_status = ?", status)
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		pattern := "%" + strings.ToLower(trimmed) + "%"
		query = query.Where("LOWER(station_code) LIKE ? OR LOWER(name) LIKE ?", pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count ground stations: %w", err)
	}
	var stations []model.GroundStation
	if err := query.Order("station_code ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&stations).Error; err != nil {
		return nil, 0, fmt.Errorf("list ground stations: %w", err)
	}
	return stations, total, nil
}

func (repository *GroundStationRepository) ListAll() ([]model.GroundStation, error) {
	var stations []model.GroundStation
	if err := repository.db.Order("station_code ASC").Find(&stations).Error; err != nil {
		return nil, fmt.Errorf("list all ground stations: %w", err)
	}
	return stations, nil
}

func (repository *GroundStationRepository) Get(id uint) (model.GroundStation, error) {
	var station model.GroundStation
	if err := repository.db.First(&station, id).Error; err != nil {
		return station, fmt.Errorf("get ground station %d: %w", id, err)
	}
	return station, nil
}

func (repository *GroundStationRepository) Create(station *model.GroundStation) error {
	if err := repository.db.Create(station).Error; err != nil {
		return fmt.Errorf("create ground station: %w", err)
	}
	return nil
}

func (repository *GroundStationRepository) Update(id, expectedVersion uint, values map[string]any) (bool, error) {
	values["version"] = gorm.Expr("version + 1")
	result := repository.db.Model(&model.GroundStation{}).Where("id = ? AND version = ?", id, expectedVersion).Updates(values)
	if result.Error != nil {
		return false, fmt.Errorf("update ground station: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}
