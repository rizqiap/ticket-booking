package payment

import (
	"context"
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
}

// PaymentGateway defines transaction creation with the payment provider.
type PaymentGateway interface {
	CreateTransaction(
		orderID string,
		amount int64,
		bookingCreatedAt time.Time,
	) (SnapResult, error)
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
