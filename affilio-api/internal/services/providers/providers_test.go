package providers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestByName(t *testing.T) {
	for _, name := range []string{"stripe", "lemonsqueezy", "polar"} {
		adapter := ByName(name)
		if adapter == nil || adapter.Name() != name {
			t.Fatalf("expected adapter for %s", name)
		}
	}
	if ByName("paypal") != nil {
		t.Fatalf("expected nil for unsupported provider")
	}
}

func stripeSign(secret string, body []byte, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(body)
	return "t=" + timestamp + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestStripeSignatureAndParse(t *testing.T) {
	adapter := &Stripe{}
	secret := "whsec_test"
	body := []byte(`{
		"id": "evt_1",
		"type": "checkout.session.completed",
		"created": 1750000000,
		"data": {"object": {
			"id": "cs_1",
			"amount_total": 4900,
			"currency": "usd",
			"client_reference_id": "11111111-2222-3333-4444-555555555555",
			"payment_intent": "pi_1"
		}}
	}`)
	headers := http.Header{}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	headers.Set("Stripe-Signature", stripeSign(secret, body, timestamp))
	if !adapter.VerifySignature(headers, body, secret) {
		t.Fatalf("expected valid stripe signature")
	}
	headers.Set("Stripe-Signature", stripeSign("wrong-secret", body, timestamp))
	if adapter.VerifySignature(headers, body, secret) {
		t.Fatalf("expected invalid stripe signature with wrong secret")
	}

	event, err := adapter.ParseEvent(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if event.Kind != "sale" || event.Amount != 49 || event.Currency != "USD" ||
		event.Ref != "11111111-2222-3333-4444-555555555555" || event.ObjectID != "pi_1" || event.EventID != "evt_1" {
		t.Fatalf("unexpected stripe event: %+v", event)
	}

	refund, err := adapter.ParseEvent([]byte(`{"id":"evt_2","type":"charge.refunded","created":1750000100,"data":{"object":{"id":"ch_1","payment_intent":"pi_1"}}}`))
	if err != nil || refund.Kind != "refund" || refund.ObjectID != "pi_1" {
		t.Fatalf("unexpected stripe refund: %+v err=%v", refund, err)
	}

	other, err := adapter.ParseEvent([]byte(`{"id":"evt_3","type":"customer.created","data":{"object":{"id":"cus_1"}}}`))
	if err != nil || other.Kind != "ignore" {
		t.Fatalf("expected ignore for unrelated event, got %+v err=%v", other, err)
	}
}

func TestLemonSqueezySignatureAndParse(t *testing.T) {
	adapter := &LemonSqueezy{}
	secret := "ls-secret"
	body := []byte(`{
		"meta": {"event_name": "order_created", "custom_data": {"dtr": "11111111-2222-3333-4444-555555555555"}},
		"data": {"id": "42", "attributes": {"identifier": "order-abc", "total": 1900, "currency": "usd", "created_at": "2026-08-30T10:00:00Z"}}
	}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	headers := http.Header{}
	headers.Set("X-Signature", hex.EncodeToString(mac.Sum(nil)))
	if !adapter.VerifySignature(headers, body, secret) {
		t.Fatalf("expected valid lemonsqueezy signature")
	}
	headers.Set("X-Signature", "deadbeef")
	if adapter.VerifySignature(headers, body, secret) {
		t.Fatalf("expected invalid lemonsqueezy signature")
	}

	event, err := adapter.ParseEvent(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if event.Kind != "sale" || event.Amount != 19 || event.ObjectID != "order-abc" ||
		event.Ref != "11111111-2222-3333-4444-555555555555" || event.EventID != "order_created:order-abc" {
		t.Fatalf("unexpected lemonsqueezy event: %+v", event)
	}

	refund, err := adapter.ParseEvent([]byte(`{"meta":{"event_name":"order_refunded"},"data":{"id":"42","attributes":{"identifier":"order-abc"}}}`))
	if err != nil || refund.Kind != "refund" || refund.ObjectID != "order-abc" {
		t.Fatalf("unexpected lemonsqueezy refund: %+v err=%v", refund, err)
	}
}

func TestPolarSignatureAndParse(t *testing.T) {
	adapter := &Polar{}
	rawSecret := []byte("polar-secret-bytes")
	secret := "whsec_" + base64.StdEncoding.EncodeToString(rawSecret)
	body := []byte(`{
		"type": "order.created",
		"data": {"id": "ord_1", "amount": 2900, "currency": "usd", "created_at": "2026-08-30T10:00:00Z", "metadata": {"dtr": "11111111-2222-3333-4444-555555555555"}}
	}`)
	msgID := "msg_1"
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, rawSecret)
	mac.Write([]byte(msgID + "." + timestamp + "."))
	mac.Write(body)
	headers := http.Header{}
	headers.Set("webhook-id", msgID)
	headers.Set("webhook-timestamp", timestamp)
	headers.Set("webhook-signature", "v1,"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	if !adapter.VerifySignature(headers, body, secret) {
		t.Fatalf("expected valid polar signature")
	}
	headers.Set("webhook-signature", "v1,AAAA")
	if adapter.VerifySignature(headers, body, secret) {
		t.Fatalf("expected invalid polar signature")
	}

	event, err := adapter.ParseEvent(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if event.Kind != "sale" || event.Amount != 29 || event.ObjectID != "ord_1" ||
		event.Ref != "11111111-2222-3333-4444-555555555555" || event.EventID != "order.created:ord_1" {
		t.Fatalf("unexpected polar event: %+v", event)
	}
}
