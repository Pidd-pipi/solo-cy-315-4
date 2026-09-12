package repository

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// normalizeError maps low-level GORM errors to repository sentinel errors.
func normalizeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return fmt.Errorf("repository: %w", err)
}

// paginate applies page/page_size to a GORM query and returns the total count.
func paginate(db *gorm.DB, page, pageSize int) *gorm.DB {
	offset := (page - 1) * pageSize
	return db.Offset(offset).Limit(pageSize)
}

// isConstraintError reports whether err was caused by a database constraint
// violation such as a duplicate unique key.
func isConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed") || strings.Contains(msg, "duplicated key not allowed")
}
