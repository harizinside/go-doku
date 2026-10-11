// Package creditcard is the DOKU Credit Card API (payment page and refund).
package creditcard

import (
	"context"
	"encoding/json"
	doku "github.com/harizinside/go-doku"
)

// Service exposes the Non-SNAP Credit Card endpoints (Payment
// Page + Refund). Hand-typed from developers.doku.com.
type Service struct{ c *doku.Client }

// New returns the service bound to c.
func New(c *doku.Client) *Service { return &Service{c: c} }

// PaymentPageRequest is the body for POST /credit-card/v1/payment-page.
type PaymentPageRequest struct {
	Order    Order     `json:"order"`
	Customer *Customer `json:"customer,omitempty"`
	// Conditional: for AUTHORIZE / INSTALLMENT flows
	Payment *PaymentDetails `json:"payment,omitempty"`
	// Card tokenization (pre-fill / force save)
	Card                  *Tokenization   `json:"card,omitempty"`
	OverrideConfiguration *OverrideConfig `json:"override_configuration,omitempty"`
	AdditionalInfo        *AdditionalInfo `json:"additional_info,omitempty"`
}

// Order is the order block.
type Order struct {
	// In IDR and without decimal
	Amount float64 `json:"amount"`
	// Max length: 64 (30 for Mandiri acquirer)
	InvoiceNumber string     `json:"invoice_number"`
	LineItems     []LineItem `json:"line_items,omitempty"`
	// Conditional: mandatory when `auto_redirect` is true
	CallbackURL *string `json:"callback_url,omitempty"`
	// Redirect target on failure; defaults to callback_url
	FailedURL *string `json:"failed_url,omitempty"`
	// Redirection to callback_url after payment completes. Default: false
	AutoRedirect *bool `json:"auto_redirect,omitempty"`
	// Custom statement descriptor (max 22 chars; must be activated by DOKU first)
	Descriptor *string `json:"descriptor,omitempty"`
	SessionID  *string `json:"session_id,omitempty"`
}

// LineItem is one line item.
type LineItem struct {
	Name     *string  `json:"name,omitempty"`
	Price    *float64 `json:"price,omitempty"`
	Quantity *float64 `json:"quantity,omitempty"`
}

// Customer is the customer block.
type Customer struct {
	// Conditional: mandatory for tokenization
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
	// Conditional: mandatory if phone is blank
	Email *string `json:"email,omitempty"`
	// Conditional: mandatory if email is blank. Format: {calling_code}{number}
	Phone   *string `json:"phone,omitempty"`
	Address *string `json:"address,omitempty"`
	Country *string `json:"country,omitempty"`
	City    *string `json:"city,omitempty"`
}

// PaymentDetails is the payment-type block.
type PaymentDetails struct {
	// Conditional: mandatory if you support multiple card transaction types
	Type *string `json:"type,omitempty"` // SALE | AUTHORIZE | INSTALLMENT
	// true: capture immediately after AUTHORIZE (behaves like SALE)
	AutoCapture *bool `json:"auto_capture,omitempty"`
	// Conditional: mandatory for INSTALLMENT (BNI, BRI, BANK_CIMB, BANK_MANDIRI, BCA, ...)
	Acquirer *string `json:"acquirer,omitempty"`
	// Conditional: mandatory for INSTALLMENT
	Tenor *float64 `json:"tenor,omitempty"`
}

// Tokenization is the card block.
type Tokenization struct {
	// Pre-fills the card number field
	Token *string `json:"token,omitempty"`
	// Force the customer to save the card token for the next payment
	Save *bool `json:"save,omitempty"`
}

// OverrideConfig is the override_configuration block.
type OverrideConfig struct {
	Themes *Themes `json:"themes,omitempty"`
	Promo  []Promo `json:"promo,omitempty"`
	// Only accept these BINs (6 or 8 digits)
	AllowBin []float64 `json:"allow_bin,omitempty"`
	// Only accept these installment tenors
	AllowTenor []float64 `json:"allow_tenor,omitempty"`
}

// Themes is the theme override block.
type Themes struct {
	// EN (default) | ID
	Language              *string `json:"language,omitempty"`
	BackgroundColor       *string `json:"background_color,omitempty"`
	FontColor             *string `json:"font_color,omitempty"`
	ButtonBackgroundColor *string `json:"button_background_color,omitempty"`
	ButtonFontColor       *string `json:"button_font_color,omitempty"`
}

// Promo is one promo entry.
type Promo struct {
	// BIN that gets the promo (6 or 8 digits)
	Bin *string `json:"bin,omitempty"`
	// final amount = order.amount - discount_amount
	DiscountAmount *float64 `json:"discount_amount,omitempty"`
}

type AdditionalInfoDisclaimer struct {
	ID json.RawMessage `json:"id,omitempty"`
	EN json.RawMessage `json:"en,omitempty"`
}

// AdditionalInfo is the additional_info block.
type AdditionalInfo struct {
	OverrideNotificationURL *string                   `json:"override_notification_url,omitempty"`
	Disclaimer              *AdditionalInfoDisclaimer `json:"disclaimer,omitempty"`
}

type PaymentPageResponseOrder struct {
	InvoiceNumber *string           `json:"invoice_number,omitempty"`
	LineItems     []json.RawMessage `json:"line_items,omitempty"`
}

type PaymentPageResponseCreditCardPaymentPage struct {
	// Card payment page URL to show the customer
	URL *string `json:"url,omitempty"`
}

type PaymentPageResponseCreditCardJS struct {
	SessionID        *string           `json:"session_id,omitempty"`
	PaymentPlanCodes []json.RawMessage `json:"payment_plan_codes,omitempty"`
}

// PaymentPageResponse is the response from the payment-page endpoint.
type PaymentPageResponse struct {
	Order                 *PaymentPageResponseOrder                 `json:"order,omitempty"`
	CreditCardPaymentPage *PaymentPageResponseCreditCardPaymentPage `json:"credit_card_payment_page,omitempty"`
	CreditCardJS          *PaymentPageResponseCreditCardJS          `json:"credit_card_js,omitempty"`
}

// GenerateCreditCardPaymentPage calls POST /credit-card/v1/payment-page.
func (s *Service) GenerateCreditCardPaymentPage(ctx context.Context, body PaymentPageRequest) (*PaymentPageResponse, error) {
	var out PaymentPageResponse
	if err := s.c.RequestNonSnap(ctx, "POST", "/credit-card/v1/payment-page", body, &out, ""); err != nil {
		return nil, err
	}
	return &out, nil
}

type RefundRequestOrder struct {
	// Invoice number of the transaction being refunded
	InvoiceNumber string `json:"invoice_number"`
}

type RefundRequestPayment struct {
	// Request ID from the payment initiation (or Capture) of the original transaction
	OriginalRequestID string `json:"original_request_id"`
}

type RefundRequestRefund struct {
	// Amount to refund; partial refunds allowed until the original amount is reached
	Amount float64 `json:"amount"`
}

// RefundRequest is the body for the refund endpoint.
type RefundRequest struct {
	Order   RefundRequestOrder   `json:"order"`
	Payment RefundRequestPayment `json:"payment"`
	Refund  RefundRequestRefund  `json:"refund"`
}

type RefundResponseOrder struct {
	InvoiceNumber *string `json:"invoice_number,omitempty"`
}

type RefundResponsePayment struct {
	OriginalRequestID *string `json:"original_request_id,omitempty"`
}

type RefundResponseRefund struct {
	Amount       *float64 `json:"amount,omitempty"`
	Reason       *string  `json:"reason,omitempty"`
	Type         *string  `json:"type,omitempty"`   // VOID | PARTIAL_REFUND | FULL_REFUND | MANUAL_PARTIAL_REFUND | MANUAL_FULL_REFUND
	Status       *string  `json:"status,omitempty"` // SUCCESS | FAILED
	Message      *string  `json:"message,omitempty"`
	ApprovalCode *string  `json:"approval_code,omitempty"`
}

type RefundResponseError struct {
	Code    *string `json:"code,omitempty"`
	Message *string `json:"message,omitempty"`
	Type    *string `json:"type,omitempty"`
}

// RefundResponse is the response from the refund endpoint.
type RefundResponse struct {
	Order   *RefundResponseOrder   `json:"order,omitempty"`
	Payment *RefundResponsePayment `json:"payment,omitempty"`
	Refund  *RefundResponseRefund  `json:"refund,omitempty"`
	Error   *RefundResponseError   `json:"error,omitempty"`
}

// RefundCreditCard calls POST /credit-card/v1/cancellation/credit-card/refund.
//
// NOTE: docs.doku.com currently documents `/cancellation/credit-card/refund` (no
// /credit-card/v1 prefix) while the DOKU Postman collection uses
// `/credit-card/v1/cancellation/credit-card/refund`; this SDK follows the collection.
func (s *Service) RefundCreditCard(ctx context.Context, body RefundRequest) (*RefundResponse, error) {
	var out RefundResponse
	if err := s.c.RequestNonSnap(ctx, "POST", "/credit-card/v1/cancellation/credit-card/refund", body, &out, ""); err != nil {
		return nil, err
	}
	return &out, nil
}
