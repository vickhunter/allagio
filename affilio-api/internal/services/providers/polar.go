package providers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Polar consumes order.created as the sale signal (the click id rides in
// checkout metadata under "dtr") and order.refunded as the refund signal.
// Polar deliberately ships no affiliate rails of its own; this adapter is the
// partner-integration path it sanctions.
type Polar struct{}

func (p *Polar) Name() string { return "polar" }

// VerifySignature implements the Standard Webhooks scheme Polar uses:
// webhook-signature: "v1,<base64(hmac_sha256(secret, "<id>.<timestamp>.<body>"))>".
// The secret may carry the spec's "whsec_" prefix over base64-encoded bytes.
func (p *Polar) VerifySignature(headers http.Header, body []byte, secret string) bool {
	msgID := headers.Get("webhook-id")
	timestamp := headers.Get("webhook-timestamp")
	sigHeader := headers.Get("webhook-signature")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if msgID == "" || timestamp == "" || sigHeader == "" || secret == "" || err != nil || time.Since(time.Unix(seconds, 0)).Abs() > 5*time.Minute {
		return false
	}
	key := []byte(secret)
	if trimmed, ok := strings.CutPrefix(secret, "whsec_"); ok {
		if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
			key = decoded
		}
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msgID + "." + timestamp + "."))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	for _, part := range strings.Fields(sigHeader) {
		if sig, ok := strings.CutPrefix(part, "v1,"); ok && hmac.Equal([]byte(expected), []byte(sig)) {
			return true
		}
	}
	return false
}

type polarEvent struct {
	Type string `json:"type"`
	Data struct {
		ID        string            `json:"id"`
		Amount    int64             `json:"amount"`
		Currency  string            `json:"currency"`
		CreatedAt string            `json:"created_at"`
		Metadata  map[string]string `json:"metadata"`
	} `json:"data"`
}

func (p *Polar) ParseEvent(body []byte) (*ConversionEvent, error) {
	var event polarEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	if event.Type == "" || event.Data.ID == "" {
		return nil, errors.New("missing event type or order id")
	}
	occurred := time.Now().UTC()
	if ts, err := time.Parse(time.RFC3339, event.Data.CreatedAt); err == nil {
		occurred = ts.UTC()
	}
	out := &ConversionEvent{
		EventID:    event.Type + ":" + event.Data.ID,
		ObjectID:   event.Data.ID,
		Currency:   strings.ToUpper(event.Data.Currency),
		OccurredAt: occurred,
	}
	switch event.Type {
	case "order.created":
		out.Kind = "sale"
		out.Amount = float64(event.Data.Amount) / 100
		out.Ref = event.Data.Metadata["dtr"]
	case "order.refunded":
		out.Kind = "refund"
	default:
		out.Kind = "ignore"
	}
	return out, nil
}
