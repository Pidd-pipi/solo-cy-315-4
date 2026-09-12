package repository

import (
	"context"
	"fmt"

	"github.com/gbschedule/gbschedule/internal/model"
	"gorm.io/gorm"
)

// ClassroomRepository defines persistence operations for classrooms.
type ClassroomRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Classroom, int64, error)
	GetByID(ctx context.Context, id uint) (*model.Classroom, error)
	GetByIDs(ctx context.Context, ids []uint) ([]model.Classroom, error)
	Create(ctx context.Context, classroom *model.Classroom) error
	Update(ctx context.Context, classroom *model.Classroom) error
	Delete(ctx context.Context, id uint) error
}

type classroomRepository struct {
	db *gorm.DB
}

// NewClassroomRepository constructs a classroom repository.
func NewClassroomRepository(db *gorm.DB) ClassroomRepository {
	return &classroomRepository{db: db}
}

func (r *classroomRepository) List(ctx context.Context, page, pageSize int) ([]model.Classroom, int64, error) {
	var items []model.Classroom
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Classroom{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count classrooms: %w", err)
	}
	if err := paginate(r.db.WithContext(ctx).Model(&model.Classroom{}), page, pageSize).
		Order("id ASC").Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list classrooms: %w", err)
	}
	return items, total, nil
}

func (r *classroomRepository) GetByID(ctx context.Context, id uint) (*model.Classroom, error) {
	var item model.Classroom
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, normalizeError(err)
	}
	return &item, nil
}

func (r *classroomRepository) GetByIDs(ctx context.Context, ids []uint) ([]model.Classroom, error) {
	var items []model.Classroom
	if len(ids) == 0 {
		return items, nil
	}
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("get classrooms by ids: %w", err)
	}
	return items, nil
}

func (r *classroomRepository) Create(ctx context.Context, classroom *model.Classroom) error {
	if err := r.db.WithContext(ctx).Create(classroom).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("create classroom: %w", err)
	}
	return nil
}

func (r *classroomRepository) Update(ctx context.Context, classroom *model.Classroom) error {
	if err := r.db.WithContext(ctx).Save(classroom).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("update classroom: %w", err)
	}
	return nil
}

func (r *classroomRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.Classroom{}, id).Error; err != nil {
		return fmt.Errorf("delete classroom: %w", err)
	}
	return nil
}
