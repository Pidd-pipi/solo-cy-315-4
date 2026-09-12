package model

import "gorm.io/gorm"

// Class represents a student group.
type Class struct {
	gorm.Model
	Name         string `gorm:"size:128;not null" json:"name"`
	StudentCount int    `gorm:"not null;default:0" json:"student_count"`
	Grade        string `gorm:"size:64" json:"grade"`
}
