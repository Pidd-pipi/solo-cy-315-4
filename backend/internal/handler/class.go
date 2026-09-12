package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/service"
)

// ClassHandler handles class HTTP endpoints.
type ClassHandler struct {
	service service.ClassService
	logger  *slog.Logger
}

// NewClassHandler constructs a class handler.
func NewClassHandler(service service.ClassService, logger *slog.Logger) *ClassHandler {
	return &ClassHandler{service: service, logger: logger}
}

// List godoc
// @Summary List classes
// @Tags classes
// @Produce json
// @Param page query int false "page"
// @Param page_size query int false "page size"
// @Success 200 {object} dto.Response
// @Router /api/v1/classes [get]
func (h *ClassHandler) List(c *gin.Context) {
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
// @Summary Get a class
// @Tags classes
// @Produce json
// @Param id path int true "class id"
// @Success 200 {object} dto.Response
// @Router /api/v1/classes/{id} [get]
func (h *ClassHandler) Get(c *gin.Context) {
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
// @Summary Create a class
// @Tags classes
// @Accept json
// @Produce json
// @Param class body dto.CreateClassRequest true "class payload"
// @Success 201 {object} dto.Response
// @Router /api/v1/classes [post]
func (h *ClassHandler) Create(c *gin.Context) {
	var req dto.CreateClassRequest
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
// @Summary Update a class
// @Tags classes
// @Accept json
// @Produce json
// @Param id path int true "class id"
// @Param class body dto.UpdateClassRequest true "class payload"
// @Success 200 {object} dto.Response
// @Router /api/v1/classes/{id} [put]
func (h *ClassHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateClassRequest
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
// @Summary Delete a class
// @Tags classes
// @Produce json
// @Param id path int true "class id"
// @Success 200 {object} dto.Response
// @Router /api/v1/classes/{id} [delete]
func (h *ClassHandler) Delete(c *gin.Context) {
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
