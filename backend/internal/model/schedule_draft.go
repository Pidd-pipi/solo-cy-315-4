package model

import (
	"time"

	"gorm.io/gorm"
)

// Draft lifecycle states. Once a draft reaches one of the terminal states
// (succeeded / failed / canceled) it never changes again.
const (
	DraftStatusQueued    = "queued"
	DraftStatusRunning   = "running"
	DraftStatusSucceeded = "succeeded"
	DraftStatusFailed    = "failed"
	DraftStatusCanceled  = "canceled"
)

// ScheduleDraft is an asynchronous timetable generation job.
//
// The submitted scheduling conditions are stored verbatim in Params so that
// the job can be (re)executed without relying on in-memory state. ResultJSON
// holds the generated timetable, conflicts and summary statistics when the
// draft succeeds. The published "schedules" table is never modified while a
// draft is generated; a draft has to be published explicitly to take effect.
type ScheduleDraft struct {
	gorm.Model
	Status      string     `gorm:"size:16;index;not null;default:queued" json:"status"`
	Params      string     `gorm:"type:text;not null" json:"params"`
	Progress    int        `gorm:"not null;default:0" json:"progress"`
	TotalSteps  int        `gorm:"not null;default:0" json:"total_steps"`
	CurrentStep string     `gorm:"size:255;not null;default:''" json:"current_step"`
	Conflicts   string     `gorm:"type:text;not null;default:'[]'" json:"conflicts"`
	ResultJSON  string     `gorm:"type:text" json:"result_json,omitempty"`
	FailReason  string     `gorm:"type:text;not null;default:''" json:"fail_reason"`
	StartedAt   *time.Time `gorm:"index" json:"started_at,omitempty"`
	FinishedAt  *time.Time `gorm:"index" json:"finished_at,omitempty"`
}

// IsTerminal reports whether the draft has reached a terminal state.
func (d ScheduleDraft) IsTerminal() bool {
	switch d.Status {
	case DraftStatusSucceeded, DraftStatusFailed, DraftStatusCanceled:
		return true
	default:
		return false
	}
}

// TableName pins the table name for GORM.
func (ScheduleDraft) TableName() string { return "schedule_drafts" }
