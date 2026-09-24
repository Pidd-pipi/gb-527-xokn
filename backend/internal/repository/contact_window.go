package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/model"
)

type ContactWindowRepository struct{ db *gorm.DB }

func NewContactWindowRepository(db *gorm.DB) *ContactWindowRepository {
	return &ContactWindowRepository{db: db}
}

func (repository *ContactWindowRepository) WithDB(db *gorm.DB) *ContactWindowRepository {
	return &ContactWindowRepository{db: db}
}

func (repository *ContactWindowRepository) List(filter dto.ContactWindowFilter) ([]model.ContactWindow, int64, error) {
	query := repository.db.Model(&model.ContactWindow{})
	if filter.StationID > 0 {
		query = query.Where("station_id = ?", filter.StationID)
	}
	if filter.SatelliteID > 0 {
		query = query.Where("satellite_id = ?", filter.SatelliteID)
	}
	if filter.Status != "" {
		query = query.Where("window_status = ?", filter.Status)
	}
	if filter.From != nil {
		query = query.Where("end_at > ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("start_at < ?", *filter.To)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count contact windows: %w", err)
	}
	var windows []model.ContactWindow
	if err := query.Order("start_at ASC, id ASC").Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Find(&windows).Error; err != nil {
		return nil, 0, fmt.Errorf("list contact windows: %w", err)
	}
	return windows, total, nil
}

func (repository *ContactWindowRepository) ListRange(from, to time.Time) ([]model.ContactWindow, error) {
	var windows []model.ContactWindow
	if err := repository.db.Where("end_at > ? AND start_at < ? AND window_status <> ?", from, to, "cancelled").Order("start_at ASC, id ASC").Find(&windows).Error; err != nil {
		return nil, fmt.Errorf("list contact windows in range: %w", err)
	}
	return windows, nil
}

func (repository *ContactWindowRepository) Get(id uint) (model.ContactWindow, error) {
	var window model.ContactWindow
	if err := repository.db.First(&window, id).Error; err != nil {
		return window, fmt.Errorf("get contact window %d: %w", id, err)
	}
	return window, nil
}

func (repository *ContactWindowRepository) GetMany(ids []uint) ([]model.ContactWindow, error) {
	var windows []model.ContactWindow
	if err := repository.db.Where("id IN ?", ids).Order("id ASC").Find(&windows).Error; err != nil {
		return nil, fmt.Errorf("get contact windows: %w", err)
	}
	return windows, nil
}

func (repository *ContactWindowRepository) Create(window *model.ContactWindow) error {
	if err := repository.db.Create(window).Error; err != nil {
		return fmt.Errorf("create contact window: %w", err)
	}
	return nil
}

func (repository *ContactWindowRepository) Update(id, expectedVersion uint, values map[string]any) (bool, error) {
	values["version"] = gorm.Expr("version + 1")
	result := repository.db.Model(&model.ContactWindow{}).Where("id = ? AND version = ?", id, expectedVersion).Updates(values)
	if result.Error != nil {
		return false, fmt.Errorf("update contact window: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (repository *ContactWindowRepository) FindForUpdate(db *gorm.DB, id uint) (model.ContactWindow, error) {
	var window model.ContactWindow
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&window, id).Error; err != nil {
		return window, fmt.Errorf("lock contact window %d: %w", id, err)
	}
	return window, nil
}
