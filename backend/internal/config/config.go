package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/model"
)

type Weights struct {
	PriorityLoss     float64 `json:"priority_loss"`
	MovementDistance float64 `json:"movement_distance"`
	ContactDuration  float64 `json:"contact_duration"`
	ResourceMargin   float64 `json:"resource_margin"`
}

type Config struct {
	Port               string
	DBDriver           string
	DBDSN              string
	DBAutoMigrate      bool
	JWTSecret          string
	JWTTTL             time.Duration
	CORSOrigin         string
	LogLevel           string
	RateLimitPerMinute int
	Weights            Weights
}

func Load() (Config, error) {
	cfg := Config{
		Port:               envString("PORT", "8080"),
		DBDriver:           envString("DB_DRIVER", "postgres"),
		DBDSN:              envString("DB_DSN", "host=localhost user=planner password=planner_local_527 dbname=contact_planning port=57527 sslmode=disable TimeZone=UTC"),
		DBAutoMigrate:      envBool("DB_AUTO_MIGRATE", true),
		JWTSecret:          envString("JWT_SECRET", ""),
		JWTTTL:             time.Duration(envInt("JWT_TTL_MINUTES", 480)) * time.Minute,
		CORSOrigin:         envString("CORS_ORIGIN", "http://localhost:18527"),
		LogLevel:           envString("LOG_LEVEL", "info"),
		RateLimitPerMinute: envInt("RATE_LIMIT_PER_MINUTE", 180),
		Weights: Weights{
			PriorityLoss:     envFloat("WEIGHT_PRIORITY_LOSS", 4.0),
			MovementDistance: envFloat("WEIGHT_MOVEMENT_DISTANCE", 0.02),
			ContactDuration:  envFloat("WEIGHT_CONTACT_DURATION", 0.003),
			ResourceMargin:   envFloat("WEIGHT_RESOURCE_MARGIN", 2.0),
		},
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	if cfg.RateLimitPerMinute < 10 {
		return Config{}, errors.New("RATE_LIMIT_PER_MINUTE must be at least 10")
	}
	return cfg, nil
}

func OpenDatabase(cfg Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DBDriver {
	case "postgres":
		dialector = postgres.Open(cfg.DBDSN)
	case "sqlite":
		dialector = sqlite.Open(cfg.DBDSN)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", cfg.DBDriver)
	}
	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if cfg.DBAutoMigrate {
		if err := db.AutoMigrate(
			&model.User{}, &model.GroundStation{}, &model.SatelliteAsset{},
			&model.ContactWindow{}, &model.ConflictResolution{}, &model.AuditEvent{},
		); err != nil {
			return nil, fmt.Errorf("migrate database: %w", err)
		}
	}
	if err := seed(db); err != nil {
		return nil, fmt.Errorf("seed database: %w", err)
	}
	return db, nil
}

func seed(db *gorm.DB) error {
	for _, item := range []struct{ username, password, role string }{
		{"scheduler", "Scheduler#527", constants.RoleScheduler},
		{"reviewer", "Reviewer#527", constants.RoleReviewer},
		{"admin", "Admin#527", constants.RoleAdmin},
	} {
		var count int64
		if err := db.Model(&model.User{}).Where("username = ?", item.username).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			hash, err := bcrypt.GenerateFromPassword([]byte(item.password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if err := db.Create(&model.User{Username: item.username, PasswordHash: string(hash), Role: item.role, Active: true}).Error; err != nil {
				return err
			}
		}
	}
	var stationCount int64
	if err := db.Model(&model.GroundStation{}).Count(&stationCount).Error; err != nil {
		return err
	}
	if stationCount > 0 {
		return nil
	}
	bands := func(values ...string) string { encoded, _ := json.Marshal(values); return string(encoded) }
	stations := []model.GroundStation{
		{StationCode: "ALP-01", Name: "Alpine Relay", Latitude: 46.82, Longitude: 8.23, AntennaCount: 1, SupportedBandsJSON: bands("S", "X"), SlewBufferSec: 180, StationStatus: "active", Version: 1},
		{StationCode: "PAC-02", Name: "Pacific Array", Latitude: 19.72, Longitude: -155.05, AntennaCount: 2, SupportedBandsJSON: bands("S", "X", "Ka"), SlewBufferSec: 90, StationStatus: "active", Version: 1},
		{StationCode: "DES-03", Name: "Desert Aperture", Latitude: -23.7, Longitude: 133.88, AntennaCount: 1, SupportedBandsJSON: bands("S", "Ka"), SlewBufferSec: 120, StationStatus: "maintenance", Version: 1},
	}
	if err := db.Create(&stations).Error; err != nil {
		return err
	}
	satellites := []model.SatelliteAsset{
		{SatelliteCode: "AURORA-7", Name: "Aurora Earth Observer", OrbitClass: "LEO", SupportedBandsJSON: bands("X", "Ka"), PriorityWeight: 8.5, MinimumContactSec: 420, AssetStatus: "active", Version: 1},
		{SatelliteCode: "MERIDIAN-3", Name: "Meridian Climate Relay", OrbitClass: "LEO", SupportedBandsJSON: bands("S", "X"), PriorityWeight: 6.2, MinimumContactSec: 360, AssetStatus: "active", Version: 1},
		{SatelliteCode: "HORIZON-12", Name: "Horizon Navigation Testbed", OrbitClass: "MEO", SupportedBandsJSON: bands("S"), PriorityWeight: 4.8, MinimumContactSec: 480, AssetStatus: "standby", Version: 1},
	}
	if err := db.Create(&satellites).Error; err != nil {
		return err
	}
	base := time.Now().UTC().Truncate(time.Hour).Add(2 * time.Hour)
	windows := []model.ContactWindow{
		{StationID: stations[0].ID, SatelliteID: satellites[0].ID, StartAt: base, EndAt: base.Add(10 * time.Minute), Band: "X", ElevationPeakDeg: 61.4, WindowStatus: constants.WindowStatusSubmitted, Priority: 9, SourceVersion: "orbit-2026.234-a", Version: 1},
		{StationID: stations[0].ID, SatelliteID: satellites[1].ID, StartAt: base.Add(7 * time.Minute), EndAt: base.Add(16 * time.Minute), Band: "S", ElevationPeakDeg: 48.8, WindowStatus: constants.WindowStatusSubmitted, Priority: 6, SourceVersion: "orbit-2026.234-a", Version: 1},
		{StationID: stations[1].ID, SatelliteID: satellites[0].ID, StartAt: base.Add(9 * time.Minute), EndAt: base.Add(20 * time.Minute), Band: "Ka", ElevationPeakDeg: 72.1, WindowStatus: constants.WindowStatusCandidate, Priority: 8, SourceVersion: "orbit-2026.234-a", Version: 1},
		{StationID: stations[1].ID, SatelliteID: satellites[1].ID, StartAt: base.Add(21 * time.Minute), EndAt: base.Add(25 * time.Minute), Band: "X", ElevationPeakDeg: 34.0, WindowStatus: constants.WindowStatusCandidate, Priority: 5, SourceVersion: "orbit-2026.234-a", Version: 1},
	}
	return db.Create(&windows).Error
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return value
}

func envFloat(key string, fallback float64) float64 {
	value, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
