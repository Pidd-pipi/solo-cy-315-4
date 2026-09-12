package repository_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/repository"
)

func newTestDB(t *testing.T) *gorm.DB {
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

func TestClassroomRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewClassroomRepository(newTestDB(t))

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "create and get",
			run: func(t *testing.T) {
				item := &model.Classroom{Code: "R101", Name: "101教室", Capacity: 40, Equipment: []string{"投影仪"}}
				if err := repo.Create(ctx, item); err != nil {
					t.Fatalf("create: %v", err)
				}
				if item.ID == 0 {
					t.Fatal("expected auto generated id")
				}
				got, err := repo.GetByID(ctx, item.ID)
				if err != nil {
					t.Fatalf("get: %v", err)
				}
				if got.Code != "R101" || got.Capacity != 40 || len(got.Equipment) != 1 {
					t.Fatalf("unexpected classroom: %+v", got)
				}
			},
		},
		{
			name: "update and list",
			run: func(t *testing.T) {
				item := &model.Classroom{Code: "R102", Name: "102教室", Capacity: 30}
				if err := repo.Create(ctx, item); err != nil {
					t.Fatalf("create: %v", err)
				}
				item.Capacity = 35
				if err := repo.Update(ctx, item); err != nil {
					t.Fatalf("update: %v", err)
				}
				items, total, err := repo.List(ctx, 1, 10)
				if err != nil {
					t.Fatalf("list: %v", err)
				}
				if total < 1 {
					t.Fatalf("expected at least one classroom, got %d", total)
				}
				found := false
				for _, it := range items {
					if it.ID == item.ID && it.Capacity == 35 {
						found = true
					}
				}
				if !found {
					t.Fatal("updated classroom not found")
				}
			},
		},
		{
			name: "delete returns not found",
			run: func(t *testing.T) {
				item := &model.Classroom{Code: "R103", Name: "103教室", Capacity: 20}
				if err := repo.Create(ctx, item); err != nil {
					t.Fatalf("create: %v", err)
				}
				if err := repo.Delete(ctx, item.ID); err != nil {
					t.Fatalf("delete: %v", err)
				}
				if _, err := repo.GetByID(ctx, item.ID); !errors.Is(err, repository.ErrNotFound) {
					t.Fatalf("expected ErrNotFound, got %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
