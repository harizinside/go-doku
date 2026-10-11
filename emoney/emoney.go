// Package emoney is the DOKU E-Money API: legacy OVO push payment, SNAP
// debit host-to-host (DANA / ShopeePay / OVO) and ShopeePay orders.
package emoney

import (
	"context"
	"encoding/json"
	doku "github.com/harizinside/go-doku"
	"strconv"
)

// Service exposes the E-Money endpoints that don't fit the generic
// codegen shape: Legacy OVO Push Payment (check_sum), generic SNAP debit
// host-to-host (DANA / ShopeePay / OVO SNAP e-money), ShopeePay order.
type Service struct{ c *doku.Client }

// New returns the service bound to c.
func New(c *doku.Client) *Service { return &Service{c: c} }

// ---------------------------------------------------------------------------
// Legacy OVO Push Payment (check_sum based)
// ---------------------------------------------------------------------------

// OVOPaymentRequest is the input for ovoPayment.
type OVOPaymentRequest struct {
	// Merchant invoice number
	InvoiceNumber string `json:"invoice_number"`
	// Payment amount
	Amount float64 `json:"amount"`
	// Customer's OVO registered phone number, e.g. "081211111111"
	OvoID string `json:"ovo_id"`
}

// OVOVoidRequest extends the payment request with payment identifiers.
type OVOVoidRequest struct {
	OVOPaymentRequest
	BatchNumber     any `json:"batch_number"` // string | number
	TraceNumber     any `json:"trace_number"`
	ReferenceNumber any `json:"reference_number"`
	ApprovalCode    any `json:"approval_code"`
}

func (s *Service) ovoBody(input OVOPaymentRequest, paymentFields map[string]any) map[string]any {
	checkSum := doku.OVOCheckSum(doku.OvoCheckSumInput{
		Amount:          anyToString(input.Amount),
		ClientID:        s.c.ClientID(),
		InvoiceNumber:   input.InvoiceNumber,
		OvoID:           input.OvoID,
		SecretKey:       s.c.SecretKey(),
		ApprovalCode:    stringFromAny(paymentFields["approval_code"]),
		BatchNumber:     stringFromAny(paymentFields["batch_number"]),
		ReferenceNumber: stringFromAny(paymentFields["reference_number"]),
		TraceNumber:     stringFromAny(paymentFields["trace_number"]),
	})
	body := map[string]any{
		"client": map[string]any{"id": s.c.ClientID()},
		"order": map[string]any{
			"invoice_number": input.InvoiceNumber,
			"amount":         input.Amount,
		},
		"ovo_info": map[string]any{
			"ovo_id": input.OvoID,
		},
		"security": map[string]any{
			"check_sum": checkSum,
		},
	}
	if len(paymentFields) > 0 {
		body["ovo_payment"] = paymentFields
	}
	return body
}

func anyToString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// Default JSON marshaling uses float64; preserve precision with no
		// scientific notation via fmt's default. Most amounts are short.
		return formatFloat(x)
	case int:
		return formatFloat(float64(x))
	case int64:
		return formatFloat(float64(x))
	default:
		return ""
	}
}

func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return anyToString(v)
}

// OVOPayment calls POST /ovo-emoney/v1/payment.
func (s *Service) OVOPayment(ctx context.Context, input OVOPaymentRequest) (json.RawMessage, error) {
	var out json.RawMessage
	err := s.c.RequestNonSnap(ctx, "POST", "/ovo-emoney/v1/payment", s.ovoBody(input, nil), &out, "")
	return out, err
}

// OVOVoid calls POST /ovo-emoney/v1/cancel.
func (s *Service) OVOVoid(ctx context.Context, input OVOVoidRequest) (json.RawMessage, error) {
	fields := map[string]any{
		"batch_number":     input.BatchNumber,
		"trace_number":     input.TraceNumber,
		"reference_number": input.ReferenceNumber,
		"approval_code":    input.ApprovalCode,
	}
	var out json.RawMessage
	err := s.c.RequestNonSnap(ctx, "POST", "/ovo-emoney/v1/cancel", s.ovoBody(input.OVOPaymentRequest, fields), &out, "")
	return out, err
}

// ---------------------------------------------------------------------------
// Generic SNAP debit payment host-to-host (DANA / ShopeePay / OVO SNAP e-money)
// ---------------------------------------------------------------------------

type DebitPaymentHostToHostRequestURLParam struct {
	URL *string `json:"url,omitempty"`
	// e.g. PAY_RETURN
	Type *string `json:"type,omitempty"`
	// "Y" | "N"
	IsDeepLink *string `json:"isDeepLink,omitempty"`
}

type DebitPaymentHostToHostRequestAmount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

type DebitPaymentHostToHostRequestAdditionalInfo struct {
	// e.g. EMONEY_DANA_SNAP, EMONEY_OVO_SNAP, EMONEY_SHOPEEPAY_SNAP
	Channel                    string  `json:"channel"`
	OrderTitle                 *string `json:"orderTitle,omitempty"`
	SupportDeepLinkCheckoutURL *string `json:"supportDeepLinkCheckoutUrl,omitempty"`
}

// DebitPaymentHostToHostRequest is the body for the SNAP debit h2h e-money
// payment.
type DebitPaymentHostToHostRequest struct {
	// Merchant invoice / reference number
	PartnerReferenceNo string `json:"partnerReferenceNo"`
	// ISO8601 datetime with offset, e.g. 2024-07-10T11:57:58+07:00
	ValidUpTo *string `json:"validUpTo,omitempty"`
	// e.g. "app"
	PointOfInitiation *string                                     `json:"pointOfInitiation,omitempty"`
	URLParam          *DebitPaymentHostToHostRequestURLParam      `json:"urlParam,omitempty"`
	Amount            DebitPaymentHostToHostRequestAmount         `json:"amount"`
	AdditionalInfo    DebitPaymentHostToHostRequestAdditionalInfo `json:"additionalInfo"`
}

// DebitPaymentHostToHost calls POST /direct-debit/core/v1/debit/payment-host-to-host
// (signed with secretKey, per DOKU's E-Money h2h reference script).
func (s *Service) DebitPaymentHostToHost(ctx context.Context, body DebitPaymentHostToHostRequest) (json.RawMessage, error) {
	var out json.RawMessage
	if err := s.c.RequestSnap(ctx, "POST", "/direct-debit/core/v1/debit/payment-host-to-host", doku.SnapOptions{Body: body}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// ShopeePay (Non-SNAP)
// ---------------------------------------------------------------------------

type ShopeePayCreateOrderRequestOrder struct {
	InvoiceNumber string  `json:"invoice_number"`
	Amount        float64 `json:"amount"`
}

// ShopeePayCreateOrderRequest is the body for POST /shopeepay-emoney/v2/order.
type ShopeePayCreateOrderRequest struct {
	Order ShopeePayCreateOrderRequestOrder `json:"order"`
}

// ShopeePayCreateOrder calls POST /shopeepay-emoney/v2/order.
func (s *Service) ShopeePayCreateOrder(ctx context.Context, body ShopeePayCreateOrderRequest) (json.RawMessage, error) {
	var out json.RawMessage
	if err := s.c.RequestNonSnap(ctx, "POST", "/shopeepay-emoney/v2/order", body, &out, ""); err != nil {
		return nil, err
	}
	return out, nil
}

// formatFloat formats a float64 with no trailing zeros, matching JS
// `String(number)` for the OVO check_sum field.
func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
