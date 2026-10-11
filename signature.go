// Non-SNAP and SNAP signing primitives. Every function mirrors its
// counterpart in @harizinside/doku (src/core/signature.ts) byte-for-byte.

package doku

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// NonSnapTimestamp returns the Non-SNAP Request-Timestamp header value:
// "YYYY-MM-DDTHH:MM:SSZ" (UTC, with the trailing "Z"). Matches
// `date.toISOString().slice(0, 19) + "Z"` in Node.
func NonSnapTimestamp(t ...time.Time) string {
	d := time.Now().UTC()
	if len(t) > 0 {
		d = t[0].UTC()
	}
	return d.Format("2006-01-02T15:04:05Z")
}

// SnapTimestamp returns the SNAP X-TIMESTAMP header value:
// "YYYY-MM-DDTHH:MM:SS+HH:MM" — the date shifted to local time with the
// numeric offset of the host. Matches DOKU's Postman
// `moment().format("YYYY-MM-DDTHH:mm:ssZ")`.
func SnapTimestamp(t ...time.Time) string {
	d := time.Now()
	if len(t) > 0 {
		d = t[0]
	}
	return d.Format("2006-01-02T15:04:05-07:00")
}

// NonSnapDigest returns the Digest component for the Non-SNAP signature:
// Base64(SHA256(rawJsonBody)). Only used for requests with a body; GET
// requests omit the Digest line.
func NonSnapDigest(rawBody string) string {
	sum := sha256.Sum256([]byte(rawBody))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// NonSnapSignatureInput is the argument shape for SignNonSnap. Body is the
// already-serialized (raw) JSON string; pass an empty string for GET.
type NonSnapSignatureInput struct {
	ClientID         string
	RequestID        string
	RequestTimestamp string
	RequestTarget    string
	Body             string
	SecretKey        string
}

// SignNonSnap builds the Non-SNAP Signature header value:
// "HMACSHA256=" + Base64(HMAC-SHA256(stringToSign, secretKey)).
//
// stringToSign is the lines joined with "\n" (no trailing newline):
//
//	Client-Id:{id}
//	Request-Id:{rid}
//	Request-Timestamp:{ts}
//	Request-Target:{path}
//	Digest:{base64 sha256 of body}   (omitted when Body == "")
func SignNonSnap(in NonSnapSignatureInput) string {
	lines := []string{
		"Client-Id:" + in.ClientID,
		"Request-Id:" + in.RequestID,
		"Request-Timestamp:" + in.RequestTimestamp,
		"Request-Target:" + in.RequestTarget,
	}
	if in.Body != "" {
		lines = append(lines, "Digest:"+NonSnapDigest(in.Body))
	}
	stringToSign := strings.Join(lines, "\n")
	mac := hmac.New(sha256.New, []byte(in.SecretKey))
	mac.Write([]byte(stringToSign))
	return "HMACSHA256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// normalizePEM converts literal "\n" escape sequences (from .env files) into
// real newlines so the PEM decoder can read them.
func normalizePEM(privateKey string) string {
	if strings.Contains(privateKey, `\n`) {
		return strings.ReplaceAll(privateKey, `\n`, "\n")
	}
	return privateKey
}

// parseRSAPrivateKey decodes a PEM RSA private key (PKCS#8 or PKCS#1),
// matching Node's `createSign("RSA-SHA256")` which auto-detects the format.
func parseRSAPrivateKey(pemData string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(normalizePEM(pemData)))
	if block == nil {
		return nil, errors.New("doku: failed to decode PEM private key")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsaKey, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("doku: PKCS#8 key is not RSA")
		}
		return rsaKey, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// SnapTokenSignatureInput is the argument shape for SignSnapToken.
type SnapTokenSignatureInput struct {
	ClientID   string
	Timestamp  string
	PrivateKey string
}

// SignSnapToken builds the SNAP asymmetric signature for the access-token
// request: Base64(RSA-SHA256(`${clientId}|${timestamp}`, privateKey)). The
// PEM may use either PKCS#8 or PKCS#1, and literal "\n" sequences are
// normalized.
func SignSnapToken(in SnapTokenSignatureInput) (string, error) {
	stringToSign := in.ClientID + "|" + in.Timestamp
	key, err := parseRSAPrivateKey(normalizePEM(in.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("doku: parse private key: %w", err)
	}
	sum := sha256.Sum256([]byte(stringToSign))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("doku: rsa sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// SnapRequestSignatureInput is the argument shape for SignSnapRequest.
// Body is the already-serialized (raw) JSON string; pass an empty string
// for requests with no body.
type SnapRequestSignatureInput struct {
	HTTPMethod   string
	EndpointURL  string
	AccessToken  string
	Body         string
	Timestamp    string
	ClientSecret string
}

// SignSnapRequest builds the SNAP symmetric signature for resource requests:
// Base64(HMAC-SHA512("METHOD:path:accessToken:lowerHex(SHA256(body)):timestamp", clientSecret)).
func SignSnapRequest(in SnapRequestSignatureInput) string {
	bodySHA := sha256.Sum256([]byte(in.Body))
	bodyHex := strings.ToLower(hex.EncodeToString(bodySHA[:]))
	stringToSign := strings.Join([]string{
		in.HTTPMethod,
		in.EndpointURL,
		in.AccessToken,
		bodyHex,
		in.Timestamp,
	}, ":")
	mac := hmac.New(sha512.New, []byte(in.ClientSecret))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// OvoCheckSumInput is the argument shape for OVOCheckSum. Approval/Batch/
// Reference/Trace fields are optional (empty when not applicable).
type OvoCheckSumInput struct {
	Amount          string // stringified as-is (number or string)
	ClientID        string
	InvoiceNumber   string
	OvoID           string
	SecretKey       string
	ApprovalCode    string
	BatchNumber     string
	ReferenceNumber string
	TraceNumber     string
}

// OVOCheckSum returns the legacy OVO Push Payment `security.check_sum`:
// hex(SHA256(amount + approvalCode + batchNumber + clientId + invoiceNumber
// + ovoId + referenceNumber + traceNumber + secretKey)). Field order and
// plain concatenation match DOKU's reference implementation.
func OVOCheckSum(in OvoCheckSumInput) string {
	concat := in.Amount + in.ApprovalCode + in.BatchNumber +
		in.ClientID + in.InvoiceNumber + in.OvoID +
		in.ReferenceNumber + in.TraceNumber + in.SecretKey
	sum := sha256.Sum256([]byte(concat))
	return hex.EncodeToString(sum[:])
}

// PartnerServiceID derives the 8-char SNAP VA partner service id from the
// acquirer BIN, replicating DOKU's Postman helper. If BIN is shorter than 8
// chars, takes the first `binLength` chars and left-pads with spaces to 8.
// If BIN is 8+ chars, takes the first 8.
func PartnerServiceID(bin string, binLength ...int) string {
	bl := 6
	if len(binLength) > 0 {
		bl = binLength[0]
	}
	trimmed := strings.TrimSpace(bin)
	if len(trimmed) < 8 {
		sub := trimmed
		if len(sub) > bl {
			sub = sub[:bl]
		}
		return strings.Repeat(" ", 8-len(sub)) + sub
	}
	return trimmed[:8]
}
