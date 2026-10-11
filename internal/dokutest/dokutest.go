// Package dokutest provides a fake DOKU server and client for product
// package tests.
package dokutest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	doku "github.com/harizinside/go-doku"
)

// Recorder captures the last non-token request the fake server received.
type Recorder struct {
	mu     sync.Mutex
	Header http.Header
	Body   []byte
	Hits   int
}

// Last returns the last recorded request headers and body.
func (r *Recorder) Last() (http.Header, []byte, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.Header, r.Body, r.Hits
}

// Server starts a fake DOKU host that issues SNAP B2B tokens ("tok-abc")
// and answers every other request with `{}`.
func Server(t *testing.T) (*httptest.Server, *Recorder) {
	t.Helper()
	rec := &Recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/authorization/v1/access-token/b2b" {
			_, _ = w.Write([]byte(`{"accessToken":"tok-abc","expiresIn":"900"}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.Header, rec.Body = r.Header.Clone(), body
		rec.Hits++
		rec.mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv, rec
}

// Client returns a doku.Client pointed at baseURL with a fresh RSA key.
func Client(t *testing.T, baseURL, secretKey, sharedKey string) *doku.Client {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return doku.New(doku.Config{
		ClientID:   "M-TEST",
		SecretKey:  secretKey,
		SharedKey:  sharedKey,
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		BaseURL:    baseURL,
	})
}

// SnapSignature recomputes the X-SIGNATURE a request with these headers and
// body should carry.
func SnapSignature(h http.Header, body []byte, path, secret string) string {
	return doku.SignSnapRequest(doku.SnapRequestSignatureInput{
		HTTPMethod:   "POST",
		EndpointURL:  path,
		AccessToken:  h.Get("Authorization")[len("Bearer "):],
		Body:         string(body),
		Timestamp:    h.Get("X-TIMESTAMP"),
		ClientSecret: secret,
	})
}
