package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/service"
)

// CourseHandler handles course HTTP endpoints.
type CourseHandler struct {
	service service.CourseService
	logger  *slog.Logger
}

// NewCourseHandler constructs a course handler.
func NewCourseHandler(service service.CourseService, logger *slog.Logger) *CourseHandler {
	return &CourseHandler{service: service, logger: logger}
}

// List godoc
// @Summary List courses
// @Tags courses
// @Produce json
// @Param page query int false "page"
// @Param page_size query int false "page size"
// @Success 200 {object} dto.Response
// @Router /api/v1/courses [get]
func (h *CourseHandler) List(c *gin.Context) {
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
// @Summary Get a course
// @Tags courses
// @Produce json
// @Param id path int true "course id"
// @Success 200 {object} dto.Response
// @Router /api/v1/courses/{id} [get]
func (h *CourseHandler) Get(c *gin.Context) {
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
// @Summary Create a course
// @Tags courses
// @Accept json
// @Produce json
// @Param course body dto.CreateCourseRequest true "course payload"
// @Success 201 {object} dto.Response
// @Router /api/v1/courses [post]
func (h *CourseHandler) Create(c *gin.Context) {
	var req dto.CreateCourseRequest
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
// @Summary Update a course
// @Tags courses
// @Accept json
// @Produce json
// @Param id path int true "course id"
// @Param course body dto.UpdateCourseRequest true "course payload"
// @Success 200 {object} dto.Response
// @Router /api/v1/courses/{id} [put]
func (h *CourseHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateCourseRequest
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
// @Summary Delete a course
// @Tags courses
// @Produce json
// @Param id path int true "course id"
// @Success 200 {object} dto.Response
// @Router /api/v1/courses/{id} [delete]
func (h *CourseHandler) Delete(c *gin.Context) {
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
