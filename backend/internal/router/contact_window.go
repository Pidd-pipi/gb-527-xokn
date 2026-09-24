package router

import (
	"github.com/gin-gonic/gin"

	"satellite-contact-window-deconfliction/backend/internal/constants"
	"satellite-contact-window-deconfliction/backend/internal/handler"
	"satellite-contact-window-deconfliction/backend/internal/middleware"
)

func registerWindowRoutes(api *gin.RouterGroup, windowHandler *handler.ContactWindowHandler) {
	api.GET("/windows", windowHandler.List)
	api.GET("/windows/:id", windowHandler.Get)
	planning := api.Group("/windows", middleware.RBAC(constants.RoleScheduler, constants.RoleAdmin))
	planning.POST("", windowHandler.Create)
	planning.PUT("/:id", windowHandler.Update)
	planning.POST("/:id/submit", windowHandler.Submit)
	planning.POST("/:id/lock", windowHandler.Lock)
}
