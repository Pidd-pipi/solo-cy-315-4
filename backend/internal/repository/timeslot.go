package repository

import (
	"context"
	"fmt"
	"sort"

	"github.com/gbschedule/gbschedule/internal/model"
	"gorm.io/gorm"
)

// TimeSlotRepository defines persistence operations for time slots.
type TimeSlotRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.TimeSlot, int64, error)
	GetByID(ctx context.Context, id uint) (*model.TimeSlot, error)
	GetByIDs(ctx context.Context, ids []uint) ([]model.TimeSlot, error)
	Create(ctx context.Context, slot *model.TimeSlot) error
	Update(ctx context.Context, slot *model.TimeSlot) error
	Delete(ctx context.Context, id uint) error
}

type timeSlotRepository struct {
	db *gorm.DB
}

// NewTimeSlotRepository constructs a time slot repository.
func NewTimeSlotRepository(db *gorm.DB) TimeSlotRepository {
	return &timeSlotRepository{db: db}
}

func (r *timeSlotRepository) List(ctx context.Context, page, pageSize int) ([]model.TimeSlot, int64, error) {
	var items []model.TimeSlot
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.TimeSlot{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count time slots: %w", err)
	}
	if err := paginate(r.db.WithContext(ctx).Model(&model.TimeSlot{}), page, pageSize).
		Order("start_time ASC").Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list time slots: %w", err)
	}
	return items, total, nil
}

func (r *timeSlotRepository) GetByID(ctx context.Context, id uint) (*model.TimeSlot, error) {
	var item model.TimeSlot
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, normalizeError(err)
	}
	return &item, nil
}

func (r *timeSlotRepository) GetByIDs(ctx context.Context, ids []uint) ([]model.TimeSlot, error) {
	var items []model.TimeSlot
	if len(ids) == 0 {
		return items, nil
	}
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("get time slots by ids: %w", err)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].StartTime < items[j].StartTime })
	return items, nil
}

func (r *timeSlotRepository) Create(ctx context.Context, slot *model.TimeSlot) error {
	if err := r.db.WithContext(ctx).Create(slot).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("create time slot: %w", err)
	}
	return nil
}

func (r *timeSlotRepository) Update(ctx context.Context, slot *model.TimeSlot) error {
	if err := r.db.WithContext(ctx).Save(slot).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("update time slot: %w", err)
	}
	return nil
}

func (r *timeSlotRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.TimeSlot{}, id).Error; err != nil {
		return fmt.Errorf("delete time slot: %w", err)
	}
	return nil
}
