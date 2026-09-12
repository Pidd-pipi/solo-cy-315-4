package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/service"
)

// TeacherHandler handles teacher HTTP endpoints.
type TeacherHandler struct {
	service service.TeacherService
	logger  *slog.Logger
}

// NewTeacherHandler constructs a teacher handler.
func NewTeacherHandler(service service.TeacherService, logger *slog.Logger) *TeacherHandler {
	return &TeacherHandler{service: service, logger: logger}
}

// List godoc
// @Summary List teachers
// @Tags teachers
// @Produce json
// @Param page query int false "page"
// @Param page_size query int false "page size"
// @Success 200 {object} dto.Response
// @Router /api/v1/teachers [get]
func (h *TeacherHandler) List(c *gin.Context) {
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
// @Summary Get a teacher
// @Tags teachers
// @Produce json
// @Param id path int true "teacher id"
// @Success 200 {object} dto.Response
// @Router /api/v1/teachers/{id} [get]
func (h *TeacherHandler) Get(c *gin.Context) {
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
// @Summary Create a teacher
// @Tags teachers
// @Accept json
// @Produce json
// @Param teacher body dto.CreateTeacherRequest true "teacher payload"
// @Success 201 {object} dto.Response
// @Router /api/v1/teachers [post]
func (h *TeacherHandler) Create(c *gin.Context) {
	var req dto.CreateTeacherRequest
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
// @Summary Update a teacher
// @Tags teachers
// @Accept json
// @Produce json
// @Param id path int true "teacher id"
// @Param teacher body dto.UpdateTeacherRequest true "teacher payload"
// @Success 200 {object} dto.Response
// @Router /api/v1/teachers/{id} [put]
func (h *TeacherHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateTeacherRequest
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
// @Summary Delete a teacher
// @Tags teachers
// @Produce json
// @Param id path int true "teacher id"
// @Success 200 {object} dto.Response
// @Router /api/v1/teachers/{id} [delete]
func (h *TeacherHandler) Delete(c *gin.Context) {
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
