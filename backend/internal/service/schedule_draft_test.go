package service_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/gbschedule/gbschedule/internal/constants"
	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/repository"
	"github.com/gbschedule/gbschedule/internal/service"
)

// fakePlanner is a controllable service.DraftPlanner used to drive the draft
// state machine without running the real scheduling algorithm.
type fakePlanner struct {
	mu        sync.Mutex
	plans     []model.Schedule
	err       error
	started   chan struct{}
	release   chan struct{}
	progressN int
	failAfter int
}

func newFakePlanner() *fakePlanner {
	return &fakePlanner{started: make(chan struct{}, 8), release: make(chan struct{})}
}

func (f *fakePlanner) BuildDraft(ctx context.Context, req *dto.GenerateScheduleRequest, onProgress service.DraftProgressCallback) ([]model.Schedule, []dto.ConflictResponse, int, error) {
	f.mu.Lock()
	f.started <- struct{}{}
	failAfter := f.failAfter
	f.mu.Unlock()

	if onProgress != nil && req.Weeks > 0 {
		for week := 1; week <= req.Weeks; week++ {
			if failAfter > 0 && week > failAfter {
				return nil, nil, 0, errors.New("synthetic planner failure")
			}
			onProgress(service.DraftProgress{Week: week, TotalWeeks: req.Weeks, CurrentStep: fmt.Sprintf("week %d/%d", week, req.Weeks), Placed: week})
			f.mu.Lock()
			f.progressN++
			f.mu.Unlock()
		}
	}

	select {
	case <-f.release:
	case <-ctx.Done():
		return nil, nil, 0, ctx.Err()
	}
	if f.err != nil {
		return nil, nil, 0, f.err
	}
	return f.plans, nil, len(f.plans), nil
}

func (f *fakePlanner) BuildDraftResponses(ctx context.Context, plans []model.Schedule) ([]dto.ScheduleResponse, error) {
	out := make([]dto.ScheduleResponse, 0, len(plans))
	for i := range plans {
		out = append(out, dto.ScheduleResponse{ID: uint(i + 1), Week: plans[i].Week})
	}
	return out, nil
}

func (f *fakePlanner) DetectConflicts(ctx context.Context, items []model.Schedule) []dto.ConflictResponse {
	return []dto.ConflictResponse{{Type: constants.ConflictTeacherTime, Week: 1, Suggestion: "test conflict"}}
}

func TestMain(m *testing.M) {
	// Make the background worker claim queued drafts promptly in tests.
	service.DraftPollInterval = 10 * time.Millisecond
	os.Exit(m.Run())
}

func newDraftService(t *testing.T, db *gorm.DB, planner service.DraftPlanner) service.ScheduleDraftService {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewScheduleDraftService(repository.NewScheduleDraftRepository(db), planner, logger)
	svc.Start(context.Background())
	t.Cleanup(svc.Shutdown)
	return svc
}

var draftDBSeq atomic.Int64

func newDraftDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:draftsvc_%d_%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", os.Getpid(), draftDBSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ScheduleDraft{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

func validDraftRequest() *dto.GenerateScheduleRequest {
	return &dto.GenerateScheduleRequest{
		Weeks: 1, DaysPerWeek: 1, PeriodsPerDay: 1,
		Courses: []dto.CourseRequirement{{CourseID: 1, WeeklyPeriods: 1}},
	}
}

func waitForDraftStatus(t *testing.T, svc service.ScheduleDraftService, id uint, want string) *dto.DraftResponse {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := svc.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("get draft: %v", err)
		}
		if got.Status == want {
			return got
		}
		if (want != model.DraftStatusRunning) && (got.Status == model.DraftStatusSucceeded || got.Status == model.DraftStatusFailed || got.Status == model.DraftStatusCanceled) && got.Status != want {
			t.Fatalf("draft reached %s while waiting for %s", got.Status, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, _ := svc.Get(context.Background(), id)
	t.Fatalf("draft %d did not reach %s within timeout, last=%+v", id, want, got)
	return nil
}

func TestDraftQueuedCancelNeverRuns(t *testing.T) {
	db := newDraftDB(t)
	planner := newFakePlanner()
	// Start a draft service WITHOUT the worker so the draft stays queued.
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewScheduleDraftService(repository.NewScheduleDraftRepository(db), planner, logger)

	created, err := svc.Submit(context.Background(), validDraftRequest())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if created.Status != model.DraftStatusQueued || created.ID == 0 {
		t.Fatalf("unexpected create response: %+v", created)
	}

	canceled, err := svc.Cancel(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if canceled.Status != model.DraftStatusCanceled {
		t.Fatalf("expected canceled, got %s", canceled.Status)
	}

	// Repeated cancel keeps the original terminal result.
	canceled2, err := svc.Cancel(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("repeat cancel: %v", err)
	}
	if canceled2.Status != model.DraftStatusCanceled || canceled2.FinishedAt != canceled.FinishedAt {
		t.Fatalf("repeat cancel changed result: %+v vs %+v", canceled, canceled2)
	}

	select {
	case <-planner.started:
		t.Fatal("planner must never execute a draft canceled while queued")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestDraftRunningCancelStopsGeneration(t *testing.T) {
	db := newDraftDB(t)
	planner := newFakePlanner()
	planner.plans = []model.Schedule{{Week: 1, DayOfWeek: 1}}
	svc := newDraftService(t, db, planner)

	created, err := svc.Submit(context.Background(), validDraftRequest())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Wait until the worker picked up the draft and the planner is blocked.
	select {
	case <-planner.started:
	case <-time.After(5 * time.Second):
		t.Fatal("planner never started")
	}

	running := waitForDraftStatus(t, svc, created.ID, model.DraftStatusRunning)
	if running.TotalSteps != 1 {
		t.Fatalf("expected total_steps=1, got %d", running.TotalSteps)
	}
	// The first progress write is throttled; wait until it is observable.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if running, _ = svc.Get(context.Background(), created.ID); running.Progress > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if running.Progress == 0 {
		t.Fatalf("expected progress to be reported while running, got %+v", running)
	}

	canceled, err := svc.Cancel(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if canceled.Status != model.DraftStatusCanceled {
		t.Fatalf("expected canceled, got %s", canceled.Status)
	}
	// Unblock the planner; it must observe cancellation and must not flip the
	// draft to succeeded/failed.
	close(planner.release)
	waitForDraftStatus(t, svc, created.ID, model.DraftStatusCanceled)

	// Repeated cancel cannot change the outcome.
	canceled2, err := svc.Cancel(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("repeat cancel: %v", err)
	}
	if canceled2.Status != model.DraftStatusCanceled {
		t.Fatalf("repeat cancel altered state: %s", canceled2.Status)
	}

	// A canceled draft has no result payload.
	if _, err := svc.GetResult(context.Background(), created.ID); !errors.Is(err, service.ErrConflict) {
		t.Fatalf("expected conflict error when reading canceled result, got %v", err)
	}
}

func TestDraftFailureRecordsReasonAndIsTerminal(t *testing.T) {
	db := newDraftDB(t)
	planner := newFakePlanner()
	planner.err = errors.New("boom: no teachers available")
	close(planner.release)
	svc := newDraftService(t, db, planner)

	created, err := svc.Submit(context.Background(), validDraftRequest())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	failed := waitForDraftStatus(t, svc, created.ID, model.DraftStatusFailed)
	if failed.FailReason == "" {
		t.Fatal("expected fail_reason to be populated")
	}
	if !strings.Contains(failed.FailReason, "boom") {
		t.Fatalf("unexpected fail reason: %s", failed.FailReason)
	}

	// A failed draft is terminal: cancel cannot rewrite it.
	again, err := svc.Cancel(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("cancel failed draft: %v", err)
	}
	if again.Status != model.DraftStatusFailed {
		t.Fatalf("cancel altered failed draft: %s", again.Status)
	}
}

func TestDraftSuccessStoresResultConflictsAndSummary(t *testing.T) {
	db := newDraftDB(t)
	planner := newFakePlanner()
	planner.plans = []model.Schedule{
		{Week: 1, DayOfWeek: 1, TimeSlotID: 1, ClassroomID: 1, TeacherID: 1, ClassID: 1, CourseID: 1},
		{Week: 1, DayOfWeek: 2, TimeSlotID: 2, ClassroomID: 2, TeacherID: 2, ClassID: 2, CourseID: 2},
	}
	close(planner.release)
	svc := newDraftService(t, db, planner)

	created, err := svc.Submit(context.Background(), validDraftRequest())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	succeeded := waitForDraftStatus(t, svc, created.ID, model.DraftStatusSucceeded)
	if succeeded.Progress != 100 {
		t.Fatalf("expected progress 100, got %d", succeeded.Progress)
	}
	if len(succeeded.Conflicts) != 1 {
		t.Fatalf("expected conflicts to be visible on the draft, got %+v", succeeded.Conflicts)
	}

	result, err := svc.GetResult(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get result: %v", err)
	}
	if result.Result == nil || len(result.Result.Schedules) != 2 {
		t.Fatalf("expected full timetable in result: %+v", result.Result)
	}
	if result.Result.Summary.Generated != 2 || result.Result.Summary.ConflictCount != 1 {
		t.Fatalf("unexpected summary: %+v", result.Result.Summary)
	}
	if result.Result.Summary.ClassroomCount != 2 || result.Result.Summary.TeacherCount != 2 {
		t.Fatalf("unexpected usage counts: %+v", result.Result.Summary)
	}

	// Terminal draft is immutable to further cancel attempts.
	again, err := svc.Cancel(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("cancel succeeded draft: %v", err)
	}
	if again.Status != model.DraftStatusSucceeded {
		t.Fatalf("cancel altered succeeded draft: %s", again.Status)
	}
}

func TestDraftListNewestFirstAndPaginated(t *testing.T) {
	db := newDraftDB(t)
	planner := newFakePlanner()
	close(planner.release)
	svc := newDraftService(t, db, planner)

	for range 3 {
		if _, err := svc.Submit(context.Background(), validDraftRequest()); err != nil {
			t.Fatalf("submit: %v", err)
		}
		time.Sleep(5 * time.Millisecond) // distinct created_at ordering
	}
	// Let all three reach a terminal state.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		items, total, err := svc.List(context.Background(), "", 1, 10)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total == 3 {
			allDone := true
			for _, item := range items {
				if item.Status != model.DraftStatusSucceeded {
					allDone = false
				}
			}
			if allDone {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	page1, total, err := svc.List(context.Background(), "", 1, 2)
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if total != 3 || len(page1) != 2 {
		t.Fatalf("unexpected page: total=%d len=%d", total, len(page1))
	}
	if page1[0].ID < page1[1].ID {
		t.Fatalf("expected newest first, got ids %d, %d", page1[0].ID, page1[1].ID)
	}
	page2, _, err := svc.List(context.Background(), "", 2, 2)
	if err != nil || len(page2) != 1 {
		t.Fatalf("list page 2: len=%d err=%v", len(page2), err)
	}

	if _, _, err := svc.List(context.Background(), "bogus", 1, 10); !errors.Is(err, service.ErrInvalid) {
		t.Fatalf("expected invalid status error, got %v", err)
	}
}

func TestDraftRestartRecoversRunningAndKeepsResults(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "drafts.db")
	openFileDB := func(t *testing.T) *gorm.DB {
		t.Helper()
		db, err := gorm.Open(sqlite.Open(tmp+"?_pragma=busy_timeout(5000)"), &gorm.Config{})
		if err != nil {
			t.Fatalf("open file db: %v", err)
		}
		if err := db.AutoMigrate(&model.ScheduleDraft{}); err != nil {
			t.Fatalf("migrate: %v", err)
		}
		return db
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// First process: one draft succeeds, then a draft is interrupted mid-run.
	db1 := openFileDB(t)
	okPlanner := newFakePlanner()
	close(okPlanner.release)
	okPlanner.plans = []model.Schedule{{Week: 1}}
	svc1 := service.NewScheduleDraftService(repository.NewScheduleDraftRepository(db1), okPlanner, logger)
	svc1.Start(context.Background())

	created1, err := svc1.Submit(context.Background(), validDraftRequest())
	if err != nil {
		t.Fatalf("submit 1: %v", err)
	}
	waitForDraftStatus(t, svc1, created1.ID, model.DraftStatusSucceeded)

	// Simulate a draft interrupted by process death: insert a running row.
	interrupted := &model.ScheduleDraft{Status: model.DraftStatusRunning, Params: `{"weeks":1,"days_per_week":1,"periods_per_day":1,"courses":[]}`, Conflicts: "[]"}
	if err := db1.Create(interrupted).Error; err != nil {
		t.Fatalf("create interrupted draft: %v", err)
	}
	svc1.Shutdown()

	// Second process opens the same database file.
	db2 := openFileDB(t)
	svc2 := service.NewScheduleDraftService(repository.NewScheduleDraftRepository(db2), newFakePlanner(), logger)
	svc2.Start(context.Background())
	t.Cleanup(svc2.Shutdown)

	// Result of the successful draft must still be queryable.
	res1, err := svc2.GetResult(context.Background(), created1.ID)
	if err != nil {
		t.Fatalf("read old result after restart: %v", err)
	}
	if res1.Status != model.DraftStatusSucceeded || res1.Result == nil || len(res1.Result.Schedules) != 1 {
		t.Fatalf("old result lost after restart: %+v", res1)
	}

	failed := waitForDraftStatus(t, svc2, interrupted.ID, model.DraftStatusFailed)
	if !strings.Contains(failed.FailReason, "restart") {
		t.Fatalf("expected restart recovery reason, got %q", failed.FailReason)
	}

	// Sanity: db file really exists on disk.
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("db file missing: %v", err)
	}
}
