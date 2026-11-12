package payment

import (
	"context"
	"errors"
	"testing"
)

func TestVerifySignatureUsesSeparateSigningKey(t *testing.T) {
	gateway, err := NewMidtransGateway("test-api-server-key", "test-signature-secret")
	if err != nil {
		t.Fatal(err)
	}
	n := Notification{
		OrderID: "BOOKING-6", StatusCode: "200", GrossAmount: "200000.00",
		SignatureKey: "7523a33d0d271d7cab68b8b1367eaa2db4b89cd74abdcd2d92ec13e94e73b7e34fb9d583d7607bceb844d9aa51d9eac4979c57c25a86729b1069add37b00e9d8",
	}
	if !gateway.VerifySignature(n) {
		t.Fatal("valid notification signed with the separate signing key was rejected")
	}
	n.GrossAmount = "200000"
	if gateway.VerifySignature(n) {
		t.Fatal("changed amount representation must invalidate the signature")
	}
	n.GrossAmount = "200000.00"
	n.SignatureKey = "not-hex"
	if gateway.VerifySignature(n) {
		t.Fatal("malformed signature must be rejected")
	}
}

func TestNewMidtransGatewayRequiresBothKeys(t *testing.T) {
	for _, keys := range [][2]string{{"", "signing-key"}, {"api-key", " "}} {
		if _, err := NewMidtransGateway(keys[0], keys[1]); err == nil {
			t.Fatal("missing API or signing key must be rejected")
		}
	}
}

type notificationRepositoryStub struct {
	PaymentRepository
	inserted      bool
	saveErr       error
	confirmErr    error
	confirmedID   int64
	paymentStatus PaymentStatus
	bookingStatus BookingStatus
}

func (r *notificationRepositoryStub) SaveNotification(context.Context, Notification, int64) (int64, bool, error) {
	return 77, r.inserted, r.saveErr
}

func (r *notificationRepositoryStub) ConfirmNotification(_ context.Context, id int64, paymentStatus PaymentStatus, bookingStatus BookingStatus) error {
	r.confirmedID, r.paymentStatus, r.bookingStatus = id, paymentStatus, bookingStatus
	return r.confirmErr
}

func TestServiceMapsNotificationBeforeConfirmation(t *testing.T) {
	gateway, err := NewMidtransGateway("test-api-key", "test-signature-secret")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		status, fraud string
		payment       PaymentStatus
		booking       BookingStatus
	}{
		{"settlement", "accept", "success", "confirmed"},
		{"capture", "accept", "success", "confirmed"},
		{"settlement", "", "success", "confirmed"},
		{"capture", "challenge", "", ""},
		{"capture", "", "", ""},
		{"settlement", "deny", "", ""},
		{"pending", "accept", "", ""},
		{"deny", "deny", "failed", "pending"},
		{"cancel", "", "failed", "cancelled"},
		{"failure", "", "failed", "cancelled"},
		{"expire", "", "failed", "expired"},
		{"refund", "accept", "", ""},
	} {
		t.Run(tt.status+tt.fraud, func(t *testing.T) {
			repo := &notificationRepositoryStub{inserted: false}
			n := Notification{OrderID: "BOOKING-6", TransactionID: "transaction", GrossAmount: "200000.00", StatusCode: "200",
				TransactionStatus: tt.status, FraudStatus: tt.fraud,
				SignatureKey: "7523a33d0d271d7cab68b8b1367eaa2db4b89cd74abdcd2d92ec13e94e73b7e34fb9d583d7607bceb844d9aa51d9eac4979c57c25a86729b1069add37b00e9d8"}
			inserted, err := NewService(repo, gateway).HandleNotification(context.Background(), n)
			if err != nil || inserted {
				t.Fatalf("inserted=%v err=%v", inserted, err)
			}
			if repo.paymentStatus != tt.payment || repo.bookingStatus != tt.booking {
				t.Fatalf("mapped %s/%s", repo.paymentStatus, repo.bookingStatus)
			}
			if tt.payment != "" && repo.confirmedID != 77 {
				t.Fatal("confirmation did not receive saved notification ID on retry")
			}
			if tt.payment == "" && repo.confirmedID != 0 {
				t.Fatal("nonfinal/unsupported notification was confirmed")
			}
		})
	}
}

func TestServicePropagatesNotificationStorageErrors(t *testing.T) {
	gateway, err := NewMidtransGateway("test-api-key", "test-signature-secret")
	if err != nil {
		t.Fatal(err)
	}
	n := Notification{OrderID: "BOOKING-6", TransactionID: "transaction", GrossAmount: "200000.00", StatusCode: "200", TransactionStatus: "settlement",
		SignatureKey: "7523a33d0d271d7cab68b8b1367eaa2db4b89cd74abdcd2d92ec13e94e73b7e34fb9d583d7607bceb844d9aa51d9eac4979c57c25a86729b1069add37b00e9d8"}
	want := errors.New("database failure")
	for _, saveFailure := range []bool{true, false} {
		repo := &notificationRepositoryStub{inserted: true}
		if saveFailure {
			repo.saveErr = want
		} else {
			repo.confirmErr = want
		}
		if _, err := NewService(repo, gateway).HandleNotification(context.Background(), n); !errors.Is(err, want) {
			t.Fatalf("got %v", err)
		}
		if saveFailure && repo.confirmedID != 0 {
			t.Fatal("confirmed notification after receipt failed")
		}
	}
}

func TestMapMidtransStatusDefaultsToPending(t *testing.T) {
	for _, status := range []string{"pending", "refund", "unknown"} {
		p, b := mapMidtransStatus(status, "accept")
		if p != PaymentPending || b != BookingPending {
			t.Fatalf("%s mapped to %s/%s", status, p, b)
		}
	}
}
