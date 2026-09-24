package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/service"
)

type GroundStationHandler struct{ service *service.GroundStationService }

func NewGroundStationHandler(service *service.GroundStationService) *GroundStationHandler {
	return &GroundStationHandler{service: service}
}

func (handler *GroundStationHandler) List(context *gin.Context) {
	page, pageSize := Pagination(context)
	stations, meta, err := handler.service.List(page, pageSize, context.Query("status"), context.Query("search"))
	if err != nil {
		WriteError(context, err)
		return
	}
	WritePage(context, stations, meta)
}

func (handler *GroundStationHandler) Get(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	station, err := handler.service.Get(id)
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, station)
}

func (handler *GroundStationHandler) Create(context *gin.Context) {
	var request dto.CreateGroundStationRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	station, err := handler.service.Create(request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusCreated, station)
}

func (handler *GroundStationHandler) Update(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request dto.UpdateGroundStationRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	station, err := handler.service.Update(id, request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, station)
}
