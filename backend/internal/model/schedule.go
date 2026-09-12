package model

import "gorm.io/gorm"

// Schedule is one lesson entry in a timetable.
type Schedule struct {
	gorm.Model
	Week        uint `gorm:"index;not null" json:"week"`
	DayOfWeek   int  `gorm:"index;not null" json:"day_of_week"`
	TimeSlotID  uint `gorm:"index;not null" json:"time_slot_id"`
	ClassroomID uint `gorm:"index;not null" json:"classroom_id"`
	TeacherID   uint `gorm:"index;not null" json:"teacher_id"`
	ClassID     uint `gorm:"index;not null" json:"class_id"`
	CourseID    uint `gorm:"index;not null" json:"course_id"`
}

// TableName explicitly names the table to avoid GORM's default "schedules".
func (Schedule) TableName() string { return "schedules" }
