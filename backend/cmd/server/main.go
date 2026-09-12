package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	_ "github.com/gbschedule/gbschedule/docs"
	"github.com/gbschedule/gbschedule/internal/config"
	"github.com/gbschedule/gbschedule/internal/handler"
	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/repository"
	"github.com/gbschedule/gbschedule/internal/router"
	"github.com/gbschedule/gbschedule/internal/service"
)

// @title           教室排课助手 API
// @version         1.0.0
// @description     教室排课、教室资源管理和冲突检测 RESTful API。
// @BasePath        /
func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	db, err := openDatabase(cfg.DBPath)
	if err != nil {
		logger.Error("open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := migrate(db); err != nil {
		logger.Error("migrate database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	app, err := newApp(db, logger)
	if err != nil {
		logger.Error("assemble app", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Recover drafts interrupted by a previous run and start the background
	// worker. Tasks and results are persisted, so they survive restarts.
	rootCtx, stopRoot := context.WithCancel(context.Background())
	defer stopRoot()
	app.drafts.Start(rootCtx)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:      app.engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", slog.Int("port", cfg.ServerPort), slog.String("db_path", cfg.DBPath))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErr:
		logger.Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	case sig := <-sigCh:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))
	}

	// Stop accepting new background draft work and wait for in-flight
	// generation to notice cancellation, then shut down HTTP.
	app.drafts.Shutdown()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", slog.String("error", err.Error()))
	}
	logger.Info("server stopped cleanly")
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
}

func openDatabase(path string) (*gorm.DB, error) {
	if path == "" {
		path = "./data/gbschedule.db"
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := gorm.Open(sqlite.Open(sqliteDSN(path)), &gorm.Config{
		Logger:         gormlogger.Default.LogMode(gormlogger.Silent),
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return db, nil
}

// sqliteDSN adds a busy timeout so the background worker and HTTP requests do
// not fail with "database is locked" under concurrent writes.
func sqliteDSN(path string) string {
	return path + "?_pragma=busy_timeout(5000)"
}

func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.Classroom{},
		&model.Teacher{},
		&model.Class{},
		&model.Course{},
		&model.TimeSlot{},
		&model.Schedule{},
		&model.AdjustmentLog{},
		&model.ScheduleDraft{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}

// app bundles the HTTP engine with background services that need lifecycle
// management.
type app struct {
	engine *gin.Engine
	drafts service.ScheduleDraftService
}

func newApp(db *gorm.DB, logger *slog.Logger) (*app, error) {
	classroomRepo := repository.NewClassroomRepository(db)
	teacherRepo := repository.NewTeacherRepository(db)
	classRepo := repository.NewClassRepository(db)
	courseRepo := repository.NewCourseRepository(db)
	timeSlotRepo := repository.NewTimeSlotRepository(db)
	scheduleRepo := repository.NewScheduleRepository(db)
	adjustmentRepo := repository.NewAdjustmentLogRepository(db)
	draftRepo := repository.NewScheduleDraftRepository(db)

	classroomService := service.NewClassroomService(classroomRepo, logger)
	teacherService := service.NewTeacherService(teacherRepo, logger)
	classService := service.NewClassService(classRepo, logger)
	courseService := service.NewCourseService(courseRepo, logger)
	timeSlotService := service.NewTimeSlotService(timeSlotRepo, logger)
	scheduleService := service.NewScheduleService(scheduleRepo, classroomRepo, teacherRepo, classRepo, courseRepo, timeSlotRepo, adjustmentRepo, logger)
	draftService := service.NewScheduleDraftService(draftRepo, scheduleService, logger)

	h := router.Handlers{
		Classroom:     handler.NewClassroomHandler(classroomService, logger),
		Teacher:       handler.NewTeacherHandler(teacherService, logger),
		Class:         handler.NewClassHandler(classService, logger),
		Course:        handler.NewCourseHandler(courseService, logger),
		TimeSlot:      handler.NewTimeSlotHandler(timeSlotService, logger),
		Schedule:      handler.NewScheduleHandler(scheduleService, logger),
		ScheduleDraft: handler.NewScheduleDraftHandler(draftService, logger),
		Statistics:    handler.NewStatisticsHandler(scheduleService, logger),
	}
	return &app{engine: router.New(h, logger), drafts: draftService}, nil
}
