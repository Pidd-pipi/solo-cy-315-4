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

// CourseService exposes course business operations.
type CourseService interface {
	List(ctx context.Context, page, pageSize int) ([]dto.CourseResponse, int64, error)
	Get(ctx context.Context, id uint) (*dto.CourseResponse, error)
	Create(ctx context.Context, req *dto.CreateCourseRequest) (*dto.CourseResponse, error)
	Update(ctx context.Context, id uint, req *dto.UpdateCourseRequest) (*dto.CourseResponse, error)
	Delete(ctx context.Context, id uint) error
}

type courseService struct {
	repo   repository.CourseRepository
	logger *slog.Logger
}

// NewCourseService constructs a course service.
func NewCourseService(repo repository.CourseRepository, logger *slog.Logger) CourseService {
	return &courseService{repo: repo, logger: logger}
}

func (s *courseService) List(ctx context.Context, page, pageSize int) ([]dto.CourseResponse, int64, error) {
	items, total, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list courses: %w", err)
	}
	return courseResponses(items), total, nil
}

func (s *courseService) Get(ctx context.Context, id uint) (*dto.CourseResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get course: %w", err)
	}
	resp := courseResponse(item)
	return &resp, nil
}

func (s *courseService) Create(ctx context.Context, req *dto.CreateCourseRequest) (*dto.CourseResponse, error) {
	item := &model.Course{
		Name:     req.Name,
		Code:     req.Code,
		Duration: req.Duration,
		RoomType: req.RoomType,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, mapWriteError("create course", err)
	}
	resp := courseResponse(item)
	return &resp, nil
}

func (s *courseService) Update(ctx context.Context, id uint, req *dto.UpdateCourseRequest) (*dto.CourseResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get course: %w", err)
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Code != "" {
		item.Code = req.Code
	}
	if req.Duration != nil {
		item.Duration = *req.Duration
	}
	if req.RoomType != "" {
		item.RoomType = req.RoomType
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, mapWriteError("update course", err)
	}
	resp := courseResponse(item)
	return &resp, nil
}

func (s *courseService) Delete(ctx context.Context, id uint) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get course: %w", err)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete course: %w", err)
	}
	return nil
}

func courseResponse(item *model.Course) dto.CourseResponse {
	return dto.CourseResponse{
		ID:        item.ID,
		Name:      item.Name,
		Code:      item.Code,
		Duration:  item.Duration,
		RoomType:  item.RoomType,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
}

func courseResponses(items []model.Course) []dto.CourseResponse {
	out := make([]dto.CourseResponse, 0, len(items))
	for i := range items {
		out = append(out, courseResponse(&items[i]))
	}
	return out
}
