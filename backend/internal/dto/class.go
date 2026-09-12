package dto

// CreateClassRequest is the payload for creating a class.
type CreateClassRequest struct {
	Name         string `json:"name" binding:"required,max=128"`
	StudentCount int    `json:"student_count" binding:"required,gte=1"`
	Grade        string `json:"grade" binding:"omitempty,max=64"`
}

// UpdateClassRequest is the payload for updating a class.
type UpdateClassRequest struct {
	Name         string `json:"name" binding:"omitempty,max=128"`
	StudentCount *int   `json:"student_count" binding:"omitempty,gte=1"`
	Grade        string `json:"grade" binding:"omitempty,max=64"`
}

// ClassResponse is the class representation returned by the API.
type ClassResponse struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	StudentCount int    `json:"student_count"`
	Grade        string `json:"grade"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}
