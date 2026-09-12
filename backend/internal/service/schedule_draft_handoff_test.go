package service_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/service"
)

// handoffPlanner blocks the first draft (until it is canceled) and completes
// every subsequent draft immediately, which lets a test prove that canceling
// the running draft frees the worker for the next queued one promptly.
type handoffPlanner struct {
	calls atomic.Int64
}

func (p *handoffPlanner) BuildDraft(ctx context.Context, req *dto.GenerateScheduleRequest, onProgress service.DraftProgressCallback) ([]model.Schedule, []dto.ConflictResponse, int, error) {
	n := p.calls.Add(1)
	if n == 1 {
		// Simulate a long single-week generation. Block until canceled.
		<-ctx.Done()
		return nil, nil, 0, ctx.Err()
	}
	if onProgress != nil {
		onProgress(service.DraftProgress{Week: 1, TotalWeeks: 1, CurrentStep: "week 1/1"})
	}
	return []model.Schedule{{Week: 1}}, nil, 1, nil
}

func (p *handoffPlanner) BuildDraftResponses(ctx context.Context, plans []model.Schedule) ([]dto.ScheduleResponse, error) {
	return []dto.ScheduleResponse{{ID: 1, Week: 1}}, nil
}

func (p *handoffPlanner) DetectConflicts(ctx context.Context, items []model.Schedule) []dto.ConflictResponse {
	return nil
}

// TestDraftCancelFreesWorkerForNextDraft verifies that after canceling the
// running draft, the next queued draft starts immediately (worker handoff),
// rather than waiting for the idle poll interval or being blocked by the
// canceled task.
func TestDraftCancelFreesWorkerForNextDraft(t *testing.T) {
	db := newDraftDB(t)
	planner := &handoffPlanner{}
	svc := newDraftService(t, db, planner)

	// Job A: claimed and blocked inside BuildDraft.
	a, err := svc.Submit(context.Background(), validDraftRequest())
	if err != nil {
		t.Fatalf("submit a: %v", err)
	}
	waitForDraftStatus(t, svc, a.ID, model.DraftStatusRunning)

	// Job B: queued behind A.
	b, err := svc.Submit(context.Background(), validDraftRequest())
	if err != nil {
		t.Fatalf("submit b: %v", err)
	}
	// Give the worker a moment; with a single worker B must remain queued.
	time.Sleep(50 * time.Millisecond)
	if got, _ := svc.Get(context.Background(), b.ID); got.Status != model.DraftStatusQueued {
		t.Fatalf("expected B queued while A runs, got %s", got.Status)
	}

	// Cancel A and time how quickly B takes over.
	cancelStart := time.Now()
	canceled, err := svc.Cancel(context.Background(), a.ID)
	if err != nil {
		t.Fatalf("cancel a: %v", err)
	}
	cancelReturnedIn := time.Since(cancelStart)
	if canceled.Status != model.DraftStatusCanceled {
		t.Fatalf("expected A canceled, got %s", canceled.Status)
	}
	if cancelReturnedIn > 200*time.Millisecond {
		t.Fatalf("Cancel request did not return promptly: %s", cancelReturnedIn)
	}

	// A must stay canceled (no late failed/succeeded write).
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		got, _ := svc.Get(context.Background(), a.ID)
		if got.Status != model.DraftStatusCanceled {
			t.Fatalf("A changed after cancel: %s", got.Status)
		}
		if got.FailReason != "" {
			t.Fatalf("A got a failure reason after cancel: %q", got.FailReason)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// B must reach success promptly via worker handoff.
	bDone := time.Now()
	waitForDraftStatus(t, svc, b.ID, model.DraftStatusSucceeded)
	handoffIn := time.Since(bDone)
	if handoffIn > 500*time.Millisecond {
		t.Fatalf("next draft did not start promptly after cancel: %s", handoffIn)
	}
	if planner.calls.Load() < 2 {
		t.Fatalf("expected the planner to execute the second draft, calls=%d", planner.calls.Load())
	}
}

// TestDraftCanceledPlannerErrorDoesNotBecomeFailure confirms a planner that
// returns a plain error after cancellation cannot turn the draft into failed.
type canceledErrorPlanner struct{}

func (canceledErrorPlanner) BuildDraft(ctx context.Context, req *dto.GenerateScheduleRequest, onProgress service.DraftProgressCallback) ([]model.Schedule, []dto.ConflictResponse, int, error) {
	<-ctx.Done()
	// Even if the planner returns a wrapped non-context error while the job
	// context is canceled, the canceled terminal state must win.
	return nil, nil, 0, errors.New("boom after cancel: " + ctx.Err().Error())
}

func (canceledErrorPlanner) BuildDraftResponses(context.Context, []model.Schedule) ([]dto.ScheduleResponse, error) {
	return nil, nil
}

func (canceledErrorPlanner) DetectConflicts(context.Context, []model.Schedule) []dto.ConflictResponse {
	return nil
}

func TestDraftCanceledStateWinsOverPlannerError(t *testing.T) {
	db := newDraftDB(t)
	svc := newDraftService(t, db, canceledErrorPlanner{})

	created, err := svc.Submit(context.Background(), validDraftRequest())
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	waitForDraftStatus(t, svc, created.ID, model.DraftStatusRunning)

	canceled, err := svc.Cancel(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if canceled.Status != model.DraftStatusCanceled {
		t.Fatalf("expected canceled, got %s", canceled.Status)
	}

	// Wait beyond any unwinding write; status must remain canceled, never failed.
	time.Sleep(200 * time.Millisecond)
	got, _ := svc.Get(context.Background(), created.ID)
	if got.Status != model.DraftStatusCanceled {
		t.Fatalf("expected canceled terminal state, got %s (fail_reason=%q)", got.Status, got.FailReason)
	}
}
