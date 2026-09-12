package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/gbschedule/gbschedule/internal/handler"
	"github.com/gbschedule/gbschedule/internal/middleware"
)

// Handlers aggregates all HTTP handlers for dependency injection.
type Handlers struct {
	Classroom  *handler.ClassroomHandler
	Teacher    *handler.TeacherHandler
	Class      *handler.ClassHandler
	Course     *handler.CourseHandler
	TimeSlot   *handler.TimeSlotHandler
	Schedule   *handler.ScheduleHandler
	Statistics *handler.StatisticsHandler
}

// New constructs the Gin engine with all routes and middleware.
func New(h Handlers, logger *slog.Logger) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), middleware.RequestLogger(logger), middleware.CORS())

	engine.GET("/healthz", health)
	engine.GET("/health", health)

	engine.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger/doc.json")))

	api := engine.Group("/api/v1")
	{
		classrooms := api.Group("/classrooms")
		{
			classrooms.GET("", h.Classroom.List)
			classrooms.POST("", h.Classroom.Create)
			classrooms.GET("/:id", h.Classroom.Get)
			classrooms.PUT("/:id", h.Classroom.Update)
			classrooms.DELETE("/:id", h.Classroom.Delete)
		}
		teachers := api.Group("/teachers")
		{
			teachers.GET("", h.Teacher.List)
			teachers.POST("", h.Teacher.Create)
			teachers.GET("/:id", h.Teacher.Get)
			teachers.PUT("/:id", h.Teacher.Update)
			teachers.DELETE("/:id", h.Teacher.Delete)
		}
		classes := api.Group("/classes")
		{
			classes.GET("", h.Class.List)
			classes.POST("", h.Class.Create)
			classes.GET("/:id", h.Class.Get)
			classes.PUT("/:id", h.Class.Update)
			classes.DELETE("/:id", h.Class.Delete)
		}
		courses := api.Group("/courses")
		{
			courses.GET("", h.Course.List)
			courses.POST("", h.Course.Create)
			courses.GET("/:id", h.Course.Get)
			courses.PUT("/:id", h.Course.Update)
			courses.DELETE("/:id", h.Course.Delete)
		}
		timeSlots := api.Group("/time-slots")
		{
			timeSlots.GET("", h.TimeSlot.List)
			timeSlots.POST("", h.TimeSlot.Create)
			timeSlots.GET("/:id", h.TimeSlot.Get)
			timeSlots.PUT("/:id", h.TimeSlot.Update)
			timeSlots.DELETE("/:id", h.TimeSlot.Delete)
		}
		schedules := api.Group("/schedules")
		{
			schedules.GET("", h.Schedule.List)
			schedules.POST("/generate", h.Schedule.Generate)
			schedules.GET("/conflicts", h.Schedule.Conflicts)
			schedules.GET("/adjustments", h.Schedule.Adjustments)
			schedules.GET("/export", h.Schedule.Export)
			schedules.POST("/swap", h.Schedule.Swap)
			schedules.POST("/move", h.Schedule.Move)
			schedules.GET("/:id", h.Schedule.Get)
		}
		statistics := api.Group("/statistics")
		{
			statistics.GET("/classrooms", h.Statistics.ClassroomUtilization)
			statistics.GET("/teachers", h.Statistics.TeacherWorkload)
			statistics.GET("/density", h.Statistics.CourseDensity)
		}
	}
	return engine
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
