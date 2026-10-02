package providers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
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
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(seconds, 0)).Abs() > 5*time.Minute || len(signatures) == 0 {
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
	ID       string `json:"id"`
	Type     string `json:"type"`
	Created  int64  `json:"created"`
	LiveMode bool   `json:"livemode"`
	Data     struct {
		Object struct {
			ID             string `json:"id"`
			AmountTotal    int64  `json:"amount_total"`
			AmountRefunded int64  `json:"amount_refunded"`
			PaymentStatus  string `json:"payment_status"`
			TotalDetails   struct {
				Tax      int64 `json:"amount_tax"`
				Shipping int64 `json:"amount_shipping"`
			} `json:"total_details"`
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
		LiveMode:   event.LiveMode,
		ProductID:  object.Metadata["dtr_product_id"],
		Kind:       "ignore",
	}
	switch event.Type {
	case "checkout.session.completed", "checkout.session.async_payment_succeeded":
		if object.PaymentStatus != "paid" || object.AmountTotal <= 0 {
			return out, nil
		}
		if object.PaymentIntent == "" || !stripeCurrency(out.Currency) || object.AmountTotal > 9999999999 || object.TotalDetails.Tax < 0 || object.TotalDetails.Shipping < 0 {
			return nil, errors.New("unsupported paid checkout")
		}
		out.GrossMinor = object.AmountTotal
		out.AmountMinor = object.AmountTotal - object.TotalDetails.Tax - object.TotalDetails.Shipping
		if out.AmountMinor <= 0 {
			return out, nil
		}
		out.Kind = "sale"
		out.Amount = float64(out.AmountMinor) / 100
		out.Ref = object.ClientReferenceID
		if out.Ref == "" {
			out.Ref = object.Metadata["dtr"]
		}
	case "charge.refunded":
		if object.PaymentIntent == "" || object.AmountRefunded <= 0 || !stripeCurrency(out.Currency) {
			return nil, errors.New("unsupported refund")
		}
		out.Kind = "refund"
		out.RefundedMinor = object.AmountRefunded
	}
	return out, nil
}

func stripeCurrency(currency string) bool {
	return currency == "USD" || currency == "EUR" || currency == "GBP"
}
