// Package misc holds cross-product DOKU endpoints (Check Status API).
package misc

import (
	"context"
	"encoding/json"
	"errors"
	doku "github.com/harizinside/go-doku"
	"net/url"
)

// Service exposes cross-product standalone endpoints. Currently just the
// Non-SNAP Check Status API.
type Service struct{ c *doku.Client }

// New returns the service bound to c.
func New(c *doku.Client) *Service { return &Service{c: c} }

// CheckOrderStatus calls GET /orders/v1/status/{invoiceNumber}.
// DOKU's docs recommend hitting it at least 60 seconds after payment
// completion. GET requests are signed without a Digest line.
func (s *Service) CheckOrderStatus(ctx context.Context, invoiceNumber string) (json.RawMessage, error) {
	if invoiceNumber == "" {
		return nil, errEmptyInvoiceNumber
	}
	path := "/orders/v1/status/" + url.PathEscape(invoiceNumber)
	var out json.RawMessage
	if err := s.c.RequestNonSnap(ctx, "GET", path, nil, &out, ""); err != nil {
		return nil, err
	}
	return out, nil
}

var errEmptyInvoiceNumber = errors.New("doku: invoiceNumber is required")
