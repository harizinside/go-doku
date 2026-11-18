package checkout

import (
	"context"
	"regexp"
	"strings"
	"testing"

	doku "github.com/harizinside/go-doku"
	"github.com/harizinside/go-doku/internal/dokutest"
)

const (
	testClientID  = "M-TEST"
	testSecretKey = "sk-test"
)

// uuidV4 matches the lowercase UUIDv4 that RequestNonSnap generates when no
// request id is supplied.
var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func testRequest() Request {
	return Request{
		Order:   RequestOrder{Amount: 20000, InvoiceNumber: "INV-0001"},
		Payment: RequestPayment{PaymentDueDate: doku.Ptr(60.0)},
	}
}

// A caller-supplied request id must reach the server verbatim and be the id
// the Non-SNAP signature was built from.
func TestInitiateCheckoutPaymentWithRequestID(t *testing.T) {
	srv, rec := dokutest.Server(t)
	svc := New(dokutest.Client(t, srv.URL, testSecretKey, ""))

	const requestID = "order-NMW-261118-0040"
	if _, err := svc.InitiateCheckoutPayment(context.Background(), testRequest(), doku.WithRequestID(requestID)); err != nil {
		t.Fatalf("InitiateCheckoutPayment: %v", err)
	}

	header, body, hits := rec.Last()
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1", hits)
	}
	if got := header.Get("Request-Id"); got != requestID {
		t.Errorf("Request-Id = %q, want %q", got, requestID)
	}
	wantSig := doku.SignNonSnap(doku.NonSnapSignatureInput{
		ClientID:         testClientID,
		RequestID:        requestID,
		RequestTimestamp: header.Get("Request-Timestamp"),
		RequestTarget:    "/checkout/v1/payment",
		Body:             string(body),
		SecretKey:        testSecretKey,
	})
	if got := header.Get("Signature"); got != wantSig {
		t.Errorf("Signature = %q, want %q", got, wantSig)
	}
}

// Without an option the request id stays a random UUIDv4.
func TestInitiateCheckoutPaymentDefaultRequestID(t *testing.T) {
	srv, rec := dokutest.Server(t)
	svc := New(dokutest.Client(t, srv.URL, testSecretKey, ""))

	if _, err := svc.InitiateCheckoutPayment(context.Background(), testRequest()); err != nil {
		t.Fatalf("InitiateCheckoutPayment: %v", err)
	}

	header, _, hits := rec.Last()
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1", hits)
	}
	got := header.Get("Request-Id")
	if !uuidV4.MatchString(got) {
		t.Errorf("Request-Id = %q, want a UUIDv4", got)
	}
}

// An unusable request id must fail before the request leaves the process.
func TestInitiateCheckoutPaymentInvalidRequestID(t *testing.T) {
	cases := []struct {
		name string
		id   string
	}{
		{name: "empty", id: ""},
		{name: "too long", id: strings.Repeat("a", 129)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, rec := dokutest.Server(t)
			svc := New(dokutest.Client(t, srv.URL, testSecretKey, ""))

			_, err := svc.InitiateCheckoutPayment(context.Background(), testRequest(), doku.WithRequestID(tc.id))
			if err == nil {
				t.Fatalf("expected an error for request id %q", tc.id)
			}
			if _, _, hits := rec.Last(); hits != 0 {
				t.Errorf("server hits = %d, want 0", hits)
			}
		})
	}
}
