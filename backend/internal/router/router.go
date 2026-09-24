package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"satellite-contact-window-deconfliction/backend/internal/config"
	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/handler"
	"satellite-contact-window-deconfliction/backend/internal/middleware"
	"satellite-contact-window-deconfliction/backend/internal/repository"
	"satellite-contact-window-deconfliction/backend/internal/service"
)

type handlers struct {
	system     *handler.SystemHandler
	stations   *handler.GroundStationHandler
	satellites *handler.SatelliteAssetHandler
	windows    *handler.ContactWindowHandler
	conflicts  *handler.ConflictResolutionHandler
	auth       *service.AuthService
}

func New(db *gorm.DB, cfg config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.AccessLog(), middleware.Recovery(), middleware.CORS(cfg.CORSOrigin), middleware.RateLimit(cfg.RateLimitPerMinute))
	handlers := wire(db, cfg)
	engine.GET("/healthz", handlers.system.Health)
	engine.GET("/readyz", handlers.system.Ready)
	api := engine.Group("/api/v1")
	api.POST("/auth/login", handlers.system.Login)
	protected := api.Group("")
	protected.Use(middleware.Auth(handlers.auth))
	registerStationRoutes(protected, handlers.stations)
	registerSatelliteRoutes(protected, handlers.satellites)
	registerWindowRoutes(protected, handlers.windows)
	registerConflictRoutes(protected, handlers.conflicts)
	protected.GET("/audit", middleware.RBAC(constants.RoleReviewer, constants.RoleAdmin), handlers.system.Audit)
	engine.NoRoute(func(context *gin.Context) {
		context.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "route_not_found", "message": "route was not found"}, "request_id": context.GetString("request_id")})
	})
	return engine
}

func wire(db *gorm.DB, cfg config.Config) handlers {
	stationRepository := repository.NewGroundStationRepository(db)
	assetRepository := repository.NewSatelliteAssetRepository(db)
	windowRepository := repository.NewContactWindowRepository(db)
	conflictRepository := repository.NewConflictResolutionRepository(db)
	systemRepository := repository.NewSystemRepository(db)
	auditService := service.NewAuditService(systemRepository)
	authService := service.NewAuthService(systemRepository, cfg.JWTSecret, cfg.JWTTTL)
	stationService := service.NewGroundStationService(stationRepository, auditService)
	assetService := service.NewSatelliteAssetService(assetRepository, auditService)
	windowService := service.NewContactWindowService(windowRepository, stationRepository, assetRepository, auditService)
	conflictService := service.NewConflictResolutionService(conflictRepository, windowRepository, stationRepository, assetRepository, auditService, cfg.Weights)
	return handlers{
		system: handler.NewSystemHandler(authService, auditService, db), stations: handler.NewGroundStationHandler(stationService),
		satellites: handler.NewSatelliteAssetHandler(assetService), windows: handler.NewContactWindowHandler(windowService),
		conflicts: handler.NewConflictResolutionHandler(conflictService), auth: authService,
	}
}
