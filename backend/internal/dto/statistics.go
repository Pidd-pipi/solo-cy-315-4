package dto

// ClassroomUtilizationItem reports weekly usage for one classroom.
type ClassroomUtilizationItem struct {
	ClassroomID   uint    `json:"classroom_id"`
	ClassroomName string  `json:"classroom_name"`
	UsedPeriods   int64   `json:"used_periods"`
	TotalPeriods  int64   `json:"total_periods"`
	Utilization   float64 `json:"utilization"`
}

// TeacherWorkloadItem reports weekly teaching load for one teacher.
type TeacherWorkloadItem struct {
	TeacherID   uint   `json:"teacher_id"`
	TeacherName string `json:"teacher_name"`
	Periods     int64  `json:"periods"`
}

// CourseDensityItem reports lesson density for a day/slot pair.
type CourseDensityItem struct {
	DayOfWeek  int   `json:"day_of_week"`
	TimeSlotID uint  `json:"time_slot_id"`
	Count      int64 `json:"count"`
}
