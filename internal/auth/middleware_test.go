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

func TestAuthMiddlewareTokenValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "secret-for-testing-only"
	for _, tt := range []struct {
		name   string
		change func(*AccessClaims)
		key    string
		method jwt.SigningMethod
	}{
		{name: "wrong signature", key: "different-secret"},
		{name: "wrong algorithm", method: jwt.SigningMethodHS384},
		{name: "wrong issuer", change: func(c *AccessClaims) { c.Issuer = "other-app" }},
		{name: "missing expiry", change: func(c *AccessClaims) { c.ExpiresAt = nil }},
		{name: "missing subject", change: func(c *AccessClaims) { c.Subject = "" }},
		{name: "non-numeric subject", change: func(c *AccessClaims) { c.Subject = "abc" }},
		{name: "zero subject", change: func(c *AccessClaims) { c.Subject = "0" }},
		{name: "negative subject", change: func(c *AccessClaims) { c.Subject = "-1" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			claims := AccessClaims{
				Role: "admin",
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: "7", Issuer: "ticket-booking",
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				},
			}
			if tt.change != nil {
				tt.change(&claims)
			}
			key, method := tt.key, tt.method
			if key == "" {
				key = secret
			}
			if method == nil {
				method = jwt.SigningMethodHS256
			}
			signed, err := jwt.NewWithClaims(method, claims).SignedString([]byte(key))
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.GET("/private", AuthMiddleware(secret), func(c *gin.Context) {
				t.Error("rejected token reached protected handler")
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", "Bearer "+signed)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("want 401, got %d: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestAuthMiddlewareStoresIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "secret-for-testing-only"
	signed, err := NewService(nil, secret).signAccessToken(&User{ID: 7, Role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name   string
		header string
		want   int
	}{
		{"valid bearer", "Bearer " + signed, http.StatusOK},
		{"case insensitive scheme", "bEaReR " + signed, http.StatusOK},
		{"wrong scheme", "Basic " + signed, http.StatusUnauthorized},
		{"missing token", "Bearer", http.StatusUnauthorized},
		{"extra header field", "Bearer " + signed + " extra", http.StatusUnauthorized},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			router := gin.New()
			router.GET("/private", AuthMiddleware(secret), func(c *gin.Context) {
				called = true
				if c.GetInt64("user_id") != 7 || c.GetString("role") != "admin" {
					t.Error("verified identity was not stored in Gin context")
				}
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			req.Header.Set("Authorization", tt.header)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			valid := tt.want == http.StatusOK
			if recorder.Code != tt.want || called != valid {
				t.Fatalf("want status %d, called=%v; got %d, called=%v", tt.want, valid, recorder.Code, called)
			}
		})
	}
}
