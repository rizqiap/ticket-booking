package event

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SeladaKeju/ticket-booking/internal/auth"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Verify that every write route uses the real auth and role middleware.
func TestEventRoutesAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "event-route-test-secret"
	want := &Event{ID: 42, Status: "published"}
	repo := stubRepository{
		create:  func(context.Context, string, string, string, time.Time) (*Event, error) { return want, nil },
		getByID: func(context.Context, int64) (*Event, error) { return want, nil },
		list:    func(context.Context, int, int) ([]Event, error) { return []Event{*want}, nil },
		update:  func(context.Context, int64, UpdateEventInput) (*Event, error) { return want, nil },
		publish: func(context.Context, int64) (*Event, error) { return want, nil },
	}
	router := gin.New()
	RegisterRoutes(router.Group("/api/v1"), NewHandler(NewService(repo)), auth.AuthMiddleware(secret), auth.RequireRole("admin"))
	makeToken := func(role string) string {
		t.Helper()
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, auth.AccessClaims{
			Role: role,
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: "1", Issuer: "ticket-booking", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		}).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	for _, route := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/events", `{"title":"Concert","location":"Jakarta","starts_at":"2099-01-01T00:00:00Z"}`, http.StatusCreated},
		{"PATCH", "/events/42", `{"title":"Updated"}`, http.StatusOK},
		{"PATCH", "/events/42/publish", "", http.StatusOK},
	} {
		for _, caller := range []struct {
			name, token string
			status      int
		}{
			{"anonymous", "", http.StatusUnauthorized},
			{"customer", makeToken("customer"), http.StatusForbidden},
			{"admin", makeToken("admin"), route.status},
		} {
			t.Run(route.method+route.path+"/"+caller.name, func(t *testing.T) {
				req := httptest.NewRequest(route.method, "/api/v1"+route.path, strings.NewReader(route.body))
				req.Header.Set("Content-Type", "application/json")
				if caller.token != "" {
					req.Header.Set("Authorization", "Bearer "+caller.token)
				}
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				if response.Code != caller.status {
					t.Fatalf("want %d, got %d: %s", caller.status, response.Code, response.Body.String())
				}
			})
		}
	}
	for _, path := range []string{"/api/v1/events", "/api/v1/events/42"} {
		t.Run("public"+path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK {
				t.Fatalf("public route requires auth: %d", response.Code)
			}
		})
	}
}
