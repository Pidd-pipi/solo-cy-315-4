package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/gbschedule/gbschedule/internal/model"
)

// TestPlaceGreedyCancelsMidSearch verifies fine-grained cancellation inside
// the position/classroom search: with a large candidate space, a context
// canceled shortly after start aborts the search well before all periods are
// attempted, instead of waiting for the whole placement to finish.
func TestPlaceGreedyCancelsMidSearch(t *testing.T) {
	teacher := model.Teacher{Model: gorm.Model{ID: 1}}
	class := model.Class{Model: gorm.Model{ID: 1}, StudentCount: 40}
	course := model.Course{Model: gorm.Model{ID: 1}}
	slot := model.TimeSlot{Model: gorm.Model{ID: 1}, Code: "S1"}

	// Many positions (days) and many zero-capacity classrooms, so every period
	// must scan the full candidate space before failing to place.
	var positions []position
	var rooms []model.Classroom
	const posN, roomN = 2000, 1000
	for i := 0; i < posN; i++ {
		positions = append(positions, position{Day: 1 + i%7, Slot: slot})
	}
	for i := 0; i < roomN; i++ {
		rooms = append(rooms, model.Classroom{Model: gorm.Model{ID: uint(i + 100)}, Capacity: 0})
	}

	// Baseline: attempt several periods without cancellation.
	const periods = 20
	start := time.Now()
	_, _, ok, err := placeGreedy(context.Background(), 1, positions, periods, newOccupancy(), class, teacher, course, rooms, nil)
	baseline := time.Since(start)
	if err != nil || ok {
		t.Fatalf("baseline: expected placement failure, got ok=%v err=%v", ok, err)
	}

	// Cancel almost immediately: the search must unwind with context.Canceled
	// far sooner than the full uncanceled run.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start = time.Now()
	_, _, ok, err = placeGreedy(ctx, 1, positions, periods, newOccupancy(), class, teacher, course, rooms, nil)
	stoppedIn := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled mid-search, got ok=%v err=%v", ok, err)
	}
	if stoppedIn > baseline/2 {
		t.Fatalf("search did not stop promptly: stopped_in=%s baseline=%s", stoppedIn, baseline)
	}
	t.Logf("baseline=%s canceled_run_stopped_in=%s", baseline, stoppedIn)
}
