package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/SeladaKeju/ticket-booking/internal/auth"
	"github.com/gin-gonic/gin"
)

func bookingRequest(body, key string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/bookings",
		strings.NewReader(body),
	)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", key)
	c.Set("user_id", int64(7))
	return c, response
}

func TestCreateHandlerRequestValidation(t *testing.T) {
	for _, tt := range []struct {
		name    string
		body    string
		key     string
		message string
	}{
		{
			name:    "malformed JSON",
			body:    `{`,
			key:     "booking-001",
			message: "invalid request data",
		},
		{
			name:    "missing ticket ID",
			body:    `{"quantity":2}`,
			key:     "booking-001",
			message: "invalid request data",
		},
		{
			name:    "missing quantity",
			body:    `{"ticket_type_id":12}`,
			key:     "booking-001",
			message: "invalid request data",
		},
		{
			name:    "wrong field type",
			body:    `{"ticket_type_id":12,"quantity":"two"}`,
			key:     "booking-001",
			message: "invalid request data",
		},
		{
			name:    "missing key header",
			body:    `{"ticket_type_id":12,"quantity":2}`,
			message: "invalid booking data",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := repositoryStub{
				create: func(
					context.Context,
					int64,
					CreateInput,
					string,
				) (*Booking, error) {
					t.Fatal("invalid request must not reach the repository")
					return nil, nil
				},
			}
			c, response := bookingRequest(tt.body, tt.key)
			NewHandler(NewService(repo)).Create(c)

			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusBadRequest || body.Error != tt.message {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body)
			}
		})
	}
}

func TestCreateHandlerSuccessResponse(t *testing.T) {
	want := &Booking{
		ID:             1,
		UserID:         7,
		TicketTypeID:   12,
		Quantity:       2,
		TotalPrice:     300000,
		Status:         "pending",
		IdempotencyKey: "booking-001",
	}
	c, response := bookingRequest(
		`{"ticket_type_id":12,"quantity":2,"user_id":999,"total_price":1}`,
		"booking-001",
	)
	repo := repositoryStub{
		create: func(
			ctx context.Context,
			userID int64,
			input CreateInput,
			key string,
		) (*Booking, error) {
			if ctx != c.Request.Context() ||
				userID != 7 ||
				key != "booking-001" ||
				input != (CreateInput{TicketTypeID: 12, Quantity: 2}) {
				t.Fatal("handler forwarded incorrect booking arguments")
			}
			return want, nil
		},
	}
	NewHandler(NewService(repo)).Create(c)

	var body struct {
		Message string   `json:"message"`
		Data    *Booking `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusCreated ||
		body.Message != "booking created successfully" ||
		body.Data == nil || *body.Data != *want {
		t.Fatalf("unexpected response: %d %s", response.Code, response.Body)
	}
}

func TestCreateHandlerErrorHandling(t *testing.T) {
	for _, tt := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{
			name:    "invalid booking",
			err:     ErrInvalidBooking,
			status:  http.StatusBadRequest,
			message: "invalid booking data",
		},
		{
			name:    "user not found",
			err:     ErrUserNotFound,
			status:  http.StatusUnauthorized,
			message: "unauthorized",
		},
		{
			name:    "ticket unavailable",
			err:     ErrTicketUnavailable,
			status:  http.StatusConflict,
			message: "ticket unavailable",
		},
		{
			name:    "duplicate booking",
			err:     fmt.Errorf("create: %w", ErrDuplicateBooking),
			status:  http.StatusConflict,
			message: "idempotency key used for different request",
		},

		{
			name:    "internal error hides details",
			err:     errors.New("private database details"),
			status:  http.StatusInternalServerError,
			message: "internal server error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := repositoryStub{
				create: func(
					context.Context,
					int64,
					CreateInput,
					string,
				) (*Booking, error) {
					return nil, tt.err
				},
			}
			c, response := bookingRequest(
				`{"ticket_type_id":12,"quantity":2}`,
				"booking-001",
			)
			NewHandler(NewService(repo)).Create(c)

			var body struct {
				Error string          `json:"error"`
				Data  json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != tt.status || body.Error != tt.message || body.Data != nil {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body)
			}
		})
	}
}

func TestListHandler(t *testing.T) {
	want := []Booking{{ID: 12, UserID: 7, Status: "confirmed"}}
	for _, tt := range []struct {
		name     string
		query    string
		userID   int64
		bookings []Booking
		repoErr  error
		status   int
		message  string
		page     int
		limit    int
	}{
		{
			name: "defaults", userID: 7, bookings: want,
			status: 200, page: 1, limit: 10,
		},
		{
			name:  "pagination ignores supplied user ID",
			query: "?page=2&limit=2&user_id=999", userID: 7, bookings: want,
			status: 200, page: 2, limit: 2,
		},
		{
			name: "empty history", userID: 7, bookings: []Booking{},
			status: 200, page: 1, limit: 10,
		},
		{
			name: "invalid page", query: "?page=0", userID: 7,
			status: 400, message: "invalid pagination parameters",
		},
		{
			name: "non-numeric page", query: "?page=abc", userID: 7,
			status: 400, message: "invalid pagination parameters",
		},
		{
			name: "page above limit", query: "?page=100001", userID: 7,
			status: 400, message: "invalid pagination parameters",
		},
		{
			name: "invalid limit", query: "?limit=101", userID: 7,
			status: 400, message: "invalid pagination parameters",
		},
		{
			name: "zero limit", query: "?limit=0", userID: 7,
			status: 400, message: "invalid pagination parameters",
		},
		{
			name: "non-numeric limit", query: "?limit=abc", userID: 7,
			status: 400, message: "invalid pagination parameters",
		},
		{
			name: "missing identity", status: 401, message: "unauthorized",
		},
		{
			name: "internal error hides details", userID: 7,
			repoErr: errors.New("private database details"),
			status:  500, message: "internal server error", page: 1, limit: 10,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			repo := repositoryStub{
				listByUserID: func(
					_ context.Context,
					userID int64,
					limit, offset int,
				) ([]Booking, error) {
					called = true
					if userID != 7 || limit != tt.limit || offset != (tt.page-1)*tt.limit {
						t.Fatal("handler used incorrect user or pagination")
					}
					return tt.bookings, tt.repoErr
				},
			}
			c, response := bookingRequest("", "")
			c.Set("user_id", tt.userID)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/bookings"+tt.query, nil)
			NewHandler(NewService(repo)).List(c)
			if response.Code != tt.status || called != (tt.status == 200 || tt.status == 500) {
				t.Fatalf("status=%d called=%v body=%s", response.Code, called, response.Body)
			}
			var body struct {
				Data       []Booking `json:"data"`
				Error      string    `json:"error"`
				Pagination struct {
					Page  int `json:"page"`
					Limit int `json:"limit"`
				} `json:"pagination"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error != tt.message {
				t.Fatalf("unexpected error: %s", response.Body)
			}
			if tt.status == 200 {
				if !reflect.DeepEqual(body.Data, tt.bookings) ||
					body.Pagination.Page != tt.page || body.Pagination.Limit != tt.limit {
					t.Fatalf("unexpected history response: %s", response.Body)
				}
			} else if body.Data != nil {
				t.Fatalf("error response included booking data: %s", response.Body)
			}
		})
	}
}

func TestBookingReadRoutesRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router.Group("/api/v1"), NewHandler(nil), auth.AuthMiddleware("test-secret"))
	for _, path := range []string{"/api/v1/bookings", "/api/v1/bookings/12"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("booking read without token returned %d", response.Code)
			}
		})
	}
}

func TestGetBookingDetailHandler(t *testing.T) {
	want := &Booking{
		ID: 12, UserID: 7, TicketTypeID: 1, Quantity: 2,
		TotalPrice: 300000, Status: "pending", IdempotencyKey: "booking-001",
	}
	for _, tt := range []struct {
		name    string
		id      string
		userID  int64
		repoErr error
		status  int
		message string
	}{
		{
			name: "detail ignores user ID from query", id: "12", userID: 7, status: 200,
		},
		{
			name: "invalid ID", id: "abc", userID: 7,
			status: 400, message: "invalid booking id",
		},
		{
			name: "zero ID", id: "0", userID: 7,
			status: 400, message: "invalid booking id",
		},
		{
			name: "negative ID", id: "-1", userID: 7,
			status: 400, message: "invalid booking id",
		},
		{
			name: "overflow ID", id: "9223372036854775808", userID: 7,
			status: 400, message: "invalid booking id",
		},
		{
			name: "missing identity", id: "12", status: 401, message: "unauthorized",
		},
		{
			name: "missing or other user's booking", id: "12", userID: 7,
			repoErr: fmt.Errorf("lookup: %w", errBookingNotFound),
			status:  404, message: "booking not found",
		},
		{
			name: "internal error hides details", id: "12", userID: 7,
			repoErr: errors.New("private database details"),
			status:  500, message: "internal server error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			repo := repositoryStub{
				getByID: func(_ context.Context, userID, id int64) (*Booking, error) {
					called = true
					if userID != 7 || id != 12 {
						t.Fatal("handler used incorrect user or booking ID")
					}
					if tt.repoErr != nil {
						return nil, tt.repoErr
					}
					return want, nil
				},
			}
			c, response := bookingRequest("", "")
			c.Set("user_id", tt.userID)
			c.Params = gin.Params{{Key: "id", Value: tt.id}}
			c.Request = httptest.NewRequest(
				http.MethodGet, "/api/v1/bookings/"+tt.id+"?user_id=999", nil,
			)
			NewHandler(NewService(repo)).GetByID(c)
			wantCalled := tt.status == 200 || tt.status == 404 || tt.status == 500
			if response.Code != tt.status || called != wantCalled {
				t.Fatalf("status=%d called=%v body=%s", response.Code, called, response.Body)
			}
			var body struct {
				Data  *Booking `json:"data"`
				Error string   `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error != tt.message ||
				(tt.status == 200 && (body.Data == nil || *body.Data != *want)) ||
				(tt.status != 200 && body.Data != nil) {
				t.Fatalf("unexpected detail response: %s", response.Body)
			}
		})
	}
}
