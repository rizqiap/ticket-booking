package payment

import (
	"errors"
	"strings"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/snap"
)

type MidtransGateway struct {
	client *snap.Client
}

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
