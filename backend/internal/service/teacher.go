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

// TeacherService exposes teacher business operations.
type TeacherService interface {
	List(ctx context.Context, page, pageSize int) ([]dto.TeacherResponse, int64, error)
	Get(ctx context.Context, id uint) (*dto.TeacherResponse, error)
	Create(ctx context.Context, req *dto.CreateTeacherRequest) (*dto.TeacherResponse, error)
	Update(ctx context.Context, id uint, req *dto.UpdateTeacherRequest) (*dto.TeacherResponse, error)
	Delete(ctx context.Context, id uint) error
}

type teacherService struct {
	repo   repository.TeacherRepository
	logger *slog.Logger
}

// NewTeacherService constructs a teacher service.
func NewTeacherService(repo repository.TeacherRepository, logger *slog.Logger) TeacherService {
	return &teacherService{repo: repo, logger: logger}
}

func (s *teacherService) List(ctx context.Context, page, pageSize int) ([]dto.TeacherResponse, int64, error) {
	items, total, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list teachers: %w", err)
	}
	return teacherResponses(items), total, nil
}

func (s *teacherService) Get(ctx context.Context, id uint) (*dto.TeacherResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get teacher: %w", err)
	}
	resp := teacherResponse(item)
	return &resp, nil
}

func (s *teacherService) Create(ctx context.Context, req *dto.CreateTeacherRequest) (*dto.TeacherResponse, error) {
	item := &model.Teacher{
		Name:             req.Name,
		EmployeeNo:       req.EmployeeNo,
		Contact:          req.Contact,
		Subjects:         req.Subjects,
		UnavailableSlots: req.UnavailableSlots,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, mapWriteError("create teacher", err)
	}
	resp := teacherResponse(item)
	return &resp, nil
}

func (s *teacherService) Update(ctx context.Context, id uint, req *dto.UpdateTeacherRequest) (*dto.TeacherResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get teacher: %w", err)
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.EmployeeNo != "" {
		item.EmployeeNo = req.EmployeeNo
	}
	if req.Contact != "" {
		item.Contact = req.Contact
	}
	if req.Subjects != nil {
		item.Subjects = req.Subjects
	}
	if req.UnavailableSlots != nil {
		item.UnavailableSlots = req.UnavailableSlots
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, mapWriteError("update teacher", err)
	}
	resp := teacherResponse(item)
	return &resp, nil
}

func (s *teacherService) Delete(ctx context.Context, id uint) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get teacher: %w", err)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete teacher: %w", err)
	}
	return nil
}

func teacherResponse(item *model.Teacher) dto.TeacherResponse {
	return dto.TeacherResponse{
		ID:               item.ID,
		Name:             item.Name,
		EmployeeNo:       item.EmployeeNo,
		Contact:          item.Contact,
		Subjects:         item.Subjects,
		UnavailableSlots: item.UnavailableSlots,
		CreatedAt:        item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.Format(time.RFC3339),
	}
}

func teacherResponses(items []model.Teacher) []dto.TeacherResponse {
	out := make([]dto.TeacherResponse, 0, len(items))
	for i := range items {
		out = append(out, teacherResponse(&items[i]))
	}
	return out
}
