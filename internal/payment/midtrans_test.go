package payment

import "testing"

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
