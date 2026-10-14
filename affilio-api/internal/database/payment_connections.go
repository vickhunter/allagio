package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type ProductConnection struct {
	ProductID        string
	SecretCiphertext []byte
	LiveMode         bool
	Currency         string
}

func (d *DB) CreateProductConnection(ctx context.Context, connection ProductConnection) (bool, error) {
	res, err := d.Pool.Exec(ctx, `
INSERT INTO product_connections(product_id, secret_ciphertext, live_mode, currency)
SELECT id, $2, $3, $4 FROM products WHERE id=$1 AND payment_provider='stripe'
ON CONFLICT (product_id) DO NOTHING`, connection.ProductID, connection.SecretCiphertext, connection.LiveMode, connection.Currency)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() == 1, nil
}

func (d *DB) ProductConnection(ctx context.Context, productID string) (*ProductConnection, error) {
	var out ProductConnection
	err := d.Pool.QueryRow(ctx, `SELECT product_id, secret_ciphertext, live_mode, currency FROM product_connections WHERE product_id=$1 AND enabled`, productID).
		Scan(&out.ProductID, &out.SecretCiphertext, &out.LiveMode, &out.Currency)
	return &out, err
}

func (d *DB) CreateVerifiedProductConnection(ctx context.Context, makerID string, connection ProductConnection, accountID, priceID, endpointID string) (bool, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	res, err := tx.Exec(ctx, `INSERT INTO product_connections(product_id,secret_ciphertext,live_mode,currency,stripe_account_id,stripe_price_id,stripe_endpoint_id,configuration_verified_at)
SELECT id,$3,$4,$5,$6,$7,$8,NOW() FROM products WHERE id=$1 AND maker_id=$2 AND COALESCE(NULLIF(payment_provider, ''), 'none') IN ('none', 'stripe')
ON CONFLICT(product_id) DO UPDATE SET secret_ciphertext=EXCLUDED.secret_ciphertext,stripe_account_id=EXCLUDED.stripe_account_id,
stripe_price_id=EXCLUDED.stripe_price_id,stripe_endpoint_id=EXCLUDED.stripe_endpoint_id,configuration_verified_at=NOW()
WHERE product_connections.enabled AND product_connections.live_mode=EXCLUDED.live_mode AND product_connections.currency=EXCLUDED.currency
AND (product_connections.stripe_account_id IS NULL OR product_connections.stripe_account_id=EXCLUDED.stripe_account_id)
AND (product_connections.stripe_price_id IS NULL OR product_connections.stripe_price_id=EXCLUDED.stripe_price_id)`, connection.ProductID, makerID, connection.SecretCiphertext, connection.LiveMode, connection.Currency, accountID, priceID, endpointID)
	if err != nil {
		return false, err
	}
	if res.RowsAffected() != 1 {
		return false, nil
	}
	if _, err = tx.Exec(ctx, `UPDATE products SET payment_provider='stripe', updated_at=NOW() WHERE id=$1 AND maker_id=$2 AND COALESCE(NULLIF(payment_provider, ''), 'none') IN ('none', 'stripe')`, connection.ProductID, makerID); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

type PendingWebhook struct {
	Provider  string
	EventID   string
	ProductID string
	Payload   []byte
}

func (d *DB) StoreProductWebhook(ctx context.Context, productID, provider, eventID string, payload []byte) (bool, []byte, error) {
	_, err := d.Pool.Exec(ctx, `INSERT INTO webhook_events(product_id, provider, provider_event_id, payload, signature_valid)
VALUES($1,$2,$3,$4,true) ON CONFLICT(provider, provider_event_id) DO NOTHING`, productID, provider, eventID, payload)
	if err != nil {
		return false, nil, err
	}
	var storedProductID string
	var processed bool
	var canonical []byte
	err = d.Pool.QueryRow(ctx, `SELECT product_id, processed_at IS NOT NULL,payload FROM webhook_events WHERE provider=$1 AND provider_event_id=$2`, provider, eventID).Scan(&storedProductID, &processed, &canonical)
	if err == nil && storedProductID != productID {
		return false, nil, errors.New("event belongs to another product")
	}
	return processed, canonical, err
}

func (d *DB) PendingWebhooks(ctx context.Context) ([]PendingWebhook, error) {
	rows, err := d.Pool.Query(ctx, `SELECT provider, provider_event_id, product_id, payload FROM webhook_events
WHERE processed_at IS NULL AND signature_valid AND product_id IS NOT NULL AND next_attempt_at<=NOW()
ORDER BY next_attempt_at LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PendingWebhook{}
	for rows.Next() {
		var event PendingWebhook
		if err := rows.Scan(&event.Provider, &event.EventID, &event.ProductID, &event.Payload); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func (d *DB) RetryWebhook(ctx context.Context, provider, eventID, reason string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE webhook_events SET attempts=attempts+1, error=$3,
next_attempt_at=NOW()+make_interval(secs=>LEAST(3600, 5*power(2, LEAST(attempts,10)))::int)
WHERE provider=$1 AND provider_event_id=$2 AND processed_at IS NULL`, provider, eventID, reason)
	return err
}

func (d *DB) ProductOwnsClick(ctx context.Context, productID, clickID, currency string, occurredAt time.Time) (bool, error) {
	var valid bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM clicks c JOIN campaigns cam ON cam.id=c.campaign_id
JOIN offers o ON o.id=cam.offer_id WHERE c.id=$2 AND o.product_id=$1 AND COALESCE(c.terms_snapshot->>'currency',o.commission_currency)=$3
AND c.clicked_at <= $4::timestamp + INTERVAL '1 second'
AND c.clicked_at + make_interval(days=>COALESCE((c.terms_snapshot->>'cookie_duration_days')::int,o.cookie_duration_days)) >= $4::timestamp)`, productID, clickID, currency, occurredAt).Scan(&valid)
	return valid, err
}

func (d *DB) ProductOwnsPayment(ctx context.Context, productID, objectID, currency string) (bool, error) {
	if objectID == "" {
		return false, ErrConversionNotFound
	}
	var valid bool
	err := d.Pool.QueryRow(ctx, `SELECT o.product_id::text=$1 AND cv.currency=$3 FROM conversions cv JOIN offers o ON o.id=cv.offer_id
WHERE cv.provider_object_id=$2 AND cv.kind IN ('sale','renewal')
ORDER BY cv.created_at DESC LIMIT 1`, productID, objectID, currency).Scan(&valid)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrConversionNotFound
	}
	return valid, err
}

func (d *DB) EligibleRevenueOfferIDs(ctx context.Context, requireVerified ...bool) (map[string]bool, error) {
	verified := len(requireVerified) > 0 && requireVerified[0]
	rows, err := d.Pool.Query(ctx, `SELECT o.id FROM offers o
 JOIN products p ON p.id=o.product_id
 LEFT JOIN product_connections pc ON pc.product_id=o.product_id
 WHERE o.status='listed' AND o.commission_type IN ('percentage','flat')
 AND (
   COALESCE(NULLIF(p.payment_provider, ''), 'none') = 'none'
   OR (
     p.payment_provider = 'stripe'
     AND pc.enabled AND pc.currency = o.commission_currency
     AND (NOT $1 OR pc.configuration_verified_at IS NOT NULL)
   )
 )`, verified)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
