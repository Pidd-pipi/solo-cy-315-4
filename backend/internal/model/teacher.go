package model

import "gorm.io/gorm"

// Teacher represents an instructor with subjects and unavailable periods.
type Teacher struct {
	gorm.Model
	Name             string   `gorm:"size:128;not null" json:"name"`
	EmployeeNo       string   `gorm:"size:64;uniqueIndex;not null" json:"employee_no"`
	Contact          string   `gorm:"size:128" json:"contact"`
	Subjects         []string `gorm:"serializer:json" json:"subjects"`
	UnavailableSlots []string `gorm:"serializer:json" json:"unavailable_slots"`
}
