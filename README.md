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
go get github.com/harizinside/go-doku@v0.2.0
```

Pin a release tag so `go get` never picks up untested WIP from `main`.

> **Server-only.** This SDK signs requests with your DOKU secret/private keys.
> Never load it from an end-user browser.

## Layout

The root package `doku` holds the `Client` (signing, SNAP token cache),
signing helpers and notification verification. Every product API is its own
package, bound to a client with `New` (VA has `NewNonSnap` / `NewSnap`):

| Package | Endpoints | API |
| --- | --- | --- |
| `checkout` | 1 | DOKU Checkout (hosted payment page) |
| `creditcard` | 2 | Credit Card payment page + refund |
| `va` | 14 + 12 | Virtual Accounts: legacy Non-SNAP (7 banks × generate/update) and BI-SNAP (11 banks create-va + check status) |
| `emoney` | 4 | Legacy OVO (check_sum), SNAP debit h2h, ShopeePay |
| `settlement` | 7 | Split settlement, hold & release, settlement bank accounts |
| `misc` | 1 | Check Status API (cross-product) |
| `directdebitsnap` | 34 | SNAP Direct Debit: OVO, CIMB, Allo, BRI, Mandiri, DANA, ShopeePay |
| `directdebitlegacy` | 9 | BRI Direct Debit (Non-SNAP) |
| `kki` | 7 | Kartu Kredit Indonesia |
| `subaccountv1` / `subaccountv2` | 4 / 16 | Sub Account (Wallet-as-a-Service) |
| `o2o`, `paylater`, `directtransfer` | 2 / 3 / 1 | Convenience store, Paylater, BCA Direct Transfer |

**Typing tiers.** `checkout`, `creditcard`, `va`, `emoney`, `settlement` and
`misc` are hand-typed against developers.doku.com. The other packages are
generated (`*_gen.go`) from the TS sources: request types are inferred from
DOKU's Postman examples and responses are `json.RawMessage`.

## Quick start

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    doku "github.com/harizinside/go-doku"
    "github.com/harizinside/go-doku/checkout"
    "github.com/harizinside/go-doku/misc"
    "github.com/harizinside/go-doku/settlement"
    "github.com/harizinside/go-doku/va"
)

func main() {
    client := doku.New(doku.Config{
        ClientID:   os.Getenv("DOKU_CLIENT_ID"),
        SecretKey:  os.Getenv("DOKU_SECRET_KEY"),
        PrivateKey: os.Getenv("DOKU_PRIVATE_KEY"), // PKCS#8 PEM, only for SNAP APIs (\n escapes are fine)
        Env:        doku.Sandbox,
    })
    ctx := context.Background()

    // DOKU Checkout — get a payment page URL
    page, err := checkout.New(client).InitiateCheckoutPayment(ctx, checkout.Request{
        Order:   checkout.RequestOrder{Amount: 20000, InvoiceNumber: "INV-0001"},
        Payment: checkout.RequestPayment{PaymentDueDate: doku.Ptr(60.0)},
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(*page.Response.Payment.URL)

    // Legacy (Non-SNAP) Mandiri VA
    _, err = va.NewNonSnap(client).MandiriGeneratePaymentCode(ctx, va.MandiriVaGenerateRequest{
        Order:              va.Order{InvoiceNumber: "INV-0002", Amount: doku.Ptr(50000.0)},
        VirtualAccountInfo: va.MandiriVaGenerateRequestVirtualAccountInfo{ReusableStatus: doku.Ptr(true), BillingType: doku.Ptr("NO_BILL")},
        Customer:           &va.Customer{Name: doku.Ptr("Anton"), Email: doku.Ptr("anton@example.com")},
    })

    // SNAP Mandiri VA (B2B token fetch + caching is automatic)
    _, err = va.NewSnap(client).MandiriSnapCreateVa(ctx, va.SnapCreateVaRequest{
        PartnerServiceId:   doku.PartnerServiceID("8899"), // -> "    8899"
        TrxId:              "INV-0002",
        TotalAmount:        va.SnapMoneyAmount{Value: "50000.00", Currency: "IDR"},
        VirtualAccountName: doku.Ptr("Anton"),
        ExpiredDate:        doku.Ptr(time.Now().Add(time.Hour).Format(time.RFC3339)),
        AdditionalInfo: &va.SnapVaAdditionalInfo{
            VirtualAccountConfig: &va.SnapVaConfig{ReusableStatus: doku.Ptr(true)},
        },
    })

    // Check order status
    _, err = misc.New(client).CheckOrderStatus(ctx, "INV-0001")

    // Release a held settlement
    _, err = settlement.New(client).ReleaseSettlement(ctx, settlement.ReleaseSettlementRequest{
        Order:       settlement.ReleaseSettlementRequestOrder{InvoiceNumber: "INV-0001", Amount: 20000},
        Transaction: settlement.ReleaseSettlementRequestTransaction{OriginalRequestID: "the-request-id-from-initiation"},
    })
}
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
customer token take a `*SnapOptions{CustomerToken: …}` argument — fetch one
with `client.SnapB2B2CToken(ctx, authCode)`.

## Finance & Settlement

- **Hold settlement**: set `AdditionalInfo.HoldSettlement` to `true` on any
  payment request, then call `settlement.New(client).ReleaseSettlement(...)`
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

## Development

```sh
go vet ./...
go test ./...   # no network: signatures, client behavior, per-package request checks
```

The generated packages (`*_gen.go`) come from `scripts/ts2go.mjs`, which reads
`../doku/src/products/*.ts` with the TypeScript compiler installed in that repo:

```sh
(cd ../doku && npm install)   # one-time
node scripts/ts2go.mjs ../doku
gofmt -w .
```

## Known caveats

- DOKU's docs currently list CC refund as `/cancellation/credit-card/refund`
  while the collection uses `/credit-card/v1/cancellation/credit-card/refund`;
  this SDK follows the collection.
- The split-settlement host is unconfirmed (see above); a `FinanceBaseURL`
  override exists.
- Generated packages are inferred from examples — tighten types
  endpoint-by-endpoint as you use them.

## License

MIT