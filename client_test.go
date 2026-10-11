package doku

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	testClientID  = "M-TEST"
	testSecretKey = "sk-test"
)

func mustKey(t *testing.T) string {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	b, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b}))
}

func newTestClient(t *testing.T, baseURL string, override ...func(*Client)) *Client {
	t.Helper()
	pk := mustKey(t)
	c := New(Config{
		ClientID:   testClientID,
		SecretKey:  testSecretKey,
		PrivateKey: pk,
		Env:        Sandbox,
	})
	c.cfg.BaseURL = baseURL
	for _, o := range override {
		o(c)
	}
	return c
}

// TestRequestNonSnap verifies headers + body byte-for-byte match the signing
// digest, plus a successful decode of the response body.
func TestRequestNonSnap(t *testing.T) {
	var (
		gotMethod  string
		gotPath    string
		gotHeaders http.Header
		gotBody    []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":{"payment":{"url":"https://example.test/pay"}}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	type body struct {
		Order struct {
			Amount        int    `json:"amount"`
			InvoiceNumber string `json:"invoice_number"`
		} `json:"order"`
		Payment struct {
			DueDate int `json:"payment_due_date"`
		} `json:"payment"`
	}
	var in body
	in.Order.Amount = 10000
	in.Order.InvoiceNumber = "INV-001"
	in.Payment.DueDate = 60

	var out CheckoutResponse
	if err := c.RequestNonSnap(context.Background(), "POST", "/checkout/v1/payment", in, &out, ""); err != nil {
		t.Fatalf("RequestNonSnap: %v", err)
	}

	if gotMethod != "POST" || gotPath != "/checkout/v1/payment" {
		t.Fatalf("server received %s %s", gotMethod, gotPath)
	}
	for _, h := range []string{"Client-Id", "Request-Id", "Request-Timestamp", "Signature"} {
		if gotHeaders.Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
	if gotHeaders.Get("Client-Id") != testClientID {
		t.Errorf("Client-Id header mismatch: %q", gotHeaders.Get("Client-Id"))
	}
	// Re-sign the received body and confirm it matches the Signature header —
	// i.e. the bytes that were signed are the bytes that were sent.
	wantSig := SignNonSnap(NonSnapSignatureInput{
		ClientID:         testClientID,
		RequestID:        gotHeaders.Get("Request-Id"),
		RequestTimestamp: gotHeaders.Get("Request-Timestamp"),
		RequestTarget:    "/checkout/v1/payment",
		Body:             string(gotBody),
		SecretKey:        testSecretKey,
	})
	if gotHeaders.Get("Signature") != wantSig {
		t.Errorf("server signature does not match re-signed body\n  got=%s\n  want=%s", gotHeaders.Get("Signature"), wantSig)
	}
	if out.Response == nil || out.Response.Payment == nil || out.Response.Payment.URL == nil {
		t.Errorf("decoded response missing payment.url: %#v", out)
	}
}

// TestRequestNonSnap_GET signs without a Digest line.
func TestRequestNonSnap_GET(t *testing.T) {
	var gotHeaders http.Header
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":{"invoice_number":"INV-001","amount":"10000","status":"PAID"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if _, err := c.Misc.CheckOrderStatus(context.Background(), "INV-001"); err != nil {
		t.Fatalf("CheckOrderStatus: %v", err)
	}
	if gotHeaders.Get("Signature") == "" {
		t.Fatalf("no Signature header on GET")
	}
	wantSig := SignNonSnap(NonSnapSignatureInput{
		ClientID:         testClientID,
		RequestID:        gotHeaders.Get("Request-Id"),
		RequestTimestamp: gotHeaders.Get("Request-Timestamp"),
		RequestTarget:    "/orders/v1/status/INV-001",
		SecretKey:        testSecretKey,
	})
	if gotHeaders.Get("Signature") != wantSig {
		t.Errorf("GET signature mismatch\n  got=%s\n  want=%s", gotHeaders.Get("Signature"), wantSig)
	}
	if len(gotBody) != 0 {
		t.Errorf("GET body should be empty, got %d bytes", len(gotBody))
	}
}

// TestAPIError surfaces a non-2xx response as *APIError.
func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"INVALID_AMOUNT","message":"amount too low"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	var out CheckoutResponse
	err := c.RequestNonSnap(context.Background(), "POST", "/checkout/v1/payment", map[string]any{"x": 1}, &out, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Status != 400 {
		t.Errorf("Status=%d, want 400", apiErr.Status)
	}
	var body map[string]any
	if err := json.Unmarshal(apiErr.Body, &body); err != nil {
		t.Fatalf("Body not JSON: %v", err)
	}
	if apiErr.Header.Get("Content-Type") == "" {
		t.Errorf("Header not captured")
	}
}

// TestSnapTokenCached verifies that two SNAP calls reuse one B2B token fetch.
func TestSnapTokenCached(t *testing.T) {
	var tokenHits int32
	var resourceHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authorization/v1/access-token/b2b":
			atomic.AddInt32(&tokenHits, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accessToken":"tok-abc","expiresIn":"900"}`))
		default:
			atomic.AddInt32(&resourceHits, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)

	for range 3 {
		_, err := c.VirtualAccountSnap.MandiriSnapCreateVa(context.Background(), SnapCreateVaRequest{
			PartnerServiceId: PartnerServiceID("8899"),
			TrxId:            "INV-001",
			TotalAmount:      SnapMoneyAmount{Value: "50000.00", Currency: "IDR"},
		})
		if err != nil {
			t.Fatalf("MandiriSnapCreateVa: %v", err)
		}
	}
	if got := atomic.LoadInt32(&tokenHits); got != 1 {
		t.Errorf("token endpoint hit %d times, want 1", got)
	}
	if got := atomic.LoadInt32(&resourceHits); got != 3 {
		t.Errorf("resource endpoint hit %d times, want 3", got)
	}
}

// TestSnapTokenConcurrent: parallel callers should coalesce on a single
// token fetch (cache hit) rather than each issuing their own.
func TestSnapTokenConcurrent(t *testing.T) {
	var tokenHits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/authorization/v1/access-token/b2b" {
			atomic.AddInt32(&tokenHits, 1)
			time.Sleep(50 * time.Millisecond) // force overlap
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accessToken":"tok-abc","expiresIn":"900"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	const N = 8
	var wg sync.WaitGroup
	wg.Add(N)
	for range N {
		go func() {
			defer wg.Done()
			_, err := c.VirtualAccountSnap.MandiriSnapCreateVa(context.Background(), SnapCreateVaRequest{
				PartnerServiceId: PartnerServiceID("8899"),
				TrxId:            "INV-001",
				TotalAmount:      SnapMoneyAmount{Value: "50000.00", Currency: "IDR"},
			})
			if err != nil {
				t.Errorf("MandiriSnapCreateVa: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&tokenHits); got > 1 {
		t.Errorf("token endpoint hit %d times concurrently, want ≤1", got)
	}
}

// TestSnapRequestSignatureHeaders verifies SNAP resource-request headers.
func TestSnapRequestSignatureHeaders(t *testing.T) {
	var gotHeaders http.Header
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authorization/v1/access-token/b2b":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accessToken":"tok-abc","expiresIn":"900"}`))
			return
		}
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if _, err := c.VirtualAccountSnap.MandiriSnapCreateVa(context.Background(), SnapCreateVaRequest{
		PartnerServiceId: PartnerServiceID("8899"),
		TrxId:            "INV-001",
		TotalAmount:      SnapMoneyAmount{Value: "50000.00", Currency: "IDR"},
	}); err != nil {
		t.Fatalf("MandiriSnapCreateVa: %v", err)
	}
	for _, h := range []string{"X-PARTNER-ID", "X-EXTERNAL-ID", "X-TIMESTAMP", "X-SIGNATURE", "Authorization"} {
		if gotHeaders.Get(h) == "" {
			t.Errorf("missing SNAP header %s", h)
		}
	}
	if gotHeaders.Get("CHANNEL-ID") != "H2H" {
		t.Errorf("CHANNEL-ID header: got %q, want H2H", gotHeaders.Get("CHANNEL-ID"))
	}
	// Body bytes that arrived at the server must equal the bytes the
	// X-SIGNATURE header was computed over.
	wantSig := SignSnapRequest(SnapRequestSignatureInput{
		HTTPMethod:   "POST",
		EndpointURL:  "/bi-snap-va/mandiri/v1/transfer-va/create-va",
		AccessToken:  gotHeaders.Get("Authorization")[len("Bearer "):],
		Body:         string(gotBody),
		Timestamp:    gotHeaders.Get("X-TIMESTAMP"),
		ClientSecret: testSecretKey,
	})
	if gotHeaders.Get("X-SIGNATURE") != wantSig {
		t.Errorf("SNAP X-SIGNATURE mismatch\n  got=%s\n  want=%s", gotHeaders.Get("X-SIGNATURE"), wantSig)
	}
}

// TestSnapUseSharedKey: signing key comes from SharedKey when set.
func TestSnapUseSharedKey(t *testing.T) {
	shared := "shared-key-xyz"
	var gotHeaders http.Header
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/authorization/v1/access-token/b2b":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"accessToken":"tok-abc","expiresIn":"900"}`))
			return
		}
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL, func(c *Client) { c.cfg.SharedKey = shared })
	if _, err := c.KKI.BindingOnly(context.Background(), BindingOnlyBody{
		AccountName: "Tester",
		CardData:    "X1234",
		AdditionalInfo: struct {
			Channel      string `json:"channel,omitempty"`
			CustomerName string `json:"customerName,omitempty"`
			Email        string `json:"email,omitempty"`
			DateOfBirth  string `json:"dateOfBirth,omitempty"`
		}{Channel: "H2H"},
	}, nil); err != nil {
		t.Fatalf("BindingOnly: %v", err)
	}
	wantSig := SignSnapRequest(SnapRequestSignatureInput{
		HTTPMethod:   "POST",
		EndpointURL:  "/direct-debit/core/v1/registration-card-bind",
		AccessToken:  gotHeaders.Get("Authorization")[len("Bearer "):],
		Body:         string(gotBody),
		Timestamp:    gotHeaders.Get("X-TIMESTAMP"),
		ClientSecret: shared,
	})
	if gotHeaders.Get("X-SIGNATURE") != wantSig {
		t.Errorf("SharedKey signing\n  got=%s\n  want=%s", gotHeaders.Get("X-SIGNATURE"), wantSig)
	}
}

// TestSnapTokenMissingPrivateKey: PrivateKey required → *APIError, status 0.
func TestSnapTokenMissingPrivateKey(t *testing.T) {
	c := New(Config{
		ClientID:  testClientID,
		SecretKey: testSecretKey,
		Env:       Sandbox,
	})
	_, err := c.SnapB2BToken(context.Background(), false)
	if err == nil {
		t.Fatalf("expected error when PrivateKey is nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Status != 0 {
		t.Errorf("Status=%d, want 0", apiErr.Status)
	}
}

// expiresIn as a numeric string must set the cache TTL (minus margin), not
// fall back to the 14-minute default.
func TestSnapTokenStringExpiresIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"accessToken":"tok-abc","expiresIn":"60"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	if _, err := c.SnapB2BToken(context.Background(), false); err != nil {
		t.Fatalf("SnapB2BToken: %v", err)
	}
	if ttl := time.Until(c.b2bToken.expiresAt); ttl > 31*time.Second || ttl < 25*time.Second {
		t.Errorf("token TTL=%v, want ~30s (60s - 30s margin)", ttl)
	}
}

// VA helpers must honor the caller's ctx.
func TestVirtualAccountHonorsContext(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.VirtualAccountNonSnap.MandiriGeneratePaymentCode(ctx, MandiriVaGenerateRequest{}); !errors.Is(err, context.Canceled) {
		t.Errorf("non-SNAP VA: err=%v, want context.Canceled", err)
	}
	if _, err := c.VirtualAccountSnap.BniCreateVa(ctx, SnapCreateVaRequest{}); !errors.Is(err, context.Canceled) {
		t.Errorf("SNAP VA: err=%v, want context.Canceled", err)
	}
	if n := hits.Load(); n != 0 {
		t.Errorf("server hit %d times with a cancelled ctx", n)
	}
}
