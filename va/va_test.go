package va_test

import (
	"context"
	"errors"
	"testing"

	doku "github.com/harizinside/go-doku"
	"github.com/harizinside/go-doku/internal/dokutest"
	"github.com/harizinside/go-doku/va"
)

func TestSnapCreateVaHeaders(t *testing.T) {
	srv, rec := dokutest.Server(t)
	c := dokutest.Client(t, srv.URL, "sk-test", "")
	if _, err := va.NewSnap(c).MandiriSnapCreateVa(context.Background(), va.SnapCreateVaRequest{
		PartnerServiceId: doku.PartnerServiceID("8899"),
		TrxId:            "INV-001",
		TotalAmount:      va.SnapMoneyAmount{Value: "50000.00", Currency: "IDR"},
	}); err != nil {
		t.Fatalf("MandiriSnapCreateVa: %v", err)
	}
	h, body, _ := rec.Last()
	for _, name := range []string{"X-PARTNER-ID", "X-EXTERNAL-ID", "X-TIMESTAMP", "X-SIGNATURE", "Authorization"} {
		if h.Get(name) == "" {
			t.Errorf("missing SNAP header %s", name)
		}
	}
	if h.Get("CHANNEL-ID") != "H2H" {
		t.Errorf("CHANNEL-ID=%q, want H2H", h.Get("CHANNEL-ID"))
	}
	if want := dokutest.SnapSignature(h, body, "/bi-snap-va/mandiri/v1/transfer-va/create-va", "sk-test"); h.Get("X-SIGNATURE") != want {
		t.Errorf("X-SIGNATURE=%s, want %s", h.Get("X-SIGNATURE"), want)
	}
}

// VA calls must honor the caller's ctx.
func TestHonorsContext(t *testing.T) {
	srv, rec := dokutest.Server(t)
	c := dokutest.Client(t, srv.URL, "sk-test", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := va.NewNonSnap(c).MandiriGeneratePaymentCode(ctx, va.MandiriVaGenerateRequest{}); !errors.Is(err, context.Canceled) {
		t.Errorf("non-SNAP: err=%v, want context.Canceled", err)
	}
	if _, err := va.NewSnap(c).BniCreateVa(ctx, va.SnapCreateVaRequest{}); !errors.Is(err, context.Canceled) {
		t.Errorf("SNAP: err=%v, want context.Canceled", err)
	}
	if _, _, hits := rec.Last(); hits != 0 {
		t.Errorf("server hit %d times with a cancelled ctx", hits)
	}
}
