package services

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

type StripeConnectionCheck struct {
	AccountID  string
	PriceID    string
	EndpointID string
	Currency   string
	LiveMode   bool
}

type StripeConnectionVerifier interface {
	Verify(context.Context, string, string, string, string) (StripeConnectionCheck, error)
}

type StripeAPI struct{ HTTP *http.Client }

func (s *StripeAPI) get(ctx context.Context, key, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.stripe.com/v1"+path, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(key, "")
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("Stripe verification unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("Stripe credentials or permissions could not verify the configuration")
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(out)
}

func liveStripeKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.HasPrefix(key, "sk_live_") || strings.HasPrefix(key, "rk_live_") || strings.HasPrefix(key, "pk_live_")
}

func (s *StripeAPI) Verify(ctx context.Context, key, priceID, endpointID, webhookURL string) (StripeConnectionCheck, error) {
	if liveStripeKey(key) {
		return StripeConnectionCheck{}, errors.New("Stripe test mode only. Live keys are not accepted.")
	}
	if !(strings.HasPrefix(key, "rk_test_") || strings.HasPrefix(key, "sk_test_")) || len(key) > 512 || !strings.HasPrefix(priceID, "price_") || len(priceID) > 255 || !strings.HasPrefix(endpointID, "we_") || len(endpointID) > 255 {
		return StripeConnectionCheck{}, errors.New("valid Stripe key, price and webhook endpoint IDs required")
	}
	var account struct {
		ID             string `json:"id"`
		ChargesEnabled bool   `json:"charges_enabled"`
	}
	if err := s.get(ctx, key, "/account", &account); err != nil {
		return StripeConnectionCheck{}, err
	}
	var price struct {
		ID         string `json:"id"`
		Active     bool   `json:"active"`
		Currency   string `json:"currency"`
		LiveMode   bool   `json:"livemode"`
		Type       string `json:"type"`
		UnitAmount int64  `json:"unit_amount"`
	}
	if err := s.get(ctx, key, "/prices/"+url.PathEscape(priceID), &price); err != nil {
		return StripeConnectionCheck{}, err
	}
	var endpoint struct {
		ID       string   `json:"id"`
		URL      string   `json:"url"`
		Status   string   `json:"status"`
		LiveMode bool     `json:"livemode"`
		Events   []string `json:"enabled_events"`
	}
	if err := s.get(ctx, key, "/webhook_endpoints/"+url.PathEscape(endpointID), &endpoint); err != nil {
		return StripeConnectionCheck{}, err
	}
	if price.LiveMode || endpoint.LiveMode {
		return StripeConnectionCheck{}, errors.New("Stripe test mode only. Live keys are not accepted.")
	}
	if !strings.HasPrefix(account.ID, "acct_") || price.ID != priceID || !price.Active || price.Type != "one_time" || price.UnitAmount <= 0 || !slices.Contains([]string{"usd", "eur", "gbp"}, price.Currency) || endpoint.ID != endpointID || endpoint.URL != webhookURL || endpoint.Status != "enabled" {
		return StripeConnectionCheck{}, errors.New("Stripe price, account or webhook configuration is not ready for the pilot")
	}
	for _, event := range []string{"checkout.session.completed", "checkout.session.async_payment_succeeded", "charge.refunded"} {
		if !slices.Contains(endpoint.Events, event) && !slices.Contains(endpoint.Events, "*") {
			return StripeConnectionCheck{}, errors.New("webhook must include paid checkout and refund events")
		}
	}
	return StripeConnectionCheck{AccountID: account.ID, PriceID: price.ID, EndpointID: endpoint.ID, Currency: strings.ToUpper(price.Currency), LiveMode: price.LiveMode}, nil
}
