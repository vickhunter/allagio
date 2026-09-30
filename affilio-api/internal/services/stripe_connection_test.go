package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type stripeRoundTripper func(*http.Request) (*http.Response, error)

func (f stripeRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestStripeConnectionVerification(t *testing.T) {
	for _, failure := range []string{"", "foreign_endpoint", "missing_refund", "inactive_price", "subscription", "currency", "mode", "disabled_account", "invalid_key", "unauthorized"} {
		t.Run(failure, func(t *testing.T) {
			requests := 0
			api := &StripeAPI{HTTP: &http.Client{Transport: stripeRoundTripper(func(r *http.Request) (*http.Response, error) {
				requests++
				key, _, ok := r.BasicAuth()
				if !ok || key != "rk_test_pilot" || r.URL.Host != "api.stripe.com" || r.Method != "GET" {
					t.Fatalf("unsafe verification request: %s", r.URL)
				}
				var out map[string]any
				switch r.URL.Path {
				case "/v1/account":
					out = map[string]any{"id": "acct_pilot", "charges_enabled": true}
					if failure == "disabled_account" {
						out["charges_enabled"] = false
					}
				case "/v1/prices/price_pilot":
					out = map[string]any{"id": "price_pilot", "active": true, "currency": "usd", "livemode": false, "type": "one_time", "unit_amount": 4900}
					if failure == "inactive_price" {
						out["active"] = false
					}
					if failure == "subscription" {
						out["type"] = "recurring"
					}
					if failure == "currency" {
						out["currency"] = "jpy"
					}
					if failure == "disabled_account" {
						out["livemode"] = true
					}
				case "/v1/webhook_endpoints/we_pilot":
					out = map[string]any{"id": "we_pilot", "url": "https://pilot.example/v1/webhooks/stripe/product", "status": "enabled", "livemode": false, "enabled_events": []string{"checkout.session.completed", "checkout.session.async_payment_succeeded", "charge.refunded"}}
					if failure == "foreign_endpoint" {
						out["url"] = "https://elsewhere.example/webhook"
					}
					if failure == "missing_refund" {
						out["enabled_events"] = []string{"checkout.session.completed"}
					}
					if failure == "mode" || failure == "disabled_account" {
						out["livemode"] = true
					}
				default:
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
				raw, _ := json.Marshal(out)
				status := 200
				if failure == "unauthorized" {
					status = 401
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(raw))), Header: make(http.Header)}, nil
			})}}
			key := "rk_test_pilot"
			if failure == "invalid_key" {
				key = "not-a-secret"
			}
			check, err := api.Verify(context.Background(), key, "price_pilot", "we_pilot", "https://pilot.example/v1/webhooks/stripe/product")
			if failure == "" {
				if err != nil || check.AccountID != "acct_pilot" || check.Currency != "USD" || check.LiveMode || requests != 3 {
					t.Fatalf("valid configuration failed: %+v %v", check, err)
				}
			} else if err == nil {
				t.Fatalf("invalid configuration accepted: %s", failure)
			}
		})
	}
}
