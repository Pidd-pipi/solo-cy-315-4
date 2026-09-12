package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/gbschedule/gbschedule/internal/constants"
	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/repository"
)

// ScheduleService exposes scheduling, conflict detection, adjustment and statistics operations.
type ScheduleService interface {
	Generate(ctx context.Context, req *dto.GenerateScheduleRequest) (*dto.GenerateScheduleResponse, error)
	List(ctx context.Context, week, classID, teacherID, classroomID *uint) ([]dto.ScheduleResponse, error)
	Get(ctx context.Context, id uint) (*dto.ScheduleResponse, error)
	CheckConflicts(ctx context.Context) ([]dto.ConflictResponse, error)
	Swap(ctx context.Context, req *dto.SwapScheduleRequest) (*dto.AdjustmentResponse, error)
	Move(ctx context.Context, req *dto.MoveScheduleRequest) (*dto.AdjustmentResponse, error)
	ListAdjustments(ctx context.Context, page, pageSize int) ([]dto.AdjustmentLogResponse, int64, error)
	ClassroomUtilization(ctx context.Context) ([]dto.ClassroomUtilizationItem, error)
	TeacherWorkload(ctx context.Context) ([]dto.TeacherWorkloadItem, error)
	CourseDensity(ctx context.Context) ([]dto.CourseDensityItem, error)
}

type scheduleService struct {
	schedules   repository.ScheduleRepository
	classrooms  repository.ClassroomRepository
	teachers    repository.TeacherRepository
	classes     repository.ClassRepository
	courses     repository.CourseRepository
	timeSlots   repository.TimeSlotRepository
	adjustments repository.AdjustmentLogRepository
	logger      *slog.Logger
}

// NewScheduleService constructs a schedule service.
func NewScheduleService(
	schedules repository.ScheduleRepository,
	classrooms repository.ClassroomRepository,
	teachers repository.TeacherRepository,
	classes repository.ClassRepository,
	courses repository.CourseRepository,
	timeSlots repository.TimeSlotRepository,
	adjustments repository.AdjustmentLogRepository,
	logger *slog.Logger,
) ScheduleService {
	return &scheduleService{
		schedules:   schedules,
		classrooms:  classrooms,
		teachers:    teachers,
		classes:     classes,
		courses:     courses,
		timeSlots:   timeSlots,
		adjustments: adjustments,
		logger:      logger,
	}
}

// Generate creates a timetable with a greedy scheduling algorithm.
func (s *scheduleService) Generate(ctx context.Context, req *dto.GenerateScheduleRequest) (*dto.GenerateScheduleResponse, error) {
	allSlots, _, err := s.timeSlots.List(ctx, 1, constants.MaxPageSize)
	if err != nil {
		return nil, fmt.Errorf("load time slots: %w", err)
	}
	if req.PeriodsPerDay > len(allSlots) {
		return nil, fmt.Errorf("generate schedule: %w: periods_per_day %d exceeds available time slots %d", ErrInvalid, req.PeriodsPerDay, len(allSlots))
	}
	slots := allSlots[:req.PeriodsPerDay]

	courses, err := s.courses.GetByIDs(ctx, requirementCourseIDs(req.Courses))
	if err != nil {
		return nil, fmt.Errorf("load courses: %w", err)
	}
	courseMap := entityMap(courses, func(c model.Course) uint { return c.ID })

	classes, err := s.resolveClasses(ctx, req)
	if err != nil {
		return nil, err
	}
	teachers, err := s.resolveTeachers(ctx, req)
	if err != nil {
		return nil, err
	}
	classrooms, err := s.resolveClassrooms(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(teachers) == 0 || len(classes) == 0 || len(classrooms) == 0 {
		return nil, fmt.Errorf("generate schedule: %w: teachers, classes and classrooms must not be empty", ErrInvalid)
	}

	teacherCursor := 0
	var allSchedules []model.Schedule
	var conflicts []dto.ConflictResponse
	required := 0

	for week := 1; week <= req.Weeks; week++ {
		occ := newOccupancy()
		for _, requirement := range req.Courses {
			targetClasses := targetClassesForRequirement(classes, requirement, req.ClassIDs)
			for _, class := range targetClasses {
				teacher := pickTeacher(teachers, requirement, courses, courseMap, &teacherCursor)
				if teacher == nil {
					continue
				}
				course, ok := courseMap[requirement.CourseID]
				if !ok {
					conflicts = append(conflicts, dto.ConflictResponse{
						Type: constants.ConflictTeacherTime, EntityType: "course", EntityID: requirement.CourseID,
						EntityName: fmt.Sprintf("course %d", requirement.CourseID), Week: uint(week),
						Suggestion: "course not found",
					})
					continue
				}
				required += requirement.WeeklyPeriods

				positions := buildCandidatePositions(req.DaysPerWeek, slots, requirement.Consecutive, requirement.WeeklyPeriods)
				chosen, classrooms, ok := placeGreedy(uint(week), positions, requirement.WeeklyPeriods, occ, class, *teacher, course, classrooms, slots)
				if !ok {
					conflicts = append(conflicts, dto.ConflictResponse{
						Type:       constants.ConflictTeacherTime,
						EntityType: "course",
						EntityID:   requirement.CourseID,
						EntityName: course.Name,
						Week:       uint(week),
						Suggestion: fmt.Sprintf("unable to place %d periods for course %s in week %d; add teachers/classrooms or relax constraints", requirement.WeeklyPeriods, course.Name, week),
					})
					continue
				}
				for i := range chosen {
					allSchedules = append(allSchedules, model.Schedule{
						Week:        uint(week),
						DayOfWeek:   chosen[i].Day,
						TimeSlotID:  chosen[i].Slot.ID,
						ClassroomID: classrooms[i].ID,
						TeacherID:   teacher.ID,
						ClassID:     class.ID,
						CourseID:    course.ID,
					})
				}
			}
		}
	}

	// Regenerate the full timetable for the requested semester so stale
	// weeks from a previous longer run are not left behind.
	if err := s.schedules.DeleteAll(ctx); err != nil {
		return nil, fmt.Errorf("clear old schedules: %w", err)
	}
	if err := s.schedules.CreateBatch(ctx, allSchedules); err != nil {
		return nil, fmt.Errorf("persist schedules: %w", err)
	}

	generatedConflicts, err := s.CheckConflicts(ctx)
	if err != nil {
		return nil, fmt.Errorf("check generated conflicts: %w", err)
	}

	responses, err := s.enrichSchedules(ctx, allSchedules)
	if err != nil {
		return nil, fmt.Errorf("enrich schedules: %w", err)
	}
	resp := &dto.GenerateScheduleResponse{
		Schedules: responses,
		Conflicts: append(conflicts, generatedConflicts...),
		Generated: len(allSchedules),
		Required:  required,
	}
	return resp, nil
}

func (s *scheduleService) List(ctx context.Context, week, classID, teacherID, classroomID *uint) ([]dto.ScheduleResponse, error) {
	filter := repository.ScheduleFilter{Week: week, ClassID: classID, TeacherID: teacherID, ClassroomID: classroomID}
	items, err := s.schedules.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	return s.enrichSchedules(ctx, items)
}

func (s *scheduleService) Get(ctx context.Context, id uint) (*dto.ScheduleResponse, error) {
	item, err := s.schedules.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get schedule: %w", err)
	}
	responses, err := s.enrichSchedules(ctx, []model.Schedule{*item})
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, ErrNotFound
	}
	return &responses[0], nil
}

func (s *scheduleService) CheckConflicts(ctx context.Context) ([]dto.ConflictResponse, error) {
	items, err := s.schedules.List(ctx, repository.ScheduleFilter{})
	if err != nil {
		return nil, fmt.Errorf("list schedules for conflict check: %w", err)
	}
	return s.detectConflicts(ctx, items), nil
}

func (s *scheduleService) Swap(ctx context.Context, req *dto.SwapScheduleRequest) (*dto.AdjustmentResponse, error) {
	a, err := s.schedules.GetByID(ctx, req.ScheduleAID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get schedule a: %w", err)
	}
	b, err := s.schedules.GetByID(ctx, req.ScheduleBID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get schedule b: %w", err)
	}
	a.Week, b.Week = b.Week, a.Week
	a.DayOfWeek, b.DayOfWeek = b.DayOfWeek, a.DayOfWeek
	a.TimeSlotID, b.TimeSlotID = b.TimeSlotID, a.TimeSlotID
	a.ClassroomID, b.ClassroomID = b.ClassroomID, a.ClassroomID
	if err := s.schedules.Update(ctx, a); err != nil {
		return nil, fmt.Errorf("update schedule a: %w", err)
	}
	if err := s.schedules.Update(ctx, b); err != nil {
		return nil, fmt.Errorf("update schedule b: %w", err)
	}
	logID, err := s.recordAdjustment(ctx, req.ScheduleAID, constants.ActionSwap, map[string]any{"schedule_a_id": req.ScheduleAID, "schedule_b_id": req.ScheduleBID})
	if err != nil {
		return nil, err
	}
	responses, err := s.enrichSchedules(ctx, []model.Schedule{*a})
	if err != nil {
		return nil, err
	}
	conflicts, err := s.CheckConflicts(ctx)
	if err != nil {
		return nil, err
	}
	return &dto.AdjustmentResponse{Schedule: responses[0], Conflicts: conflicts, LogID: logID}, nil
}

func (s *scheduleService) Move(ctx context.Context, req *dto.MoveScheduleRequest) (*dto.AdjustmentResponse, error) {
	item, err := s.schedules.GetByID(ctx, req.ScheduleID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get schedule: %w", err)
	}
	item.Week = req.Week
	item.DayOfWeek = req.DayOfWeek
	item.TimeSlotID = req.TimeSlotID
	item.ClassroomID = req.ClassroomID
	if err := s.schedules.Update(ctx, item); err != nil {
		return nil, fmt.Errorf("update schedule: %w", err)
	}
	logID, err := s.recordAdjustment(ctx, req.ScheduleID, constants.ActionMove, map[string]any{"week": req.Week, "day_of_week": req.DayOfWeek, "time_slot_id": req.TimeSlotID, "classroom_id": req.ClassroomID})
	if err != nil {
		return nil, err
	}
	responses, err := s.enrichSchedules(ctx, []model.Schedule{*item})
	if err != nil {
		return nil, err
	}
	conflicts, err := s.CheckConflicts(ctx)
	if err != nil {
		return nil, err
	}
	return &dto.AdjustmentResponse{Schedule: responses[0], Conflicts: conflicts, LogID: logID}, nil
}

func (s *scheduleService) ListAdjustments(ctx context.Context, page, pageSize int) ([]dto.AdjustmentLogResponse, int64, error) {
	items, total, err := s.adjustments.List(ctx, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list adjustment logs: %w", err)
	}
	out := make([]dto.AdjustmentLogResponse, 0, len(items))
	for i := range items {
		out = append(out, dto.AdjustmentLogResponse{
			ID:         items[i].ID,
			ScheduleID: items[i].ScheduleID,
			Action:     items[i].Action,
			Detail:     items[i].Detail,
			CreatedAt:  items[i].CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return out, total, nil
}

func (s *scheduleService) ClassroomUtilization(ctx context.Context) ([]dto.ClassroomUtilizationItem, error) {
	classrooms, _, err := s.classrooms.List(ctx, 1, constants.MaxPageSize)
	if err != nil {
		return nil, fmt.Errorf("list classrooms: %w", err)
	}
	schedules, err := s.schedules.List(ctx, repository.ScheduleFilter{})
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	weeks := distinctWeeks(schedules)
	days := maxDayOfWeek(schedules)
	slots, _, err := s.timeSlots.List(ctx, 1, constants.MaxPageSize)
	if err != nil {
		return nil, fmt.Errorf("list time slots: %w", err)
	}
	totalPeriods := int64(len(weeks)) * int64(days) * int64(len(slots))
	usedByClassroom := map[uint]int64{}
	for _, item := range schedules {
		usedByClassroom[item.ClassroomID]++
	}
	out := make([]dto.ClassroomUtilizationItem, 0, len(classrooms))
	for i := range classrooms {
		used := usedByClassroom[classrooms[i].ID]
		utilization := float64(0)
		if totalPeriods > 0 {
			utilization = float64(used) / float64(totalPeriods)
		}
		out = append(out, dto.ClassroomUtilizationItem{
			ClassroomID:   classrooms[i].ID,
			ClassroomName: classrooms[i].Name,
			UsedPeriods:   used,
			TotalPeriods:  totalPeriods,
			Utilization:   utilization,
		})
	}
	return out, nil
}

func (s *scheduleService) TeacherWorkload(ctx context.Context) ([]dto.TeacherWorkloadItem, error) {
	teachers, _, err := s.teachers.List(ctx, 1, constants.MaxPageSize)
	if err != nil {
		return nil, fmt.Errorf("list teachers: %w", err)
	}
	schedules, err := s.schedules.List(ctx, repository.ScheduleFilter{})
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	counts := map[uint]int64{}
	for _, item := range schedules {
		counts[item.TeacherID]++
	}
	out := make([]dto.TeacherWorkloadItem, 0, len(teachers))
	for i := range teachers {
		out = append(out, dto.TeacherWorkloadItem{TeacherID: teachers[i].ID, TeacherName: teachers[i].Name, Periods: counts[teachers[i].ID]})
	}
	return out, nil
}

func (s *scheduleService) CourseDensity(ctx context.Context) ([]dto.CourseDensityItem, error) {
	schedules, err := s.schedules.List(ctx, repository.ScheduleFilter{})
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	type key struct {
		day  int
		slot uint
	}
	counts := map[key]int64{}
	for _, item := range schedules {
		k := key{day: item.DayOfWeek, slot: item.TimeSlotID}
		counts[k]++
	}
	out := make([]dto.CourseDensityItem, 0, len(counts))
	for k, v := range counts {
		out = append(out, dto.CourseDensityItem{DayOfWeek: k.day, TimeSlotID: k.slot, Count: v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DayOfWeek != out[j].DayOfWeek {
			return out[i].DayOfWeek < out[j].DayOfWeek
		}
		return out[i].TimeSlotID < out[j].TimeSlotID
	})
	return out, nil
}

func (s *scheduleService) recordAdjustment(ctx context.Context, scheduleID uint, action string, detail any) (uint, error) {
	data, err := json.Marshal(detail)
	if err != nil {
		return 0, fmt.Errorf("marshal adjustment detail: %w", err)
	}
	log := &model.AdjustmentLog{ScheduleID: scheduleID, Action: action, Detail: string(data)}
	if err := s.adjustments.Create(ctx, log); err != nil {
		return 0, fmt.Errorf("record adjustment: %w", err)
	}
	return log.ID, nil
}

func (s *scheduleService) enrichSchedules(ctx context.Context, items []model.Schedule) ([]dto.ScheduleResponse, error) {
	timeSlots, _, err := s.timeSlots.List(ctx, 1, constants.MaxPageSize)
	if err != nil {
		return nil, fmt.Errorf("load time slots: %w", err)
	}
	slotMap := map[uint]model.TimeSlot{}
	for i := range timeSlots {
		slotMap[timeSlots[i].ID] = timeSlots[i]
	}
	classroomMap, err := s.classroomMap(ctx, items)
	if err != nil {
		return nil, err
	}
	teacherMap, err := s.teacherMap(ctx, items)
	if err != nil {
		return nil, err
	}
	classMap, err := s.classMap(ctx, items)
	if err != nil {
		return nil, err
	}
	courseMap, err := s.courseMap(ctx, items)
	if err != nil {
		return nil, err
	}
	return enrichSchedules(items, slotMap, classroomMap, teacherMap, classMap, courseMap), nil
}

func (s *scheduleService) detectConflicts(ctx context.Context, items []model.Schedule) []dto.ConflictResponse {
	var conflicts []dto.ConflictResponse
	teacherSlots := map[string]model.Schedule{}
	classSlots := map[string]model.Schedule{}
	classroomSlots := map[string]model.Schedule{}

	classes, _ := s.classes.GetByIDs(ctx, uniqueClassIDs(items))
	classroomList, _ := s.classrooms.GetByIDs(ctx, uniqueClassroomIDs(items))
	teacherList, _ := s.teachers.GetByIDs(ctx, uniqueTeacherIDs(items))
	slotList, _, _ := s.timeSlots.List(ctx, 1, constants.MaxPageSize)

	classMap := map[uint]model.Class{}
	for i := range classes {
		classMap[classes[i].ID] = classes[i]
	}
	classroomMap := map[uint]model.Classroom{}
	for i := range classroomList {
		classroomMap[classroomList[i].ID] = classroomList[i]
	}
	teacherMap := map[uint]model.Teacher{}
	for i := range teacherList {
		teacherMap[teacherList[i].ID] = teacherList[i]
	}
	slotMap := map[uint]model.TimeSlot{}
	for i := range slotList {
		slotMap[slotList[i].ID] = slotList[i]
	}

	for _, item := range items {
		slotKey := fmt.Sprintf("%d-%d-%d", item.Week, item.DayOfWeek, item.TimeSlotID)
		if existing, ok := teacherSlots[slotKey+"-t-"+fmt.Sprint(item.TeacherID)]; ok && existing.ID != item.ID {
			conflicts = append(conflicts, dto.ConflictResponse{
				Type: constants.ConflictTeacherTime, EntityType: "teacher", EntityID: item.TeacherID,
				EntityName: teacherMap[item.TeacherID].Name, Week: item.Week, DayOfWeek: item.DayOfWeek, TimeSlotID: item.TimeSlotID,
				Suggestion: fmt.Sprintf("teacher already has a lesson at week %d day %d slot %d; move one of the lessons", item.Week, item.DayOfWeek, item.TimeSlotID),
			})
		}
		if existing, ok := classSlots[slotKey+"-c-"+fmt.Sprint(item.ClassID)]; ok && existing.ID != item.ID {
			conflicts = append(conflicts, dto.ConflictResponse{
				Type: constants.ConflictClassTime, EntityType: "class", EntityID: item.ClassID,
				EntityName: classMap[item.ClassID].Name, Week: item.Week, DayOfWeek: item.DayOfWeek, TimeSlotID: item.TimeSlotID,
				Suggestion: fmt.Sprintf("class already has a lesson at week %d day %d slot %d; move one of the lessons", item.Week, item.DayOfWeek, item.TimeSlotID),
			})
		}
		if existing, ok := classroomSlots[slotKey+"-r-"+fmt.Sprint(item.ClassroomID)]; ok && existing.ID != item.ID {
			conflicts = append(conflicts, dto.ConflictResponse{
				Type: constants.ConflictClassroomTime, EntityType: "classroom", EntityID: item.ClassroomID,
				EntityName: classroomMap[item.ClassroomID].Name, Week: item.Week, DayOfWeek: item.DayOfWeek, TimeSlotID: item.TimeSlotID,
				Suggestion: fmt.Sprintf("classroom already has a lesson at week %d day %d slot %d; move one of the lessons", item.Week, item.DayOfWeek, item.TimeSlotID),
			})
		}
		if class, ok := classMap[item.ClassID]; ok {
			if classroom, ok2 := classroomMap[item.ClassroomID]; ok2 && class.StudentCount > classroom.Capacity {
				conflicts = append(conflicts, dto.ConflictResponse{
					Type: constants.ConflictClassroomCap, EntityType: "class", EntityID: item.ClassID,
					EntityName: class.Name, Week: item.Week, DayOfWeek: item.DayOfWeek, TimeSlotID: item.TimeSlotID,
					Suggestion: fmt.Sprintf("class size %d exceeds classroom capacity %d; choose a larger classroom", class.StudentCount, classroom.Capacity),
				})
			}
		}
		if teacher, ok := teacherMap[item.TeacherID]; ok {
			if slot, ok2 := slotMap[item.TimeSlotID]; ok2 && contains(teacher.UnavailableSlots, slot.Code) {
				conflicts = append(conflicts, dto.ConflictResponse{
					Type: constants.ConflictTeacherPref, EntityType: "teacher", EntityID: item.TeacherID,
					EntityName: teacher.Name, Week: item.Week, DayOfWeek: item.DayOfWeek, TimeSlotID: item.TimeSlotID,
					Suggestion: fmt.Sprintf("slot %s is in the teacher's unavailable periods; choose another time", slot.Code),
				})
			}
		}
		teacherSlots[slotKey+"-t-"+fmt.Sprint(item.TeacherID)] = item
		classSlots[slotKey+"-c-"+fmt.Sprint(item.ClassID)] = item
		classroomSlots[slotKey+"-r-"+fmt.Sprint(item.ClassroomID)] = item
	}
	return conflicts
}
