# go-doku

DOKU Payment Gateway SDK for Go — **stdlib only** (signing uses `crypto/hmac`,
`crypto/rsa`, `crypto/sha256`, `crypto/sha512`; transport uses `net/http`).

A 1:1 Go port of [`@harizinside/doku`](https://github.com/harizinside/doku).
Covers the full DOKU Direct API surface (Credit Card, Virtual Accounts — legacy
Non-SNAP and SNAP, O2O, E-Money, Direct Debit — legacy BRI and SNAP providers,
Paylater, Direct Transfer, KKI, Sub Account v1/v2, Check Status), DOKU
Checkout, and the Finance & Settlement APIs (Split Settlement, Hold & Release,
Custom Settlement Report).

## Install

```sh
go get github.com/harizinside/go-doku
```

Pin to a release tag so `go get` never picks up untested WIP from `main`:

```sh
go get github.com/harizinside/go-doku@v0.1.0
```

> **Server-only.** This SDK signs requests with your DOKU secret/private keys.
> Never load it from an end-user browser.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/harizinside/go-doku"
)

func main() {
    privateKey := os.Getenv("DOKU_PRIVATE_KEY") // PKCS#8 PEM (\n escapes in .env are fine)

    client := doku.New(doku.Config{
        ClientID:   os.Getenv("DOKU_CLIENT_ID"),
        SecretKey:  os.Getenv("DOKU_SECRET_KEY"),
        PrivateKey: privateKey, // required only for SNAP APIs
        Env:        doku.Sandbox,
    })

    ctx := context.Background()

    // DOKU Checkout — get a payment page URL
    checkout, err := client.Checkout.InitiateCheckoutPayment(ctx, doku.CheckoutRequest{
        Order: doku.CheckoutRequestOrder{
            Amount:        20000,
            InvoiceNumber: "INV-0001",
        },
        Payment: doku.CheckoutRequestPayment{PaymentDueDate: ptr(60.0)},
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(*checkout.Response.Payment.URL)

    // Legacy (Non-SNAP) Mandiri VA
    _, err = client.VirtualAccountNonSnap.MandiriGeneratePaymentCode(ctx, doku.MandiriVaGenerateRequest{
        Order: doku.VaOrder{InvoiceNumber: "INV-0002", Amount: ptr(50000.0)},
        VirtualAccountInfo: doku.MandiriVaGenerateRequestVirtualAccountInfo{ReusableStatus: ptr(true), BillingType: ptr("NO_BILL")},
        Customer: &doku.VaCustomer{Name: ptr("Anton"), Email: ptr("anton@example.com")},
    })

    // SNAP Mandiri VA (B2B token fetch + caching is automatic)
    _, err = client.VirtualAccountSnap.MandiriSnapCreateVa(ctx, doku.SnapCreateVaRequest{
        PartnerServiceId:  doku.PartnerServiceID("8899"), // -> "   8899"
        TrxId:             "INV-0002",
        TotalAmount:       doku.SnapMoneyAmount{Value: "50000.00", Currency: "IDR"},
        VirtualAccountName: ptr("Anton"),
        ExpiredDate:        ptr(time.Now().Add(time.Hour).Format(time.RFC3339)),
        AdditionalInfo: &doku.SnapVaAdditionalInfo{
            VirtualAccountConfig: &doku.SnapVaConfig{ReusableStatus: ptr(true)},
        },
    })

    // Check order status
    _, err = client.Misc.CheckOrderStatus(ctx, "INV-0001")

    // Release a held settlement
    _, err = client.Settlement.ReleaseSettlement(ctx, doku.ReleaseSettlementRequest{
        Order:       doku.ReleaseSettlementRequestOrder{InvoiceNumber: "INV-0001", Amount: 20000},
        Transaction: doku.ReleaseSettlementRequestTransaction{OriginalRequestID: "the-request-id-from-initiation"},
    })
}

func ptr[T any](v T) *T { return &v }
```

## Credentials & signing

| Config key | Used for |
| --- | --- |
| `ClientID` | `Client-Id` / `X-PARTNER-ID` / `X-CLIENT-KEY` |
| `SecretKey` | Non-SNAP HMAC-SHA256 signature; SNAP HMAC-SHA512 symmetric signature |
| `PrivateKey` | SNAP B2B/B2B2C access-token requests (RSA-SHA256 asymmetric signature) |
| `SharedKey` | Optional separate shared key for Direct Debit/KKI SNAP signing (defaults to `SecretKey`) |
| `Env` | `Sandbox` → `https://api-sandbox.doku.com`, `Production` → `https://api.doku.com` |

Both DOKU signing schemes are implemented exactly as documented:

- **Non-SNAP** (`Client-Id`/`Request-Id`/`Request-Timestamp`/`Signature`):
  `HMACSHA256=` + Base64(HMAC-SHA256(stringToSign)) where stringToSign is
  `Client-Id:…\nRequest-Id:…\nRequest-Timestamp:…\nRequest-Target:…\nDigest:…`
  (no `Digest` line for GET requests).
- **SNAP**: B2B token via `X-SIGNATURE` = Base64(RSA-SHA256(`clientId|timestamp`)), then
  `X-SIGNATURE` = Base64(HMAC-SHA512(`METHOD:path:token:lowerHex(SHA256(minBody)):timestamp`)).

B2B tokens are cached in-memory (mutex-guarded) until shortly before expiry;
B2B2C tokens are cached per `authCode`. Direct Debit SNAP flows that need a
customer token accept a `customerToken` option on the call — fetch one with
`client.SnapB2B2CToken(ctx, authCode)`.

## Finance & Settlement

- **Hold settlement**: set `AdditionalInfo.HoldSettlement` to `true` on any
  payment request, then call `client.Settlement.ReleaseSettlement(...)`
  (optionally with `OverrideSettlement`).
- **Split settlement**: set `AdditionalInfo.Settlement` (`BankAccountSettlementID`,
  `Value`, `Type: "FIX" | "PERCENTAGE"`) on the payment; manage existing rules
  with `SplitSettlementCheckStatus` / `SplitSettlementEdit` /
  `SplitSettlementCancel`.
- **Custom settlement report**: set `AdditionalInfo.Report` (`Key` ≤ 64 chars,
  `Value` ≤ 128 chars); the pairs become columns in the settlement report
  export.
- **Bulk registration bank**: manage settlement bank accounts host-to-host with
  `CreateSettlementBankAccount` / `UpdateSettlementBankAccount` /
  `GetSettlementBankAccount` (`BankAccount: { Code, Number, Name, Currency,
  Country }`). Must be activated for your merchant account in the DOKU
  dashboard first; `Update`/`Get` need the account's
  `BankAccountSettlementID` / `{BankCode}/{AccountNumber}`.

> **Host caveat:** the docs show the split-settlement and bulk-registration-bank
> host only as the placeholder
> `{sandbox|production}.doku.com/fc-h2h-api`. `ReleaseSettlement` uses the
> confirmed `{api|api-sandbox}.doku.com/finance/v1/release`. If you learn the
> concrete fc-h2h host, pass `FinanceBaseURL: "https://…/fc-h2h-api"` to
> `doku.New`.

## Handling notifications

DOKU calls your Notification URL (configured in DOKU Back Office) with the same
Non-SNAP signature scheme, just in reverse — verify it with the raw body and
headers:

```go
http.HandleFunc("/api/doku/notifications", func(w http.ResponseWriter, r *http.Request) {
    rawBody, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, err.Error(), 500)
        return
    }

    ok := client.VerifyNotification(r.Header, rawBody, "/api/doku/notifications")
    if !ok {
        http.Error(w, "invalid signature", 401)
        return
    }

    notification, err := doku.ParseNotification(rawBody)
    if err != nil {
        http.Error(w, err.Error(), 400)
        return
    }

    if notification.Transaction.Status == "SUCCESS" {
        // mark notification.Order.InvoiceNumber as paid — make this idempotent,
        // DOKU may redeliver the same notification more than once
    }

    w.WriteHeader(200)
})
```

> **Raw body caveat:** the signature is a digest of the exact bytes DOKU sent.
> A body parser that parses-then-lets-you-re-`json.Marshal` it can reorder keys
> or change whitespace, breaking verification — read the raw body bytes first.

## API coverage

| Service | Endpoints | Notes |
| --- | --- | --- |
| `Checkout` | 1 | Initiate payment, deep-typed from docs |
| `VirtualAccountNonSnap` | 14 | 7 banks × generate/update, doc-annotated |
| `VirtualAccountSnap` | 12 | 11 banks create-va + SNAP check status, incl. `PartnerServiceID` helper |
| `CreditCard` | 2 | Payment page + refund |
| `Emoney` | 4 | Legacy OVO (check_sum), SNAP debit h2h, ShopeePay |
| `O2O`, `Paylater`, `DirectTransfer` | 6 | Generated |
| `DirectDebitLegacy` | 9 | BRI Direct Debit v1 |
| `DirectDebitSnap` | 34 | OVO/CIMB/ALLO/BRI/Mandiri/Dana/ShopeePay |
| `KKI` | 7 | Kartu Kredit Indonesia |
| `SubAccountV1` / `SubAccountV2` | 4 / 16 | Wallet-as-a-Service |
| `Misc` | 1 | Check Status API (Non-SNAP, cross-product) |
| `Settlement` | 7 | Split check-status/edit/cancel, release, bulk bank account create/update/get (hand-typed from docs) |

**Typing tiers.** Checkout, Credit Card, both VA families, Check Status and
all of `Settlement` are hand-typed against developers.doku.com. The remaining
generated modules have request interfaces inferred from the Postman collection's
example bodies (leaf fields optional, marked `TODO: verify against
developers.doku.com`) and return `json.RawMessage` — narrow as needed.

## Development

```sh
go test ./...          # go:test (signature + client behavior, no network)
go vet ./...
```

The 8 generated modules are produced from
`scripts/ts2go.mjs` reading `../doku/src/products/*.ts`:

```sh
cd scripts && npm install typescript   # one-time
cd ..
node scripts/ts2go.mjs ../doku        # writes zz_<module>_gen.go
gofmt -w .
```

## Known caveats

- DOKU's docs currently list CC refund as `/cancellation/credit-card/refund`
  while the collection uses `/credit-card/v1/cancellation/credit-card/refund`;
  this SDK follows the collection.
- The split-settlement host is unconfirmed (see above); a `FinanceBaseURL`
  override exists.
- Generated modules carry `TODO: verify` markers — tighten endpoint-by-endpoint
  as you use them.

## License

MIT