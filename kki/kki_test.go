package kki_test

import (
	"context"
	"testing"

	"github.com/harizinside/go-doku/internal/dokutest"
	"github.com/harizinside/go-doku/kki"
)

// Generated SNAP Direct Debit/KKI calls sign with SharedKey and forward options.
func TestBindingOnlyUsesSharedKey(t *testing.T) {
	srv, rec := dokutest.Server(t)
	c := dokutest.Client(t, srv.URL, "sk-test", "shared-key-xyz")
	if _, err := kki.New(c).BindingOnly(context.Background(), kki.BindingOnlyBody{
		AccountName:    "Tester",
		AdditionalInfo: kki.BindingOnlyBodyAdditionalInfo{Channel: "H2H"},
	}, &kki.SnapOptions{ChannelID: "H2H", CustomerToken: "cust-1"}); err != nil {
		t.Fatalf("BindingOnly: %v", err)
	}
	h, body, _ := rec.Last()
	if want := dokutest.SnapSignature(h, body, "/direct-debit/core/v1/registration-card-bind", "shared-key-xyz"); h.Get("X-SIGNATURE") != want {
		t.Errorf("X-SIGNATURE=%s, want %s (shared key)", h.Get("X-SIGNATURE"), want)
	}
	if h.Get("CHANNEL-ID") != "H2H" || h.Get("Authorization-Customer") != "Bearer cust-1" {
		t.Errorf("options not forwarded: CHANNEL-ID=%q Authorization-Customer=%q", h.Get("CHANNEL-ID"), h.Get("Authorization-Customer"))
	}
}
