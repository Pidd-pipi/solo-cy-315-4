package dto

// CreateCourseRequest is the payload for creating a course.
type CreateCourseRequest struct {
	Name     string `json:"name" binding:"required,max=128"`
	Code     string `json:"code" binding:"required,max=64"`
	Duration int    `json:"duration" binding:"required,gte=1"`
	RoomType string `json:"room_type" binding:"omitempty,max=128"`
}

// UpdateCourseRequest is the payload for updating a course.
type UpdateCourseRequest struct {
	Name     string `json:"name" binding:"omitempty,max=128"`
	Code     string `json:"code" binding:"omitempty,max=64"`
	Duration *int   `json:"duration" binding:"omitempty,gte=1"`
	RoomType string `json:"room_type" binding:"omitempty,max=128"`
}

// CourseResponse is the course representation returned by the API.
type CourseResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Code      string `json:"code"`
	Duration  int    `json:"duration"`
	RoomType  string `json:"room_type"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
