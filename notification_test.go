package doku

import (
	"net/http"
	"strings"
	"testing"
)

// TestVerifyNotificationSignature_Valid: a notification with the correct
// signature verifies. Tampered or missing fields don't.
func TestVerifyNotificationSignature_Valid(t *testing.T) {
	const path = "/api/doku/notifications"
	const secret = "secret-xyz"
	body := []byte(`{"order":{"invoice_number":"INV-001","amount":10000},"transaction":{"status":"SUCCESS"}}`)

	clientID := "M-123"
	requestID := "req-abc"
	requestTimestamp := "2024-03-26T09:01:41Z"
	sig := SignNonSnap(NonSnapSignatureInput{
		ClientID:         clientID,
		RequestID:        requestID,
		RequestTimestamp: requestTimestamp,
		RequestTarget:    path,
		Body:             string(body),
		SecretKey:        secret,
	})

	h := http.Header{}
	h.Set("Client-Id", clientID)
	h.Set("Request-Id", requestID)
	h.Set("Request-Timestamp", requestTimestamp)
	h.Set("Signature", sig)
	if !VerifyNotificationSignature(h, body, path, secret) {
		t.Fatalf("valid signature did not verify")
	}
}

func TestVerifyNotificationSignature_TamperedBody(t *testing.T) {
	const path = "/api/doku/notifications"
	const secret = "secret-xyz"
	body := []byte(`{"order":{"invoice_number":"INV-001","amount":10000},"transaction":{"status":"SUCCESS"}}`)
	tampered := []byte(`{"order":{"invoice_number":"INV-001","amount":99999},"transaction":{"status":"SUCCESS"}}`)
	sig := SignNonSnap(NonSnapSignatureInput{
		ClientID:         "M-123",
		RequestID:        "req-abc",
		RequestTimestamp: "2024-03-26T09:01:41Z",
		RequestTarget:    path,
		Body:             string(body),
		SecretKey:        secret,
	})
	h := http.Header{}
	h.Set("Client-Id", "M-123")
	h.Set("Request-Id", "req-abc")
	h.Set("Request-Timestamp", "2024-03-26T09:01:41Z")
	h.Set("Signature", sig)
	if VerifyNotificationSignature(h, tampered, path, secret) {
		t.Fatalf("tampered body must NOT verify")
	}
}

func TestVerifyNotificationSignature_TamperedSignature(t *testing.T) {
	const path = "/api/doku/notifications"
	const secret = "secret-xyz"
	body := []byte(`{"order":{"invoice_number":"INV-001","amount":10000},"transaction":{"status":"SUCCESS"}}`)
	h := http.Header{}
	h.Set("Client-Id", "M-123")
	h.Set("Request-Id", "req-abc")
	h.Set("Request-Timestamp", "2024-03-26T09:01:41Z")
	h.Set("Signature", "HMACSHA256=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if VerifyNotificationSignature(h, body, path, secret) {
		t.Fatalf("bogus signature must NOT verify")
	}
}

func TestVerifyNotificationSignature_MissingHeader(t *testing.T) {
	const path = "/api/doku/notifications"
	body := []byte(`{}`)
	h := http.Header{}
	h.Set("Client-Id", "M-123")
	h.Set("Request-Id", "req-abc")
	// missing Request-Timestamp and Signature
	if VerifyNotificationSignature(h, body, path, "secret") {
		t.Fatalf("missing headers must NOT verify")
	}
}

func TestVerifyNotificationSignature_WrongSecret(t *testing.T) {
	const path = "/api/doku/notifications"
	body := []byte(`{}`)
	sig := SignNonSnap(NonSnapSignatureInput{
		ClientID:         "M-123",
		RequestID:        "req-abc",
		RequestTimestamp: "2024-03-26T09:01:41Z",
		RequestTarget:    path,
		Body:             string(body),
		SecretKey:        "correct",
	})
	h := http.Header{}
	h.Set("Client-Id", "M-123")
	h.Set("Request-Id", "req-abc")
	h.Set("Request-Timestamp", "2024-03-26T09:01:41Z")
	h.Set("Signature", sig)
	if VerifyNotificationSignature(h, body, path, "wrong") {
		t.Fatalf("wrong secret must NOT verify")
	}
}

func TestVerifyNotificationSignature_HeaderCasing(t *testing.T) {
	const path = "/api/doku/notifications"
	const secret = "secret-xyz"
	body := []byte(`{}`)
	sig := SignNonSnap(NonSnapSignatureInput{
		ClientID:         "M-123",
		RequestID:        "req-abc",
		RequestTimestamp: "2024-03-26T09:01:41Z",
		RequestTarget:    path,
		Body:             string(body),
		SecretKey:        secret,
	})
	// Various casings should all verify.
	for _, m := range []http.Header{
		func() http.Header {
			h := http.Header{}
			h.Set("client-id", "M-123")
			h.Set("REQUEST-ID", "req-abc")
			h.Set("Request-Timestamp", "2024-03-26T09:01:41Z")
			h.Set("signature", sig)
			return h
		}(),
		func() http.Header {
			h := http.Header{}
			h.Set("Client-Id", "M-123")
			h.Set("Request-Id", "req-abc")
			h.Set("Request-Timestamp", "2024-03-26T09:01:41Z")
			h.Set("Signature", sig)
			return h
		}(),
	} {
		if !VerifyNotificationSignature(m, body, path, secret) {
			t.Fatalf("header casing should not affect verification")
		}
	}
}

func TestParseNotification_Success(t *testing.T) {
	body := []byte(`{
		"order": {"invoice_number":"INV-001","amount":10000,"currency":"IDR"},
		"customer": {"id":"CUST-1","name":"Anton","email":"a@example.com"},
		"transaction": {"status":"SUCCESS","date":"2024-03-26T16:01:41+07:00","original_request_id":"req-1"},
		"service": {"id":"SVC-1"},
		"acquirer": {"id":"ACQ-1"},
		"channel": {"id":"CH-1"},
		"card_payment": {"masked_card":"48111111XXXXX1111"}
	}`)
	n, err := ParseNotification(body)
	if err != nil {
		t.Fatalf("ParseNotification: %v", err)
	}
	if n.Transaction.Status != "SUCCESS" {
		t.Errorf("Transaction.Status=%q, want SUCCESS", n.Transaction.Status)
	}
	if n.Order.InvoiceNumber != "INV-001" {
		t.Errorf("Order.InvoiceNumber=%q, want INV-001", n.Order.InvoiceNumber)
	}
	if !strings.Contains(string(n.Raw), `"card_payment"`) {
		t.Errorf("Raw missing card_payment field: %s", n.Raw)
	}
}

func TestParseNotification_MissingStatus(t *testing.T) {
	body := []byte(`{"order":{"invoice_number":"INV-001","amount":10000}}`)
	if _, err := ParseNotification(body); err == nil {
		t.Fatalf("expected error when transaction.status is missing")
	}
}

func TestParseNotification_NotJSON(t *testing.T) {
	if _, err := ParseNotification([]byte("not json")); err == nil {
		t.Fatalf("expected error for non-JSON body")
	}
}

func TestClient_VerifyNotification(t *testing.T) {
	c := New(Config{
		ClientID:  "M-123",
		SecretKey: "secret-xyz",
		Env:       Sandbox,
	})
	const path = "/api/doku/notifications"
	body := []byte(`{"order":{"invoice_number":"INV-001","amount":10000},"transaction":{"status":"SUCCESS"}}`)
	sig := SignNonSnap(NonSnapSignatureInput{
		ClientID:         "M-123",
		RequestID:        "req-abc",
		RequestTimestamp: "2024-03-26T09:01:41Z",
		RequestTarget:    path,
		Body:             string(body),
		SecretKey:        "secret-xyz",
	})
	h := http.Header{}
	h.Set("Client-Id", "M-123")
	h.Set("Request-Id", "req-abc")
	h.Set("Request-Timestamp", "2024-03-26T09:01:41Z")
	h.Set("Signature", sig)
	if !c.VerifyNotification(h, body, path) {
		t.Fatalf("client wrapper failed to verify")
	}
}

// DOKU may send order.amount as a string; parsing must not reject it.
func TestParseNotification_StringAmount(t *testing.T) {
	n, err := ParseNotification([]byte(`{"order":{"invoice_number":"INV-1","amount":"20000"},"transaction":{"status":"SUCCESS"}}`))
	if err != nil {
		t.Fatalf("ParseNotification: %v", err)
	}
	if n.Order.Amount.String() != "20000" {
		t.Errorf("Order.Amount=%q, want 20000", n.Order.Amount)
	}
}
