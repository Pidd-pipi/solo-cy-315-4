package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/repository"
)

// DraftPlanner is the scheduling capability the draft worker needs. It is a
// narrow view of ScheduleService so draft generation never touches the
// published timetable.
type DraftPlanner interface {
	BuildDraft(ctx context.Context, req *dto.GenerateScheduleRequest, onProgress DraftProgressCallback) ([]model.Schedule, []dto.ConflictResponse, int, error)
	BuildDraftResponses(ctx context.Context, plans []model.Schedule) ([]dto.ScheduleResponse, error)
	DetectConflicts(ctx context.Context, items []model.Schedule) []dto.ConflictResponse
}

// DraftPollInterval controls how often the background worker claims queued
// drafts. It is a variable so deployments (and tests) may tune it.
var DraftPollInterval = 200 * time.Millisecond

// ScheduleDraftService manages asynchronous timetable generation drafts.
type ScheduleDraftService interface {
	Submit(ctx context.Context, req *dto.GenerateScheduleRequest) (*dto.CreateDraftResponse, error)
	Get(ctx context.Context, id uint) (*dto.DraftResponse, error)
	GetResult(ctx context.Context, id uint) (*dto.DraftResultResponse, error)
	List(ctx context.Context, status string, page, pageSize int) ([]dto.DraftResponse, int64, error)
	// Cancel requests cancellation. It is idempotent: canceling a draft
	// already in a terminal state returns its current state unchanged.
	Cancel(ctx context.Context, id uint) (*dto.DraftResponse, error)
	// Start launches the background worker and recovers tasks interrupted by
	// a previous process.
	Start(ctx context.Context)
	// Shutdown stops the background worker.
	Shutdown()
}

type scheduleDraftService struct {
	drafts  repository.ScheduleDraftRepository
	planner DraftPlanner
	logger  *slog.Logger

	wg      sync.WaitGroup
	stop    chan struct{}
	stopped bool
	mu      sync.Mutex

	// cancels holds cancellation funcs for drafts currently executing in
	// this process, keyed by draft id.
	cancels map[uint]context.CancelFunc
}

// NewScheduleDraftService constructs a schedule draft service.
func NewScheduleDraftService(drafts repository.ScheduleDraftRepository, planner DraftPlanner, logger *slog.Logger) ScheduleDraftService {
	return &scheduleDraftService{
		drafts:  drafts,
		planner: planner,
		logger:  logger,
		stop:    make(chan struct{}),
		cancels: map[uint]context.CancelFunc{},
	}
}

// Submit persists the conditions and queues background generation. The draft
// id is returned immediately; generation itself happens in the worker.
func (s *scheduleDraftService) Submit(ctx context.Context, req *dto.GenerateScheduleRequest) (*dto.CreateDraftResponse, error) {
	params, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal draft params: %w", err)
	}
	totalSteps := req.Weeks
	draft := &model.ScheduleDraft{
		Status:     model.DraftStatusQueued,
		Params:     string(params),
		Progress:   0,
		TotalSteps: totalSteps,
		Conflicts:  "[]",
	}
	if err := s.drafts.Create(ctx, draft); err != nil {
		return nil, mapWriteError("create draft", err)
	}
	return &dto.CreateDraftResponse{ID: draft.ID, Status: dto.DraftStatusQueued}, nil
}

func (s *scheduleDraftService) Get(ctx context.Context, id uint) (*dto.DraftResponse, error) {
	draft, err := s.drafts.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get draft: %w", err)
	}
	return toDraftResponse(draft)
}

func (s *scheduleDraftService) GetResult(ctx context.Context, id uint) (*dto.DraftResultResponse, error) {
	draft, err := s.drafts.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get draft: %w", err)
	}
	resp, err := toDraftResponse(draft)
	if err != nil {
		return nil, err
	}
	out := &dto.DraftResultResponse{DraftResponse: *resp}
	if draft.Status != model.DraftStatusSucceeded {
		// The immutable result only exists for successful drafts.
		return out, fmt.Errorf("get draft result: %w", ErrConflict)
	}
	var result dto.DraftResultData
	if err := json.Unmarshal([]byte(draft.ResultJSON), &result); err != nil {
		return nil, fmt.Errorf("decode draft result: %w", err)
	}
	out.Result = &result
	return out, nil
}

func (s *scheduleDraftService) List(ctx context.Context, status string, page, pageSize int) ([]dto.DraftResponse, int64, error) {
	if status != "" && !isValidDraftStatus(status) {
		return nil, 0, ErrInvalid
	}
	items, total, err := s.drafts.List(ctx, repository.DraftListFilter{Status: status}, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list drafts: %w", err)
	}
	out := make([]dto.DraftResponse, 0, len(items))
	for i := range items {
		resp, err := toDraftResponse(&items[i])
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *resp)
	}
	return out, total, nil
}

func (s *scheduleDraftService) Cancel(ctx context.Context, id uint) (*dto.DraftResponse, error) {
	draft, err := s.drafts.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get draft for cancel: %w", err)
	}
	if draft.IsTerminal() {
		// Idempotent: a terminal state (including a previous cancel) is
		// reported back unchanged.
		return toDraftResponse(draft)
	}
	if _, err := s.drafts.RequestCancel(ctx, id, time.Now()); err != nil {
		return nil, fmt.Errorf("cancel draft: %w", err)
	}
	s.cancelLocal(id)

	updated, err := s.drafts.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("reload draft after cancel: %w", err)
	}
	return toDraftResponse(updated)
}

// Start recovers drafts interrupted by a restart and launches the worker loop.
func (s *scheduleDraftService) Start(ctx context.Context) {
	s.mu.Lock()
	if s.stopped {
		s.stop = make(chan struct{})
		s.stopped = false
	}
	s.mu.Unlock()

	if n, err := s.drafts.RecoverInterrupted(ctx, "server restarted before generation finished", time.Now()); err != nil {
		s.logger.Error("recover interrupted drafts", slog.String("error", err.Error()))
	} else if n > 0 {
		s.logger.Info("marked interrupted drafts as failed", slog.Int64("count", n))
	}

	s.wg.Add(1)
	go s.workerLoop()
}

func (s *scheduleDraftService) Shutdown() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	close(s.stop)
	cancels := make([]context.CancelFunc, 0, len(s.cancels))
	for _, cancel := range s.cancels {
		cancels = append(cancels, cancel)
	}
	s.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	s.wg.Wait()
}

func (s *scheduleDraftService) workerLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(DraftPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
		}
		// A single in-process worker serializes SQLite writes and keeps
		// ordering equivalent to the queued-at timestamps.
		draft, err := s.drafts.ClaimNext(context.Background(), time.Now())
		if err != nil {
			s.logger.Error("claim draft", slog.String("error", err.Error()))
			continue
		}
		if draft == nil {
			continue
		}
		s.execute(draft)
	}
}

func (s *scheduleDraftService) execute(draft *model.ScheduleDraft) {
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancels[draft.ID] = cancel
	s.mu.Unlock()
	defer func() {
		cancel()
		s.cancelLocal(draft.ID)
	}()

	var req dto.GenerateScheduleRequest
	if err := json.Unmarshal([]byte(draft.Params), &req); err != nil {
		s.failDraft(draft.ID, fmt.Sprintf("invalid stored draft parameters: %v", err))
		return
	}

	lastPersist := time.Time{}

	plans, placementConflicts, required, err := s.planner.BuildDraft(ctx, &req, func(p DraftProgress) {
		if p.TotalWeeks <= 0 {
			return
		}
		progress := p.Week * 100 / p.TotalWeeks
		if progress > 99 {
			progress = 99
		}
		// Throttle progress writes: at most one per second per draft, but
		// always persist the first callback so progress is visible promptly.
		if !lastPersist.IsZero() && time.Since(lastPersist) < time.Second {
			return
		}
		lastPersist = time.Now()
		s.persistProgress(draft.ID, progress, p.TotalWeeks, p.CurrentStep, p.Conflicts)
	})
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(context.Cause(ctx), context.Canceled) {
			// Cancellation wins: the draft is already in canceled state and
			// must never flip back to failed.
			s.logger.Info("draft canceled during generation", slog.Uint64("draft_id", uint64(draft.ID)))
			return
		}
		s.failDraft(draft.ID, err.Error())
		return
	}

	// A cancel arriving between the last progress write and completion is
	// honored: the context is checked through BuildDraft, but re-confirm here
	// so a result can never overwrite the canceled terminal state.
	canceled, cerr := s.drafts.IsCanceled(ctx, draft.ID)
	if cerr != nil {
		s.failDraft(draft.ID, cerr.Error())
		return
	}
	if canceled {
		return
	}

	detected := s.planner.DetectConflicts(ctx, plans)
	allConflicts := append(append([]dto.ConflictResponse{}, placementConflicts...), detected...)
	responses, err := s.planner.BuildDraftResponses(ctx, plans)
	if err != nil {
		s.failDraft(draft.ID, err.Error())
		return
	}
	result := dto.DraftResultData{
		Schedules: responses,
		Conflicts: allConflicts,
		Summary:   buildDraftSummary(&req, plans, allConflicts, required),
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		s.failDraft(draft.ID, fmt.Sprintf("encode draft result: %v", err))
		return
	}
	conflictsJSON, err := json.Marshal(allConflicts)
	if err != nil {
		s.failDraft(draft.ID, fmt.Sprintf("encode draft conflicts: %v", err))
		return
	}
	if err := s.drafts.Succeed(ctx, draft.ID, string(resultJSON), string(conflictsJSON), time.Now()); err != nil {
		if errors.Is(err, repository.ErrConcurrentUpdate) {
			// The draft was canceled while we finished; leave it canceled.
			return
		}
		s.logger.Error("store draft result", slog.Uint64("draft_id", uint64(draft.ID)), slog.String("error", err.Error()))
	}
}

func (s *scheduleDraftService) persistProgress(id uint, progress, totalSteps int, step string, conflicts []dto.ConflictResponse) {
	payload, err := json.Marshal(conflicts)
	if err != nil {
		payload = []byte("[]")
	}
	if err := s.drafts.UpdateProgress(context.Background(), id, repository.DraftProgressUpdate{
		Progress:    progress,
		TotalSteps:  totalSteps,
		CurrentStep: step,
		Conflicts:   string(payload),
	}); err != nil {
		s.logger.Error("persist draft progress", slog.Uint64("draft_id", uint64(id)), slog.String("error", err.Error()))
	}
}

func (s *scheduleDraftService) failDraft(id uint, reason string) {
	if err := s.drafts.Fail(context.Background(), id, reason, time.Now()); err != nil {
		if errors.Is(err, repository.ErrConcurrentUpdate) {
			return
		}
		s.logger.Error("mark draft failed", slog.Uint64("draft_id", uint64(id)), slog.String("error", err.Error()))
	}
}

func (s *scheduleDraftService) cancelLocal(id uint) {
	s.mu.Lock()
	cancel, ok := s.cancels[id]
	if ok {
		delete(s.cancels, id)
	}
	s.mu.Unlock()
	if ok {
		cancel()
	}
}

func isValidDraftStatus(status string) bool {
	switch status {
	case dto.DraftStatusQueued, dto.DraftStatusRunning, dto.DraftStatusSucceeded, dto.DraftStatusFailed, dto.DraftStatusCanceled:
		return true
	default:
		return false
	}
}

func toDraftResponse(draft *model.ScheduleDraft) (*dto.DraftResponse, error) {
	var params dto.GenerateScheduleRequest
	if err := json.Unmarshal([]byte(draft.Params), &params); err != nil {
		return nil, fmt.Errorf("decode draft params: %w", err)
	}
	conflicts := []dto.ConflictResponse{}
	if draft.Conflicts != "" {
		if err := json.Unmarshal([]byte(draft.Conflicts), &conflicts); err != nil {
			return nil, fmt.Errorf("decode draft conflicts: %w", err)
		}
	}
	resp := &dto.DraftResponse{
		ID:          draft.ID,
		Status:      draft.Status,
		Params:      params,
		Progress:    draft.Progress,
		TotalSteps:  draft.TotalSteps,
		CurrentStep: draft.CurrentStep,
		Conflicts:   conflicts,
		FailReason:  draft.FailReason,
		CreatedAt:   draft.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   draft.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if draft.StartedAt != nil {
		resp.StartedAt = draft.StartedAt.Format("2006-01-02 15:04:05")
	}
	if draft.FinishedAt != nil {
		resp.FinishedAt = draft.FinishedAt.Format("2006-01-02 15:04:05")
	}
	return resp, nil
}

func buildDraftSummary(req *dto.GenerateScheduleRequest, plans []model.Schedule, conflicts []dto.ConflictResponse, required int) dto.DraftSummary {
	summary := dto.DraftSummary{
		Generated:     len(plans),
		Required:      required,
		Unplaced:      0,
		ConflictCount: len(conflicts),
		Weeks:         req.Weeks,
		DaysPerWeek:   req.DaysPerWeek,
		PeriodsPerDay: req.PeriodsPerDay,
	}
	if required > len(plans) {
		summary.Unplaced = required - len(plans)
	}
	classrooms := map[uint]struct{}{}
	teachers := map[uint]struct{}{}
	classes := map[uint]struct{}{}
	courses := map[uint]struct{}{}
	for _, p := range plans {
		classrooms[p.ClassroomID] = struct{}{}
		teachers[p.TeacherID] = struct{}{}
		classes[p.ClassID] = struct{}{}
		courses[p.CourseID] = struct{}{}
	}
	summary.ClassroomCount = len(classrooms)
	summary.TeacherCount = len(teachers)
	summary.ClassCount = len(classes)
	summary.CourseCount = len(courses)
	return summary
}
