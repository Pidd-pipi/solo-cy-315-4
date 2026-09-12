package model

import "gorm.io/gorm"

// Classroom represents a schedulable physical room.
type Classroom struct {
	gorm.Model
	Code      string   `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name      string   `gorm:"size:128;not null" json:"name"`
	Capacity  int      `gorm:"not null;default:0" json:"capacity"`
	Equipment []string `gorm:"serializer:json" json:"equipment"`
}
