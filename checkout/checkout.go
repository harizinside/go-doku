// Package checkout is the DOKU Checkout API (hosted payment page).
package checkout

import (
	"context"
	"encoding/json"
	doku "github.com/harizinside/go-doku"
	"github.com/harizinside/go-doku/settlement"
)

// Service exposes the DOKU Checkout API. Deep-typed from
// developers.doku.com.
type Service struct{ c *doku.Client }

// New returns the service bound to c.
func New(c *doku.Client) *Service { return &Service{c: c} }

// PaymentMethodType is a Checkout page payment method filter.
type PaymentMethodType string

const (
	CheckoutPaymentMethodVirtualAccountBCA                PaymentMethodType = "VIRTUAL_ACCOUNT_BCA"
	CheckoutPaymentMethodVirtualAccountBankMandiri        PaymentMethodType = "VIRTUAL_ACCOUNT_BANK_MANDIRI"
	CheckoutPaymentMethodVirtualAccountBankSyariahMandiri PaymentMethodType = "VIRTUAL_ACCOUNT_BANK_SYARIAH_MANDIRI"
	CheckoutPaymentMethodVirtualAccountDoku               PaymentMethodType = "VIRTUAL_ACCOUNT_DOKU"
	CheckoutPaymentMethodVirtualAccountBRI                PaymentMethodType = "VIRTUAL_ACCOUNT_BRI"
	CheckoutPaymentMethodVirtualAccountBNI                PaymentMethodType = "VIRTUAL_ACCOUNT_BNI"
	CheckoutPaymentMethodVirtualAccountBankPermata        PaymentMethodType = "VIRTUAL_ACCOUNT_BANK_PERMATA"
	CheckoutPaymentMethodVirtualAccountBankCIMB           PaymentMethodType = "VIRTUAL_ACCOUNT_BANK_CIMB"
	CheckoutPaymentMethodVirtualAccountBankDanamon        PaymentMethodType = "VIRTUAL_ACCOUNT_BANK_DANAMON"
	CheckoutPaymentMethodVirtualAccountBNC                PaymentMethodType = "VIRTUAL_ACCOUNT_BNC"
	CheckoutPaymentMethodVirtualAccountBTN                PaymentMethodType = "VIRTUAL_ACCOUNT_BTN"
	CheckoutPaymentMethodVirtualAccountSinarmas           PaymentMethodType = "VIRTUAL_ACCOUNT_SINARMAS"
	CheckoutPaymentMethodOnlineToOfflineAlfa              PaymentMethodType = "ONLINE_TO_OFFLINE_ALFA"
	CheckoutPaymentMethodOnlineToOfflineIndomaret         PaymentMethodType = "ONLINE_TO_OFFLINE_INDOMARET"
	CheckoutPaymentMethodCreditCard                       PaymentMethodType = "CREDIT_CARD"
	CheckoutPaymentMethodDirectDebitBRI                   PaymentMethodType = "DIRECT_DEBIT_BRI"
	CheckoutPaymentMethodDirectDebitCIMB                  PaymentMethodType = "DIRECT_DEBIT_CIMB"
	CheckoutPaymentMethodDirectDebitAllo                  PaymentMethodType = "DIRECT_DEBIT_ALLO"
	CheckoutPaymentMethodEmoneyShopeePay                  PaymentMethodType = "EMONEY_SHOPEEPAY"
	CheckoutPaymentMethodEmoneyOVO                        PaymentMethodType = "EMONEY_OVO"
	CheckoutPaymentMethodEmoneyDANA                       PaymentMethodType = "EMONEY_DANA"
	CheckoutPaymentMethodQRIS                             PaymentMethodType = "QRIS"
	CheckoutPaymentMethodPeerToPeerAkulaku                PaymentMethodType = "PEER_TO_PEER_AKULAKU"
	CheckoutPaymentMethodPeerToPeerKredivo                PaymentMethodType = "PEER_TO_PEER_KREDIVO"
	CheckoutPaymentMethodPeerToPeerIndodana               PaymentMethodType = "PEER_TO_PEER_INDODANA"
)

// LineItem is one line item in the Checkout order.
type LineItem struct {
	// Conditional: mandatory for Akulaku, Kredivo, Indodana, Allobank. Max length: 64
	ID *string `json:"id,omitempty"`
	// Conditional: mandatory for Jenius, Kredivo, Indodana, KKI, Akulaku, Allobank
	Name *string `json:"name,omitempty"`
	// Conditional: must total `order.amount` together with quantity
	Price *float64 `json:"price,omitempty"`
	// Conditional
	Quantity *float64 `json:"quantity,omitempty"`
	// Conditional: mandatory for Akulaku, Kredivo, Indodana
	SKU *string `json:"sku,omitempty"`
	// Conditional: mandatory for Akulaku, Kredivo, Indodana
	Category *string `json:"category,omitempty"`
	// Conditional: mandatory for Kredivo
	URL *string `json:"url,omitempty"`
	// Conditional: mandatory for Indodana
	ImageURL *string `json:"image_url,omitempty"`
	// Conditional: mandatory for Indodana, Kredivo
	Type *string `json:"type,omitempty"`
}

// Request is the body for POST /checkout/v1/payment.
type Request struct {
	Order           RequestOrder           `json:"order"`
	Payment         RequestPayment         `json:"payment"`
	Customer        *RequestCustomer       `json:"customer,omitempty"`
	ShippingAddress *RequestAddress        `json:"shipping_address,omitempty"`
	BillingAddress  *RequestAddress        `json:"billing_address,omitempty"`
	AdditionalInfo  *RequestAdditionalInfo `json:"additional_info,omitempty"`
}

// RequestOrder is the order block.
type RequestOrder struct {
	// In IDR and without decimal. Max length: 12
	Amount float64 `json:"amount"`
	// Max length: 64 (30 with Credit Card; no symbols for KKI)
	InvoiceNumber string `json:"invoice_number"`
	// 3-letter ISO 4217. Default: IDR
	Currency *string `json:"currency,omitempty"`
	// Conditional: mandatory for Jenius ("Back to Merchant" button)
	CallbackURL *string `json:"callback_url,omitempty"`
	// Conditional: only for Indodana
	CallbackURLCancel *string `json:"callback_url_cancel,omitempty"`
	// "Back to merchant" button on the result page
	CallbackURLResult *string `json:"callback_url_result,omitempty"`
	// Checkout page default language. Max length: 2
	Language *string `json:"language,omitempty"`
	// true: result page redirects to callback URL
	AutoRedirect *bool `json:"auto_redirect,omitempty"`
	// Only applied for Credit Card, DOKU Wallet, Akulaku, OVO, ShopeePay when true
	DisableRetryPayment *bool `json:"disable_retry_payment,omitempty"`
	// Only for VA, O2O, Credit Card
	RecoverAbandonedCart *bool `json:"recover_abandoned_cart,omitempty"`
	// Only for VA, O2O, Credit Card. Max: 44640 minutes
	ExpiredRecoveredCart *float64   `json:"expired_recovered_cart,omitempty"`
	LineItems            []LineItem `json:"line_items,omitempty"`
}

// RequestPayment is the payment block.
type RequestPayment struct {
	// Checkout page due date in minutes. Default: 60. Max length: 6
	PaymentDueDate *float64 `json:"payment_due_date,omitempty"`
	// Only for Credit Card: SALE | INSTALLMENT | AUTHORIZE
	Type *string `json:"type,omitempty"`
	// Omit to show all activated payment methods on the Checkout page
	PaymentMethodTypes []PaymentMethodType `json:"payment_method_types,omitempty"`
}

// RequestCustomer is the customer block.
type RequestCustomer struct {
	// Conditional: mandatory for tokenization (BRI DD, Allobank, CC) and Akulaku. Max length: 50
	ID *string `json:"id,omitempty"`
	// Conditional: mandatory for Jenius, Akulaku, Indodana, Kredivo. Max length: 255
	Name     *string `json:"name,omitempty"`
	LastName *string `json:"last_name,omitempty"`
	// Conditional: mandatory for Indodana, Kredivo, Allobank. Max length: 128
	Email *string `json:"email,omitempty"`
	// Conditional: mandatory for Indodana, Akulaku, Kredivo. Format: {calling_code}{number}. Max length: 16
	Phone *string `json:"phone,omitempty"`
	// Conditional: mandatory for Akulaku. Max length: 400
	Address *string `json:"address,omitempty"`
	// Conditional: mandatory for Akulaku
	Postcode *string `json:"postcode,omitempty"`
	// Conditional: mandatory for Akulaku
	State *string `json:"state,omitempty"`
	// Conditional: mandatory for Akulaku
	City *string `json:"city,omitempty"`
	// 2-letter ISO 3166-1
	Country *string `json:"country,omitempty"`
}

// RequestAddress is the shared address shape (shipping/billing).
type RequestAddress struct {
	FirstName   *string `json:"first_name,omitempty"`
	LastName    *string `json:"last_name,omitempty"`
	Address     *string `json:"address,omitempty"`
	City        *string `json:"city,omitempty"`
	PostalCode  *string `json:"postal_code,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	CountryCode *string `json:"country_code,omitempty"`
}

// RequestAdditionalInfo is the Checkout additional_info block.
// Combines the generic settlement extensions (hold/split/report — defined in
// settlement.go) with Checkout-specific knobs.
type RequestAdditionalInfo struct {
	// Hold & Release: defer the payout for this payment until released
	HoldSettlement *bool `json:"hold_settlement,omitempty"`
	// Split Settlement: route the net settlement amount to one or more sub bank accounts
	Settlement []settlement.Split `json:"settlement,omitempty"`
	// Custom Settlement Report: extra columns in the settlement report export
	Report []settlement.ReportEntry `json:"report,omitempty"`
	// Installment tenors allowed on the page (0, 3, 6, 12)
	AllowTenor []float64 `json:"allow_tenor,omitempty"`
	// Only for DOKU Wallet
	DokuWalletNotifyURL *string `json:"doku_wallet_notify_url,omitempty"`
	// Overrides the notification URL configured in the Back Office
	OverrideNotificationURL *string `json:"override_notification_url,omitempty"`
}

// Response is the response of POST /checkout/v1/payment.
type Response struct {
	Message  []string          `json:"message,omitempty"`
	Response *ResponseResponse `json:"response,omitempty"`
}

// ResponseResponse is the inner response block.
type ResponseResponse struct {
	Order           *ResponseOrder   `json:"order,omitempty"`
	Payment         *ResponsePayment `json:"payment,omitempty"`
	Customer        json.RawMessage  `json:"customer,omitempty"`
	ShippingAddress json.RawMessage  `json:"shipping_address,omitempty"`
	BillingAddress  json.RawMessage  `json:"billing_address,omitempty"`
	AdditionalInfo  json.RawMessage  `json:"additional_info,omitempty"`
	UUID            json.RawMessage  `json:"uuid,omitempty"`
	Headers         json.RawMessage  `json:"headers,omitempty"`
}

// ResponseOrder echoes the order block.
type ResponseOrder struct {
	Amount               json.RawMessage `json:"amount,omitempty"`
	InvoiceNumber        *string         `json:"invoice_number,omitempty"`
	Currency             *string         `json:"currency,omitempty"`
	SessionID            *string         `json:"session_id,omitempty"`
	CallbackURL          *string         `json:"callback_url,omitempty"`
	CallbackURLCancel    *string         `json:"callback_url_cancel,omitempty"`
	CallbackURLResult    *string         `json:"callback_url_result,omitempty"`
	Language             *string         `json:"language,omitempty"`
	AutoRedirect         *bool           `json:"auto_redirect,omitempty"`
	DisableRetryPayment  *bool           `json:"disable_retry_payment,omitempty"`
	RecoverAbandonedCart *bool           `json:"recover_abandoned_cart,omitempty"`
	ExpiredRecoveredCart *float64        `json:"expired_recovered_cart,omitempty"`
	LineItems            []LineItem      `json:"line_items,omitempty"`
}

// ResponsePayment echoes the payment block.
type ResponsePayment struct {
	PaymentMethodTypes []string `json:"payment_method_types,omitempty"`
	PaymentDueDate     *float64 `json:"payment_due_date,omitempty"`
	TokenID            *string  `json:"token_id,omitempty"`
	// Checkout page URL to show the customer
	URL *string `json:"url,omitempty"`
	// yyyyMMddHHmmss, UTC+7
	ExpiredDate *string `json:"expired_date,omitempty"`
}

// InitiateCheckoutPayment initiates a DOKU Checkout payment and returns the
// Checkout page URL.
func (s *Service) InitiateCheckoutPayment(ctx context.Context, body Request) (*Response, error) {
	var out Response
	if err := s.c.RequestNonSnap(ctx, "POST", "/checkout/v1/payment", body, &out, ""); err != nil {
		return nil, err
	}
	return &out, nil
}
