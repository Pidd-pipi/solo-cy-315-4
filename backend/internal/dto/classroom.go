package dto

// CreateClassroomRequest is the payload for creating a classroom.
type CreateClassroomRequest struct {
	Code      string   `json:"code" binding:"required,max=64"`
	Name      string   `json:"name" binding:"required,max=128"`
	Capacity  int      `json:"capacity" binding:"required,gte=1"`
	Equipment []string `json:"equipment" binding:"omitempty,dive,max=64"`
}

// UpdateClassroomRequest is the payload for updating a classroom.
type UpdateClassroomRequest struct {
	Code      string   `json:"code" binding:"omitempty,max=64"`
	Name      string   `json:"name" binding:"omitempty,max=128"`
	Capacity  *int     `json:"capacity" binding:"omitempty,gte=1"`
	Equipment []string `json:"equipment" binding:"omitempty,dive,max=64"`
}

// ClassroomResponse is the classroom representation returned by the API.
type ClassroomResponse struct {
	ID        uint     `json:"id"`
	Code      string   `json:"code"`
	Name      string   `json:"name"`
	Capacity  int      `json:"capacity"`
	Equipment []string `json:"equipment"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}
