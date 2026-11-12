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
	ErrInvalidNotification = errors.New("invalid payment notification")
	ErrInvalidSignature    = errors.New("invalid midtrans signature")
	ErrPaymentNotFound     = errors.New("payment not found")
	ErrAmountMismatch      = errors.New("payment amount mismatch")
)

type PaymentStatus string

const (
	PaymentPending PaymentStatus = "pending"
	PaymentSuccess PaymentStatus = "success"
	PaymentFailed  PaymentStatus = "failed"
)

type BookingStatus string

const (
	BookingPending   BookingStatus = "pending"
	BookingConfirmed BookingStatus = "confirmed"
	BookingCancelled BookingStatus = "cancelled"
	BookingExpired   BookingStatus = "expired"
)

// Payment holds payment data and its associated Snap session.
type Payment struct {
	ID               int64
	BookingID        int64
	Amount           int64
	Status           PaymentStatus
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

type Notification struct {
	OrderID           string `json:"order_id"`
	TransactionID     string `json:"transaction_id"`
	TransactionStatus string `json:"transaction_status"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	FraudStatus       string `json:"fraud_status"`
	Currency          string `json:"currency"`
	SignatureKey      string `json:"signature_key"`
}
