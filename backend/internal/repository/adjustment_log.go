package repository

import (
	"context"
	"fmt"

	"github.com/gbschedule/gbschedule/internal/model"
	"gorm.io/gorm"
)

// AdjustmentLogRepository persists manual adjustment history.
type AdjustmentLogRepository interface {
	Create(ctx context.Context, log *model.AdjustmentLog) error
	List(ctx context.Context, page, pageSize int) ([]model.AdjustmentLog, int64, error)
}

type adjustmentLogRepository struct {
	db *gorm.DB
}

// NewAdjustmentLogRepository constructs an adjustment log repository.
func NewAdjustmentLogRepository(db *gorm.DB) AdjustmentLogRepository {
	return &adjustmentLogRepository{db: db}
}

func (r *adjustmentLogRepository) Create(ctx context.Context, log *model.AdjustmentLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("create adjustment log: %w", err)
	}
	return nil
}

func (r *adjustmentLogRepository) List(ctx context.Context, page, pageSize int) ([]model.AdjustmentLog, int64, error) {
	var items []model.AdjustmentLog
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.AdjustmentLog{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count adjustment logs: %w", err)
	}
	if err := paginate(r.db.WithContext(ctx).Model(&model.AdjustmentLog{}), page, pageSize).
		Order("id DESC").Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list adjustment logs: %w", err)
	}
	return items, total, nil
}
