package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"satellite-contact-window-deconfliction/backend/internal/dto"
	"satellite-contact-window-deconfliction/backend/internal/service"
)

type ContactWindowHandler struct{ service *service.ContactWindowService }

func NewContactWindowHandler(service *service.ContactWindowService) *ContactWindowHandler {
	return &ContactWindowHandler{service: service}
}

func (handler *ContactWindowHandler) List(context *gin.Context) {
	page, pageSize := Pagination(context)
	filter := dto.ContactWindowFilter{Page: page, PageSize: pageSize, Status: context.Query("status")}
	filter.StationID = queryUint(context.Query("station_id"))
	filter.SatelliteID = queryUint(context.Query("satellite_id"))
	if value := context.Query("from"); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			WriteError(context, service.BadRequest("invalid_time_range", "from must be RFC3339"))
			return
		}
		filter.From = &parsed
	}
	if value := context.Query("to"); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			WriteError(context, service.BadRequest("invalid_time_range", "to must be RFC3339"))
			return
		}
		filter.To = &parsed
	}
	windows, meta, err := handler.service.List(filter)
	if err != nil {
		WriteError(context, err)
		return
	}
	WritePage(context, windows, meta)
}

func (handler *ContactWindowHandler) Get(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	window, err := handler.service.Get(id)
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, window)
}

func (handler *ContactWindowHandler) Create(context *gin.Context) {
	var request dto.CreateContactWindowRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	window, err := handler.service.Create(request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusCreated, window)
}

func (handler *ContactWindowHandler) Update(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request dto.UpdateContactWindowRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	window, err := handler.service.Update(id, request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, window)
}

func (handler *ContactWindowHandler) Submit(context *gin.Context) { handler.action(context, false) }
func (handler *ContactWindowHandler) Lock(context *gin.Context)   { handler.action(context, true) }

func (handler *ContactWindowHandler) action(context *gin.Context, lock bool) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request dto.WindowActionRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	var window dto.ContactWindowResponse
	if lock {
		window, err = handler.service.Lock(id, request, Actor(context), RequestID(context))
	} else {
		window, err = handler.service.Submit(id, request, Actor(context), RequestID(context))
	}
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, window)
}

func queryUint(value string) uint { parsed, _ := strconv.ParseUint(value, 10, 32); return uint(parsed) }
