package model

import "gorm.io/gorm"

// TimeSlot defines a period of a school day, for example 08:00-09:40.
type TimeSlot struct {
	gorm.Model
	Code      string `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name      string `gorm:"size:128;not null" json:"name"`
	StartTime string `gorm:"size:8;not null" json:"start_time"`
	EndTime   string `gorm:"size:8;not null" json:"end_time"`
}
