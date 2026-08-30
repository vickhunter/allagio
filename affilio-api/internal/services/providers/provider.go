// Package providers turns payment-provider webhooks into a single normalized
// conversion signal. None of the supported providers offers affiliate rails of
// its own, so Distributoor owns the attribution ledger and each provider is
// only a signal source: adding one means implementing Adapter in one file.
package providers

import (
	"net/http"
	"time"
)

// ConversionEvent is the normalized signal an adapter extracts from a webhook.
type ConversionEvent struct {
	EventID  string  // provider-unique event id (idempotency key)
	ObjectID string  // order / checkout-session id, joins refunds to sales
	Kind     string  // sale | renewal | refund | ignore
	Amount   float64 // gross amount in currency units (not cents)
	Currency string
	Ref      string // the dtr click id carried through checkout metadata
	OccurredAt time.Time
}

type Adapter interface {
	Name() string
	// VerifySignature authenticates the raw webhook body against the shared secret.
	VerifySignature(headers http.Header, body []byte, secret string) bool
	// ParseEvent extracts the normalized conversion signal. Events the network
	// doesn't care about return Kind "ignore", not an error.
	ParseEvent(body []byte) (*ConversionEvent, error)
}

func ByName(name string) Adapter {
	switch name {
	case "stripe":
		return &Stripe{}
	case "lemonsqueezy":
		return &LemonSqueezy{}
	case "polar":
		return &Polar{}
	}
	return nil
}
