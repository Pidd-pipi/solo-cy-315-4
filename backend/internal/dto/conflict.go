package dto

// ConflictResponse describes a single scheduling conflict and a suggested fix.
type ConflictResponse struct {
	Type       string `json:"type"`
	EntityType string `json:"entity_type"`
	EntityID   uint   `json:"entity_id"`
	EntityName string `json:"entity_name"`
	Week       uint   `json:"week"`
	DayOfWeek  int    `json:"day_of_week"`
	TimeSlotID uint   `json:"time_slot_id"`
	Suggestion string `json:"suggestion"`
}
