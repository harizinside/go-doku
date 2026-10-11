package doku

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

// Golden vectors captured from the Node package via scripts/ts2go golden
// (run `node scripts/ts2go.mjs ../doku && node /tmp/doku-golden.mjs` to
// re-capture). Hardcoded here so the Go port stays byte-for-byte aligned
// with @harizinside/doku.

func TestNonSnapTimestamp(t *testing.T) {
	got := NonSnapTimestamp(time.Date(2024, 3, 26, 9, 1, 41, 0, time.UTC))
	want := "2024-03-26T09:01:41Z"
	if got != want {
		t.Fatalf("NonSnapTimestamp: got %q, want %q", got, want)
	}
}

func TestSnapTimestamp(t *testing.T) {
	// Run in a fixed local zone (Asia/Jakarta, UTC+7) so the offset is
	// deterministic regardless of the developer's machine. The signature is
	// "[local-time-in-Jakarta]+07:00" for any UTC instant that falls in
	// Asia/Jakarta, matching DOKU's Postman reference.
	tz := time.FixedZone("WIB", 7*60*60)
	got := SnapTimestamp(time.Date(2024, 3, 26, 9, 1, 41, 0, time.UTC).In(tz))
	want := "2024-03-26T16:01:41+07:00"
	if got != want {
		t.Fatalf("SnapTimestamp: got %q, want %q", got, want)
	}
}

func TestNonSnapDigest(t *testing.T) {
	got := NonSnapDigest("{\n  \"a\": 1,\n  \"b\": \"x\"\n}")
	want := "HR5COjFdWwhR2t5D/SYKfFMzJ3jy6o8QQctR//RO2T4="
	if got != want {
		t.Fatalf("NonSnapDigest: got %q, want %q", got, want)
	}
}

func TestSignNonSnapNoBody(t *testing.T) {
	got := SignNonSnap(NonSnapSignatureInput{
		ClientID:         "M-123",
		RequestID:        "req-abc",
		RequestTimestamp: "2024-03-26T09:01:41Z",
		RequestTarget:    "/orders/v1/status/INV-001",
		SecretKey:        "secret-xyz",
	})
	want := "HMACSHA256=OOefUefXw25LIgCiskNb60WQjyALpRBmvLO0WmbvIwo="
	if got != want {
		t.Fatalf("SignNonSnap(no body): got %q, want %q", got, want)
	}
}

func TestSignNonSnapWithBody(t *testing.T) {
	got := SignNonSnap(NonSnapSignatureInput{
		ClientID:         "M-123",
		RequestID:        "req-abc",
		RequestTimestamp: "2024-03-26T09:01:41Z",
		RequestTarget:    "/checkout/v1/payment",
		Body:             "{\n  \"order\": {\n    \"invoice_number\": \"INV-001\",\n    \"amount\": 10000\n  }\n}",
		SecretKey:        "secret-xyz",
	})
	want := "HMACSHA256=4KsPV9Ku3VjDcKR+bmMTRcJ/IjKlLRu5ZMd7OHH8R/c="
	if got != want {
		t.Fatalf("SignNonSnap(with body): got %q, want %q", got, want)
	}
}

func TestSignSnapRequestEmptyBody(t *testing.T) {
	got := SignSnapRequest(SnapRequestSignatureInput{
		HTTPMethod:   "POST",
		EndpointURL:  "/virtual-accounts/bi-snap-va/v1/transfer-va/create-va",
		AccessToken:  "tok-xyz",
		Body:         "",
		Timestamp:    "2024-03-26T16:01:41+07:00",
		ClientSecret: "secret-xyz",
	})
	want := "vV0YbX50SzfJD+lL1CECICXHVavvp81hM4Z+tzDzgabOtmmvmMDAzSYbBFQlauUrfU+8owqLhZEshXcFoQUYSA=="
	if got != want {
		t.Fatalf("SignSnapRequest(empty body): got %q, want %q", got, want)
	}
}

func TestSignSnapRequestWithBody(t *testing.T) {
	got := SignSnapRequest(SnapRequestSignatureInput{
		HTTPMethod:   "POST",
		EndpointURL:  "/orders/v1.0/transfer-va/status",
		AccessToken:  "tok-xyz",
		Body:         "{\n  \"partnerServiceId\": \"   8899\",\n  \"customerNo\": \"123\"\n}",
		Timestamp:    "2024-03-26T16:01:41+07:00",
		ClientSecret: "secret-xyz",
	})
	want := "s0IuQqxMg/8GXBeJ4aMNLTntOYRAtJCRRVrZzyBt1T0pnfZExtuxDMKmMJ3IpZVIb0tmW9nR9HGbTEsFK5FBYg=="
	if got != want {
		t.Fatalf("SignSnapRequest(with body): got %q, want %q", got, want)
	}
}

func TestOVOCheckSumPayment(t *testing.T) {
	got := OVOCheckSum(OvoCheckSumInput{
		Amount:        "10000",
		ClientID:      "M-123",
		InvoiceNumber: "INV-001",
		OvoID:         "081211111111",
		SecretKey:     "secret-xyz",
	})
	want := "e62d42babccb2e5fcd8f28097f7b0e01de9eec188bd78a08fc9b6fb260512145"
	if got != want {
		t.Fatalf("OVOCheckSum(payment): got %q, want %q", got, want)
	}
}

func TestOVOCheckSumVoid(t *testing.T) {
	got := OVOCheckSum(OvoCheckSumInput{
		Amount:          "10000",
		ClientID:        "M-123",
		InvoiceNumber:   "INV-001",
		OvoID:           "081211111111",
		SecretKey:       "secret-xyz",
		ApprovalCode:    "APP-1",
		BatchNumber:     "B-1",
		ReferenceNumber: "R-1",
		TraceNumber:     "T-1",
	})
	want := "52a77c7a3e5c1f17c0085396dc475196ca3b8e4904bc25bf3058ed1b19c55ebf"
	if got != want {
		t.Fatalf("OVOCheckSum(void): got %q, want %q", got, want)
	}
}

func TestPartnerServiceID(t *testing.T) {
	cases := []struct {
		bin  string
		blen []int
		want string
	}{
		{"8899", nil, "    8899"},
		{"8899", []int{4}, "    8899"},
		{"12345678901", nil, "12345678"},
	}
	for _, c := range cases {
		got := PartnerServiceID(c.bin, c.blen...)
		if got != c.want {
			t.Errorf("PartnerServiceID(%q, %v): got %q, want %q", c.bin, c.blen, got, c.want)
		}
	}
}

// SignSnapToken uses RSA, so the signature is dependent on the key. Generate
// a fresh keypair inside the test and assert two properties that are
// independent of the random key: PKCS#1 and PKCS#8 forms of the SAME key
// produce identical signatures, and "\n"-escaped forms match their raw
// counterparts. We also cross-check that the signature verifies against the
// public key (no Node round-trip required).
func TestSignSnapToken(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	pkcs1Bytes := x509.MarshalPKCS1PrivateKey(priv)
	pkcs8PEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes}))
	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pkcs1Bytes}))
	pkcs8Esc := strings.ReplaceAll(strings.ReplaceAll(pkcs8PEM, "\n", "\\n"), "-----", "-----")
	// Above is intentionally naïve — we want the test to verify that literal
	// "\n" sequences are normalized to real newlines before PEM decode.
	pkcs8Esc = strings.ReplaceAll(pkcs8PEM, "\n", "\\n")
	pkcs1Esc := strings.ReplaceAll(pkcs1PEM, "\n", "\\n")

	in := SnapTokenSignatureInput{
		ClientID:  "M-123",
		Timestamp: "2024-03-26T16:01:41+07:00",
	}
	sigs := map[string]string{}
	for _, p := range []struct {
		name, key string
	}{
		{"pkcs8", pkcs8PEM},
		{"pkcs1", pkcs1PEM},
		{"pkcs8_escaped", pkcs8Esc},
		{"pkcs1_escaped", pkcs1Esc},
	} {
		in.PrivateKey = p.key
		got, err := SignSnapToken(in)
		if err != nil {
			t.Errorf("SignSnapToken(%s): %v", p.name, err)
			continue
		}
		sigs[p.name] = got
	}
	if sigs["pkcs8"] != sigs["pkcs1"] {
		t.Errorf("PKCS#8 vs PKCS#1: signatures differ\n  pkcs8=%s\n  pkcs1=%s", sigs["pkcs8"], sigs["pkcs1"])
	}
	if sigs["pkcs8"] != sigs["pkcs8_escaped"] {
		t.Errorf("PKCS#8 vs PKCS#8 (\\n escaped): signatures differ\n  pkcs8=%s\n  escaped=%s", sigs["pkcs8"], sigs["pkcs8_escaped"])
	}
	if sigs["pkcs1"] != sigs["pkcs1_escaped"] {
		t.Errorf("PKCS#1 vs PKCS#1 (\\n escaped): signatures differ\n  pkcs1=%s\n  escaped=%s", sigs["pkcs1"], sigs["pkcs1_escaped"])
	}

	// Verify the signature against the public key (independent sanity check).
	sig, err := base64.StdEncoding.DecodeString(sigs["pkcs8"])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	stringToSign := []byte(in.ClientID + "|" + in.Timestamp)
	digest := sha256.Sum256(stringToSign)
	if err := rsa.VerifyPKCS1v15(&priv.PublicKey, crypto.SHA256, digest[:], sig); err != nil {
		t.Errorf("signature does not verify against public key: %v", err)
	}
}
