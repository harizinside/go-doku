package doku

import (
	"context"
	"encoding/json"
)

// CreditCardService exposes the Non-SNAP Credit Card endpoints (Payment
// Page + Refund). Hand-typed from developers.doku.com.
type CreditCardService struct{ c *Client }

// CreditCardPaymentPageRequest is the body for POST /credit-card/v1/payment-page.
type CreditCardPaymentPageRequest struct {
	Order    CreditCardOrder     `json:"order"`
	Customer *CreditCardCustomer `json:"customer,omitempty"`
	// Conditional: for AUTHORIZE / INSTALLMENT flows
	Payment *CreditCardPaymentDetails `json:"payment,omitempty"`
	// Card tokenization (pre-fill / force save)
	Card                  *CreditCardTokenization   `json:"card,omitempty"`
	OverrideConfiguration *CreditCardOverrideConfig `json:"override_configuration,omitempty"`
	AdditionalInfo        *CreditCardAdditionalInfo `json:"additional_info,omitempty"`
}

// CreditCardOrder is the order block.
type CreditCardOrder struct {
	// In IDR and without decimal
	Amount float64 `json:"amount"`
	// Max length: 64 (30 for Mandiri acquirer)
	InvoiceNumber string               `json:"invoice_number"`
	LineItems     []CreditCardLineItem `json:"line_items,omitempty"`
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

// CreditCardLineItem is one line item.
type CreditCardLineItem struct {
	Name     *string  `json:"name,omitempty"`
	Price    *float64 `json:"price,omitempty"`
	Quantity *float64 `json:"quantity,omitempty"`
}

// CreditCardCustomer is the customer block.
type CreditCardCustomer struct {
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

// CreditCardPaymentDetails is the payment-type block.
type CreditCardPaymentDetails struct {
	// Conditional: mandatory if you support multiple card transaction types
	Type *string `json:"type,omitempty"` // SALE | AUTHORIZE | INSTALLMENT
	// true: capture immediately after AUTHORIZE (behaves like SALE)
	AutoCapture *bool `json:"auto_capture,omitempty"`
	// Conditional: mandatory for INSTALLMENT (BNI, BRI, BANK_CIMB, BANK_MANDIRI, BCA, ...)
	Acquirer *string `json:"acquirer,omitempty"`
	// Conditional: mandatory for INSTALLMENT
	Tenor *float64 `json:"tenor,omitempty"`
}

// CreditCardTokenization is the card block.
type CreditCardTokenization struct {
	// Pre-fills the card number field
	Token *string `json:"token,omitempty"`
	// Force the customer to save the card token for the next payment
	Save *bool `json:"save,omitempty"`
}

// CreditCardOverrideConfig is the override_configuration block.
type CreditCardOverrideConfig struct {
	Themes *CreditCardThemes `json:"themes,omitempty"`
	Promo  []CreditCardPromo `json:"promo,omitempty"`
	// Only accept these BINs (6 or 8 digits)
	AllowBin []float64 `json:"allow_bin,omitempty"`
	// Only accept these installment tenors
	AllowTenor []float64 `json:"allow_tenor,omitempty"`
}

// CreditCardThemes is the theme override block.
type CreditCardThemes struct {
	// EN (default) | ID
	Language              *string `json:"language,omitempty"`
	BackgroundColor       *string `json:"background_color,omitempty"`
	FontColor             *string `json:"font_color,omitempty"`
	ButtonBackgroundColor *string `json:"button_background_color,omitempty"`
	ButtonFontColor       *string `json:"button_font_color,omitempty"`
}

// CreditCardPromo is one promo entry.
type CreditCardPromo struct {
	// BIN that gets the promo (6 or 8 digits)
	Bin *string `json:"bin,omitempty"`
	// final amount = order.amount - discount_amount
	DiscountAmount *float64 `json:"discount_amount,omitempty"`
}

// CreditCardAdditionalInfo is the additional_info block.
type CreditCardAdditionalInfoDisclaimer struct {
	ID json.RawMessage `json:"id,omitempty"`
	EN json.RawMessage `json:"en,omitempty"`
}

type CreditCardAdditionalInfo struct {
	OverrideNotificationURL *string                             `json:"override_notification_url,omitempty"`
	Disclaimer              *CreditCardAdditionalInfoDisclaimer `json:"disclaimer,omitempty"`
}

// CreditCardPaymentPageResponse is the response from the payment-page endpoint.
type CreditCardPaymentPageResponseOrder struct {
	InvoiceNumber *string           `json:"invoice_number,omitempty"`
	LineItems     []json.RawMessage `json:"line_items,omitempty"`
}

type CreditCardPaymentPageResponseCreditCardPaymentPage struct {
	// Card payment page URL to show the customer
	URL *string `json:"url,omitempty"`
}

type CreditCardPaymentPageResponseCreditCardJS struct {
	SessionID        *string           `json:"session_id,omitempty"`
	PaymentPlanCodes []json.RawMessage `json:"payment_plan_codes,omitempty"`
}

type CreditCardPaymentPageResponse struct {
	Order                 *CreditCardPaymentPageResponseOrder                 `json:"order,omitempty"`
	CreditCardPaymentPage *CreditCardPaymentPageResponseCreditCardPaymentPage `json:"credit_card_payment_page,omitempty"`
	CreditCardJS          *CreditCardPaymentPageResponseCreditCardJS          `json:"credit_card_js,omitempty"`
}

// GenerateCreditCardPaymentPage calls POST /credit-card/v1/payment-page.
func (s *CreditCardService) GenerateCreditCardPaymentPage(ctx context.Context, body CreditCardPaymentPageRequest) (*CreditCardPaymentPageResponse, error) {
	var out CreditCardPaymentPageResponse
	if err := s.c.RequestNonSnap(ctx, "POST", "/credit-card/v1/payment-page", body, &out, ""); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreditCardRefundRequest is the body for the refund endpoint.
type CreditCardRefundRequestOrder struct {
	// Invoice number of the transaction being refunded
	InvoiceNumber string `json:"invoice_number"`
}

type CreditCardRefundRequestPayment struct {
	// Request ID from the payment initiation (or Capture) of the original transaction
	OriginalRequestID string `json:"original_request_id"`
}

type CreditCardRefundRequestRefund struct {
	// Amount to refund; partial refunds allowed until the original amount is reached
	Amount float64 `json:"amount"`
}

type CreditCardRefundRequest struct {
	Order   CreditCardRefundRequestOrder   `json:"order"`
	Payment CreditCardRefundRequestPayment `json:"payment"`
	Refund  CreditCardRefundRequestRefund  `json:"refund"`
}

// CreditCardRefundResponse is the response from the refund endpoint.
type CreditCardRefundResponseOrder struct {
	InvoiceNumber *string `json:"invoice_number,omitempty"`
}

type CreditCardRefundResponsePayment struct {
	OriginalRequestID *string `json:"original_request_id,omitempty"`
}

type CreditCardRefundResponseRefund struct {
	Amount       *float64 `json:"amount,omitempty"`
	Reason       *string  `json:"reason,omitempty"`
	Type         *string  `json:"type,omitempty"`   // VOID | PARTIAL_REFUND | FULL_REFUND | MANUAL_PARTIAL_REFUND | MANUAL_FULL_REFUND
	Status       *string  `json:"status,omitempty"` // SUCCESS | FAILED
	Message      *string  `json:"message,omitempty"`
	ApprovalCode *string  `json:"approval_code,omitempty"`
}

type CreditCardRefundResponseError struct {
	Code    *string `json:"code,omitempty"`
	Message *string `json:"message,omitempty"`
	Type    *string `json:"type,omitempty"`
}

type CreditCardRefundResponse struct {
	Order   *CreditCardRefundResponseOrder   `json:"order,omitempty"`
	Payment *CreditCardRefundResponsePayment `json:"payment,omitempty"`
	Refund  *CreditCardRefundResponseRefund  `json:"refund,omitempty"`
	Error   *CreditCardRefundResponseError   `json:"error,omitempty"`
}

// RefundCreditCard calls POST /credit-card/v1/cancellation/credit-card/refund.
//
// NOTE: docs.doku.com currently documents `/cancellation/credit-card/refund` (no
// /credit-card/v1 prefix) while the DOKU Postman collection uses
// `/credit-card/v1/cancellation/credit-card/refund`; this SDK follows the collection.
func (s *CreditCardService) RefundCreditCard(ctx context.Context, body CreditCardRefundRequest) (*CreditCardRefundResponse, error) {
	var out CreditCardRefundResponse
	if err := s.c.RequestNonSnap(ctx, "POST", "/credit-card/v1/cancellation/credit-card/refund", body, &out, ""); err != nil {
		return nil, err
	}
	return &out, nil
}
