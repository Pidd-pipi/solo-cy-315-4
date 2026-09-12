package repository

import (
	"context"
	"fmt"

	"github.com/gbschedule/gbschedule/internal/model"
	"gorm.io/gorm"
)

// ClassRepository defines persistence operations for classes.
type ClassRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Class, int64, error)
	GetByID(ctx context.Context, id uint) (*model.Class, error)
	GetByIDs(ctx context.Context, ids []uint) ([]model.Class, error)
	Create(ctx context.Context, class *model.Class) error
	Update(ctx context.Context, class *model.Class) error
	Delete(ctx context.Context, id uint) error
}

type classRepository struct {
	db *gorm.DB
}

// NewClassRepository constructs a class repository.
func NewClassRepository(db *gorm.DB) ClassRepository {
	return &classRepository{db: db}
}

func (r *classRepository) List(ctx context.Context, page, pageSize int) ([]model.Class, int64, error) {
	var items []model.Class
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Class{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count classes: %w", err)
	}
	if err := paginate(r.db.WithContext(ctx).Model(&model.Class{}), page, pageSize).
		Order("id ASC").Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list classes: %w", err)
	}
	return items, total, nil
}

func (r *classRepository) GetByID(ctx context.Context, id uint) (*model.Class, error) {
	var item model.Class
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, normalizeError(err)
	}
	return &item, nil
}

func (r *classRepository) GetByIDs(ctx context.Context, ids []uint) ([]model.Class, error) {
	var items []model.Class
	if len(ids) == 0 {
		return items, nil
	}
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("get classes by ids: %w", err)
	}
	return items, nil
}

func (r *classRepository) Create(ctx context.Context, class *model.Class) error {
	if err := r.db.WithContext(ctx).Create(class).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("create class: %w", err)
	}
	return nil
}

func (r *classRepository) Update(ctx context.Context, class *model.Class) error {
	if err := r.db.WithContext(ctx).Save(class).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("update class: %w", err)
	}
	return nil
}

func (r *classRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.Class{}, id).Error; err != nil {
		return fmt.Errorf("delete class: %w", err)
	}
	return nil
}
