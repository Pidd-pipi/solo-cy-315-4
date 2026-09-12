package service_test

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/gbschedule/gbschedule/internal/constants"
	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/model"
	"github.com/gbschedule/gbschedule/internal/repository"
	"github.com/gbschedule/gbschedule/internal/service"
)

func newScheduleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Classroom{}, &model.Teacher{}, &model.Class{}, &model.Course{}, &model.TimeSlot{}, &model.Schedule{}, &model.AdjustmentLog{}); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	return db
}

func newScheduleService(t *testing.T, db *gorm.DB) service.ScheduleService {
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	return service.NewScheduleService(
		repository.NewScheduleRepository(db),
		repository.NewClassroomRepository(db),
		repository.NewTeacherRepository(db),
		repository.NewClassRepository(db),
		repository.NewCourseRepository(db),
		repository.NewTimeSlotRepository(db),
		repository.NewAdjustmentLogRepository(db),
		logger,
	)
}

func TestScheduleServiceDetectConflicts(t *testing.T) {
	ctx := context.Background()
	db := newScheduleTestDB(t)

	slot1 := &model.TimeSlot{Code: "1", Name: "第一节", StartTime: "08:00", EndTime: "09:40"}
	slot2 := &model.TimeSlot{Code: "2", Name: "第二节", StartTime: "10:00", EndTime: "11:40"}
	if err := db.Create(slot1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(slot2).Error; err != nil {
		t.Fatal(err)
	}

	smallRoom := &model.Classroom{Code: "R301", Name: "小教室", Capacity: 30}
	bigRoom := &model.Classroom{Code: "R302", Name: "大教室", Capacity: 50}
	if err := db.Create(smallRoom).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(bigRoom).Error; err != nil {
		t.Fatal(err)
	}

	teacher := &model.Teacher{Name: "张老师", EmployeeNo: "T001", Subjects: []string{"数学"}, UnavailableSlots: []string{"1"}}
	if err := db.Create(teacher).Error; err != nil {
		t.Fatal(err)
	}
	class := &model.Class{Name: "一班", StudentCount: 40, Grade: "高一"}
	if err := db.Create(class).Error; err != nil {
		t.Fatal(err)
	}
	course := &model.Course{Name: "数学", Code: "MATH", Duration: 1}
	if err := db.Create(course).Error; err != nil {
		t.Fatal(err)
	}

	schedules := []model.Schedule{
		{Week: 1, DayOfWeek: 1, TimeSlotID: slot1.ID, ClassroomID: smallRoom.ID, TeacherID: teacher.ID, ClassID: class.ID, CourseID: course.ID},
		{Week: 1, DayOfWeek: 1, TimeSlotID: slot1.ID, ClassroomID: bigRoom.ID, TeacherID: teacher.ID, ClassID: class.ID, CourseID: course.ID},
	}
	if err := db.Create(&schedules).Error; err != nil {
		t.Fatal(err)
	}

	svc := newScheduleService(t, db)
	conflicts, err := svc.CheckConflicts(ctx)
	if err != nil {
		t.Fatalf("check conflicts: %v", err)
	}

	wantTypes := []string{
		constants.ConflictTeacherTime,
		constants.ConflictClassTime,
		constants.ConflictClassroomCap,
		constants.ConflictTeacherPref,
	}
	gotTypes := map[string]bool{}
	for _, c := range conflicts {
		gotTypes[c.Type] = true
	}
	for _, want := range wantTypes {
		if !gotTypes[want] {
			t.Errorf("missing conflict type %s in %v", want, conflicts)
		}
	}
}

func TestScheduleServiceGenerateAllowsParallelClasses(t *testing.T) {
	ctx := context.Background()
	db := newScheduleTestDB(t)

	slot := &model.TimeSlot{Code: "1", Name: "第一节", StartTime: "08:00", EndTime: "09:40"}
	if err := db.Create(slot).Error; err != nil {
		t.Fatal(err)
	}
	room1 := &model.Classroom{Code: "R301", Name: "301教室", Capacity: 50}
	room2 := &model.Classroom{Code: "R302", Name: "302教室", Capacity: 50}
	if err := db.Create(room1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(room2).Error; err != nil {
		t.Fatal(err)
	}
	teacher1 := &model.Teacher{Name: "张老师", EmployeeNo: "T001", Subjects: []string{"数学"}}
	teacher2 := &model.Teacher{Name: "李老师", EmployeeNo: "T002", Subjects: []string{"语文"}}
	if err := db.Create(teacher1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(teacher2).Error; err != nil {
		t.Fatal(err)
	}
	class1 := &model.Class{Name: "一班", StudentCount: 40, Grade: "高一"}
	class2 := &model.Class{Name: "二班", StudentCount: 40, Grade: "高一"}
	if err := db.Create(class1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(class2).Error; err != nil {
		t.Fatal(err)
	}
	course1 := &model.Course{Name: "数学", Code: "MATH", Duration: 1}
	course2 := &model.Course{Name: "语文", Code: "CHN", Duration: 1}
	if err := db.Create(course1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(course2).Error; err != nil {
		t.Fatal(err)
	}

	svc := newScheduleService(t, db)
	resp, err := svc.Generate(ctx, &dto.GenerateScheduleRequest{
		Weeks:         1,
		DaysPerWeek:   1,
		PeriodsPerDay: 1,
		Courses: []dto.CourseRequirement{
			{CourseID: course1.ID, WeeklyPeriods: 1, ClassID: class1.ID, TeacherID: teacher1.ID},
			{CourseID: course2.ID, WeeklyPeriods: 1, ClassID: class2.ID, TeacherID: teacher2.ID},
		},
		TeacherIDs:   []uint{teacher1.ID, teacher2.ID},
		ClassIDs:     []uint{class1.ID, class2.ID},
		ClassroomIDs: []uint{room1.ID, room2.ID},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if resp.Generated != 2 || len(resp.Schedules) != 2 {
		t.Fatalf("expected 2 generated schedules, got generated=%d schedules=%d", resp.Generated, len(resp.Schedules))
	}
	if len(resp.Conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %+v", resp.Conflicts)
	}
}

func TestScheduleServiceGenerateReplacesStaleWeeks(t *testing.T) {
	ctx := context.Background()
	db := newScheduleTestDB(t)

	slot := &model.TimeSlot{Code: "1", Name: "第一节", StartTime: "08:00", EndTime: "09:40"}
	if err := db.Create(slot).Error; err != nil {
		t.Fatal(err)
	}
	room := &model.Classroom{Code: "R301", Name: "301教室", Capacity: 50}
	if err := db.Create(room).Error; err != nil {
		t.Fatal(err)
	}
	teacher := &model.Teacher{Name: "张老师", EmployeeNo: "T001", Subjects: []string{"数学"}}
	if err := db.Create(teacher).Error; err != nil {
		t.Fatal(err)
	}
	class := &model.Class{Name: "一班", StudentCount: 40, Grade: "高一"}
	if err := db.Create(class).Error; err != nil {
		t.Fatal(err)
	}
	course := &model.Course{Name: "数学", Code: "MATH", Duration: 1}
	if err := db.Create(course).Error; err != nil {
		t.Fatal(err)
	}

	svc := newScheduleService(t, db)
	req := func(weeks int) *dto.GenerateScheduleRequest {
		return &dto.GenerateScheduleRequest{
			Weeks:         weeks,
			DaysPerWeek:   1,
			PeriodsPerDay: 1,
			Courses: []dto.CourseRequirement{
				{CourseID: course.ID, WeeklyPeriods: 1, ClassID: class.ID, TeacherID: teacher.ID},
			},
			TeacherIDs:   []uint{teacher.ID},
			ClassIDs:     []uint{class.ID},
			ClassroomIDs: []uint{room.ID},
		}
	}

	if _, err := svc.Generate(ctx, req(2)); err != nil {
		t.Fatalf("generate 2 weeks: %v", err)
	}
	if _, err := svc.Generate(ctx, req(1)); err != nil {
		t.Fatalf("generate 1 week: %v", err)
	}
	items, err := svc.List(ctx, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("list schedules: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 active schedule after regeneration, got %d", len(items))
	}
	if items[0].Week != 1 {
		t.Fatalf("expected only week 1 to remain, got week %d", items[0].Week)
	}
}
