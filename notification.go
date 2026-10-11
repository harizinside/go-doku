package doku

import (
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Notification is a parsed DOKU HTTP Notification body. Structure and
// `Transaction.Status` values (SUCCESS | FAILED) follow
// developers.doku.com's notification guide. The payment-method-specific
// block (CardPayment, VirtualAccountInfo, Emoney, etc.) varies by channel,
// so it stays accessible via Raw as a generic JSON object.
type Notification struct {
	Order struct {
		InvoiceNumber string `json:"invoice_number"`
		// Amount accepts both 20000 and "20000" (DOKU sends either).
		Amount   json.Number `json:"amount"`
		Currency string      `json:"currency,omitempty"`
	} `json:"order"`
	Customer    *NotificationCustomer `json:"customer,omitempty"`
	Transaction struct {
		Type              string `json:"type,omitempty"`
		Status            string `json:"status"`
		Date              string `json:"date,omitempty"`
		OriginalRequestID string `json:"original_request_id,omitempty"`
	} `json:"transaction"`
	Service  *NotificationParty `json:"service,omitempty"`
	Acquirer *NotificationParty `json:"acquirer,omitempty"`
	Channel  *NotificationParty `json:"channel,omitempty"`
	// Raw holds the full notification body for callers that need fields
	// beyond the typed surface above (card_payment, virtual_account_info,
	// emoney, etc.).
	Raw json.RawMessage `json:"-"`
}

// NotificationCustomer mirrors the documented customer block; other fields
// are available via Notification.Raw.
type NotificationCustomer struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// NotificationParty is the small `{id?: string}` shape used by service /
// acquirer / channel blocks.
type NotificationParty struct {
	ID string `json:"id,omitempty"`
}

// header looks up `name` case-insensitively, returning the first value when
// the slot is an array.
func header(h http.Header, name string) string {
	for k, vals := range h {
		if strings.EqualFold(k, name) && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

// VerifyNotificationSignature verifies a DOKU HTTP Notification's `Signature`
// header — the same Non-SNAP HMAC-SHA256 scheme as outgoing requests, just
// checked in reverse. Returns false on any missing header or mismatch;
// never panics. The rawBody MUST be the exact bytes DOKU sent (do NOT
// parse-then-restringify first — re-serializing can reorder keys or change
// whitespace and silently break verification).
func VerifyNotificationSignature(h http.Header, rawBody []byte, notificationPath, secretKey string) bool {
	clientID := header(h, "Client-Id")
	requestID := header(h, "Request-Id")
	requestTimestamp := header(h, "Request-Timestamp")
	receivedSignature := header(h, "Signature")
	if clientID == "" || requestID == "" || requestTimestamp == "" || receivedSignature == "" {
		return false
	}
	expected := SignNonSnap(NonSnapSignatureInput{
		ClientID:         clientID,
		RequestID:        requestID,
		RequestTimestamp: requestTimestamp,
		RequestTarget:    notificationPath,
		Body:             string(rawBody),
		SecretKey:        secretKey,
	})
	return hmac.Equal([]byte(expected), []byte(receivedSignature))
}

// ParseNotification parses a DOKU notification body. Returns an error if
// `transaction.status` is missing.
func ParseNotification(rawBody []byte) (*Notification, error) {
	if !json.Valid(rawBody) {
		return nil, fmt.Errorf("doku: parse notification: body is not valid JSON")
	}
	var n Notification
	if err := json.Unmarshal(rawBody, &n); err != nil {
		return nil, fmt.Errorf("doku: parse notification: %w", err)
	}
	if n.Transaction.Status == "" {
		return nil, fmt.Errorf("doku: not a DOKU notification: missing transaction.status")
	}
	n.Raw = append(n.Raw[:0], rawBody...)
	return &n, nil
}
