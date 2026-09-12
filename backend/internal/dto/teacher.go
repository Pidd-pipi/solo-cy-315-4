package dto

// CreateTeacherRequest is the payload for creating a teacher.
type CreateTeacherRequest struct {
	Name             string   `json:"name" binding:"required,max=128"`
	EmployeeNo       string   `json:"employee_no" binding:"required,max=64"`
	Contact          string   `json:"contact" binding:"omitempty,max=128"`
	Subjects         []string `json:"subjects" binding:"omitempty,dive,max=128"`
	UnavailableSlots []string `json:"unavailable_slots" binding:"omitempty,dive,max=64"`
}

// UpdateTeacherRequest is the payload for updating a teacher.
type UpdateTeacherRequest struct {
	Name             string   `json:"name" binding:"omitempty,max=128"`
	EmployeeNo       string   `json:"employee_no" binding:"omitempty,max=64"`
	Contact          string   `json:"contact" binding:"omitempty,max=128"`
	Subjects         []string `json:"subjects" binding:"omitempty,dive,max=128"`
	UnavailableSlots []string `json:"unavailable_slots" binding:"omitempty,dive,max=64"`
}

// TeacherResponse is the teacher representation returned by the API.
type TeacherResponse struct {
	ID               uint     `json:"id"`
	Name             string   `json:"name"`
	EmployeeNo       string   `json:"employee_no"`
	Contact          string   `json:"contact"`
	Subjects         []string `json:"subjects"`
	UnavailableSlots []string `json:"unavailable_slots"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}
