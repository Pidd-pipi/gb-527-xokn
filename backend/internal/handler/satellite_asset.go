package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/service"
)

type SatelliteAssetHandler struct {
	service *service.SatelliteAssetService
}

func NewSatelliteAssetHandler(service *service.SatelliteAssetService) *SatelliteAssetHandler {
	return &SatelliteAssetHandler{service: service}
}

func (handler *SatelliteAssetHandler) List(context *gin.Context) {
	page, pageSize := Pagination(context)
	assets, meta, err := handler.service.List(page, pageSize, context.Query("status"), context.Query("search"))
	if err != nil {
		WriteError(context, err)
		return
	}
	WritePage(context, assets, meta)
}

func (handler *SatelliteAssetHandler) Get(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	asset, err := handler.service.Get(id)
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, asset)
}

func (handler *SatelliteAssetHandler) Create(context *gin.Context) {
	var request dto.CreateSatelliteAssetRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	asset, err := handler.service.Create(request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusCreated, asset)
}

func (handler *SatelliteAssetHandler) Update(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request dto.UpdateSatelliteAssetRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	asset, err := handler.service.Update(id, request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, asset)
}
