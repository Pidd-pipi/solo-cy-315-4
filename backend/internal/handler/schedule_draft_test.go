package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/gbschedule/gbschedule/internal/dto"
	"github.com/gbschedule/gbschedule/internal/model"
)

type stubDraftService struct {
	created *dto.CreateDraftResponse
	drafts  map[uint]*dto.DraftResponse
	results map[uint]*dto.DraftResultResponse
	list    []dto.DraftResponse
	listErr error
	getErr  error
}

func (s *stubDraftService) Submit(ctx context.Context, req *dto.GenerateScheduleRequest) (*dto.CreateDraftResponse, error) {
	return s.created, nil
}

func (s *stubDraftService) Get(ctx context.Context, id uint) (*dto.DraftResponse, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.drafts[id], nil
}

func (s *stubDraftService) GetResult(ctx context.Context, id uint) (*dto.DraftResultResponse, error) {
	return s.results[id], nil
}

func (s *stubDraftService) List(ctx context.Context, status string, page, pageSize int) ([]dto.DraftResponse, int64, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	return s.list, int64(len(s.list)), nil
}

func (s *stubDraftService) Cancel(ctx context.Context, id uint) (*dto.DraftResponse, error) {
	return s.drafts[id], nil
}

func (s *stubDraftService) Start(ctx context.Context) {}
func (s *stubDraftService) Shutdown()                 {}

func newDraftRouter(svc *stubDraftService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := gin.New()
	h := NewScheduleDraftHandler(svc, logger)
	r.POST("/api/v1/schedule-drafts", h.Create)
	r.GET("/api/v1/schedule-drafts", h.List)
	r.GET("/api/v1/schedule-drafts/:id", h.Get)
	r.GET("/api/v1/schedule-drafts/:id/result", h.GetResult)
	r.POST("/api/v1/schedule-drafts/:id/cancel", h.Cancel)
	return r
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return body
}

func TestDraftCreateReturnsAcceptedWithID(t *testing.T) {
	svc := &stubDraftService{created: &dto.CreateDraftResponse{ID: 42, Status: model.DraftStatusQueued}}
	r := newDraftRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule-drafts", bytes.NewBufferString(`{
		"weeks": 2, "days_per_week": 5, "periods_per_day": 4,
		"courses": [{"course_id": 1, "weekly_periods": 3}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", w.Code, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["code"].(float64) != 0 {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["id"].(float64) != 42 || data["status"] != "queued" {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestDraftCreateRejectsInvalidBody(t *testing.T) {
	svc := &stubDraftService{created: &dto.CreateDraftResponse{ID: 1}}
	r := newDraftRouter(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/schedule-drafts", bytes.NewBufferString(`{"weeks": 0}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDraftGetAndList(t *testing.T) {
	svc := &stubDraftService{
		drafts: map[uint]*dto.DraftResponse{
			7: {ID: 7, Status: model.DraftStatusRunning, Progress: 50, Conflicts: []dto.ConflictResponse{}},
		},
		list: []dto.DraftResponse{{ID: 7, Status: "running"}},
	}
	r := newDraftRouter(svc)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/schedule-drafts/7", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("get expected 200, got %d", w.Code)
	}
	body := decodeBody(t, w)
	data := body["data"].(map[string]any)
	if data["status"] != "running" || data["progress"].(float64) != 50 {
		t.Fatalf("unexpected draft body: %+v", data)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/schedule-drafts?page=1&page_size=10", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d", w2.Code)
	}
}

func TestDraftResultEndpoint(t *testing.T) {
	svc := &stubDraftService{
		results: map[uint]*dto.DraftResultResponse{
			9: {
				DraftResponse: dto.DraftResponse{ID: 9, Status: model.DraftStatusSucceeded},
				Result: &dto.DraftResultData{
					Schedules: []dto.ScheduleResponse{{ID: 1}},
					Conflicts: []dto.ConflictResponse{},
				},
			},
		},
	}
	r := newDraftRouter(svc)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/schedule-drafts/9/result", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}
