package dto

// Draft lifecycle status values returned by the API.
const (
	DraftStatusQueued    = "queued"
	DraftStatusRunning   = "running"
	DraftStatusSucceeded = "succeeded"
	DraftStatusFailed    = "failed"
	DraftStatusCanceled  = "canceled"
)

// CreateDraftResponse is returned immediately after a draft is submitted,
// before background generation starts.
type CreateDraftResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
}

// DraftSummary holds aggregate statistics of a successfully generated draft.
type DraftSummary struct {
	Generated      int `json:"generated"`
	Required       int `json:"required"`
	Unplaced       int `json:"unplaced_requirements"`
	ConflictCount  int `json:"conflict_count"`
	Weeks          int `json:"weeks"`
	DaysPerWeek    int `json:"days_per_week"`
	PeriodsPerDay  int `json:"periods_per_day"`
	ClassroomCount int `json:"classrooms_used"`
	TeacherCount   int `json:"teachers_used"`
	ClassCount     int `json:"classes_used"`
	CourseCount    int `json:"courses_used"`
}

// DraftResultData is the immutable output stored when a draft succeeds.
type DraftResultData struct {
	Schedules []ScheduleResponse `json:"schedules"`
	Conflicts []ConflictResponse `json:"conflicts"`
	Summary   DraftSummary       `json:"summary"`
}

// DraftResponse exposes the current state of a draft. While running it also
// carries progress, the current conflict list and (on failure) the reason.
type DraftResponse struct {
	ID          uint                    `json:"id"`
	Status      string                  `json:"status"`
	Params      GenerateScheduleRequest `json:"params"`
	Progress    int                     `json:"progress"`
	TotalSteps  int                     `json:"total_steps"`
	CurrentStep string                  `json:"current_step"`
	Conflicts   []ConflictResponse      `json:"conflicts"`
	FailReason  string                  `json:"fail_reason"`
	CreatedAt   string                  `json:"created_at"`
	UpdatedAt   string                  `json:"updated_at"`
	StartedAt   string                  `json:"started_at,omitempty"`
	FinishedAt  string                  `json:"finished_at,omitempty"`
}

// DraftResultResponse is the full draft state plus the generated timetable.
type DraftResultResponse struct {
	DraftResponse
	Result *DraftResultData `json:"result"`
}

// DraftListQuery binds the draft listing query parameters.
type DraftListQuery struct {
	Status string `form:"status" binding:"omitempty,oneof=queued running succeeded failed canceled"`
	Pagination
}
