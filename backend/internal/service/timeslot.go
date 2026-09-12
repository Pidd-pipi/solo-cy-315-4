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

// TimeSlotService exposes time slot business operations.
type TimeSlotService interface {
	List(ctx context.Context, page, pageSize int) ([]dto.TimeSlotResponse, int64, error)
	Get(ctx context.Context, id uint) (*dto.TimeSlotResponse, error)
	Create(ctx context.Context, req *dto.CreateTimeSlotRequest) (*dto.TimeSlotResponse, error)
	Update(ctx context.Context, id uint, req *dto.UpdateTimeSlotRequest) (*dto.TimeSlotResponse, error)
	Delete(ctx context.Context, id uint) error
}

type timeSlotService struct {
	repo   repository.TimeSlotRepository
	logger *slog.Logger
}

// NewTimeSlotService constructs a time slot service.
func NewTimeSlotService(repo repository.TimeSlotRepository, logger *slog.Logger) TimeSlotService {
	return &timeSlotService{repo: repo, logger: logger}
}

func (s *timeSlotService) List(ctx context.Context, page, pageSize int) ([]dto.TimeSlotResponse, int64, error) {
	items, total, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list time slots: %w", err)
	}
	return timeSlotResponses(items), total, nil
}

func (s *timeSlotService) Get(ctx context.Context, id uint) (*dto.TimeSlotResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get time slot: %w", err)
	}
	resp := timeSlotResponse(item)
	return &resp, nil
}

func (s *timeSlotService) Create(ctx context.Context, req *dto.CreateTimeSlotRequest) (*dto.TimeSlotResponse, error) {
	item := &model.TimeSlot{
		Code:      req.Code,
		Name:      req.Name,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}
	if err := s.repo.Create(ctx, item); err != nil {
		return nil, mapWriteError("create time slot", err)
	}
	resp := timeSlotResponse(item)
	return &resp, nil
}

func (s *timeSlotService) Update(ctx context.Context, id uint, req *dto.UpdateTimeSlotRequest) (*dto.TimeSlotResponse, error) {
	item, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get time slot: %w", err)
	}
	if req.Code != "" {
		item.Code = req.Code
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.StartTime != "" {
		item.StartTime = req.StartTime
	}
	if req.EndTime != "" {
		item.EndTime = req.EndTime
	}
	if err := s.repo.Update(ctx, item); err != nil {
		return nil, mapWriteError("update time slot", err)
	}
	resp := timeSlotResponse(item)
	return &resp, nil
}

func (s *timeSlotService) Delete(ctx context.Context, id uint) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("get time slot: %w", err)
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete time slot: %w", err)
	}
	return nil
}

func timeSlotResponse(item *model.TimeSlot) dto.TimeSlotResponse {
	return dto.TimeSlotResponse{
		ID:        item.ID,
		Code:      item.Code,
		Name:      item.Name,
		StartTime: item.StartTime,
		EndTime:   item.EndTime,
		CreatedAt: item.CreatedAt.Format(time.RFC3339),
		UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
	}
}

func timeSlotResponses(items []model.TimeSlot) []dto.TimeSlotResponse {
	out := make([]dto.TimeSlotResponse, 0, len(items))
	for i := range items {
		out = append(out, timeSlotResponse(&items[i]))
	}
	return out
}
