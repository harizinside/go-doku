package doku

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Env selects the DOKU host. Sandbox is the default.
type Env string

const (
	Sandbox    Env = "sandbox"
	Production Env = "production"
)

// Config configures a Client. PrivateKey (PKCS#8 or PKCS#1 PEM) is required
// only for SNAP APIs. SharedKey defaults to SecretKey. HTTPClient defaults to
// http.DefaultClient.
type Config struct {
	ClientID       string
	SecretKey      string
	PrivateKey     string
	SharedKey      string
	Env            Env
	BaseURL        string
	FinanceBaseURL string
	HTTPClient     *http.Client
}

// Default base URLs.
const (
	defaultSandboxURL    = "https://api-sandbox.doku.com"
	defaultProductionURL = "https://api.doku.com"
)

func defaultBaseURL(env Env) string {
	if env == Production {
		return defaultProductionURL
	}
	return defaultSandboxURL
}

// APIError is returned for any non-2xx DOKU API response, or a failed
// transport (Status == 0 in the latter case). Body holds the parsed JSON
// body when possible (otherwise the raw bytes as a json.RawMessage so it
// round-trips through encoding/json).
type APIError struct {
	Status  int
	Body    json.RawMessage
	Header  http.Header
	Message string
	cause   error
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("doku: %s: %v", e.Message, e.cause)
	}
	return "doku: " + e.Message
}

// Unwrap allows errors.Is/As to inspect the underlying transport error.
func (e *APIError) Unwrap() error { return e.cause }

type cachedToken struct {
	token     string
	expiresAt time.Time
}

const (
	tokenRefreshMargin = 30 * time.Second
	defaultTokenTTL    = 14 * time.Minute
	maxTokenTTL        = time.Hour
)

// Client is the DOKU SDK entry point. Service fields hang off this struct
// (`client.Checkout`, `client.Settlement`, …) — Go equivalent of TS's
// `bindModule` helper.
type Client struct {
	cfg config

	// tokenMu is held across token fetches so concurrent callers share one
	// round trip instead of racing.
	tokenMu     sync.Mutex
	b2bToken    *cachedToken
	b2b2cTokens map[string]*cachedToken

	Checkout              *CheckoutService
	CreditCard            *CreditCardService
	VirtualAccountNonSnap *VirtualAccountNonSnapService
	VirtualAccountSnap    *VirtualAccountSnapService
	Emoney                *EmoneyService
	O2O                   *O2oService
	Paylater              *PaylaterService
	DirectTransfer        *DirectTransferService
	DirectDebitLegacy     *DirectDebitLegacyService
	DirectDebitSnap       *DirectDebitSnapService
	KKI                   *KkiService
	SubAccountV1          *SubAccountV1Service
	SubAccountV2          *SubAccountV2Service
	Misc                  *MiscService
	Settlement            *SettlementService
}

type config struct {
	ClientID       string
	SecretKey      string
	PrivateKey     string
	SharedKey      string
	Env            Env
	BaseURL        string
	FinanceBaseURL string
	HTTPClient     *http.Client
}

// New returns a wired-up DokuClient with every product namespace attached.
func New(cfg Config) *Client {
	env := cfg.Env
	if env == "" {
		env = Sandbox
	}
	shared := cfg.SharedKey
	if shared == "" {
		shared = cfg.SecretKey
	}
	base := cfg.BaseURL
	if base == "" {
		base = defaultBaseURL(env)
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	c := &Client{
		cfg: config{
			ClientID:       cfg.ClientID,
			SecretKey:      cfg.SecretKey,
			PrivateKey:     cfg.PrivateKey,
			SharedKey:      shared,
			Env:            env,
			BaseURL:        base,
			FinanceBaseURL: cfg.FinanceBaseURL,
			HTTPClient:     httpClient,
		},
		b2b2cTokens: map[string]*cachedToken{},
	}

	c.Checkout = &CheckoutService{c: c}
	c.CreditCard = &CreditCardService{c: c}
	c.VirtualAccountNonSnap = &VirtualAccountNonSnapService{c: c}
	c.VirtualAccountSnap = &VirtualAccountSnapService{c: c}
	c.Emoney = &EmoneyService{c: c}
	c.O2O = &O2oService{c: c}
	c.Paylater = &PaylaterService{c: c}
	c.DirectTransfer = &DirectTransferService{c: c}
	c.DirectDebitLegacy = &DirectDebitLegacyService{c: c}
	c.DirectDebitSnap = &DirectDebitSnapService{c: c}
	c.KKI = &KkiService{c: c}
	c.SubAccountV1 = &SubAccountV1Service{c: c}
	c.SubAccountV2 = &SubAccountV2Service{c: c}
	c.Misc = &MiscService{c: c}
	c.Settlement = &SettlementService{c: c}
	return c
}

// baseURL strips any trailing slash. Used for financeBaseURL fallback.
func (c *Client) baseURLTrimmed() string {
	return strings.TrimRight(c.cfg.BaseURL, "/")
}

// financeAPIBase is the base for finance settlement requests. Falls back to the
// main API host when FinanceBaseURL isn't set.
func (c *Client) financeAPIBase() string {
	if c.cfg.FinanceBaseURL != "" {
		return c.cfg.FinanceBaseURL
	}
	return c.baseURLTrimmed()
}

// encodeBody marshals body the way Node's `JSON.stringify(body, null, 2)`
// does: 2-space indent, no HTML escaping, no trailing newline. The returned
// bytes are what both the signing digest and the HTTP body use, so they
// remain stable across both.
func encodeBody(body any) ([]byte, error) {
	if body == nil {
		return nil, nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(body); err != nil {
		return nil, fmt.Errorf("doku: encode body: %w", err)
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// randomUUID returns a lowercase UUIDv4 string. Equivalent to
// `crypto.randomUUID()` in Node.
func randomUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	// Set version (4) and variant (10xx) bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	dst := make([]byte, 36)
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst), nil
}

// sendHTTP issues the request and returns the parsed body. Non-2xx responses
// raise *APIError; transport failures also raise *APIError with Status == 0.
func (c *Client) sendHTTP(ctx context.Context, method, url string, headers map[string]string, body []byte) (json.RawMessage, http.Header, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, nil, &APIError{Status: 0, Message: fmt.Sprintf("build request %s %s", method, url), cause: err}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if bodyReader != nil {
		req.ContentLength = int64(len(body))
	}
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, &APIError{Status: 0, Message: fmt.Sprintf("transport %s %s: %v", method, url, err), cause: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.Header, &APIError{Status: resp.StatusCode, Message: fmt.Sprintf("read body %s %s: %v", method, url, err), Header: resp.Header, cause: err}
	}
	respBody := jsonOrText(raw)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header, &APIError{
			Status:  resp.StatusCode,
			Body:    respBody,
			Header:  resp.Header,
			Message: fmt.Sprintf("DOKU API error %d for %s %s", resp.StatusCode, method, url),
		}
	}
	return respBody, resp.Header, nil
}

// jsonOrText returns a json.RawMessage if raw parses as JSON, otherwise wraps
// it as a JSON string so it round-trips through encoding/json unchanged.
func jsonOrText(raw []byte) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if json.Valid(raw) {
		// Copy: callers may later mutate the returned slice.
		out := make([]byte, len(raw))
		copy(out, raw)
		return out
	}
	encoded, err := json.Marshal(string(raw))
	if err != nil {
		return nil
	}
	return encoded
}

// RequestNonSnap makes a Non-SNAP signed request. The body bytes that are
// signed are the same bytes that are sent over the wire.
func (c *Client) RequestNonSnap(ctx context.Context, method, targetPath string, body, out any, baseOverride string) error {
	if !strings.HasPrefix(targetPath, "/") {
		return fmt.Errorf("doku: targetPath must start with /, got %q", targetPath)
	}
	rawBody, err := encodeBody(body)
	if err != nil {
		return err
	}
	requestID, err := randomUUID()
	if err != nil {
		return &APIError{Status: 0, Message: "generate request id", cause: err}
	}
	requestTimestamp := NonSnapTimestamp()
	signature := SignNonSnap(NonSnapSignatureInput{
		ClientID:         c.cfg.ClientID,
		RequestID:        requestID,
		RequestTimestamp: requestTimestamp,
		RequestTarget:    targetPath,
		Body:             string(rawBody),
		SecretKey:        c.cfg.SecretKey,
	})
	headers := map[string]string{
		"Content-Type":      "application/json",
		"Client-Id":         c.cfg.ClientID,
		"Request-Id":        requestID,
		"Request-Timestamp": requestTimestamp,
		"Signature":         signature,
	}
	base := baseOverride
	if base == "" {
		base = c.cfg.BaseURL
	}
	respBody, _, err := c.sendHTTP(ctx, method, base+targetPath, headers, rawBody)
	if err != nil {
		return err
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return &APIError{Status: 200, Message: "decode response body", cause: err}
	}
	return nil
}

// SnapOptions configures a SNAP request. UseSharedKey swaps the signing key
// for sharedKey (Direct Debit / KKI).
type SnapOptions struct {
	Body          any
	ChannelID     string
	CustomerToken string
	UseSharedKey  bool
}

// SnapB2BToken returns the cached B2B access token, fetching a new one when
// it is missing, expired, or force is true.
func (c *Client) SnapB2BToken(ctx context.Context, force bool) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if !force && c.b2bToken != nil && c.b2bToken.expiresAt.After(time.Now()) {
		return c.b2bToken.token, nil
	}
	token, expiresAt, err := c.fetchSnapToken(ctx, "b2b", map[string]string{"grantType": "client_credentials"})
	if err != nil {
		return "", err
	}
	c.b2bToken = &cachedToken{token: token, expiresAt: expiresAt}
	return token, nil
}

// SnapB2B2CToken returns the B2B2C customer token for authCode, cached per
// authCode.
func (c *Client) SnapB2B2CToken(ctx context.Context, authCode string) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if t := c.b2b2cTokens[authCode]; t != nil && t.expiresAt.After(time.Now()) {
		return t.token, nil
	}
	token, expiresAt, err := c.fetchSnapToken(ctx, "b2b2c", map[string]string{
		"grantType": "authorization_code",
		"authCode":  authCode,
	})
	if err != nil {
		return "", err
	}
	c.b2b2cTokens[authCode] = &cachedToken{token: token, expiresAt: expiresAt}
	return token, nil
}

func (c *Client) fetchSnapToken(ctx context.Context, kind string, body map[string]string) (string, time.Time, error) {
	if c.cfg.PrivateKey == "" {
		return "", time.Time{}, &APIError{
			Status:  0,
			Message: fmt.Sprintf("Config.PrivateKey is required for SNAP %s token requests", kind),
		}
	}
	targetPath := "/authorization/v1/access-token/" + kind
	timestamp := SnapTimestamp()
	xSig, err := SignSnapToken(SnapTokenSignatureInput{
		ClientID:   c.cfg.ClientID,
		Timestamp:  timestamp,
		PrivateKey: c.cfg.PrivateKey,
	})
	if err != nil {
		return "", time.Time{}, &APIError{Status: 0, Message: "sign snap token", cause: err}
	}
	rawBody, err := encodeBody(body)
	if err != nil {
		return "", time.Time{}, err
	}
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-CLIENT-KEY": c.cfg.ClientID,
		"X-TIMESTAMP":  timestamp,
		"X-SIGNATURE":  xSig,
	}
	respBody, _, err := c.sendHTTP(ctx, "POST", c.cfg.BaseURL+targetPath, headers, rawBody)
	if err != nil {
		return "", time.Time{}, err
	}
	var parsed struct {
		AccessToken string      `json:"accessToken"`
		ExpiresIn   json.Number `json:"expiresIn"` // DOKU may send "900" or 900
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", time.Time{}, &APIError{Status: 200, Message: "decode token body", cause: err}
	}
	if parsed.AccessToken == "" {
		return "", time.Time{}, &APIError{
			Status:  200,
			Message: fmt.Sprintf("DOKU SNAP %s token response did not contain accessToken", kind),
		}
	}
	ttl := defaultTokenTTL
	if v, err := parsed.ExpiresIn.Float64(); err == nil && v > 0 {
		ttl = min(time.Duration(v*float64(time.Second))-tokenRefreshMargin, maxTokenTTL)
	}
	return parsed.AccessToken, time.Now().Add(ttl), nil
}

// RequestSnap makes a SNAP resource request, fetching + reusing a cached B2B
// token. body bytes that are signed are the same bytes sent over the wire.
func (c *Client) RequestSnap(ctx context.Context, method, targetPath string, opt SnapOptions, out any) error {
	if !strings.HasPrefix(targetPath, "/") {
		return fmt.Errorf("doku: targetPath must start with /, got %q", targetPath)
	}
	accessToken, err := c.SnapB2BToken(ctx, false)
	if err != nil {
		return err
	}
	timestamp := SnapTimestamp()
	rawBody, err := encodeBody(opt.Body)
	if err != nil {
		return err
	}
	secret := c.cfg.SecretKey
	if opt.UseSharedKey {
		secret = c.cfg.SharedKey
	}
	xSig := SignSnapRequest(SnapRequestSignatureInput{
		HTTPMethod:   strings.ToUpper(method),
		EndpointURL:  targetPath,
		AccessToken:  accessToken,
		Body:         string(rawBody),
		Timestamp:    timestamp,
		ClientSecret: secret,
	})
	xExternal, err := randomUUID()
	if err != nil {
		return &APIError{Status: 0, Message: "generate external id", cause: err}
	}
	headers := map[string]string{
		"Content-Type":  "application/json",
		"X-PARTNER-ID":  c.cfg.ClientID,
		"X-EXTERNAL-ID": xExternal,
		"X-TIMESTAMP":   timestamp,
		"X-SIGNATURE":   xSig,
		"Authorization": "Bearer " + accessToken,
	}
	if opt.ChannelID != "" {
		headers["CHANNEL-ID"] = opt.ChannelID
	}
	if opt.CustomerToken != "" {
		headers["Authorization-Customer"] = "Bearer " + opt.CustomerToken
	}
	respBody, _, err := c.sendHTTP(ctx, method, c.cfg.BaseURL+targetPath, headers, rawBody)
	if err != nil {
		return err
	}
	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return &APIError{Status: 200, Message: "decode response body", cause: err}
	}
	return nil
}

// VerifyNotification wraps verifyNotificationSignature using this client's
// SecretKey. See notification.go for details.
func (c *Client) VerifyNotification(h http.Header, rawBody []byte, notificationPath string) bool {
	return VerifyNotificationSignature(h, rawBody, notificationPath, c.cfg.SecretKey)
}
