package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/service"
)

// TimeSlotHandler handles time slot HTTP endpoints.
type TimeSlotHandler struct {
	service service.TimeSlotService
	logger  *slog.Logger
}

// NewTimeSlotHandler constructs a time slot handler.
func NewTimeSlotHandler(service service.TimeSlotService, logger *slog.Logger) *TimeSlotHandler {
	return &TimeSlotHandler{service: service, logger: logger}
}

// List godoc
// @Summary List time-slots
// @Tags time-slots
// @Produce json
// @Param page query int false "page"
// @Param page_size query int false "page size"
// @Success 200 {object} dto.Response
// @Router /api/v1/time-slots [get]
func (h *TimeSlotHandler) List(c *gin.Context) {
	var p dto.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		BadRequest(c, "invalid pagination")
		return
	}
	p.Normalize()
	items, total, err := h.service.List(c.Request.Context(), p.Page, p.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, dto.PageData{Items: items, Total: total, Page: p.Page, PageSize: p.PageSize})
}

// Get godoc
// @Summary Get a time slot
// @Tags time-slots
// @Produce json
// @Param id path int true "time slot id"
// @Success 200 {object} dto.Response
// @Router /api/v1/time-slots/{id} [get]
func (h *TimeSlotHandler) Get(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, item)
}

// Create godoc
// @Summary Create a time slot
// @Tags time-slots
// @Accept json
// @Produce json
// @Param time_slot body dto.CreateTimeSlotRequest true "time slot payload"
// @Success 201 {object} dto.Response
// @Router /api/v1/time-slots [post]
func (h *TimeSlotHandler) Create(c *gin.Context) {
	var req dto.CreateTimeSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	item, err := h.service.Create(c.Request.Context(), &req)
	if err != nil {
		Error(c, err)
		return
	}
	Created(c, item)
}

// Update godoc
// @Summary Update a time slot
// @Tags time-slots
// @Accept json
// @Produce json
// @Param id path int true "time slot id"
// @Param time_slot body dto.UpdateTimeSlotRequest true "time slot payload"
// @Success 200 {object} dto.Response
// @Router /api/v1/time-slots/{id} [put]
func (h *TimeSlotHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateTimeSlotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, &req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, item)
}

// Delete godoc
// @Summary Delete a time slot
// @Tags time-slots
// @Produce json
// @Param id path int true "time slot id"
// @Success 200 {object} dto.Response
// @Router /api/v1/time-slots/{id} [delete]
func (h *TimeSlotHandler) Delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		Error(c, err)
		return
	}
	OK(c, nil)
}
