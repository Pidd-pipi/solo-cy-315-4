package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/service"
)

// ScheduleDraftHandler exposes asynchronous schedule draft endpoints.
type ScheduleDraftHandler struct {
	service service.ScheduleDraftService
	logger  *slog.Logger
}

// NewScheduleDraftHandler constructs a schedule draft handler.
func NewScheduleDraftHandler(service service.ScheduleDraftService, logger *slog.Logger) *ScheduleDraftHandler {
	return &ScheduleDraftHandler{service: service, logger: logger}
}

// Create godoc
// @Summary Submit scheduling conditions and create a draft
// @Description The draft id is returned immediately and timetable generation runs in the background.
// @Tags schedule-drafts
// @Accept json
// @Produce json
// @Param input body dto.GenerateScheduleRequest true "scheduling input"
// @Success 202 {object} dto.Response
// @Router /api/v1/schedule-drafts [post]
func (h *ScheduleDraftHandler) Create(c *gin.Context) {
	var req dto.GenerateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	result, err := h.service.Submit(c.Request.Context(), &req)
	if err != nil {
		Error(c, err)
		return
	}
	Accepted(c, result)
}

// List godoc
// @Summary List schedule drafts, newest first
// @Tags schedule-drafts
// @Produce json
// @Param status query string false "queued|running|succeeded|failed|canceled"
// @Param page query int false "page"
// @Param page_size query int false "page size"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedule-drafts [get]
func (h *ScheduleDraftHandler) List(c *gin.Context) {
	var q dto.DraftListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		BadRequest(c, err.Error())
		return
	}
	q.Normalize()
	items, total, err := h.service.List(c.Request.Context(), q.Status, q.Page, q.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, dto.PageData{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize})
}

// Get godoc
// @Summary Get a draft with progress, current conflicts and failure reason
// @Tags schedule-drafts
// @Produce json
// @Param id path int true "draft id"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedule-drafts/{id} [get]
func (h *ScheduleDraftHandler) Get(c *gin.Context) {
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

// GetResult godoc
// @Summary Get the full timetable, conflicts and summary of a successful draft
// @Tags schedule-drafts
// @Produce json
// @Param id path int true "draft id"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedule-drafts/{id}/result [get]
func (h *ScheduleDraftHandler) GetResult(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.service.GetResult(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, item)
}

// Cancel godoc
// @Summary Cancel a queued or running draft (idempotent)
// @Description Canceling a draft already in a terminal state returns that state unchanged.
// @Tags schedule-drafts
// @Produce json
// @Param id path int true "draft id"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedule-drafts/{id}/cancel [post]
func (h *ScheduleDraftHandler) Cancel(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.service.Cancel(c.Request.Context(), id)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, item)
}
