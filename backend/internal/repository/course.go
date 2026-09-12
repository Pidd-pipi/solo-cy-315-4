package repository

import (
	"context"
	"fmt"

	"github.com/gbschedule/gbschedule/internal/model"
	"gorm.io/gorm"
)

// CourseRepository defines persistence operations for courses.
type CourseRepository interface {
	List(ctx context.Context, page, pageSize int) ([]model.Course, int64, error)
	GetByID(ctx context.Context, id uint) (*model.Course, error)
	GetByIDs(ctx context.Context, ids []uint) ([]model.Course, error)
	Create(ctx context.Context, course *model.Course) error
	Update(ctx context.Context, course *model.Course) error
	Delete(ctx context.Context, id uint) error
}

type courseRepository struct {
	db *gorm.DB
}

// NewCourseRepository constructs a course repository.
func NewCourseRepository(db *gorm.DB) CourseRepository {
	return &courseRepository{db: db}
}

func (r *courseRepository) List(ctx context.Context, page, pageSize int) ([]model.Course, int64, error) {
	var items []model.Course
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Course{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count courses: %w", err)
	}
	if err := paginate(r.db.WithContext(ctx).Model(&model.Course{}), page, pageSize).
		Order("id ASC").Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list courses: %w", err)
	}
	return items, total, nil
}

func (r *courseRepository) GetByID(ctx context.Context, id uint) (*model.Course, error) {
	var item model.Course
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, normalizeError(err)
	}
	return &item, nil
}

func (r *courseRepository) GetByIDs(ctx context.Context, ids []uint) ([]model.Course, error) {
	var items []model.Course
	if len(ids) == 0 {
		return items, nil
	}
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("get courses by ids: %w", err)
	}
	return items, nil
}

func (r *courseRepository) Create(ctx context.Context, course *model.Course) error {
	if err := r.db.WithContext(ctx).Create(course).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("create course: %w", err)
	}
	return nil
}

func (r *courseRepository) Update(ctx context.Context, course *model.Course) error {
	if err := r.db.WithContext(ctx).Save(course).Error; err != nil {
		if isConstraintError(err) {
			return ErrConstraint
		}
		return fmt.Errorf("update course: %w", err)
	}
	return nil
}

func (r *courseRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.Course{}, id).Error; err != nil {
		return fmt.Errorf("delete course: %w", err)
	}
	return nil
}
