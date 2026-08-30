package providers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Stripe consumes checkout.session.completed as the sale signal (the click id
// rides in client_reference_id or metadata.dtr) and charge.refunded as the
// refund signal, joined back to the sale via the payment_intent id.
type Stripe struct{}

func (s *Stripe) Name() string { return "stripe" }

// VerifySignature implements Stripe's Stripe-Signature scheme:
// t=<unix>,v1=hex(hmac_sha256(secret, "<t>.<body>")).
func (s *Stripe) VerifySignature(headers http.Header, body []byte, secret string) bool {
	header := headers.Get("Stripe-Signature")
	if header == "" || secret == "" {
		return false
	}
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}
	if timestamp == "" || len(signatures) == 0 {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range signatures {
		if hmac.Equal([]byte(expected), []byte(sig)) {
			return true
		}
	}
	return false
}

type stripeEvent struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Created int64  `json:"created"`
	Data    struct {
		Object struct {
			ID                string            `json:"id"`
			AmountTotal       int64             `json:"amount_total"`
			Currency          string            `json:"currency"`
			ClientReferenceID string            `json:"client_reference_id"`
			PaymentIntent     string            `json:"payment_intent"`
			Metadata          map[string]string `json:"metadata"`
		} `json:"object"`
	} `json:"data"`
}

func (s *Stripe) ParseEvent(body []byte) (*ConversionEvent, error) {
	var event stripeEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	if event.ID == "" {
		return nil, errors.New("missing event id")
	}
	occurred := time.Unix(event.Created, 0).UTC()
	if event.Created == 0 {
		occurred = time.Now().UTC()
	}
	object := event.Data.Object
	objectID := object.PaymentIntent
	if objectID == "" {
		objectID = object.ID
	}
	out := &ConversionEvent{
		EventID:    event.ID,
		ObjectID:   objectID,
		Currency:   strings.ToUpper(object.Currency),
		OccurredAt: occurred,
	}
	switch event.Type {
	case "checkout.session.completed":
		out.Kind = "sale"
		out.Amount = float64(object.AmountTotal) / 100
		out.Ref = object.ClientReferenceID
		if out.Ref == "" {
			out.Ref = object.Metadata["dtr"]
		}
	case "invoice.paid":
		out.Kind = "renewal"
		out.Amount = float64(object.AmountTotal) / 100
		out.Ref = object.Metadata["dtr"]
	case "charge.refunded":
		out.Kind = "refund"
	default:
		out.Kind = "ignore"
	}
	return out, nil
}
