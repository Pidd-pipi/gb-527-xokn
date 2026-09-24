package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/service"
)

type ConflictResolutionHandler struct {
	service *service.ConflictResolutionService
}

func NewConflictResolutionHandler(service *service.ConflictResolutionService) *ConflictResolutionHandler {
	return &ConflictResolutionHandler{service: service}
}

func (handler *ConflictResolutionHandler) List(context *gin.Context) {
	page, pageSize := Pagination(context)
	resolutions, meta, err := handler.service.List(page, pageSize, context.Query("status"), context.Query("conflict_type"))
	if err != nil {
		WriteError(context, err)
		return
	}
	WritePage(context, resolutions, meta)
}

func (handler *ConflictResolutionHandler) Get(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	resolution, err := handler.service.Get(id)
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, resolution)
}

func (handler *ConflictResolutionHandler) Detect(context *gin.Context) {
	var request dto.DetectConflictsRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	result, err := handler.service.Detect(request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, result)
}

func (handler *ConflictResolutionHandler) Submit(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request dto.ConflictActionRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	resolution, err := handler.service.Submit(id, request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, resolution)
}

func (handler *ConflictResolutionHandler) Review(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request dto.ConflictActionRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	resolution, err := handler.service.Review(id, request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, resolution)
}

func (handler *ConflictResolutionHandler) Export(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	record, err := handler.service.Export(id)
	if err != nil {
		WriteError(context, err)
		return
	}
	context.Header("Content-Disposition", "attachment; filename=planning-resolution.json")
	WriteData(context, http.StatusOK, record)
}
