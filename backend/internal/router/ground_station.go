package router

import (
	"github.com/gin-gonic/gin"

	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/handler"
	"satellite-contact-window-deconfliction/backend/internal/middleware"
)

func registerStationRoutes(api *gin.RouterGroup, stationHandler *handler.GroundStationHandler) {
	api.GET("/stations", stationHandler.List)
	api.GET("/stations/:id", stationHandler.Get)
	planning := api.Group("/stations", middleware.RBAC(constants.RoleScheduler, constants.RoleAdmin))
	planning.POST("", stationHandler.Create)
	planning.PUT("/:id", stationHandler.Update)
}
