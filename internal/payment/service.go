package payment

import (
	"context"
	"math/big"
	"strings"
	"time"
)

// PaymentRepository defines payment reservation and Snap storage operations.
type PaymentRepository interface {
	Reserve(
		ctx context.Context,
		bookingID, userID int64,
	) (*Payment, bool, error)

	SaveSnap(
		ctx context.Context,
		paymentID int64,
		token, redirectURL string,
	) error

	SaveNotification(
		ctx context.Context,
		n Notification,
		grossAmount int64,
	) (int64, bool, error)

	ConfirmNotification(
		ctx context.Context,
		notificationID int64,
		paymentStatus PaymentStatus,
		bookingStatus BookingStatus,
	) error
}

// PaymentGateway defines transaction creation with the payment provider.
type PaymentGateway interface {
	CreateTransaction(
		orderID string,
		amount int64,
		bookingCreatedAt time.Time,
	) (SnapResult, error)

	VerifySignature(n Notification) bool
}

// Service handles payment business logic.
type Service struct {
	repo    PaymentRepository
	gateway PaymentGateway
}

// NewService creates a service with the payment repository and gateway.
func NewService(
	repo PaymentRepository,
	gateway PaymentGateway,
) *Service {
	return &Service{
		repo:    repo,
		gateway: gateway,
	}
}

// Create reserves a payment and creates or reuses its Snap session.
func (s *Service) Create(
	ctx context.Context,
	bookingID, userID int64,
) (*Payment, bool, error) {

	// Validate booking and user IDs before accessing storage.
	if bookingID <= 0 || userID <= 0 {
		return nil, false, ErrInvalidPayment
	}

	// Reserve one payment per booking.
	p, created, err := s.repo.Reserve(
		ctx, bookingID, userID,
	)
	if err != nil {
		return nil, false, err
	}

	// Reuse an existing URL, or report that another request is preparing it.
	if !created {
		if p.RedirectURL == "" {
			return nil, false, ErrPaymentPreparing
		}
		return p, false, nil
	}

	// Create the Midtrans transaction using the stored booking amount.
	snapResult, err := s.gateway.CreateTransaction(
		p.MidtransOrderID,
		p.Amount,
		p.BookingCreatedAt,
	)
	if err != nil {
		return nil, false, err
	}

	// Save the Snap token and URL for later requests.
	err = s.repo.SaveSnap(
		ctx,
		p.ID,
		snapResult.Token,
		snapResult.RedirectURL,
	)
	if err != nil {
		return nil, false, err
	}

	p.SnapToken = snapResult.Token
	p.RedirectURL = snapResult.RedirectURL

	return p, true, nil
}

func (s *Service) HandleNotification(
	ctx context.Context,
	n Notification,
) (bool, error) {

	// 1. Validate required fields.
	if n.OrderID == "" ||
		n.TransactionID == "" ||
		n.TransactionStatus == "" ||
		n.StatusCode == "" ||
		n.GrossAmount == "" ||
		n.SignatureKey == "" {
		return false, ErrInvalidNotification
	}

	// 2. Verify signature first.
	if !s.gateway.VerifySignature(n) {
		return false, ErrInvalidSignature
	}

	// 3. Check currency when present.
	if n.Currency != "" &&
		!strings.EqualFold(n.Currency, "IDR") {
		return false, ErrAmountMismatch
	}

	// 4. Parse money without floating-point rounding.
	amount, ok := new(big.Rat).SetString(
		n.GrossAmount,
	)

	if !ok ||
		amount.Sign() <= 0 ||
		!amount.IsInt() ||
		!amount.Num().IsInt64() {
		return false, ErrInvalidNotification
	}

	grossAmount := amount.Num().Int64()

	// 5. Persist the verified notification.
	notificationID, inserted, err := s.repo.SaveNotification(
		ctx,
		n,
		grossAmount,
	)
	if err != nil {
		return false, err
	}

	// 6. Map Midtrans status to local payment and booking statuses.
	paymentStatus, bookingStatus := mapMidtransStatus(
		n.TransactionStatus,
		n.FraudStatus,
	)
	if paymentStatus == PaymentSuccess && n.StatusCode != "200" {
		return inserted, nil
	}

	// 7. Apply the mapped statuses using the saved notification ID.
	if paymentStatus != PaymentPending {
		err = s.repo.ConfirmNotification(
			ctx,
			notificationID,
			paymentStatus,
			bookingStatus,
		)
		if err != nil {
			return false, err
		}
	}

	return inserted, nil
}

func mapMidtransStatus(
	transactionStatus string,
	fraudStatus string,
) (PaymentStatus, BookingStatus) {
	switch transactionStatus {
	case "settlement":
		if fraudStatus == "" || fraudStatus == "accept" {
			return PaymentSuccess, BookingConfirmed
		}

	case "capture":
		if fraudStatus == "accept" {
			return PaymentSuccess, BookingConfirmed
		}

	case "expire":
		return PaymentFailed, BookingExpired

	case "cancel", "failure":
		return PaymentFailed, BookingCancelled

	case "deny":
		return PaymentFailed, BookingPending
	}

	return PaymentPending, BookingPending
}
