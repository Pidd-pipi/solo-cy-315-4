package dto

// CreateTimeSlotRequest is the payload for creating a time slot.
type CreateTimeSlotRequest struct {
	Code      string `json:"code" binding:"required,max=64"`
	Name      string `json:"name" binding:"required,max=128"`
	StartTime string `json:"start_time" binding:"required,max=8"`
	EndTime   string `json:"end_time" binding:"required,max=8"`
}

// UpdateTimeSlotRequest is the payload for updating a time slot.
type UpdateTimeSlotRequest struct {
	Code      string `json:"code" binding:"omitempty,max=64"`
	Name      string `json:"name" binding:"omitempty,max=128"`
	StartTime string `json:"start_time" binding:"omitempty,max=8"`
	EndTime   string `json:"end_time" binding:"omitempty,max=8"`
}

// TimeSlotResponse is the time slot representation returned by the API.
type TimeSlotResponse struct {
	ID        uint   `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
