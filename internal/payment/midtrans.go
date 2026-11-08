package payment

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
)

// MidtransGateway creates Snap transactions through the Midtrans client.
type MidtransGateway struct {
	client *snap.Client
}

// NewMidtransGateway creates a sandbox gateway with the server key.
func NewMidtransGateway(
	serverKey string,
) (*MidtransGateway, error) {
	if strings.TrimSpace(serverKey) == "" {
		return nil, errors.New(
			"midtrans server key is required",
		)
	}

	client := &snap.Client{}

	client.New(
		serverKey,
		midtrans.Sandbox,
	)

	return &MidtransGateway{
		client: client,
	}, nil
}

// CreateTransaction requests a Snap session with the booking's payment deadline.
func (g *MidtransGateway) CreateTransaction(
	orderID string,
	amount int64,
	BookingCreatedAt time.Time,
) (SnapResult, error) {
	wib := time.FixedZone("WIB", 7*60*60)

	// Start the ten-minute expiry at booking creation, not payment creation.
	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  orderID,
			GrossAmt: amount,
		},

		Expiry: &snap.ExpiryDetails{
			StartTime: BookingCreatedAt.
				In(wib).
				Format("2006-01-02 15:04:05 -0700"),
			Unit:     "minute",
			Duration: 10,
		},
	}

	response, midtransErr := g.client.CreateTransaction(req)

	if midtransErr != nil {
		return SnapResult{}, fmt.Errorf(
			"%w: %v",
			ErrProviderUnavailable,
			midtransErr,
		)
	}

	// Reject incomplete responses that cannot open a payment page.
	if response == nil ||
		response.Token == "" ||
		response.RedirectURL == "" {
		return SnapResult{}, ErrProviderUnavailable
	}

	return SnapResult{
		Token:       response.Token,
		RedirectURL: response.RedirectURL,
	}, nil
}
