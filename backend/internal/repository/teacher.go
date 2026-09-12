package repository

import (
	"context"
	"fmt"

	"github.com/gbschedule/gbschedule/internal/model"
	"gorm.io/gorm"
)

// TeacherRepository defines persistence operations for teachers.
type TeacherRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Teacher, int64, error)
	GetByID(ctx context.Context, id uint) (*model.Teacher, error)
	GetByIDs(ctx context.Context, ids []uint) ([]model.Teacher, error)
	Create(ctx context.Context, teacher *model.Teacher) error
	Update(ctx context.Context, teacher *model.Teacher) error
	Delete(ctx context.Context, id uint) error
}

type teacherRepository struct {
	db *gorm.DB
}

// NewTeacherRepository constructs a teacher repository.
func NewTeacherRepository(db *gorm.DB) TeacherRepository {
	return &teacherRepository{db: db}
}

func (r *teacherRepository) List(ctx context.Context, page, pageSize int) ([]model.Teacher, int64, error) {
	var items []model.Teacher
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Teacher{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count teachers: %w", err)
	}
	if err := paginate(r.db.WithContext(ctx).Model(&model.Teacher{}), page, pageSize).
		Order("id ASC").Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list teachers: %w", err)
	}
	return items, total, nil
}

func (r *teacherRepository) GetByID(ctx context.Context, id uint) (*model.Teacher, error) {
	var item model.Teacher
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, normalizeError(err)
	}
	return &item, nil
}

func (r *teacherRepository) GetByIDs(ctx context.Context, ids []uint) ([]model.Teacher, error) {
	var items []model.Teacher
	if len(ids) == 0 {
		return items, nil
	}
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("get teachers by ids: %w", err)
	}
	return items, nil
}

func (r *teacherRepository) Create(ctx context.Context, teacher *model.Teacher) error {
	if err := r.db.WithContext(ctx).Create(teacher).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("create teacher: %w", err)
	}
	return nil
}

func (r *teacherRepository) Update(ctx context.Context, teacher *model.Teacher) error {
	if err := r.db.WithContext(ctx).Save(teacher).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("update teacher: %w", err)
	}
	return nil
}

func (r *teacherRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.Teacher{}, id).Error; err != nil {
		return fmt.Errorf("delete teacher: %w", err)
	}
	return nil
}
