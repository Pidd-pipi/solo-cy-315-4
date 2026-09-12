package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	_ "github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/service"
)

// StatisticsHandler exposes utilization and workload statistics.
type StatisticsHandler struct {
	service service.ScheduleService
	logger  *slog.Logger
}

// NewStatisticsHandler constructs a statistics handler.
func NewStatisticsHandler(service service.ScheduleService, logger *slog.Logger) *StatisticsHandler {
	return &StatisticsHandler{service: service, logger: logger}
}

// ClassroomUtilization godoc
// @Summary Get classroom utilization statistics
// @Tags statistics
// @Produce json
// @Success 200 {object} dto.Response
// @Router /api/v1/statistics/classrooms [get]
func (h *StatisticsHandler) ClassroomUtilization(c *gin.Context) {
	items, err := h.service.ClassroomUtilization(c.Request.Context())
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, items)
}

// TeacherWorkload godoc
// @Summary Get teacher workload statistics
// @Tags statistics
// @Produce json
// @Success 200 {object} dto.Response
// @Router /api/v1/statistics/teachers [get]
func (h *StatisticsHandler) TeacherWorkload(c *gin.Context) {
	items, err := h.service.TeacherWorkload(c.Request.Context())
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, items)
}

// CourseDensity godoc
// @Summary Get course density heatmap data
// @Tags statistics
// @Produce json
// @Success 200 {object} dto.Response
// @Router /api/v1/statistics/density [get]
func (h *StatisticsHandler) CourseDensity(c *gin.Context) {
	items, err := h.service.CourseDensity(c.Request.Context())
	if err != nil {
		Error(c, err)
		return
	}
	OK(c, items)
}
