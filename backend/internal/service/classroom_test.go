package service_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/repository"
	"github.com/gbschedule/gbschedule/internal/service"
)

func newServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Classroom{}, &model.Teacher{}, &model.Class{}, &model.Course{}, &model.TimeSlot{}, &model.Schedule{}, &model.AdjustmentLog{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

func TestClassroomServiceCRUD(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	repo := repository.NewClassroomRepository(newServiceTestDB(t))
	svc := service.NewClassroomService(repo, logger)

	tests := []struct {
		name string
		req  dto.CreateClassroomRequest
	}{
		{name: "normal", req: dto.CreateClassroomRequest{Code: "R201", Name: "201教室", Capacity: 50, Equipment: []string{"投影仪", "电脑"}}},
		{name: "small", req: dto.CreateClassroomRequest{Code: "R202", Name: "202教室", Capacity: 25}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := svc.Create(ctx, &tt.req)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if created.ID == 0 || created.Code != tt.req.Code {
				t.Fatalf("unexpected created classroom: %+v", created)
			}
			got, err := svc.Get(ctx, created.ID)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if got.Name != tt.req.Name {
				t.Fatalf("unexpected name: %s", got.Name)
			}
			items, total, err := svc.List(ctx, 1, 10)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if total < 1 || len(items) == 0 {
				t.Fatal("expected listed classrooms")
			}
		})
	}
}

func TestClassroomServiceGetNotFound(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	repo := repository.NewClassroomRepository(newServiceTestDB(t))
	svc := service.NewClassroomService(repo, logger)

	if _, err := svc.Get(ctx, 9999); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected service.ErrNotFound, got %v", err)
	}
}
