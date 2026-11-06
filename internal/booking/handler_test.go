package booking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
					string,
					CreateInput,
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
			key string,
			input CreateInput,
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
			message: "duplicate booking request",
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
					string,
					CreateInput,
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
