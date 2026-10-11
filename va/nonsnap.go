// Package va is the DOKU Virtual Account API: legacy Non-SNAP payment codes
// (NewNonSnap) and BI-SNAP create-va / check-status (NewSnap).
package va

import (
	"context"
	"encoding/json"
	doku "github.com/harizinside/go-doku"
	"github.com/harizinside/go-doku/settlement"
)

// NonSnap exposes the legacy Non-SNAP Virtual Account
// APIs (v2 payment-code), one pair of generate/update endpoints per bank.
// Hand-typed from the DOKU Postman collection.
type NonSnap struct{ c *doku.Client }

// NewNonSnap returns the service bound to c.
func NewNonSnap(c *doku.Client) *NonSnap { return &NonSnap{c: c} }

// Order is the shared VA order block.
type Order struct {
	// Merchant invoice number that identifies the order. Max length: 64
	InvoiceNumber string `json:"invoice_number"`
	// Payment amount. Optional for open-amount VAs (e.g. Mandiri NO_BILL)
	Amount *float64 `json:"amount,omitempty"`
}

// Customer is the shared VA customer block.
type Customer struct {
	// Customer name shown on the payment page/statement
	Name *string `json:"name,omitempty"`
	// Customer email
	Email   *string `json:"email,omitempty"`
	Phone   *string `json:"phone,omitempty"`
	Address *string `json:"address,omitempty"`
	Country *string `json:"country,omitempty"`
}

// InfoBase is the shared virtual_account_info fields across banks.
type InfoBase struct {
	// VA expiry in minutes
	ExpiredTime *float64 `json:"expired_time,omitempty"`
	// true: VA can be paid multiple times
	ReusableStatus *bool `json:"reusable_status,omitempty"`
}

type PaymentCodeResponseOrder struct {
	InvoiceNumber *string         `json:"invoice_number,omitempty"`
	Amount        json.RawMessage `json:"amount,omitempty"`
	Currency      *string         `json:"currency,omitempty"`
}

type PaymentCodeResponseVirtualAccountInfo struct {
	VirtualAccountNumber *string         `json:"virtual_account_number,omitempty"`
	CreatedDate          *string         `json:"created_date,omitempty"`
	ExpiredDate          *string         `json:"expired_date,omitempty"`
	ReusableStatus       json.RawMessage `json:"reusable_status,omitempty"`
	BillingType          *string         `json:"billing_type,omitempty"`
}

// PaymentCodeResponse is the shared response shape for VA generate/update.
type PaymentCodeResponse struct {
	Order              *PaymentCodeResponseOrder              `json:"order,omitempty"`
	Customer           *Customer                              `json:"customer,omitempty"`
	VirtualAccountInfo *PaymentCodeResponseVirtualAccountInfo `json:"virtual_account_info,omitempty"`
	AdditionalInfo     json.RawMessage                        `json:"additional_info,omitempty"`
}

func (s *NonSnap) generate(ctx context.Context, path string, body any) (*PaymentCodeResponse, error) {
	var out PaymentCodeResponse
	if err := s.c.RequestNonSnap(ctx, "POST", path, body, &out, ""); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *NonSnap) update(ctx context.Context, path string, body any) (*PaymentCodeResponse, error) {
	var out PaymentCodeResponse
	if err := s.c.RequestNonSnap(ctx, "PATCH", path, body, &out, ""); err != nil {
		return nil, err
	}
	return &out, nil
}

type MandiriAdditionalInfoAccount struct {
	// Sub Account id, e.g. SAC-2832-...
	ID *string `json:"id,omitempty"`
}

// MandiriAdditionalInfo combines settlement.AdditionalInfo with Mandiri's
// account.id sub-account routing.
type MandiriAdditionalInfo struct {
	*settlement.AdditionalInfo
	Account *MandiriAdditionalInfoAccount `json:"account,omitempty"`
}

// ---------------------------------------------------------------------------
// Mandiri VA — /mandiri-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

type MandiriVaGenerateRequestVirtualAccountInfo struct {
	// Mandiri-specific billing type, e.g. "NO_BILL"
	BillingType    *string `json:"billing_type,omitempty"`
	ReusableStatus *bool   `json:"reusable_status,omitempty"`
}

// MandiriVaGenerateRequest is the body for POST /mandiri-virtual-account/v2/payment-code.
type MandiriVaGenerateRequest struct {
	Order              Order                                      `json:"order"`
	VirtualAccountInfo MandiriVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                                  `json:"customer,omitempty"`
	// Route settlement to a Sub Account (wallet)
	AdditionalInfo *MandiriAdditionalInfo `json:"additional_info,omitempty"`
}

type MandiriVaUpdateRequestVirtualAccountInfo struct {
	InfoBase
	// VA number previously generated for this invoice
	VirtualAccountNumber *string `json:"virtual_account_number,omitempty"`
	BillingType          *string `json:"billing_type,omitempty"`
}

// MandiriVaUpdateRequest is the body for PATCH /mandiri-virtual-account/v2/payment-code.
type MandiriVaUpdateRequest struct {
	Order              Order                                    `json:"order"`
	VirtualAccountInfo MandiriVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                                `json:"customer,omitempty"`
	AdditionalInfo     *MandiriAdditionalInfo                   `json:"additional_info,omitempty"`
}

// MandiriGeneratePaymentCode calls POST /mandiri-virtual-account/v2/payment-code.
func (s *NonSnap) MandiriGeneratePaymentCode(ctx context.Context, body MandiriVaGenerateRequest) (*PaymentCodeResponse, error) {
	return s.generate(ctx, "/mandiri-virtual-account/v2/payment-code", body)
}

// MandiriUpdatePaymentCode calls PATCH /mandiri-virtual-account/v2/payment-code.
func (s *NonSnap) MandiriUpdatePaymentCode(ctx context.Context, body MandiriVaUpdateRequest) (*PaymentCodeResponse, error) {
	return s.update(ctx, "/mandiri-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// BCA VA — /bca-virtual-account/v2/payment-code (MGPC: merchant provides the number)
// ---------------------------------------------------------------------------

type BcaVaGenerateRequestVirtualAccountInfo struct {
	InfoBase
	// Full BCA VA number (company code + digits)
	VirtualAccountNumber *string `json:"virtual_account_number,omitempty"`
	// e.g. "NO_BILL"
	BillingType *string `json:"billing_type,omitempty"`
}

// BcaVaGenerateRequest is the body for POST /bca-virtual-account/v2/payment-code.
type BcaVaGenerateRequest struct {
	Order              Order                                  `json:"order"`
	VirtualAccountInfo BcaVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                              `json:"customer,omitempty"`
}

type BcaVaUpdateRequestVirtualAccountInfo struct {
	InfoBase
	VirtualAccountNumber string `json:"virtual_account_number"`
	// Free-text info lines printed on the BCA statement
	Info1 *string `json:"info1,omitempty"`
	Info2 *string `json:"info2,omitempty"`
	Info3 *string `json:"info3,omitempty"`
}

// BcaVaUpdateRequest is the body for PATCH /bca-virtual-account/v2/payment-code.
type BcaVaUpdateRequest struct {
	Order              Order                                `json:"order"`
	VirtualAccountInfo BcaVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                            `json:"customer,omitempty"`
}

// BcaGeneratePaymentCode calls POST /bca-virtual-account/v2/payment-code.
//
// Note: the source Postman collection has a copy-paste bug and points this
// request at /bri-virtual-account/v2/merchant-payment-code (a path with no
// corresponding BRI folder anywhere else in the collection). Verified against
// DOKU's official docs (jokul.doku.com) that /bca-virtual-account/v2/payment-code
// is correct here.
func (s *NonSnap) BcaGeneratePaymentCode(ctx context.Context, body BcaVaGenerateRequest) (*PaymentCodeResponse, error) {
	return s.generate(ctx, "/bca-virtual-account/v2/payment-code", body)
}

// BcaUpdatePaymentCode calls PATCH /bca-virtual-account/v2/payment-code.
func (s *NonSnap) BcaUpdatePaymentCode(ctx context.Context, body BcaVaUpdateRequest) (*PaymentCodeResponse, error) {
	return s.update(ctx, "/bca-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// BSI VA — /bsm-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

type BsiVaGenerateRequestVirtualAccountInfo struct {
	InfoBase
	Info1 *string `json:"info1,omitempty"`
	Info2 *string `json:"info2,omitempty"`
	Info3 *string `json:"info3,omitempty"`
}

// BsiVaGenerateRequest is the body for POST /bsm-virtual-account/v2/payment-code.
type BsiVaGenerateRequest struct {
	Order              Order                                  `json:"order"`
	VirtualAccountInfo BsiVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                              `json:"customer,omitempty"`
}

type BsiVaUpdateRequestVirtualAccountInfo struct {
	InfoBase
	VirtualAccountNumber string  `json:"virtual_account_number"`
	Info1                *string `json:"info1,omitempty"`
	Info2                *string `json:"info2,omitempty"`
	Info3                *string `json:"info3,omitempty"`
}

// BsiVaUpdateRequest extends the generate request with the existing VA number.
type BsiVaUpdateRequest struct {
	Order              Order                                `json:"order"`
	VirtualAccountInfo BsiVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                            `json:"customer,omitempty"`
}

// BsiGeneratePaymentCode calls POST /bsm-virtual-account/v2/payment-code.
func (s *NonSnap) BsiGeneratePaymentCode(ctx context.Context, body BsiVaGenerateRequest) (*PaymentCodeResponse, error) {
	return s.generate(ctx, "/bsm-virtual-account/v2/payment-code", body)
}

// BsiUpdatePaymentCode calls PATCH /bsm-virtual-account/v2/payment-code.
func (s *NonSnap) BsiUpdatePaymentCode(ctx context.Context, body BsiVaUpdateRequest) (*PaymentCodeResponse, error) {
	return s.update(ctx, "/bsm-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// DOKU VA — /doku-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

type DokuVaGenerateRequestVirtualAccountInfo struct {
	InfoBase
	Info1 *string `json:"info1,omitempty"`
	Info2 *string `json:"info2,omitempty"`
	Info3 *string `json:"info3,omitempty"`
}

// DokuVaGenerateRequest is the body for POST /doku-virtual-account/v2/payment-code.
type DokuVaGenerateRequest struct {
	Order              Order                                   `json:"order"`
	VirtualAccountInfo DokuVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                               `json:"customer,omitempty"`
}

type DokuVaUpdateRequestVirtualAccountInfo struct {
	InfoBase
	VirtualAccountNumber string  `json:"virtual_account_number"`
	Info1                *string `json:"info1,omitempty"`
	Info2                *string `json:"info2,omitempty"`
	Info3                *string `json:"info3,omitempty"`
}

// DokuVaUpdateRequest extends the generate request with the existing VA number.
type DokuVaUpdateRequest struct {
	Order              Order                                 `json:"order"`
	VirtualAccountInfo DokuVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                             `json:"customer,omitempty"`
}

// DokuGeneratePaymentCode calls POST /doku-virtual-account/v2/payment-code.
func (s *NonSnap) DokuGeneratePaymentCode(ctx context.Context, body DokuVaGenerateRequest) (*PaymentCodeResponse, error) {
	return s.generate(ctx, "/doku-virtual-account/v2/payment-code", body)
}

// DokuUpdatePaymentCode calls PATCH /doku-virtual-account/v2/payment-code.
func (s *NonSnap) DokuUpdatePaymentCode(ctx context.Context, body DokuVaUpdateRequest) (*PaymentCodeResponse, error) {
	return s.update(ctx, "/doku-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// Permata VA — /permata-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

// RefInfo is a Permata-style reference info line printed on the statement.
type RefInfo struct {
	RefName  string `json:"ref_name"`
	RefValue string `json:"ref_value"`
}

type PermataVaGenerateRequestVirtualAccountInfo struct {
	InfoBase
	// Custom reference info printed on the Permata statement
	RefInfo []RefInfo `json:"ref_info,omitempty"`
}

// PermataVaGenerateRequest is the body for POST /permata-virtual-account/v2/payment-code.
type PermataVaGenerateRequest struct {
	Order              Order                                      `json:"order"`
	VirtualAccountInfo PermataVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                                  `json:"customer,omitempty"`
}

type PermataVaUpdateRequestVirtualAccountInfo struct {
	InfoBase
	VirtualAccountNumber string    `json:"virtual_account_number"`
	RefInfo              []RefInfo `json:"ref_info,omitempty"`
}

// PermataVaUpdateRequest is the body for PATCH /permata-virtual-account/v2/payment-code.
type PermataVaUpdateRequest struct {
	Order              Order                                    `json:"order"`
	VirtualAccountInfo PermataVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                                `json:"customer,omitempty"`
}

// PermataGeneratePaymentCode calls POST /permata-virtual-account/v2/payment-code.
func (s *NonSnap) PermataGeneratePaymentCode(ctx context.Context, body PermataVaGenerateRequest) (*PaymentCodeResponse, error) {
	return s.generate(ctx, "/permata-virtual-account/v2/payment-code", body)
}

// PermataUpdatePaymentCode calls PATCH /permata-virtual-account/v2/payment-code.
func (s *NonSnap) PermataUpdatePaymentCode(ctx context.Context, body PermataVaUpdateRequest) (*PaymentCodeResponse, error) {
	return s.update(ctx, "/permata-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// CIMB VA — /cimb-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

type CimbVaGenerateRequestVirtualAccountInfo struct {
	InfoBase
	RefInfo []RefInfo `json:"ref_info,omitempty"`
}

// CimbVaGenerateRequest is the body for POST /cimb-virtual-account/v2/payment-code.
type CimbVaGenerateRequest struct {
	Order              Order                                   `json:"order"`
	VirtualAccountInfo CimbVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                               `json:"customer,omitempty"`
}

type CimbVaUpdateRequestVirtualAccountInfo struct {
	InfoBase
	VirtualAccountNumber string    `json:"virtual_account_number"`
	RefInfo              []RefInfo `json:"ref_info,omitempty"`
}

// CimbVaUpdateRequest is the body for PATCH /cimb-virtual-account/v2/payment-code.
type CimbVaUpdateRequest struct {
	Order              Order                                 `json:"order"`
	VirtualAccountInfo CimbVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                             `json:"customer,omitempty"`
}

// CimbGeneratePaymentCode calls POST /cimb-virtual-account/v2/payment-code.
func (s *NonSnap) CimbGeneratePaymentCode(ctx context.Context, body CimbVaGenerateRequest) (*PaymentCodeResponse, error) {
	return s.generate(ctx, "/cimb-virtual-account/v2/payment-code", body)
}

// CimbUpdatePaymentCode calls PATCH /cimb-virtual-account/v2/payment-code.
func (s *NonSnap) CimbUpdatePaymentCode(ctx context.Context, body CimbVaUpdateRequest) (*PaymentCodeResponse, error) {
	return s.update(ctx, "/cimb-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// BNI VA — /bni-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

type BniVaGenerateRequestVirtualAccountInfo struct {
	InfoBase
	// BNI billing type, e.g. "FIXED". NOTE: literally expects the misspelled `biling_type`.
	BilingType *string `json:"biling_type,omitempty"`
	// Product detail printed on the BNI statement
	Info *string `json:"info,omitempty"`
	// Merchant-side unique reference
	MerchantUniqueReference *string `json:"merchant_unique_reference,omitempty"`
}

// BniVaGenerateRequest is the body for POST /bni-virtual-account/v2/payment-code.
//
// NOTE: DOKU's API literally expects the misspelled key `biling_type`.
type BniVaGenerateRequest struct {
	Order              Order                                  `json:"order"`
	VirtualAccountInfo BniVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                              `json:"customer,omitempty"`
}

type BniVaUpdateRequestVirtualAccountInfo struct {
	InfoBase
	VirtualAccountNumber string `json:"virtual_account_number"`
	// NOTE: literally expects the misspelled `biling_type`
	BilingType              *string `json:"biling_type,omitempty"`
	Info                    *string `json:"info,omitempty"`
	MerchantUniqueReference *string `json:"merchant_unique_reference,omitempty"`
}

// BniVaUpdateRequest is the body for PATCH /bni-virtual-account/v2/payment-code.
type BniVaUpdateRequest struct {
	Order              Order                                `json:"order"`
	VirtualAccountInfo BniVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *Customer                            `json:"customer,omitempty"`
}

// BniGeneratePaymentCode calls POST /bni-virtual-account/v2/payment-code.
func (s *NonSnap) BniGeneratePaymentCode(ctx context.Context, body BniVaGenerateRequest) (*PaymentCodeResponse, error) {
	return s.generate(ctx, "/bni-virtual-account/v2/payment-code", body)
}

// BniUpdatePaymentCode calls PATCH /bni-virtual-account/v2/payment-code.
func (s *NonSnap) BniUpdatePaymentCode(ctx context.Context, body BniVaUpdateRequest) (*PaymentCodeResponse, error) {
	return s.update(ctx, "/bni-virtual-account/v2/payment-code", body)
}
