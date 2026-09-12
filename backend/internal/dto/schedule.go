package dto

// CourseRequirement describes how many periods a course needs per week.
type CourseRequirement struct {
	CourseID      uint `json:"course_id" binding:"required"`
	WeeklyPeriods int  `json:"weekly_periods" binding:"required,gte=1,lte=40"`
	ClassID       uint `json:"class_id" binding:"omitempty"`
	TeacherID     uint `json:"teacher_id" binding:"omitempty"`
	Consecutive   bool `json:"consecutive"`
}

// GenerateScheduleRequest is the input of the scheduling algorithm.
type GenerateScheduleRequest struct {
	Semester      string              `json:"semester" binding:"omitempty,max=128"`
	Weeks         int                 `json:"weeks" binding:"required,gte=1,lte=30"`
	DaysPerWeek   int                 `json:"days_per_week" binding:"required,gte=1,lte=7"`
	PeriodsPerDay int                 `json:"periods_per_day" binding:"required,gte=1,lte=20"`
	Courses       []CourseRequirement `json:"courses" binding:"required,min=1,dive"`
	TeacherIDs    []uint              `json:"teacher_ids" binding:"omitempty"`
	ClassIDs      []uint              `json:"class_ids" binding:"omitempty"`
	ClassroomIDs  []uint              `json:"classroom_ids" binding:"omitempty"`
}

// GenerateScheduleResponse is the result of a scheduling run.
type GenerateScheduleResponse struct {
	Schedules []ScheduleResponse `json:"schedules"`
	Conflicts []ConflictResponse `json:"conflicts"`
	Generated int                `json:"generated"`
	Required  int                `json:"required"`
}

// ScheduleResponse is a timetable entry enriched with related names.
type ScheduleResponse struct {
	ID            uint   `json:"id"`
	Week          uint   `json:"week"`
	DayOfWeek     int    `json:"day_of_week"`
	TimeSlotID    uint   `json:"time_slot_id"`
	TimeSlotCode  string `json:"time_slot_code"`
	TimeSlotName  string `json:"time_slot_name"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
	ClassroomID   uint   `json:"classroom_id"`
	ClassroomName string `json:"classroom_name"`
	TeacherID     uint   `json:"teacher_id"`
	TeacherName   string `json:"teacher_name"`
	ClassID       uint   `json:"class_id"`
	ClassName     string `json:"class_name"`
	CourseID      uint   `json:"course_id"`
	CourseName    string `json:"course_name"`
}

// SwapScheduleRequest swaps time and classroom of two timetable entries.
type SwapScheduleRequest struct {
	ScheduleAID uint `json:"schedule_a_id" binding:"required"`
	ScheduleBID uint `json:"schedule_b_id" binding:"required"`
}

// MoveScheduleRequest moves a timetable entry to a new free slot.
type MoveScheduleRequest struct {
	ScheduleID  uint `json:"schedule_id" binding:"required"`
	Week        uint `json:"week" binding:"required,gte=1"`
	DayOfWeek   int  `json:"day_of_week" binding:"required,gte=1,lte=7"`
	TimeSlotID  uint `json:"time_slot_id" binding:"required"`
	ClassroomID uint `json:"classroom_id" binding:"required"`
}

// AdjustmentResponse is the result of a manual adjustment.
type AdjustmentResponse struct {
	Schedule  ScheduleResponse   `json:"schedule"`
	Conflicts []ConflictResponse `json:"conflicts"`
	LogID     uint               `json:"log_id"`
}

// AdjustmentLogResponse is an audit history entry.
type AdjustmentLogResponse struct {
	ID         uint   `json:"id"`
	ScheduleID uint   `json:"schedule_id"`
	Action     string `json:"action"`
	Detail     string `json:"detail"`
	CreatedAt  string `json:"created_at"`
}

// ExportScheduleRequest is the query payload for timetable export.
type ExportScheduleRequest struct {
	Type string `form:"type" binding:"required,oneof=class teacher classroom"`
	ID   uint   `form:"id" binding:"required,gte=1"`
	Week uint   `form:"week" binding:"omitempty,gte=1"`
}
