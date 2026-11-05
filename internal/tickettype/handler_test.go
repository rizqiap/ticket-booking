package tickettype

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
)

type listRepository struct {
	ticketTypeRepository
	list func(context.Context, int64) ([]TicketType, error)
}

func (r listRepository) ListByEventID(ctx context.Context, id int64) ([]TicketType, error) {
	return r.list(ctx, id)
}

func TestListTicketTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name, id string
		tickets  []TicketType
		repoErr  error
		status   int
		message  string
	}{
		{"list", "7", []TicketType{{ID: 12, EventID: 7, Name: "VIP", Price: 150000, TotalStock: 100, AvailableStock: 80}}, nil, 200, ""},
		{"empty list", "7", []TicketType{}, nil, 200, ""},
		{"invalid ID", "abc", nil, nil, 400, "invalid event id"},
		{"zero ID", "0", nil, nil, 400, "invalid event id"},
		{"negative ID", "-1", nil, nil, 400, "invalid event id"},
		{"overflow ID", "9223372036854775808", nil, nil, 400, "invalid event id"},
		{"missing or draft event", "7", nil, fmt.Errorf("lookup: %w", ErrEventNotFound), 404, "event not found"},
		{"database failure", "7", nil, errors.New("private database details"), 500, "internal server error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			called := false
			repo := listRepository{list: func(gotCtx context.Context, id int64) ([]TicketType, error) {
				called = true
				if gotCtx != ctx || id != 7 {
					t.Fatalf("unexpected lookup: context=%v, ID=%d", gotCtx, id)
				}
				return tt.tickets, tt.repoErr
			}}
			router := gin.New()
			// A public GET must not execute either admin middleware.
			blocked := func(c *gin.Context) { t.Error("public list requires admin auth"); c.AbortWithStatus(401) }
			RegisterRoutes(router.Group("/api/v1"), NewHandler(NewService(repo)), blocked, blocked)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/events/"+tt.id+"/ticket-types", nil).WithContext(ctx)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tt.status || called != (tt.status != 400) {
				t.Fatalf("unexpected status/call: %d, called=%v: %s", response.Code, called, response.Body.String())
			}
			var body struct {
				Data  json.RawMessage `json:"data"`
				Error string          `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if tt.status != 200 {
				if body.Error != tt.message || body.Data != nil {
					t.Fatalf("unexpected error body: %s", response.Body.String())
				}
				return
			}
			var got []TicketType
			if err := json.Unmarshal(body.Data, &got); err != nil || !reflect.DeepEqual(got, tt.tickets) || body.Error != "" {
				t.Fatalf("unexpected ticket list: %s", response.Body.String())
			}
		})
	}
}
