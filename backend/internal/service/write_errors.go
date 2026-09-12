package service

import (
	"errors"
	"fmt"

	"github.com/gbschedule/gbschedule/internal/repository"
)

// mapWriteError converts repository write errors to service-level errors while
// preserving operation context for unexpected failures.
func mapWriteError(op string, err error) error {
	if errors.Is(err, repository.ErrConstraint) {
		return ErrConflict
	}
	return fmt.Errorf("%s: %w", op, err)
}
