package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gbschedule/gbschedule/internal/model"
	"gorm.io/gorm"
)

// ErrConcurrentUpdate is returned when a conditional (compare-and-swap)
// update matches no row because the draft moved on concurrently.
var ErrConcurrentUpdate = errors.New("draft changed concurrently")

// DraftListFilter filters draft listings.
type DraftListFilter struct {
	Status string
}

// DraftProgressUpdate carries the mutable progress fields of a running draft.
type DraftProgressUpdate struct {
	Progress    int
	TotalSteps  int
	CurrentStep string
	Conflicts   string
}

// ScheduleDraftRepository defines persistence operations for schedule drafts.
type ScheduleDraftRepository interface {
	Create(ctx context.Context, draft *model.ScheduleDraft) error
	GetByID(ctx context.Context, id uint) (*model.ScheduleDraft, error)
	List(ctx context.Context, filter DraftListFilter, page, pageSize int) ([]model.ScheduleDraft, int64, error)

	// ClaimNext atomically moves the oldest queued draft into the running
	// state. It returns (nil, nil) when no queued draft exists.
	ClaimNext(ctx context.Context, startedAt time.Time) (*model.ScheduleDraft, error)

	// UpdateProgress stores progress for a draft that is still running.
	UpdateProgress(ctx context.Context, id uint, up DraftProgressUpdate) error

	// Succeed stores the final result of a draft. It only succeeds while the
	// draft is still running; a draft canceled in the meantime is left
	// untouched and ErrConcurrentUpdate is returned.
	Succeed(ctx context.Context, id uint, resultJSON, conflictsJSON string, finishedAt time.Time) error

	// Fail marks a running draft as failed with a reason.
	Fail(ctx context.Context, id uint, reason string, finishedAt time.Time) error

	// RequestCancel moves queued/running drafts to canceled. It returns
	// whether the row was changed; canceling a draft already in a terminal
	// state yields false without touching the stored result.
	RequestCancel(ctx context.Context, id uint, finishedAt time.Time) (bool, error)

	// IsCanceled reports whether the draft is in the canceled state.
	IsCanceled(ctx context.Context, id uint) (bool, error)

	// RecoverInterrupted marks drafts left in running state by a previous
	// process (e.g. after a server restart) as failed.
	RecoverInterrupted(ctx context.Context, reason string, finishedAt time.Time) (int64, error)
}

type scheduleDraftRepository struct {
	db *gorm.DB
}

// NewScheduleDraftRepository constructs a schedule draft repository.
func NewScheduleDraftRepository(db *gorm.DB) ScheduleDraftRepository {
	return &scheduleDraftRepository{db: db}
}

func (r *scheduleDraftRepository) Create(ctx context.Context, draft *model.ScheduleDraft) error {
	if err := r.db.WithContext(ctx).Create(draft).Error; err != nil {
		return fmt.Errorf("create schedule draft: %w", err)
	}
	return nil
}

func (r *scheduleDraftRepository) GetByID(ctx context.Context, id uint) (*model.ScheduleDraft, error) {
	var item model.ScheduleDraft
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, normalizeError(err)
	}
	return &item, nil
}

func (r *scheduleDraftRepository) List(ctx context.Context, filter DraftListFilter, page, pageSize int) ([]model.ScheduleDraft, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ScheduleDraft{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count schedule drafts: %w", err)
	}
	var items []model.ScheduleDraft
	if err := query.
		Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list schedule drafts: %w", err)
	}
	return items, total, nil
}

func (r *scheduleDraftRepository) ClaimNext(ctx context.Context, startedAt time.Time) (*model.ScheduleDraft, error) {
	var draft model.ScheduleDraft
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidates []model.ScheduleDraft
		qerr := tx.
			Where("status = ?", model.DraftStatusQueued).
			Order("created_at ASC, id ASC").
			Limit(1).
			Find(&candidates).Error
		if qerr != nil {
			return fmt.Errorf("claim next draft: %w", qerr)
		}
		if len(candidates) == 0 {
			return nil
		}
		draft = candidates[0]
		res := tx.Model(&model.ScheduleDraft{}).
			Where("id = ? AND status = ?", draft.ID, model.DraftStatusQueued).
			Updates(map[string]any{
				"status":     model.DraftStatusRunning,
				"started_at": startedAt,
			})
		if res.Error != nil {
			return fmt.Errorf("claim next draft: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			// Another worker claimed it first; nothing to do in this poll.
			draft = model.ScheduleDraft{}
			return nil
		}
		draft.Status = model.DraftStatusRunning
		draft.StartedAt = &startedAt
		return nil
	})
	if err != nil {
		return nil, err
	}
	if draft.ID == 0 {
		return nil, nil
	}
	return &draft, nil
}

func (r *scheduleDraftRepository) UpdateProgress(ctx context.Context, id uint, up DraftProgressUpdate) error {
	res := r.db.WithContext(ctx).Model(&model.ScheduleDraft{}).
		Where("id = ? AND status = ?", id, model.DraftStatusRunning).
		Updates(map[string]any{
			"progress":     up.Progress,
			"total_steps":  up.TotalSteps,
			"current_step": up.CurrentStep,
			"conflicts":    up.Conflicts,
		})
	if res.Error != nil {
		return fmt.Errorf("update draft progress: %w", res.Error)
	}
	// Zero rows is not an error: the draft may have been canceled meanwhile.
	return nil
}

func (r *scheduleDraftRepository) Succeed(ctx context.Context, id uint, resultJSON, conflictsJSON string, finishedAt time.Time) error {
	res := r.db.WithContext(ctx).Model(&model.ScheduleDraft{}).
		Where("id = ? AND status = ?", id, model.DraftStatusRunning).
		Updates(map[string]any{
			"status":       model.DraftStatusSucceeded,
			"result_json":  resultJSON,
			"conflicts":    conflictsJSON,
			"progress":     100,
			"current_step": "",
			"fail_reason":  "",
			"finished_at":  finishedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("succeed draft: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrConcurrentUpdate
	}
	return nil
}

func (r *scheduleDraftRepository) Fail(ctx context.Context, id uint, reason string, finishedAt time.Time) error {
	res := r.db.WithContext(ctx).Model(&model.ScheduleDraft{}).
		Where("id = ? AND status = ?", id, model.DraftStatusRunning).
		Updates(map[string]any{
			"status":       model.DraftStatusFailed,
			"fail_reason":  reason,
			"current_step": "",
			"finished_at":  finishedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("fail draft: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrConcurrentUpdate
	}
	return nil
}

func (r *scheduleDraftRepository) RequestCancel(ctx context.Context, id uint, finishedAt time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&model.ScheduleDraft{}).
		Where("id = ? AND status IN ?", id, []string{model.DraftStatusQueued, model.DraftStatusRunning}).
		Updates(map[string]any{
			"status":      model.DraftStatusCanceled,
			"finished_at": finishedAt,
		})
	if res.Error != nil {
		return false, fmt.Errorf("cancel draft: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *scheduleDraftRepository) IsCanceled(ctx context.Context, id uint) (bool, error) {
	var status string
	if err := r.db.WithContext(ctx).Model(&model.ScheduleDraft{}).
		Select("status").Where("id = ?", id).Scan(&status).Error; err != nil {
		return false, fmt.Errorf("check draft canceled: %w", err)
	}
	return status == model.DraftStatusCanceled, nil
}

func (r *scheduleDraftRepository) RecoverInterrupted(ctx context.Context, reason string, finishedAt time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.ScheduleDraft{}).
		Where("status = ?", model.DraftStatusRunning).
		Updates(map[string]any{
			"status":      model.DraftStatusFailed,
			"fail_reason": reason,
			"finished_at": finishedAt,
		})
	if res.Error != nil {
		return 0, fmt.Errorf("recover interrupted drafts: %w", res.Error)
	}
	return res.RowsAffected, nil
}
