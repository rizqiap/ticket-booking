package payment

import (
	"errors"
	"time"
)

// Payment errors shared by the handler, service, and gateway.
var (
	ErrInvalidPayment      = errors.New("invalid payment request")
	ErrBookingNotFound     = errors.New("booking not found")
	ErrBookingNotPayable   = errors.New("booking cannot be paid")
	ErrPaymentPreparing    = errors.New("payment is being prepared")
	ErrPaymentState        = errors.New("invalid payment state")
	ErrProviderUnavailable = errors.New("payment provider unavailable")
)

// Payment holds payment data and its associated Snap session.
type Payment struct {
	ID               int64
	BookingID        int64
	Amount           int64
	Status           string
	MidtransOrderID  string
	SnapToken        string
	RedirectURL      string
	BookingCreatedAt time.Time
}

// SnapResult holds the token and payment URL returned by Midtrans.
type SnapResult struct {
	Token       string
	RedirectURL string
}
