package misc

import (
	"context"
	"testing"

	doku "github.com/harizinside/go-doku"
	"github.com/harizinside/go-doku/internal/dokutest"
)

// CheckOrderStatus must forward request options to the Non-SNAP request.
func TestCheckOrderStatusWithRequestID(t *testing.T) {
	srv, rec := dokutest.Server(t)
	svc := New(dokutest.Client(t, srv.URL, "sk-test", ""))

	const requestID = "status-NMW-261118-0040"
	if _, err := svc.CheckOrderStatus(context.Background(), "INV-0001", doku.WithRequestID(requestID)); err != nil {
		t.Fatalf("CheckOrderStatus: %v", err)
	}
	header, _, hits := rec.Last()
	if hits != 1 {
		t.Fatalf("server hits = %d, want 1", hits)
	}
	if got := header.Get("Request-Id"); got != requestID {
		t.Errorf("Request-Id = %q, want %q", got, requestID)
	}
}
