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

// ClassService exposes class business operations.
type ClassService interface {
	List(ctx context.Context, page, pageSize int) ([]dto.ClassResponse, int64, error)
	Get(ctx context.Context, id uint) (*dto.ClassResponse, error)
	Create(ctx context.Context, req *dto.CreateClassRequest) (*dto.ClassResponse, error)
	Update(ctx context.Context, id uint, req *dto.UpdateClassRequest) (*dto.ClassResponse, error)
	Delete(ctx context.Context, id uint) error
}

type classService struct {
	repo   repository.ClassRepository
	logger *slog.Logger
}

// NewClassService constructs a class service.
func NewClassService(repo repository.ClassRepository, logger *slog.Logger) ClassService {
	return &classService{repo: repo, logger: logger}
}

func (s *classService) List(ctx context.Context, page, pageSize int) ([]dto.ClassResponse, int64, error) {
	items, total, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list classes: %w", err)
	}
	return classResponses(items), total, nil
}

func (s *classService) Get(ctx context.Context, id uint) (*dto.ClassResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get class: %w", err)
	}
	resp := classResponse(item)
	return &resp, nil
}

func (s *classService) Create(ctx context.Context, req *dto.CreateClassRequest) (*dto.ClassResponse, error) {
	item := &model.Class{
		Name:         req.Name,
		StudentCount: req.StudentCount,
		Grade:        req.Grade,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, mapWriteError("create class", err)
	}
	resp := classResponse(item)
	return &resp, nil
}

func (s *classService) Update(ctx context.Context, id uint, req *dto.UpdateClassRequest) (*dto.ClassResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get class: %w", err)
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.StudentCount != nil {
		item.StudentCount = *req.StudentCount
	}
	if req.Grade != "" {
		item.Grade = req.Grade
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, mapWriteError("update class", err)
	}
	resp := classResponse(item)
	return &resp, nil
}

func (s *classService) Delete(ctx context.Context, id uint) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get class: %w", err)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete class: %w", err)
	}
	return nil
}

func classResponse(item *model.Class) dto.ClassResponse {
	return dto.ClassResponse{
		ID:           item.ID,
		Name:         item.Name,
		StudentCount: item.StudentCount,
		Grade:        item.Grade,
		CreatedAt:    item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    item.UpdatedAt.Format(time.RFC3339),
	}
}

func classResponses(items []model.Class) []dto.ClassResponse {
	out := make([]dto.ClassResponse, 0, len(items))
	for i := range items {
		out = append(out, classResponse(&items[i]))
	}
	return out
}
