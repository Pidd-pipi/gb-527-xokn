package repository

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"satellite-contact-window-deconfliction/backend/internal/model"
)

type ConflictResolutionRepository struct{ db *gorm.DB }

func NewConflictResolutionRepository(db *gorm.DB) *ConflictResolutionRepository {
	return &ConflictResolutionRepository{db: db}
}

func (repository *ConflictResolutionRepository) WithDB(db *gorm.DB) *ConflictResolutionRepository {
	return &ConflictResolutionRepository{db: db}
}

func (repository *ConflictResolutionRepository) DB() *gorm.DB { return repository.db }

func (repository *ConflictResolutionRepository) List(page, pageSize int, status, conflictType string) ([]model.ConflictResolution, int64, error) {
	query := repository.db.Model(&model.ConflictResolution{})
	if status != "" {
		query = query.Where("resolution_status = ?", status)
	}
	if conflictType != "" {
		query = query.Where("conflict_type = ?", conflictType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count conflict resolutions: %w", err)
	}
	var resolutions []model.ConflictResolution
	if err := query.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&resolutions).Error; err != nil {
		return nil, 0, fmt.Errorf("list conflict resolutions: %w", err)
	}
	return resolutions, total, nil
}

func (repository *ConflictResolutionRepository) Get(id uint) (model.ConflictResolution, error) {
	var resolution model.ConflictResolution
	if err := repository.db.First(&resolution, id).Error; err != nil {
		return resolution, fmt.Errorf("get conflict resolution %d: %w", id, err)
	}
	return resolution, nil
}

func (repository *ConflictResolutionRepository) GetForUpdate(db *gorm.DB, id uint) (model.ConflictResolution, error) {
	var resolution model.ConflictResolution
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&resolution, id).Error; err != nil {
		return resolution, fmt.Errorf("lock conflict resolution %d: %w", id, err)
	}
	return resolution, nil
}

func (repository *ConflictResolutionRepository) FindByKey(key string) (model.ConflictResolution, error) {
	var resolution model.ConflictResolution
	if err := repository.db.Where("conflict_key = ?", key).First(&resolution).Error; err != nil {
		return resolution, fmt.Errorf("find conflict resolution by key: %w", err)
	}
	return resolution, nil
}

func (repository *ConflictResolutionRepository) Create(resolution *model.ConflictResolution) error {
	if err := repository.db.Create(resolution).Error; err != nil {
		return fmt.Errorf("create conflict resolution: %w", err)
	}
	return nil
}

func (repository *ConflictResolutionRepository) Transition(db *gorm.DB, id, expectedVersion uint, from, to string, values map[string]any) (bool, error) {
	values["resolution_status"] = to
	values["version"] = gorm.Expr("version + 1")
	result := db.Model(&model.ConflictResolution{}).Where("id = ? AND version = ? AND resolution_status = ?", id, expectedVersion, from).Updates(values)
	if result.Error != nil {
		return false, fmt.Errorf("transition conflict resolution: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}
