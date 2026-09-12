package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/service"
)

// ClassroomHandler handles classroom HTTP endpoints.
type ClassroomHandler struct {
	service service.ClassroomService
	logger  *slog.Logger
}

// NewClassroomHandler constructs a classroom handler.
func NewClassroomHandler(service service.ClassroomService, logger *slog.Logger) *ClassroomHandler {
	return &ClassroomHandler{service: service, logger: logger}
}

// List godoc
// @Summary List classrooms
// @Tags classrooms
// @Produce json
// @Param page query int false "page"
// @Param page_size query int false "page size"
// @Success 200 {object} dto.Response
// @Router /api/v1/classrooms [get]
func (h *ClassroomHandler) List(c *gin.Context) {
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
// @Summary Get a classroom
// @Tags classrooms
// @Produce json
// @Param id path int true "classroom id"
// @Success 200 {object} dto.Response
// @Router /api/v1/classrooms/{id} [get]
func (h *ClassroomHandler) Get(c *gin.Context) {
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
// @Summary Create a classroom
// @Tags classrooms
// @Accept json
// @Produce json
// @Param classroom body dto.CreateClassroomRequest true "classroom payload"
// @Success 201 {object} dto.Response
// @Router /api/v1/classrooms [post]
func (h *ClassroomHandler) Create(c *gin.Context) {
	var req dto.CreateClassroomRequest
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
// @Summary Update a classroom
// @Tags classrooms
// @Accept json
// @Produce json
// @Param id path int true "classroom id"
// @Param classroom body dto.UpdateClassroomRequest true "classroom payload"
// @Success 200 {object} dto.Response
// @Router /api/v1/classrooms/{id} [put]
func (h *ClassroomHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateClassroomRequest
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
// @Summary Delete a classroom
// @Tags classrooms
// @Produce json
// @Param id path int true "classroom id"
// @Success 200 {object} dto.Response
// @Router /api/v1/classrooms/{id} [delete]
func (h *ClassroomHandler) Delete(c *gin.Context) {
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
