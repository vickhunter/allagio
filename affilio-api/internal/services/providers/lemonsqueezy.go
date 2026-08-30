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

// LemonSqueezy consumes order_created as the sale signal (the click id rides in
// checkout custom data under "dtr") and order_refunded as the refund signal,
// joined back to the sale via the order identifier.
type LemonSqueezy struct{}

func (l *LemonSqueezy) Name() string { return "lemonsqueezy" }

// VerifySignature implements Lemon Squeezy's X-Signature scheme:
// hex(hmac_sha256(secret, body)).
func (l *LemonSqueezy) VerifySignature(headers http.Header, body []byte, secret string) bool {
	signature := headers.Get("X-Signature")
	if signature == "" || secret == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

type lemonSqueezyEvent struct {
	Meta struct {
		EventName  string            `json:"event_name"`
		CustomData map[string]string `json:"custom_data"`
	} `json:"meta"`
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Identifier string `json:"identifier"`
			Total      int64  `json:"total"`
			Currency   string `json:"currency"`
			CreatedAt  string `json:"created_at"`
		} `json:"attributes"`
	} `json:"data"`
}

func (l *LemonSqueezy) ParseEvent(body []byte) (*ConversionEvent, error) {
	var event lemonSqueezyEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	if event.Meta.EventName == "" {
		return nil, errors.New("missing event name")
	}
	objectID := event.Data.Attributes.Identifier
	if objectID == "" {
		objectID = event.Data.ID
	}
	if objectID == "" {
		return nil, errors.New("missing order identifier")
	}
	occurred := time.Now().UTC()
	if ts, err := time.Parse(time.RFC3339, event.Data.Attributes.CreatedAt); err == nil {
		occurred = ts.UTC()
	}
	out := &ConversionEvent{
		// Lemon Squeezy has no top-level event id; the event name + order id is
		// unique per state transition and serves as the idempotency key.
		EventID:    event.Meta.EventName + ":" + objectID,
		ObjectID:   objectID,
		Currency:   strings.ToUpper(event.Data.Attributes.Currency),
		OccurredAt: occurred,
	}
	switch event.Meta.EventName {
	case "order_created":
		out.Kind = "sale"
		out.Amount = float64(event.Data.Attributes.Total) / 100
		out.Ref = event.Meta.CustomData["dtr"]
	case "order_refunded":
		out.Kind = "refund"
	default:
		out.Kind = "ignore"
	}
	return out, nil
}
