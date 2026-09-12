package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gbschedule/gbschedule/internal/constants"
	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/model"
)

type position struct {
	Day  int
	Slot model.TimeSlot
}

func positionKey(week uint, day int, slotID uint) string {
	return fmt.Sprintf("%d-%d-%d", week, day, slotID)
}

func slotIndexKey(day int, slotID uint) string {
	return fmt.Sprintf("%d-%d", day, slotID)
}

type occupancy struct {
	teacher   map[string]bool
	class     map[string]bool
	classroom map[string]bool
}

func newOccupancy() *occupancy {
	return &occupancy{
		teacher:   map[string]bool{},
		class:     map[string]bool{},
		classroom: map[string]bool{},
	}
}

func teacherPositionKey(week uint, day int, slotID uint, teacherID uint) string {
	return fmt.Sprintf("%s-t-%d", positionKey(week, day, slotID), teacherID)
}

func classPositionKey(week uint, day int, slotID uint, classID uint) string {
	return fmt.Sprintf("%s-c-%d", positionKey(week, day, slotID), classID)
}

func (o *occupancy) teacherFree(week uint, day int, slotID uint, teacherID uint) bool {
	return !o.teacher[teacherPositionKey(week, day, slotID, teacherID)]
}

func (o *occupancy) classFree(week uint, day int, slotID uint, classID uint) bool {
	return !o.class[classPositionKey(week, day, slotID, classID)]
}

func (o *occupancy) classroomFree(week uint, day int, slotID uint, classroomID uint) bool {
	return !o.classroom[fmt.Sprintf("%s-r-%d", positionKey(week, day, slotID), classroomID)]
}

func (o *occupancy) mark(week uint, p position, teacherID uint, classID uint, classroomID uint) {
	o.teacher[teacherPositionKey(week, p.Day, p.Slot.ID, teacherID)] = true
	o.class[classPositionKey(week, p.Day, p.Slot.ID, classID)] = true
	o.classroom[fmt.Sprintf("%s-r-%d", positionKey(week, p.Day, p.Slot.ID), classroomID)] = true
}

// requirementCourseIDs collects unique course IDs referenced by a request.
func requirementCourseIDs(requirements []dto.CourseRequirement) []uint {
	seen := map[uint]bool{}
	var ids []uint
	for _, r := range requirements {
		if r.CourseID != 0 && !seen[r.CourseID] {
			seen[r.CourseID] = true
			ids = append(ids, r.CourseID)
		}
	}
	return ids
}

func (s *scheduleService) resolveClasses(ctx context.Context, req *dto.GenerateScheduleRequest) ([]model.Class, error) {
	if len(req.ClassIDs) > 0 {
		items, err := s.classes.GetByIDs(ctx, req.ClassIDs)
		if err != nil {
			return nil, fmt.Errorf("load requested classes: %w", err)
		}
		return items, nil
	}
	ids := make([]uint, 0)
	for _, r := range req.Courses {
		if r.ClassID != 0 {
			ids = append(ids, r.ClassID)
		}
	}
	if len(ids) > 0 {
		items, err := s.classes.GetByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("load requirement classes: %w", err)
		}
		return items, nil
	}
	items, _, err := s.classes.List(ctx, 1, constants.MaxPageSize)
	if err != nil {
		return nil, fmt.Errorf("load all classes: %w", err)
	}
	return items, nil
}

func (s *scheduleService) resolveTeachers(ctx context.Context, req *dto.GenerateScheduleRequest) ([]model.Teacher, error) {
	if len(req.TeacherIDs) > 0 {
		items, err := s.teachers.GetByIDs(ctx, req.TeacherIDs)
		if err != nil {
			return nil, fmt.Errorf("load requested teachers: %w", err)
		}
		return items, nil
	}
	ids := make([]uint, 0)
	for _, r := range req.Courses {
		if r.TeacherID != 0 {
			ids = append(ids, r.TeacherID)
		}
	}
	if len(ids) > 0 {
		items, err := s.teachers.GetByIDs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("load requirement teachers: %w", err)
		}
		return items, nil
	}
	items, _, err := s.teachers.List(ctx, 1, constants.MaxPageSize)
	if err != nil {
		return nil, fmt.Errorf("load all teachers: %w", err)
	}
	return items, nil
}

func (s *scheduleService) resolveClassrooms(ctx context.Context, req *dto.GenerateScheduleRequest) ([]model.Classroom, error) {
	if len(req.ClassroomIDs) > 0 {
		items, err := s.classrooms.GetByIDs(ctx, req.ClassroomIDs)
		if err != nil {
			return nil, fmt.Errorf("load requested classrooms: %w", err)
		}
		return items, nil
	}
	items, _, err := s.classrooms.List(ctx, 1, constants.MaxPageSize)
	if err != nil {
		return nil, fmt.Errorf("load all classrooms: %w", err)
	}
	return items, nil
}

func targetClassesForRequirement(allClasses []model.Class, requirement dto.CourseRequirement, classIDs []uint) []model.Class {
	if requirement.ClassID != 0 {
		for _, c := range allClasses {
			if c.ID == requirement.ClassID {
				return []model.Class{c}
			}
		}
		return nil
	}
	return allClasses
}

func entityMap[T any](items []T, id func(T) uint) map[uint]T {
	out := make(map[uint]T, len(items))
	for _, item := range items {
		out[id(item)] = item
	}
	return out
}

func pickTeacher(teachers []model.Teacher, requirement dto.CourseRequirement, courses []model.Course, courseMap map[uint]model.Course, cursor *int) *model.Teacher {
	if requirement.TeacherID != 0 {
		for i := range teachers {
			if teachers[i].ID == requirement.TeacherID {
				return &teachers[i]
			}
		}
		return nil
	}
	course, ok := courseMap[requirement.CourseID]
	if ok {
		for i := range teachers {
			if teacherTeaches(teachers[i], course) {
				return &teachers[i]
			}
		}
	}
	if len(teachers) == 0 {
		return nil
	}
	idx := *cursor % len(teachers)
	*cursor++
	return &teachers[idx]
}

func teacherTeaches(teacher model.Teacher, course model.Course) bool {
	for _, subject := range teacher.Subjects {
		if strings.EqualFold(subject, course.Name) || strings.EqualFold(subject, course.Code) {
			return true
		}
	}
	return false
}

func buildCandidatePositions(days int, slots []model.TimeSlot, consecutive bool, periods int) []position {
	var positions []position
	for day := 1; day <= days; day++ {
		for _, slot := range slots {
			positions = append(positions, position{Day: day, Slot: slot})
		}
	}
	if consecutive && periods > 1 {
		var runs []position
		// Put contiguous runs first so greedy tries to satisfy consecutive preference.
		for day := 1; day <= days; day++ {
			for start := 0; start+periods <= len(slots); start++ {
				for offset := 0; offset < periods; offset++ {
					runs = append(runs, position{Day: day, Slot: slots[start+offset]})
				}
			}
		}
		seen := map[string]bool{}
		for _, p := range runs {
			seen[slotIndexKey(p.Day, p.Slot.ID)] = true
		}
		for _, p := range positions {
			if !seen[slotIndexKey(p.Day, p.Slot.ID)] {
				runs = append(runs, p)
			}
		}
		return runs
	}
	sort.SliceStable(positions, func(i, j int) bool {
		if positions[i].Day != positions[j].Day {
			return positions[i].Day < positions[j].Day
		}
		return positions[i].Slot.StartTime < positions[j].Slot.StartTime
	})
	return positions
}

func placeGreedy(week uint, positions []position, periods int, occ *occupancy, class model.Class, teacher model.Teacher, course model.Course, classrooms []model.Classroom, slots []model.TimeSlot) ([]position, []model.Classroom, bool) {
	chosen := make([]position, 0, periods)
	chosenClassrooms := make([]model.Classroom, 0, periods)
	used := map[string]bool{}

	for p := 0; p < periods; p++ {
		placed := false
		for _, pos := range positions {
			key := slotIndexKey(pos.Day, pos.Slot.ID)
			if used[key] {
				continue
			}
			if !occ.teacherFree(week, pos.Day, pos.Slot.ID, teacher.ID) || !occ.classFree(week, pos.Day, pos.Slot.ID, class.ID) {
				continue
			}
			if contains(teacher.UnavailableSlots, pos.Slot.Code) {
				continue
			}
			classroom, ok := chooseClassroom(week, pos, occ, class, course, classrooms)
			if !ok {
				continue
			}
			used[key] = true
			chosen = append(chosen, pos)
			chosenClassrooms = append(chosenClassrooms, classroom)
			placed = true
			break
		}
		if !placed {
			return nil, nil, false
		}
	}
	// Commit occupancy after the full assignment succeeds.
	for i := range chosen {
		occ.mark(week, chosen[i], teacher.ID, class.ID, chosenClassrooms[i].ID)
	}
	return chosen, chosenClassrooms, true
}

func chooseClassroom(week uint, pos position, occ *occupancy, class model.Class, course model.Course, classrooms []model.Classroom) (model.Classroom, bool) {
	var fallback model.Classroom
	hasFallback := false
	for _, classroom := range classrooms {
		if !occ.classroomFree(week, pos.Day, pos.Slot.ID, classroom.ID) {
			continue
		}
		if class.StudentCount > classroom.Capacity {
			continue
		}
		if course.RoomType != "" {
			if !classroomMatchesRoomType(classroom, course.RoomType) {
				continue
			}
		}
		if course.RoomType == "" && !hasFallback {
			fallback = classroom
			hasFallback = true
		}
		return classroom, true
	}
	if hasFallback {
		return fallback, true
	}
	return model.Classroom{}, false
}

func classroomMatchesRoomType(classroom model.Classroom, roomType string) bool {
	for _, equipment := range classroom.Equipment {
		if strings.EqualFold(equipment, roomType) || strings.Contains(strings.ToLower(equipment), strings.ToLower(roomType)) {
			return true
		}
	}
	return false
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func weekRange(weeks int) []uint {
	out := make([]uint, 0, weeks)
	for i := 1; i <= weeks; i++ {
		out = append(out, uint(i))
	}
	return out
}

func distinctWeeks(items []model.Schedule) []uint {
	seen := map[uint]bool{}
	var out []uint
	for _, item := range items {
		if !seen[item.Week] {
			seen[item.Week] = true
			out = append(out, item.Week)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func uniqueClassIDs(items []model.Schedule) []uint {
	return uniqueUint(items, func(s model.Schedule) uint { return s.ClassID })
}
func uniqueClassroomIDs(items []model.Schedule) []uint {
	return uniqueUint(items, func(s model.Schedule) uint { return s.ClassroomID })
}
func uniqueTeacherIDs(items []model.Schedule) []uint {
	return uniqueUint(items, func(s model.Schedule) uint { return s.TeacherID })
}

func uniqueUint(items []model.Schedule, pick func(model.Schedule) uint) []uint {
	seen := map[uint]bool{}
	var out []uint
	for _, item := range items {
		id := pick(item)
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func enrichSchedules(items []model.Schedule, slots map[uint]model.TimeSlot, classrooms map[uint]model.Classroom, teachers map[uint]model.Teacher, classes map[uint]model.Class, courses map[uint]model.Course) []dto.ScheduleResponse {
	out := make([]dto.ScheduleResponse, 0, len(items))
	for _, item := range items {
		slot := slots[item.TimeSlotID]
		classroom := classrooms[item.ClassroomID]
		teacher := teachers[item.TeacherID]
		class := classes[item.ClassID]
		course := courses[item.CourseID]
		out = append(out, dto.ScheduleResponse{
			ID:            item.ID,
			Week:          item.Week,
			DayOfWeek:     item.DayOfWeek,
			TimeSlotID:    item.TimeSlotID,
			TimeSlotCode:  slot.Code,
			TimeSlotName:  slot.Name,
			StartTime:     slot.StartTime,
			EndTime:       slot.EndTime,
			ClassroomID:   item.ClassroomID,
			ClassroomName: classroom.Name,
			TeacherID:     item.TeacherID,
			TeacherName:   teacher.Name,
			ClassID:       item.ClassID,
			ClassName:     class.Name,
			CourseID:      item.CourseID,
			CourseName:    course.Name,
		})
	}
	return out
}

func (s *scheduleService) classroomMap(ctx context.Context, items []model.Schedule) (map[uint]model.Classroom, error) {
	ids := uniqueClassroomIDs(items)
	list, err := s.classrooms.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return entityMap(list, func(c model.Classroom) uint { return c.ID }), nil
}

func (s *scheduleService) teacherMap(ctx context.Context, items []model.Schedule) (map[uint]model.Teacher, error) {
	ids := uniqueTeacherIDs(items)
	list, err := s.teachers.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return entityMap(list, func(t model.Teacher) uint { return t.ID }), nil
}

func (s *scheduleService) classMap(ctx context.Context, items []model.Schedule) (map[uint]model.Class, error) {
	ids := uniqueClassIDs(items)
	list, err := s.classes.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return entityMap(list, func(c model.Class) uint { return c.ID }), nil
}

func (s *scheduleService) courseMap(ctx context.Context, items []model.Schedule) (map[uint]model.Course, error) {
	ids := uniqueUint(items, func(s model.Schedule) uint { return s.CourseID })
	list, err := s.courses.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return entityMap(list, func(c model.Course) uint { return c.ID }), nil
}

func maxDayOfWeek(items []model.Schedule) int {
	maxDay := 0
	for _, item := range items {
		if item.DayOfWeek > maxDay {
			maxDay = item.DayOfWeek
		}
	}
	return maxDay
}
