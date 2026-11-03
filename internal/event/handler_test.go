package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestEventHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name, method, path, body string
		repoError                error
		status                   int
		message                  string
	}{
		{"create", "POST", "/events", `{"title":"Concert","location":"Jakarta","starts_at":"2099-01-01T00:00:00Z"}`, nil, 201, "event created successfully"},
		{"create missing fields", "POST", "/events", `{}`, nil, 400, "invalid request data"},
		{"create malformed JSON", "POST", "/events", `{`, nil, 400, "invalid request data"},
		{"create invalid date", "POST", "/events", `{"title":"Concert","location":"Jakarta","starts_at":"yesterday"}`, nil, 400, "invalid request data"},
		{"create past event", "POST", "/events", `{"title":"Concert","location":"Jakarta","starts_at":"2000-01-01T00:00:00Z"}`, nil, 400, "invalid event data"},
		{"create database failure", "POST", "/events", `{"title":"Concert","location":"Jakarta","starts_at":"2099-01-01T00:00:00Z"}`, errors.New("private database details"), 500, "internal server error"},
		{"get", "GET", "/events/42", "", nil, 200, ""},
		{"get invalid ID", "GET", "/events/abc", "", nil, 400, "invalid event id"},
		{"get zero ID", "GET", "/events/0", "", nil, 400, "invalid event id"},
		{"get missing event", "GET", "/events/42", "", fmt.Errorf("lookup: %w", ErrEventNotFound), 404, "event not found"},
		{"get database failure", "GET", "/events/42", "", errors.New("private database details"), 500, "internal server error"},
		{"list", "GET", "/events", "", nil, 200, ""},
		{"list database failure", "GET", "/events", "", errors.New("private database details"), 500, "internal server error"},
		{"update", "PATCH", "/events/42", `{"title":"New title"}`, nil, 200, "event updated successfully"},
		{"update invalid ID", "PATCH", "/events/-1", `{}`, nil, 400, "invalid event id"},
		{"update malformed JSON", "PATCH", "/events/42", `{`, nil, 400, "invalid request data"},
		{"update empty body", "PATCH", "/events/42", `{}`, nil, 400, "invalid event data"},
		{"update null field", "PATCH", "/events/42", `{"title":null}`, nil, 400, "invalid event data"},
		{"update blank title", "PATCH", "/events/42", `{"title":"  "}`, nil, 400, "invalid event data"},
		{"update missing event", "PATCH", "/events/42", `{"title":"New title"}`, fmt.Errorf("update: %w", ErrEventNotFound), 404, "event not found"},
		{"update database failure", "PATCH", "/events/42", `{"title":"New title"}`, errors.New("private database details"), 500, "internal server error"},
		{"publish", "PATCH", "/events/42/publish", "", nil, 200, "event published successfully"},
		{"publish invalid ID", "PATCH", "/events/abc/publish", "", nil, 400, "invalid event id"},
		{"publish missing event", "PATCH", "/events/42/publish", "", fmt.Errorf("publish: %w", ErrEventNotFound), 404, "event not found"},
		{"publish conflict", "PATCH", "/events/42/publish", "", fmt.Errorf("publish: %w", ErrEventCannotPublish), 409, "event cannot be published"},
		{"publish database failure", "PATCH", "/events/42/publish", "", errors.New("private database details"), 500, "internal server error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			want := &Event{ID: 42, Title: "Concert", Status: "published"}
			checkID := func(id int64) {
				t.Helper()
				if id != 42 {
					t.Fatalf("wrong ID: %d", id)
				}
			}
			repo := stubRepository{
				create:  func(context.Context, string, string, string, time.Time) (*Event, error) { return want, tt.repoError },
				getByID: func(ctx context.Context, id int64) (*Event, error) { checkID(id); return want, tt.repoError },
				list:    func(ctx context.Context, limit, offset int) ([]Event, error) { return []Event{*want}, tt.repoError },
				update: func(ctx context.Context, id int64, input UpdateEventInput) (*Event, error) {
					checkID(id)
					return want, tt.repoError
				},
				publish: func(ctx context.Context, id int64) (*Event, error) { checkID(id); return want, tt.repoError },
			}
			router := gin.New()
			RegisterRoutes(router.Group(""), NewHandler(NewService(repo)), func(c *gin.Context) { c.Next() }, func(c *gin.Context) { c.Next() })
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tt.status {
				t.Fatalf("want %d, got %d: %s", tt.status, response.Code, response.Body.String())
			}
			var body struct {
				Data    json.RawMessage `json:"data"`
				Message string          `json:"message"`
				Error   string          `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if tt.status >= 400 {
				if body.Error != tt.message || body.Data != nil {
					t.Fatalf("unexpected error response: %s", response.Body.String())
				}
			} else {
				if body.Message != tt.message || body.Error != "" {
					t.Fatalf("unexpected success response: %s", response.Body.String())
				}
				if tt.name == "list" {
					var events []Event
					if err := json.Unmarshal(body.Data, &events); err != nil || len(events) != 1 || events[0].ID != 42 {
						t.Fatalf("unexpected list: %s", body.Data)
					}
				} else {
					var event Event
					if err := json.Unmarshal(body.Data, &event); err != nil || event.ID != 42 {
						t.Fatalf("unexpected event: %s", body.Data)
					}
				}
			}
		})
	}
}

func TestListPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		query                       string
		page, limit, offset, status int
	}{
		{"", 1, 10, 0, 200},
		{"?page=3&limit=20", 3, 20, 40, 200},
		{"?page=100000&limit=100", 100000, 100, 9999900, 200},
		{"?page=0", 0, 0, 0, 400},
		{"?page=100001", 0, 0, 0, 400},
		{"?page=abc", 0, 0, 0, 400},
		{"?limit=0", 0, 0, 0, 400},
		{"?limit=101", 0, 0, 0, 400},
		{"?limit=abc", 0, 0, 0, 400},
	} {
		t.Run(tt.query, func(t *testing.T) {
			called := false
			repo := stubRepository{list: func(ctx context.Context, limit, offset int) ([]Event, error) {
				called = true
				if limit != tt.limit || offset != tt.offset {
					t.Fatalf("pagination: limit=%d offset=%d", limit, offset)
				}
				return []Event{}, nil
			}}
			router := gin.New()
			router.GET("/events", NewHandler(NewService(repo)).List)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", "/events"+tt.query, nil))
			if response.Code != tt.status || called != (tt.status == 200) {
				t.Fatalf("status=%d called=%v: %s", response.Code, called, response.Body.String())
			}
			if tt.status == 200 {
				var body struct {
					Data       json.RawMessage `json:"data"`
					Pagination struct {
						Page  int `json:"page"`
						Limit int `json:"limit"`
					} `json:"pagination"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if string(body.Data) != "[]" || body.Pagination.Page != tt.page || body.Pagination.Limit != tt.limit {
					t.Fatalf("unexpected pagination response: %s", response.Body.String())
				}
			}
		})
	}
}
