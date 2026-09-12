package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/repository"
)

// ClassroomService exposes classroom business operations.
type ClassroomService interface {
	List(ctx context.Context, page, pageSize int) ([]dto.ClassroomResponse, int64, error)
	Get(ctx context.Context, id uint) (*dto.ClassroomResponse, error)
	Create(ctx context.Context, req *dto.CreateClassroomRequest) (*dto.ClassroomResponse, error)
	Update(ctx context.Context, id uint, req *dto.UpdateClassroomRequest) (*dto.ClassroomResponse, error)
	Delete(ctx context.Context, id uint) error
}

type classroomService struct {
	repo   repository.ClassroomRepository
	logger *slog.Logger
}

// NewClassroomService constructs a classroom service.
func NewClassroomService(repo repository.ClassroomRepository, logger *slog.Logger) ClassroomService {
	return &classroomService{repo: repo, logger: logger}
}

func (s *classroomService) List(ctx context.Context, page, pageSize int) ([]dto.ClassroomResponse, int64, error) {
	items, total, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list classrooms: %w", err)
	}
	return classroomResponses(items), total, nil
}

func (s *classroomService) Get(ctx context.Context, id uint) (*dto.ClassroomResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get classroom: %w", err)
	}
	resp := classroomResponse(item)
	return &resp, nil
}

func (s *classroomService) Create(ctx context.Context, req *dto.CreateClassroomRequest) (*dto.ClassroomResponse, error) {
	item := &model.Classroom{
		Code:      req.Code,
		Name:      req.Name,
		Capacity:  req.Capacity,
		Equipment: req.Equipment,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, mapWriteError("create classroom", err)
	}
	resp := classroomResponse(item)
	return &resp, nil
}

func (s *classroomService) Update(ctx context.Context, id uint, req *dto.UpdateClassroomRequest) (*dto.ClassroomResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get classroom: %w", err)
	}
	if req.Code != "" {
		item.Code = req.Code
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Capacity != nil {
		item.Capacity = *req.Capacity
	}
	if req.Equipment != nil {
		item.Equipment = req.Equipment
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, mapWriteError("update classroom", err)
	}
	resp := classroomResponse(item)
	return &resp, nil
}

func (s *classroomService) Delete(ctx context.Context, id uint) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get classroom: %w", err)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete classroom: %w", err)
	}
	return nil
}

func classroomResponse(item *model.Classroom) dto.ClassroomResponse {
	return dto.ClassroomResponse{
		ID:        item.ID,
		Code:      item.Code,
		Name:      item.Name,
		Capacity:  item.Capacity,
		Equipment: item.Equipment,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
}

func classroomResponses(items []model.Classroom) []dto.ClassroomResponse {
	out := make([]dto.ClassroomResponse, 0, len(items))
	for i := range items {
		out = append(out, classroomResponse(&items[i]))
	}
	return out
}
