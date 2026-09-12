package repository_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/repository"
)

var draftRepoDBSeq atomic.Int64

func newDraftTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:draftrepo_%d_%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", 0, draftRepoDBSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ScheduleDraft{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

func createDraftRow(t *testing.T, db *gorm.DB, status string) *model.ScheduleDraft {
	t.Helper()
	draft := &model.ScheduleDraft{Status: status, Params: `{"weeks":1}`, Conflicts: "[]"}
	if err := db.Create(draft).Error; err != nil {
		t.Fatalf("create draft: %v", err)
	}
	return draft
}

func TestScheduleDraftClaimNext(t *testing.T) {
	ctx := context.Background()
	db := newDraftTestDB(t)
	repo := repository.NewScheduleDraftRepository(db)

	if got, err := repo.ClaimNext(ctx, time.Now()); err != nil {
		t.Fatalf("claim empty queue: %v", err)
	} else if got != nil {
		t.Fatalf("expected nil claim, got %+v", got)
	}

	first := createDraftRow(t, db, model.DraftStatusQueued)
	second := createDraftRow(t, db, model.DraftStatusQueued)

	claimed, err := repo.ClaimNext(ctx, time.Now())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed == nil || claimed.ID != first.ID {
		t.Fatalf("expected oldest draft %d to be claimed, got %+v", first.ID, claimed)
	}
	if claimed.Status != model.DraftStatusRunning {
		t.Fatalf("expected running status, got %s", claimed.Status)
	}

	// Second claim must skip the already running draft and pick the next queued one.
	claimed2, err := repo.ClaimNext(ctx, time.Now())
	if err != nil {
		t.Fatalf("claim second: %v", err)
	}
	if claimed2 == nil || claimed2.ID != second.ID {
		t.Fatalf("expected draft %d to be claimed, got %+v", second.ID, claimed2)
	}
}

func TestScheduleDraftTerminalTransitions(t *testing.T) {
	ctx := context.Background()
	db := newDraftTestDB(t)
	repo := repository.NewScheduleDraftRepository(db)
	now := time.Now()

	t.Run("succeed only from running", func(t *testing.T) {
		queued := createDraftRow(t, db, model.DraftStatusQueued)
		if err := repo.Succeed(ctx, queued.ID, "{}", "[]", now); !errors.Is(err, repository.ErrConcurrentUpdate) {
			t.Fatalf("expected concurrent update error on queued draft, got %v", err)
		}

		running := createDraftRow(t, db, model.DraftStatusRunning)
		if err := repo.Succeed(ctx, running.ID, `{"schedules":[]}`, "[]", now); err != nil {
			t.Fatalf("succeed: %v", err)
		}
		got, err := repo.GetByID(ctx, running.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if got.Status != model.DraftStatusSucceeded || got.ResultJSON == "" {
			t.Fatalf("unexpected draft after success: %+v", got)
		}
		// A terminal draft must not change anymore.
		if err := repo.Succeed(ctx, running.ID, `{"tampered":true}`, "[]", now); !errors.Is(err, repository.ErrConcurrentUpdate) {
			t.Fatalf("expected error on repeated succeed, got %v", err)
		}
		if err := repo.Fail(ctx, running.ID, "late failure", now); !errors.Is(err, repository.ErrConcurrentUpdate) {
			t.Fatalf("expected error on fail after success, got %v", err)
		}
		got2, _ := repo.GetByID(ctx, running.ID)
		if got2.Status != model.DraftStatusSucceeded || got2.ResultJSON != `{"schedules":[]}` {
			t.Fatalf("terminal draft was mutated: %+v", got2)
		}
	})

	t.Run("fail records reason and is terminal", func(t *testing.T) {
		running := createDraftRow(t, db, model.DraftStatusRunning)
		if err := repo.Fail(ctx, running.ID, "boom", now); err != nil {
			t.Fatalf("fail: %v", err)
		}
		got, _ := repo.GetByID(ctx, running.ID)
		if got.Status != model.DraftStatusFailed || got.FailReason != "boom" {
			t.Fatalf("unexpected failed draft: %+v", got)
		}
		if err := repo.Succeed(ctx, running.ID, "{}", "[]", now); !errors.Is(err, repository.ErrConcurrentUpdate) {
			t.Fatalf("succeed after fail should be rejected, got %v", err)
		}
		if changed, err := repo.RequestCancel(ctx, running.ID, now); err != nil || changed {
			t.Fatalf("cancel of failed draft must be a no-op, changed=%v err=%v", changed, err)
		}
		got2, _ := repo.GetByID(ctx, running.ID)
		if got2.Status != model.DraftStatusFailed {
			t.Fatalf("failed draft changed after cancel: %s", got2.Status)
		}
	})

	t.Run("cancel is idempotent", func(t *testing.T) {
		queued := createDraftRow(t, db, model.DraftStatusQueued)
		firstCancel := time.Now()
		changed, err := repo.RequestCancel(ctx, queued.ID, firstCancel)
		if err != nil || !changed {
			t.Fatalf("first cancel: changed=%v err=%v", changed, err)
		}
		// Repeated cancel must not change the stored (terminal) result.
		later := firstCancel.Add(time.Minute)
		changed2, err := repo.RequestCancel(ctx, queued.ID, later)
		if err != nil || changed2 {
			t.Fatalf("second cancel: changed=%v err=%v", changed2, err)
		}
		got, _ := repo.GetByID(ctx, queued.ID)
		if got.Status != model.DraftStatusCanceled {
			t.Fatalf("expected canceled, got %s", got.Status)
		}
		if got.FinishedAt == nil || !got.FinishedAt.Equal(firstCancel) {
			t.Fatalf("finished_at=%v must remain the first cancel timestamp %v", got.FinishedAt, firstCancel)
		}
	})

	t.Run("progress updates only while running", func(t *testing.T) {
		running := createDraftRow(t, db, model.DraftStatusRunning)
		if err := repo.UpdateProgress(ctx, running.ID, repository.DraftProgressUpdate{Progress: 50, TotalSteps: 4, CurrentStep: "week 2/4", Conflicts: "[]"}); err != nil {
			t.Fatalf("update progress: %v", err)
		}
		got, _ := repo.GetByID(ctx, running.ID)
		if got.Progress != 50 || got.CurrentStep != "week 2/4" {
			t.Fatalf("progress not persisted: %+v", got)
		}
		if err := repo.Succeed(ctx, running.ID, "{}", "[]", now); err != nil {
			t.Fatalf("succeed: %v", err)
		}
		// Progress writes after the terminal transition are silently ignored.
		if err := repo.UpdateProgress(ctx, running.ID, repository.DraftProgressUpdate{Progress: 99}); err != nil {
			t.Fatalf("late progress write: %v", err)
		}
		got2, _ := repo.GetByID(ctx, running.ID)
		if got2.Progress != 100 {
			t.Fatalf("terminal progress was mutated to %d", got2.Progress)
		}
	})
}

func TestScheduleDraftListOrderedAndPaginated(t *testing.T) {
	ctx := context.Background()
	db := newDraftTestDB(t)
	repo := repository.NewScheduleDraftRepository(db)

	for range 3 {
		createDraftRow(t, db, model.DraftStatusQueued)
	}
	if err := db.Create(&model.ScheduleDraft{Status: model.DraftStatusSucceeded, Params: `{}`, Conflicts: "[]"}).Error; err != nil {
		t.Fatal(err)
	}

	items, total, err := repo.List(ctx, repository.DraftListFilter{}, 1, 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 4 || len(items) != 2 {
		t.Fatalf("expected total=4 and 2 items, got total=%d len=%d", total, len(items))
	}
	// Newest first by id.
	if items[0].ID <= items[1].ID {
		t.Fatalf("expected descending order, got ids %d then %d", items[0].ID, items[1].ID)
	}

	itemsPage2, _, err := repo.List(ctx, repository.DraftListFilter{}, 2, 2)
	if err != nil || len(itemsPage2) != 2 {
		t.Fatalf("page 2: len=%d err=%v", len(itemsPage2), err)
	}
	if items[0].ID == itemsPage2[0].ID {
		t.Fatal("page 2 returned rows from page 1")
	}

	succeeded, totalSucceeded, err := repo.List(ctx, repository.DraftListFilter{Status: model.DraftStatusSucceeded}, 1, 10)
	if err != nil || totalSucceeded != 1 || len(succeeded) != 1 || succeeded[0].Status != "succeeded" {
		t.Fatalf("status filter failed: items=%v total=%d err=%v", succeeded, totalSucceeded, err)
	}
}

func TestRecoverInterruptedDrafts(t *testing.T) {
	ctx := context.Background()
	db := newDraftTestDB(t)
	repo := repository.NewScheduleDraftRepository(db)

	running1 := createDraftRow(t, db, model.DraftStatusRunning)
	running2 := createDraftRow(t, db, model.DraftStatusRunning)
	queued := createDraftRow(t, db, model.DraftStatusQueued)
	succeeded := createDraftRow(t, db, model.DraftStatusSucceeded)

	n, err := repo.RecoverInterrupted(ctx, "server restarted", time.Now())
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 recovered drafts, got %d", n)
	}
	for _, id := range []uint{running1.ID, running2.ID} {
		got, _ := repo.GetByID(ctx, id)
		if got.Status != model.DraftStatusFailed || got.FailReason != "server restarted" {
			t.Fatalf("draft %d not recovered as failed: %+v", id, got)
		}
	}
	if got, _ := repo.GetByID(ctx, queued.ID); got.Status != model.DraftStatusQueued {
		t.Fatalf("queued draft must stay queued, got %s", got.Status)
	}
	if got, _ := repo.GetByID(ctx, succeeded.ID); got.Status != model.DraftStatusSucceeded {
		t.Fatalf("succeeded draft must not change, got %s", got.Status)
	}
}
