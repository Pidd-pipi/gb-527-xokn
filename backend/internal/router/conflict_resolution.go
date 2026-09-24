package router

import (
	"github.com/gin-gonic/gin"

	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/handler"
	"satellite-contact-window-deconfliction/backend/internal/middleware"
)

func registerConflictRoutes(api *gin.RouterGroup, conflictHandler *handler.ConflictResolutionHandler) {
	api.GET("/conflicts", conflictHandler.List)
	api.GET("/conflicts/:id", conflictHandler.Get)
	planning := api.Group("/conflicts", middleware.RBAC(constants.RoleScheduler, constants.RoleAdmin))
	planning.POST("/detect", conflictHandler.Detect)
	planning.POST("/:id/submit", conflictHandler.Submit)
	review := api.Group("/conflicts", middleware.RBAC(constants.RoleReviewer, constants.RoleAdmin))
	review.POST("/:id/review", conflictHandler.Review)
	review.GET("/:id/export", conflictHandler.Export)
}
