package doku

import (
	"context"
	"encoding/json"
)

// VirtualAccountSnapService exposes the SNAP Virtual Account APIs (BI-SNAP
// create-va), one endpoint per bank. Hand-typed from the DOKU Postman
// collection.
//
// Headers sent automatically: X-PARTNER-ID, X-EXTERNAL-ID, X-TIMESTAMP,
// X-SIGNATURE (HMAC-SHA512 with secretKey), Authorization (B2B), CHANNEL-ID.
type VirtualAccountSnapService struct{ c *Client }

const snapChannelID = "H2H"

// SnapMoneyAmount is the SNAP amount shape.
type SnapMoneyAmount struct {
	// Amount string with 2 decimal places, e.g. "12500.00"
	Value string `json:"value"`
	// ISO 4217, e.g. "IDR"
	Currency string `json:"currency"`
}

// SnapVaBillDetail is one bill line.
type SnapVaBillDetailBillDescription struct {
	English   *string `json:"english,omitempty"`
	Indonesia *string `json:"indonesia,omitempty"`
}

type SnapVaBillDetail struct {
	BillCode        string                           `json:"billCode"`
	BillNo          string                           `json:"billNo"`
	BillName        string                           `json:"billName"`
	BillShortName   *string                          `json:"billShortName,omitempty"`
	BillDescription *SnapVaBillDetailBillDescription `json:"billDescription,omitempty"`
	BillSubCompany  *string                          `json:"billSubCompany,omitempty"`
	BillAmount      *SnapMoneyAmount                 `json:"billAmount,omitempty"`
	AdditionalInfo  json.RawMessage                  `json:"additionalInfo,omitempty"`
}

// SnapVaFreeText is the free-text shape used by some banks.
type SnapVaFreeText struct {
	English   *string `json:"english,omitempty"`
	Indonesia *string `json:"indonesia,omitempty"`
}

// SnapVaConfig holds the VA configuration object inside AdditionalInfo.
type SnapVaConfig struct {
	// e.g. { reusableStatus: true, minAmount, maxAmount }
	ReusableStatus *bool   `json:"reusableStatus,omitempty"`
	MinAmount      *string `json:"minAmount,omitempty"`
	MaxAmount      *string `json:"maxAmount,omitempty"`
}

// SnapVaAdditionalInfo is the additional_info block.
type SnapVaAdditionalInfo struct {
	VirtualAccountConfig *SnapVaConfig `json:"virtualAccountConfig,omitempty"`
}

// SnapCreateVaRequest is the standard DGPC body used by most banks.
type SnapCreateVaRequest struct {
	// 8-char partner service id derived from the acquirer BIN
	// (left-padded with spaces — see PartnerServiceID).
	PartnerServiceId string `json:"partnerServiceId"`
	// Merchant transaction id / invoice number. Max length: 18 per BI-SNAP
	TrxId string `json:"trxId"`
	// "1": closed amount, "O": open amount
	VirtualAccountTrxType *string         `json:"virtualAccountTrxType,omitempty"`
	TotalAmount           SnapMoneyAmount `json:"totalAmount"`
	// ISO8601 datetime
	ExpiredDate         *string               `json:"expiredDate,omitempty"`
	VirtualAccountName  *string               `json:"virtualAccountName,omitempty"`
	VirtualAccountEmail *string               `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone *string               `json:"virtualAccountPhone,omitempty"`
	BillDetails         []SnapVaBillDetail    `json:"billDetails,omitempty"`
	FreeTexts           []SnapVaFreeText      `json:"freeTexts,omitempty"`
	AdditionalInfo      *SnapVaAdditionalInfo `json:"additionalInfo,omitempty"`
}

// SnapCreateVaResponse is the create-va response.
type SnapCreateVaResponse struct {
	ResponseCode        *string          `json:"responseCode,omitempty"`
	ResponseMessage     *string          `json:"responseMessage,omitempty"`
	VirtualAccountNo    *string          `json:"virtualAccountNo,omitempty"`
	PartnerServiceID    *string          `json:"partnerServiceId,omitempty"`
	CustomerNo          *string          `json:"customerNo,omitempty"`
	VirtualAccountName  *string          `json:"virtualAccountName,omitempty"`
	VirtualAccountEmail *string          `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone *string          `json:"virtualAccountPhone,omitempty"`
	TrxId               *string          `json:"trxId,omitempty"`
	TotalAmount         *SnapMoneyAmount `json:"totalAmount,omitempty"`
	ExpiredDate         *string          `json:"expiredDate,omitempty"`
	AdditionalInfo      json.RawMessage  `json:"additionalInfo,omitempty"`
}

func (s *VirtualAccountSnapService) createVa(ctx context.Context, path string, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	var out SnapCreateVaResponse
	if err := s.c.RequestSnap(ctx, "POST", path, SnapOptions{Body: body, ChannelID: snapChannelID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BncCreateVa calls POST /virtual-accounts/bi-snap-va/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) BncCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/virtual-accounts/bi-snap-va/v1/transfer-va/create-va", body)
}

// BniCreateVa calls POST /virtual-accounts/bi-snap-va/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) BniCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/virtual-accounts/bi-snap-va/v1/transfer-va/create-va", body)
}

// BsiCreateVa calls POST /bi-snap-va/bsi/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) BsiCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/bi-snap-va/bsi/v1/transfer-va/create-va", body)
}

// BtnCreateVa calls POST /bi-snap-va/btn/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) BtnCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/bi-snap-va/btn/v1/transfer-va/create-va", body)
}

// DanamonCreateVa calls POST /bi-snap-va/danamon/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) DanamonCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/bi-snap-va/danamon/v1/transfer-va/create-va", body)
}

// MandiriSnapCreateVa calls POST /bi-snap-va/mandiri/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) MandiriSnapCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/bi-snap-va/mandiri/v1/transfer-va/create-va", body)
}

// MaybankCreateVa calls POST /bi-snap-va/maybank/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) MaybankCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/bi-snap-va/maybank/v1/transfer-va/create-va", body)
}

// PermataSnapCreateVa calls POST /bi-snap-va/permata/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) PermataSnapCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/bi-snap-va/permata/v1/transfer-va/create-va", body)
}

// SinarmasCreateVa calls POST /bi-snap-va/sinarmas/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) SinarmasCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/bi-snap-va/sinarmas/v1/transfer-va/create-va", body)
}

// CimbSnapCreateVa calls POST /bi-snap-va/cimb/v1/transfer-va/create-va.
func (s *VirtualAccountSnapService) CimbSnapCreateVa(ctx context.Context, body SnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	return s.createVa(ctx, "/bi-snap-va/cimb/v1/transfer-va/create-va", body)
}

// ---------------------------------------------------------------------------
// BRI (SNAP v1.1, MGPC-style open-amount VA)
// ---------------------------------------------------------------------------

// BriSnapAdditionalInfo is BRI's additional_info block.
type BriSnapAdditionalInfo struct {
	Channel              *string       `json:"channel,omitempty"`
	VirtualAccountConfig *SnapVaConfig `json:"virtualAccountConfig,omitempty"`
}

// BriSnapCreateVaRequest is the BRI-specific body.
type BriSnapCreateVaRequest struct {
	// 8-char partner service id, e.g. PartnerServiceID("02186") => "   02186"
	PartnerServiceID string `json:"partnerServiceId"`
	TrxId            string `json:"trxId"`
	// Customer number appended to the partner service id to form the VA number
	CustomerNo *string `json:"customerNo,omitempty"`
	// "O": open amount
	VirtualAccountTrxType *string         `json:"virtualAccountTrxType,omitempty"`
	TotalAmount           SnapMoneyAmount `json:"totalAmount"`
	ExpiredDate           *string         `json:"expiredDate,omitempty"`
	VirtualAccountName    *string         `json:"virtualAccountName,omitempty"`
	// Full VA number, space-padded to 18 chars
	VirtualAccountNo *string                `json:"virtualAccountNo,omitempty"`
	AdditionalInfo   *BriSnapAdditionalInfo `json:"additionalInfo,omitempty"`
}

// BriSnapCreateVa calls POST /virtual-accounts/bi-snap-va/v1.1/transfer-va/create-va.
func (s *VirtualAccountSnapService) BriSnapCreateVa(ctx context.Context, body BriSnapCreateVaRequest) (*SnapCreateVaResponse, error) {
	var out SnapCreateVaResponse
	if err := s.c.RequestSnap(ctx, "POST", "/virtual-accounts/bi-snap-va/v1.1/transfer-va/create-va", SnapOptions{Body: body, ChannelID: snapChannelID}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---------------------------------------------------------------------------
// Check Status (SNAP) — POST /orders/v1.0/transfer-va/status
// ---------------------------------------------------------------------------

// SnapCheckStatusRequest is the body for the SNAP check-status call.
type SnapCheckStatusRequest struct {
	// 8-char partner service id, same as used to create the VA
	PartnerServiceID string `json:"partnerServiceId"`
	// Max length: 20
	CustomerNo string `json:"customerNo"`
	// partnerServiceId + customerNo, max length: 28
	VirtualAccountNo string `json:"virtualAccountNo"`
	// Conditional: omit to get back an array of all transactions on this VA;
	// set to a specific inquiry's id to check just that one. Max length: 128
	InquiryRequestID *string `json:"inquiryRequestId,omitempty"`
	// Required if a payment has occurred. Max length: 128
	PaymentRequestID *string         `json:"paymentRequestId,omitempty"`
	AdditionalInfo   json.RawMessage `json:"additionalInfo,omitempty"`
}

// SnapCheckStatusResponse is the SNAP check-status response.
type SnapCheckStatusResponseVirtualAccountDataPaymentFlagReason struct {
	English   *string `json:"english,omitempty"`
	Indonesia *string `json:"indonesia,omitempty"`
}

type SnapCheckStatusResponseVirtualAccountData struct {
	PaymentFlagStatus *string                                                     `json:"paymentFlagStatus,omitempty"`
	PaymentFlagReason *SnapCheckStatusResponseVirtualAccountDataPaymentFlagReason `json:"paymentFlagReason,omitempty"`
	PaidAmount        *SnapMoneyAmount                                            `json:"paidAmount,omitempty"`
}

type SnapCheckStatusResponseAdditionalInfoAcquirer struct {
	ID *string `json:"id,omitempty"`
}

type SnapCheckStatusResponseAdditionalInfo struct {
	Acquirer *SnapCheckStatusResponseAdditionalInfoAcquirer `json:"acquirer,omitempty"`
}

type SnapCheckStatusResponse struct {
	ResponseCode       *string                                    `json:"responseCode,omitempty"`
	ResponseMessage    *string                                    `json:"responseMessage,omitempty"`
	VirtualAccountData *SnapCheckStatusResponseVirtualAccountData `json:"virtualAccountData,omitempty"`
	AdditionalInfo     *SnapCheckStatusResponseAdditionalInfo     `json:"additionalInfo,omitempty"`
}

// CheckVaStatus calls POST /orders/v1.0/transfer-va/status.
// Query at least 60 seconds after payment completion for an accurate status.
func (s *VirtualAccountSnapService) CheckVaStatus(ctx context.Context, body SnapCheckStatusRequest) (*SnapCheckStatusResponse, error) {
	var out SnapCheckStatusResponse
	if err := s.c.RequestSnap(ctx, "POST", "/orders/v1.0/transfer-va/status", SnapOptions{Body: body}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
