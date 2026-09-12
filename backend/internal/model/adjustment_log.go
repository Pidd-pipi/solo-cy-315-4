package model

import "gorm.io/gorm"

// AdjustmentLog records a manual timetable change for audit/history purposes.
type AdjustmentLog struct {
	gorm.Model
	ScheduleID uint   `gorm:"index" json:"schedule_id"`
	Action     string `gorm:"size:32;not null" json:"action"`
	Detail     string `gorm:"type:text" json:"detail"`
}
