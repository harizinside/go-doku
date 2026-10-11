// Package settlement is the DOKU Finance & Settlement API: split settlement,
// hold & release, and settlement bank accounts.
package settlement

import (
	"context"
	"fmt"
	doku "github.com/harizinside/go-doku"
	"net/url"
)

// Service exposes the Finance & Settlement APIs (Split Settlement,
// Hold & Release, Bulk Bank Registration). Host-typed by
// developers.doku.com.
type Service struct{ c *doku.Client }

// New returns the service bound to c.
func New(c *doku.Client) *Service { return &Service{c: c} }

// Split is one Split Settlement rule.
// Split is calculated on the *net* settlement amount (order amount minus fees).
type Split struct {
	// Bank Settlement ID retrieved from the DOKU Back Office (e.g. SBS-1004-...)
	BankAccountSettlementID string `json:"bank_account_settlement_id"`
	// Exact IDR amount when Type is FIX, or the percentage when PERCENTAGE
	Value any    `json:"value"` // number or string
	Type  string `json:"type"`  // "FIX" | "PERCENTAGE"
}

// ReportEntry is one Custom Settlement Report column.
type ReportEntry struct {
	// Column name in the settlement report export. Max length: 64
	Key string `json:"key"`
	// Column value for the transaction. Max length: 128
	Value string `json:"value"`
}

// AdditionalInfo is the generic Direct-API `additional_info`
// extension for settlement features. Documented for Checkout but valid
// wherever `additional_info` is accepted.
type AdditionalInfo struct {
	// Hold & Release: defer the payout for this payment until released via
	// ReleaseSettlement
	HoldSettlement *bool `json:"hold_settlement,omitempty"`
	// Split Settlement: route the net settlement amount to one or more sub
	// bank accounts
	Settlement []Split `json:"settlement,omitempty"`
	// Custom Settlement Report: extra columns in the settlement report export
	Report []ReportEntry `json:"report,omitempty"`
}

// ---------------------------------------------------------------------------
// Split Settlement (host-to-host, Non-SNAP signing)
// ---------------------------------------------------------------------------

// SplitSettlementQuery is the body for split-settlement operations.
type SplitSettlementQuery struct {
	// Transaction invoice number
	InvoiceNumber string `json:"invoice_number"`
	// Transaction date in ISO-8601
	TransactionDate string  `json:"transaction_date"`
	Amount          float64 `json:"amount"`
}

// SplitSettlementCheckStatusItem is one split-settlement row.
type SplitSettlementCheckStatusItem struct {
	BankAccountSettlementID *string  `json:"bank_account_settlement_id,omitempty"`
	BankName                *string  `json:"bank_name,omitempty"`
	BankAccountNumber       *string  `json:"bank_account_number,omitempty"`
	BankAccountName         *string  `json:"bank_account_name,omitempty"`
	Value                   *float64 `json:"value,omitempty"`
	Type                    *string  `json:"type,omitempty"`
}

// SplitSettlementEditRequest extends the query with the new split list.
type SplitSettlementEditRequest struct {
	SplitSettlementQuery
	// At least 1 entry; empty arrays are rejected by DOKU
	SplitTransactionBankAccount []Split `json:"split_transaction_bank_account"`
}

// SplitSettlementCancelResponse is the cancel response.
type SplitSettlementCancelResponse struct {
	InvoiceNumber *string `json:"invoice_number,omitempty"`
	// Reformatted to yyyy-MM-dd by DOKU
	TransactionDate *string  `json:"transaction_date,omitempty"`
	Amount          *float64 `json:"amount,omitempty"`
}

// SplitSettlementCheckStatus calls POST /v1/split-settlement/check-status.
func (s *Service) SplitSettlementCheckStatus(ctx context.Context, body SplitSettlementQuery) ([]SplitSettlementCheckStatusItem, error) {
	var out []SplitSettlementCheckStatusItem
	if err := s.c.RequestNonSnap(ctx, "POST", "/v1/split-settlement/check-status", body, &out, s.c.FinanceBaseURL()); err != nil {
		return nil, err
	}
	return out, nil
}

// SplitSettlementEdit calls POST /v1/split-settlement/edit.
func (s *Service) SplitSettlementEdit(ctx context.Context, body SplitSettlementEditRequest) ([]SplitSettlementCheckStatusItem, error) {
	var out []SplitSettlementCheckStatusItem
	if err := s.c.RequestNonSnap(ctx, "POST", "/v1/split-settlement/edit", body, &out, s.c.FinanceBaseURL()); err != nil {
		return nil, err
	}
	return out, nil
}

// SplitSettlementCancel calls POST /v1/split-settlement/cancel.
func (s *Service) SplitSettlementCancel(ctx context.Context, body SplitSettlementQuery) (*SplitSettlementCancelResponse, error) {
	var out SplitSettlementCancelResponse
	if err := s.c.RequestNonSnap(ctx, "POST", "/v1/split-settlement/cancel", body, &out, s.c.FinanceBaseURL()); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Hold & Release Settlement
// ---------------------------------------------------------------------------

// ReleaseSettlementRequest is the body for release.
type ReleaseSettlementRequest struct {
	Order       ReleaseSettlementRequestOrder       `json:"order"`
	Transaction ReleaseSettlementRequestTransaction `json:"transaction"`
	// Override the split settlement rule at release time
	OverrideSettlement []Split `json:"override_settlement,omitempty"`
}

// ReleaseSettlementRequestOrder echoes the original payment's order.
type ReleaseSettlementRequestOrder struct {
	// Same invoice number as the original payment initiation
	InvoiceNumber string `json:"invoice_number"`
	// Same amount as the original payment initiation
	Amount float64 `json:"amount"`
	// Same currency as the original payment. Default: IDR
	Currency *string `json:"currency,omitempty"`
}

// ReleaseSettlementRequestTransaction echoes the original request id.
type ReleaseSettlementRequestTransaction struct {
	// The Request-Id sent when the payment was initiated
	OriginalRequestID string `json:"original_request_id"`
}

// ReleaseSettlementResponse is the response from POST /finance/v1/release.
type ReleaseSettlementResponse struct {
	Order       *ReleaseSettlementResponseOrder       `json:"order,omitempty"`
	Transaction *ReleaseSettlementResponseTransaction `json:"transaction,omitempty"`
}

// ReleaseSettlementResponseOrder echoes the order.
type ReleaseSettlementResponseOrder struct {
	InvoiceNumber *string  `json:"invoice_number,omitempty"`
	Amount        *float64 `json:"amount,omitempty"`
	Currency      *string  `json:"currency,omitempty"`
}

// ReleaseSettlementResponseTransaction echoes the transaction with the
// settlement date and net amount.
type ReleaseSettlementResponseTransaction struct {
	OriginalRequestID *string `json:"original_request_id,omitempty"`
	// yyyy-mm-dd, settlement date (H+1)
	SettlementDate *string `json:"settlement_date,omitempty"`
	// Amount actually settled, after fees are deducted
	SettlementAmount *float64 `json:"settlement_amount,omitempty"`
}

// ReleaseSettlement calls POST /finance/v1/release on the finance host.
func (s *Service) ReleaseSettlement(ctx context.Context, body ReleaseSettlementRequest) (*ReleaseSettlementResponse, error) {
	var out ReleaseSettlementResponse
	if err := s.c.RequestNonSnap(ctx, "POST", "/finance/v1/release", body, &out, s.c.FinanceBaseURL()); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Bulk Registration Bank (host-to-host, Non-SNAP signing)
// ---------------------------------------------------------------------------

// BankAccount is one bank account record.
type BankAccount struct {
	// Bank transfer code
	Code string `json:"code"`
	// Account number, numeric only
	Number string `json:"number"`
	// Account holder name, min 2 chars
	Name string `json:"name"`
	// ISO 4217 currency code
	Currency string `json:"currency"`
	// ISO 3166-1 alpha-2 country code
	Country string `json:"country"`
}

// CreateSettlementBankAccountRequest is the body for create.
type CreateSettlementBankAccountRequest struct {
	BankAccount BankAccount `json:"bank_account"`
}

type UpdateSettlementBankAccountRequestBankAccount struct {
	BankAccount
	// Bank Settlement ID of the account being updated (must already be "VERIFIED")
	BankAccountSettlementID string `json:"bank_account_settlement_id"`
}

// UpdateSettlementBankAccountRequest is the body for update. The
// SettlementID identifies the existing "VERIFIED" account.
type UpdateSettlementBankAccountRequest struct {
	BankAccount UpdateSettlementBankAccountRequestBankAccount `json:"bank_account"`
}

type BankAccountResponseBankAccount struct {
	BankAccount
	// "VERIFIED" | "UNVERIFIED" | other
	Status                  *string `json:"status,omitempty"`
	BankAccountSettlementID *string `json:"bank_account_settlement_id,omitempty"`
}

// BankAccountResponse is the response from bulk-bank-account calls.
type BankAccountResponse struct {
	BankAccount *BankAccountResponseBankAccount `json:"bank_account,omitempty"`
}

// CreateSettlementBankAccount calls POST /v2/settlement-bank-account.
func (s *Service) CreateSettlementBankAccount(ctx context.Context, body CreateSettlementBankAccountRequest) (*BankAccountResponse, error) {
	var out BankAccountResponse
	if err := s.c.RequestNonSnap(ctx, "POST", "/v2/settlement-bank-account", body, &out, s.c.FinanceBaseURL()); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSettlementBankAccount calls PUT /v2/settlement-bank-account.
func (s *Service) UpdateSettlementBankAccount(ctx context.Context, body UpdateSettlementBankAccountRequest) (*BankAccountResponse, error) {
	var out BankAccountResponse
	if err := s.c.RequestNonSnap(ctx, "PUT", "/v2/settlement-bank-account", body, &out, s.c.FinanceBaseURL()); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSettlementBankAccount calls GET /v2/settlement-bank-account/{bankCode}/{accountNumber}.
func (s *Service) GetSettlementBankAccount(ctx context.Context, bankCode, accountNumber string) (*BankAccountResponse, error) {
	if bankCode == "" || accountNumber == "" {
		return nil, fmt.Errorf("doku: bankCode and accountNumber are required")
	}
	path := fmt.Sprintf("/v2/settlement-bank-account/%s/%s", url.PathEscape(bankCode), url.PathEscape(accountNumber))
	var out BankAccountResponse
	if err := s.c.RequestNonSnap(ctx, "GET", path, nil, &out, s.c.FinanceBaseURL()); err != nil {
		return nil, err
	}
	return &out, nil
}
