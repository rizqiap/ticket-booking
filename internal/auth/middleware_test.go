package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	secret := "secret-for-testing-only"

	router := gin.New()

	router.GET("/admin",
		AuthMiddleware(secret),
		RequireRole("admin"),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	makeToken := func(role string, ExpiresAt time.Time) string {
		claims := AccessClaims{
			Role: role,
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "1",
				Issuer:    "ticket-booking",
				ExpiresAt: jwt.NewNumericDate(ExpiresAt),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}

		return signed
	}

	// Test cases
	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{
			name:       "No Token",
			token:      "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Invalid Token",
			token:      "invalid-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Customer Access Admin",
			token:      makeToken("customer", time.Now().Add(time.Hour)),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "Admin Access Admin",
			token:      makeToken("admin", time.Now().Add(time.Hour)),
			wantStatus: http.StatusOK,
		},
		{
			name:       "Expired Token",
			token:      makeToken("admin", time.Now().Add(-time.Hour)),
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				"/admin",
				nil,
			)

			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Errorf(
					"expected %d, got %d",
					tt.wantStatus,
					recorder.Code,
				)
			}
		})
	}
}
