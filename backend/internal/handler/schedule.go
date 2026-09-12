package handler

import (
	"bytes"
	"encoding/csv"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/service"
)

// ScheduleHandler handles timetable scheduling HTTP endpoints.
type ScheduleHandler struct {
	service service.ScheduleService
	logger  *slog.Logger
}

// NewScheduleHandler constructs a schedule handler.
func NewScheduleHandler(service service.ScheduleService, logger *slog.Logger) *ScheduleHandler {
	return &ScheduleHandler{service: service, logger: logger}
}

// Generate godoc
// @Summary Generate a timetable
// @Tags schedules
// @Accept json
// @Produce json
// @Param input body dto.GenerateScheduleRequest true "scheduling input"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedules/generate [post]
func (h *ScheduleHandler) Generate(c *gin.Context) {
	var req dto.GenerateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	result, err := h.service.Generate(c.Request.Context(), &req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, result)
}

// List godoc
// @Summary List timetable entries
// @Tags schedules
// @Produce json
// @Param week query int false "week"
// @Param class_id query int false "class id"
// @Param teacher_id query int false "teacher id"
// @Param classroom_id query int false "classroom id"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedules [get]
func (h *ScheduleHandler) List(c *gin.Context) {
	week, classID, teacherID, classroomID, err := parseScheduleFilters(c)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}
	items, err := h.service.List(c.Request.Context(), week, classID, teacherID, classroomID)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, items)
}

// Get godoc
// @Summary Get one timetable entry
// @Tags schedules
// @Produce json
// @Param id path int true "schedule id"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedules/{id} [get]
func (h *ScheduleHandler) Get(c *gin.Context) {
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

// Conflicts godoc
// @Summary Detect conflicts in the current timetable
// @Tags schedules
// @Produce json
// @Success 200 {object} dto.Response
// @Router /api/v1/schedules/conflicts [get]
func (h *ScheduleHandler) Conflicts(c *gin.Context) {
	items, err := h.service.CheckConflicts(c.Request.Context())
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, items)
}

// Swap godoc
// @Summary Swap two timetable entries
// @Tags schedules
// @Accept json
// @Produce json
// @Param input body dto.SwapScheduleRequest true "swap payload"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedules/swap [post]
func (h *ScheduleHandler) Swap(c *gin.Context) {
	var req dto.SwapScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	result, err := h.service.Swap(c.Request.Context(), &req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, result)
}

// Move godoc
// @Summary Move a timetable entry to a free slot
// @Tags schedules
// @Accept json
// @Produce json
// @Param input body dto.MoveScheduleRequest true "move payload"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedules/move [post]
func (h *ScheduleHandler) Move(c *gin.Context) {
	var req dto.MoveScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}
	result, err := h.service.Move(c.Request.Context(), &req)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, result)
}

// Adjustments godoc
// @Summary List adjustment history
// @Tags schedules
// @Produce json
// @Param page query int false "page"
// @Param page_size query int false "page size"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedules/adjustments [get]
func (h *ScheduleHandler) Adjustments(c *gin.Context) {
	var p dto.Pagination
	if err := c.ShouldBindQuery(&p); err != nil {
		BadRequest(c, "invalid pagination")
		return
	}
	p.Normalize()
	items, total, err := h.service.ListAdjustments(c.Request.Context(), p.Page, p.PageSize)
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, dto.PageData{Items: items, Total: total, Page: p.Page, PageSize: p.PageSize})
}

// Export godoc
// @Summary Export a timetable as JSON or CSV
// @Tags schedules
// @Produce json
// @Param type query string true "class|teacher|classroom"
// @Param id query int true "entity id"
// @Param week query int false "week"
// @Param format query string false "json|csv"
// @Success 200 {object} dto.Response
// @Router /api/v1/schedules/export [get]
func (h *ScheduleHandler) Export(c *gin.Context) {
	var exportReq dto.ExportScheduleRequest
	if err := c.ShouldBindQuery(&exportReq); err != nil {
		BadRequest(c, err.Error())
		return
	}
	format := c.DefaultQuery("format", "json")
	var week *uint
	if exportReq.Week > 0 {
		w := exportReq.Week
		week = &w
	}
	var classID, teacherID, classroomID *uint
	switch exportReq.Type {
	case "class":
		id := exportReq.ID
		classID = &id
	case "teacher":
		id := exportReq.ID
		teacherID = &id
	case "classroom":
		id := exportReq.ID
		classroomID = &id
	}
	items, err := h.service.List(c.Request.Context(), week, classID, teacherID, classroomID)
	if err != nil {
		Error(c, err)
		return
	}
	if format == "csv" {
		writeScheduleCSV(c, items)
		return
	}
	OK(c, items)
}

func parseScheduleFilters(c *gin.Context) (*uint, *uint, *uint, *uint, error) {
	var week, classID, teacherID, classroomID *uint
	if v := c.Query("week"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return nil, nil, nil, nil, service.ErrInvalid
		}
		x := uint(n)
		week = &x
	}
	if v := c.Query("class_id"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return nil, nil, nil, nil, service.ErrInvalid
		}
		x := uint(n)
		classID = &x
	}
	if v := c.Query("teacher_id"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return nil, nil, nil, nil, service.ErrInvalid
		}
		x := uint(n)
		teacherID = &x
	}
	if v := c.Query("classroom_id"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return nil, nil, nil, nil, service.ErrInvalid
		}
		x := uint(n)
		classroomID = &x
	}
	return week, classID, teacherID, classroomID, nil
}

func writeScheduleCSV(c *gin.Context, items []dto.ScheduleResponse) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"id", "week", "day_of_week", "time_slot_code", "start_time", "end_time", "classroom_name", "teacher_name", "class_name", "course_name"})
	for _, item := range items {
		_ = w.Write([]string{
			strconv.FormatUint(uint64(item.ID), 10),
			strconv.FormatUint(uint64(item.Week), 10),
			strconv.Itoa(item.DayOfWeek),
			item.TimeSlotCode,
			item.StartTime,
			item.EndTime,
			item.ClassroomName,
			item.TeacherName,
			item.ClassName,
			item.CourseName,
		})
	}
	w.Flush()
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="schedule.csv"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}
