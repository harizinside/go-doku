package doku

import (
	"context"
	"encoding/json"
)

// VirtualAccountNonSnapService exposes the legacy Non-SNAP Virtual Account
// APIs (v2 payment-code), one pair of generate/update endpoints per bank.
// Hand-typed from the DOKU Postman collection.
type VirtualAccountNonSnapService struct{ c *Client }

// VaOrder is the shared VA order block.
type VaOrder struct {
	// Merchant invoice number that identifies the order. Max length: 64
	InvoiceNumber string `json:"invoice_number"`
	// Payment amount. Optional for open-amount VAs (e.g. Mandiri NO_BILL)
	Amount *float64 `json:"amount,omitempty"`
}

// VaCustomer is the shared VA customer block.
type VaCustomer struct {
	// Customer name shown on the payment page/statement
	Name *string `json:"name,omitempty"`
	// Customer email
	Email   *string `json:"email,omitempty"`
	Phone   *string `json:"phone,omitempty"`
	Address *string `json:"address,omitempty"`
	Country *string `json:"country,omitempty"`
}

// VaInfoBase is the shared virtual_account_info fields across banks.
type VaInfoBase struct {
	// VA expiry in minutes
	ExpiredTime *float64 `json:"expired_time,omitempty"`
	// true: VA can be paid multiple times
	ReusableStatus *bool `json:"reusable_status,omitempty"`
}

// VaPaymentCodeResponse is the shared response shape for VA generate/update.
type VaPaymentCodeResponseOrder struct {
	InvoiceNumber *string         `json:"invoice_number,omitempty"`
	Amount        json.RawMessage `json:"amount,omitempty"`
	Currency      *string         `json:"currency,omitempty"`
}

type VaPaymentCodeResponseVirtualAccountInfo struct {
	VirtualAccountNumber *string         `json:"virtual_account_number,omitempty"`
	CreatedDate          *string         `json:"created_date,omitempty"`
	ExpiredDate          *string         `json:"expired_date,omitempty"`
	ReusableStatus       json.RawMessage `json:"reusable_status,omitempty"`
	BillingType          *string         `json:"billing_type,omitempty"`
}

type VaPaymentCodeResponse struct {
	Order              *VaPaymentCodeResponseOrder              `json:"order,omitempty"`
	Customer           *VaCustomer                              `json:"customer,omitempty"`
	VirtualAccountInfo *VaPaymentCodeResponseVirtualAccountInfo `json:"virtual_account_info,omitempty"`
	AdditionalInfo     json.RawMessage                          `json:"additional_info,omitempty"`
}

func (s *VirtualAccountNonSnapService) generate(ctx context.Context, path string, body any) (*VaPaymentCodeResponse, error) {
	var out VaPaymentCodeResponse
	if err := s.c.RequestNonSnap(ctx, "POST", path, body, &out, ""); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *VirtualAccountNonSnapService) update(ctx context.Context, path string, body any) (*VaPaymentCodeResponse, error) {
	var out VaPaymentCodeResponse
	if err := s.c.RequestNonSnap(ctx, "PATCH", path, body, &out, ""); err != nil {
		return nil, err
	}
	return &out, nil
}

// VaMandiriAdditionalInfo combines SettlementAdditionalInfo with Mandiri's
// account.id sub-account routing.
type VaMandiriAdditionalInfoAccount struct {
	// Sub Account id, e.g. SAC-2832-...
	ID *string `json:"id,omitempty"`
}

type VaMandiriAdditionalInfo struct {
	*SettlementAdditionalInfo
	Account *VaMandiriAdditionalInfoAccount `json:"account,omitempty"`
}

// ---------------------------------------------------------------------------
// Mandiri VA — /mandiri-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

// MandiriVaGenerateRequest is the body for POST /mandiri-virtual-account/v2/payment-code.
type MandiriVaGenerateRequestVirtualAccountInfo struct {
	// Mandiri-specific billing type, e.g. "NO_BILL"
	BillingType    *string `json:"billing_type,omitempty"`
	ReusableStatus *bool   `json:"reusable_status,omitempty"`
}

type MandiriVaGenerateRequest struct {
	Order              VaOrder                                    `json:"order"`
	VirtualAccountInfo MandiriVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                                `json:"customer,omitempty"`
	// Route settlement to a Sub Account (wallet)
	AdditionalInfo *VaMandiriAdditionalInfo `json:"additional_info,omitempty"`
}

// MandiriVaUpdateRequest is the body for PATCH /mandiri-virtual-account/v2/payment-code.
type MandiriVaUpdateRequestVirtualAccountInfo struct {
	VaInfoBase
	// VA number previously generated for this invoice
	VirtualAccountNumber *string `json:"virtual_account_number,omitempty"`
	BillingType          *string `json:"billing_type,omitempty"`
}

type MandiriVaUpdateRequest struct {
	Order              VaOrder                                  `json:"order"`
	VirtualAccountInfo MandiriVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                              `json:"customer,omitempty"`
	AdditionalInfo     *VaMandiriAdditionalInfo                 `json:"additional_info,omitempty"`
}

// MandiriGeneratePaymentCode calls POST /mandiri-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) MandiriGeneratePaymentCode(ctx context.Context, body MandiriVaGenerateRequest) (*VaPaymentCodeResponse, error) {
	return s.generate(ctx, "/mandiri-virtual-account/v2/payment-code", body)
}

// MandiriUpdatePaymentCode calls PATCH /mandiri-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) MandiriUpdatePaymentCode(ctx context.Context, body MandiriVaUpdateRequest) (*VaPaymentCodeResponse, error) {
	return s.update(ctx, "/mandiri-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// BCA VA — /bca-virtual-account/v2/payment-code (MGPC: merchant provides the number)
// ---------------------------------------------------------------------------

// BcaVaGenerateRequest is the body for POST /bca-virtual-account/v2/payment-code.
type BcaVaGenerateRequestVirtualAccountInfo struct {
	VaInfoBase
	// Full BCA VA number (company code + digits)
	VirtualAccountNumber *string `json:"virtual_account_number,omitempty"`
	// e.g. "NO_BILL"
	BillingType *string `json:"billing_type,omitempty"`
}

type BcaVaGenerateRequest struct {
	Order              VaOrder                                `json:"order"`
	VirtualAccountInfo BcaVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                            `json:"customer,omitempty"`
}

// BcaVaUpdateRequest is the body for PATCH /bca-virtual-account/v2/payment-code.
type BcaVaUpdateRequestVirtualAccountInfo struct {
	VaInfoBase
	VirtualAccountNumber string `json:"virtual_account_number"`
	// Free-text info lines printed on the BCA statement
	Info1 *string `json:"info1,omitempty"`
	Info2 *string `json:"info2,omitempty"`
	Info3 *string `json:"info3,omitempty"`
}

type BcaVaUpdateRequest struct {
	Order              VaOrder                              `json:"order"`
	VirtualAccountInfo BcaVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                          `json:"customer,omitempty"`
}

// BcaGeneratePaymentCode calls POST /bca-virtual-account/v2/payment-code.
//
// Note: the source Postman collection has a copy-paste bug and points this
// request at /bri-virtual-account/v2/merchant-payment-code (a path with no
// corresponding BRI folder anywhere else in the collection). Verified against
// DOKU's official docs (jokul.doku.com) that /bca-virtual-account/v2/payment-code
// is correct here.
func (s *VirtualAccountNonSnapService) BcaGeneratePaymentCode(ctx context.Context, body BcaVaGenerateRequest) (*VaPaymentCodeResponse, error) {
	return s.generate(ctx, "/bca-virtual-account/v2/payment-code", body)
}

// BcaUpdatePaymentCode calls PATCH /bca-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) BcaUpdatePaymentCode(ctx context.Context, body BcaVaUpdateRequest) (*VaPaymentCodeResponse, error) {
	return s.update(ctx, "/bca-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// BSI VA — /bsm-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

// BsiVaGenerateRequest is the body for POST /bsm-virtual-account/v2/payment-code.
type BsiVaGenerateRequestVirtualAccountInfo struct {
	VaInfoBase
	Info1 *string `json:"info1,omitempty"`
	Info2 *string `json:"info2,omitempty"`
	Info3 *string `json:"info3,omitempty"`
}

type BsiVaGenerateRequest struct {
	Order              VaOrder                                `json:"order"`
	VirtualAccountInfo BsiVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                            `json:"customer,omitempty"`
}

// BsiVaUpdateRequest extends the generate request with the existing VA number.
type BsiVaUpdateRequestVirtualAccountInfo struct {
	VaInfoBase
	VirtualAccountNumber string  `json:"virtual_account_number"`
	Info1                *string `json:"info1,omitempty"`
	Info2                *string `json:"info2,omitempty"`
	Info3                *string `json:"info3,omitempty"`
}

type BsiVaUpdateRequest struct {
	Order              VaOrder                              `json:"order"`
	VirtualAccountInfo BsiVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                          `json:"customer,omitempty"`
}

// BsiGeneratePaymentCode calls POST /bsm-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) BsiGeneratePaymentCode(ctx context.Context, body BsiVaGenerateRequest) (*VaPaymentCodeResponse, error) {
	return s.generate(ctx, "/bsm-virtual-account/v2/payment-code", body)
}

// BsiUpdatePaymentCode calls PATCH /bsm-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) BsiUpdatePaymentCode(ctx context.Context, body BsiVaUpdateRequest) (*VaPaymentCodeResponse, error) {
	return s.update(ctx, "/bsm-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// DOKU VA — /doku-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

// DokuVaGenerateRequest is the body for POST /doku-virtual-account/v2/payment-code.
type DokuVaGenerateRequestVirtualAccountInfo struct {
	VaInfoBase
	Info1 *string `json:"info1,omitempty"`
	Info2 *string `json:"info2,omitempty"`
	Info3 *string `json:"info3,omitempty"`
}

type DokuVaGenerateRequest struct {
	Order              VaOrder                                 `json:"order"`
	VirtualAccountInfo DokuVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                             `json:"customer,omitempty"`
}

// DokuVaUpdateRequest extends the generate request with the existing VA number.
type DokuVaUpdateRequestVirtualAccountInfo struct {
	VaInfoBase
	VirtualAccountNumber string  `json:"virtual_account_number"`
	Info1                *string `json:"info1,omitempty"`
	Info2                *string `json:"info2,omitempty"`
	Info3                *string `json:"info3,omitempty"`
}

type DokuVaUpdateRequest struct {
	Order              VaOrder                               `json:"order"`
	VirtualAccountInfo DokuVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                           `json:"customer,omitempty"`
}

// DokuGeneratePaymentCode calls POST /doku-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) DokuGeneratePaymentCode(ctx context.Context, body DokuVaGenerateRequest) (*VaPaymentCodeResponse, error) {
	return s.generate(ctx, "/doku-virtual-account/v2/payment-code", body)
}

// DokuUpdatePaymentCode calls PATCH /doku-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) DokuUpdatePaymentCode(ctx context.Context, body DokuVaUpdateRequest) (*VaPaymentCodeResponse, error) {
	return s.update(ctx, "/doku-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// Permata VA — /permata-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

// VaRefInfo is a Permata-style reference info line printed on the statement.
type VaRefInfo struct {
	RefName  string `json:"ref_name"`
	RefValue string `json:"ref_value"`
}

// PermataVaGenerateRequest is the body for POST /permata-virtual-account/v2/payment-code.
type PermataVaGenerateRequestVirtualAccountInfo struct {
	VaInfoBase
	// Custom reference info printed on the Permata statement
	RefInfo []VaRefInfo `json:"ref_info,omitempty"`
}

type PermataVaGenerateRequest struct {
	Order              VaOrder                                    `json:"order"`
	VirtualAccountInfo PermataVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                                `json:"customer,omitempty"`
}

// PermataVaUpdateRequest is the body for PATCH /permata-virtual-account/v2/payment-code.
type PermataVaUpdateRequestVirtualAccountInfo struct {
	VaInfoBase
	VirtualAccountNumber string      `json:"virtual_account_number"`
	RefInfo              []VaRefInfo `json:"ref_info,omitempty"`
}

type PermataVaUpdateRequest struct {
	Order              VaOrder                                  `json:"order"`
	VirtualAccountInfo PermataVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                              `json:"customer,omitempty"`
}

// PermataGeneratePaymentCode calls POST /permata-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) PermataGeneratePaymentCode(ctx context.Context, body PermataVaGenerateRequest) (*VaPaymentCodeResponse, error) {
	return s.generate(ctx, "/permata-virtual-account/v2/payment-code", body)
}

// PermataUpdatePaymentCode calls PATCH /permata-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) PermataUpdatePaymentCode(ctx context.Context, body PermataVaUpdateRequest) (*VaPaymentCodeResponse, error) {
	return s.update(ctx, "/permata-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// CIMB VA — /cimb-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

// CimbVaGenerateRequest is the body for POST /cimb-virtual-account/v2/payment-code.
type CimbVaGenerateRequestVirtualAccountInfo struct {
	VaInfoBase
	RefInfo []VaRefInfo `json:"ref_info,omitempty"`
}

type CimbVaGenerateRequest struct {
	Order              VaOrder                                 `json:"order"`
	VirtualAccountInfo CimbVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                             `json:"customer,omitempty"`
}

// CimbVaUpdateRequest is the body for PATCH /cimb-virtual-account/v2/payment-code.
type CimbVaUpdateRequestVirtualAccountInfo struct {
	VaInfoBase
	VirtualAccountNumber string      `json:"virtual_account_number"`
	RefInfo              []VaRefInfo `json:"ref_info,omitempty"`
}

type CimbVaUpdateRequest struct {
	Order              VaOrder                               `json:"order"`
	VirtualAccountInfo CimbVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                           `json:"customer,omitempty"`
}

// CimbGeneratePaymentCode calls POST /cimb-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) CimbGeneratePaymentCode(ctx context.Context, body CimbVaGenerateRequest) (*VaPaymentCodeResponse, error) {
	return s.generate(ctx, "/cimb-virtual-account/v2/payment-code", body)
}

// CimbUpdatePaymentCode calls PATCH /cimb-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) CimbUpdatePaymentCode(ctx context.Context, body CimbVaUpdateRequest) (*VaPaymentCodeResponse, error) {
	return s.update(ctx, "/cimb-virtual-account/v2/payment-code", body)
}

// ---------------------------------------------------------------------------
// BNI VA — /bni-virtual-account/v2/payment-code
// ---------------------------------------------------------------------------

// BniVaGenerateRequest is the body for POST /bni-virtual-account/v2/payment-code.
//
// NOTE: DOKU's API literally expects the misspelled key `biling_type`.
type BniVaGenerateRequestVirtualAccountInfo struct {
	VaInfoBase
	// BNI billing type, e.g. "FIXED". NOTE: literally expects the misspelled `biling_type`.
	BilingType *string `json:"biling_type,omitempty"`
	// Product detail printed on the BNI statement
	Info *string `json:"info,omitempty"`
	// Merchant-side unique reference
	MerchantUniqueReference *string `json:"merchant_unique_reference,omitempty"`
}

type BniVaGenerateRequest struct {
	Order              VaOrder                                `json:"order"`
	VirtualAccountInfo BniVaGenerateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                            `json:"customer,omitempty"`
}

// BniVaUpdateRequest is the body for PATCH /bni-virtual-account/v2/payment-code.
type BniVaUpdateRequestVirtualAccountInfo struct {
	VaInfoBase
	VirtualAccountNumber string `json:"virtual_account_number"`
	// NOTE: literally expects the misspelled `biling_type`
	BilingType              *string `json:"biling_type,omitempty"`
	Info                    *string `json:"info,omitempty"`
	MerchantUniqueReference *string `json:"merchant_unique_reference,omitempty"`
}

type BniVaUpdateRequest struct {
	Order              VaOrder                              `json:"order"`
	VirtualAccountInfo BniVaUpdateRequestVirtualAccountInfo `json:"virtual_account_info"`
	Customer           *VaCustomer                          `json:"customer,omitempty"`
}

// BniGeneratePaymentCode calls POST /bni-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) BniGeneratePaymentCode(ctx context.Context, body BniVaGenerateRequest) (*VaPaymentCodeResponse, error) {
	return s.generate(ctx, "/bni-virtual-account/v2/payment-code", body)
}

// BniUpdatePaymentCode calls PATCH /bni-virtual-account/v2/payment-code.
func (s *VirtualAccountNonSnapService) BniUpdatePaymentCode(ctx context.Context, body BniVaUpdateRequest) (*VaPaymentCodeResponse, error) {
	return s.update(ctx, "/bni-virtual-account/v2/payment-code", body)
}
