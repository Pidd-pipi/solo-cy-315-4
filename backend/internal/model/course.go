package model

import "gorm.io/gorm"

// Course represents a subject to be scheduled.
type Course struct {
	gorm.Model
	Name     string `gorm:"size:128;not null" json:"name"`
	Code     string `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Duration int    `gorm:"not null;default:1" json:"duration"`
	RoomType string `gorm:"size:128" json:"room_type"`
}
