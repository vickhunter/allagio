package database

import (
	"context"
	"strings"
)

// SuggestCandidate is one listed offer or active curated program a cold
// suggest call can pitch. The draft text is not stored with it.
type SuggestCandidate struct {
	ID              string
	Source          string
	Name            string
	Company         string
	Blurb           string
	Website         string
	Keywords        []string
	Category        string
	CommissionUnit  string
	CommissionType  string
	CommissionValue float64
	Currency        string
	CookieDays      int
	OwnerID         string
}

// ListSuggestCandidates returns listed marketplace offers first, then active
// curated programs. Suggest reads them and does not write.
func (d *DB) ListSuggestCandidates(ctx context.Context, limit int) ([]SuggestCandidate, error) {
	if limit <= 0 {
		limit = 200
	}
	offers, err := d.listSuggestOffers(ctx, limit)
	if err != nil {
		return nil, err
	}
	programs, err := d.listSuggestPrograms(ctx, limit)
	if err != nil {
		return nil, err
	}
	return append(offers, programs...), nil
}

// ListSuggestProducts returns listed products whose name equals text, ignoring
// ASCII case and surrounding space. Unlisted products stay out, including an
// exact-name search. The match is the query, so a catalog of any size still
// finds the name. Offers and spaces are not selected.
func (d *DB) ListSuggestProducts(ctx context.Context, text string) ([]SuggestCandidate, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	rows, err := d.Pool.Query(ctx, `
SELECT DISTINCT ON (p.id) p.id::text, p.name, COALESCE(p.tagline, ''),
       COALESCE(p.website_url, ''),
       COALESCE(p.keywords, '{}'),
       COALESCE(p.primary_category, ''),
       o.id::text
FROM products p
JOIN owners o ON o.maker_id = p.maker_id
WHERE p.listed AND lower(btrim(p.name)) = lower(btrim($1))
ORDER BY p.id, p.created_at ASC`, text)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SuggestCandidate{}
	for rows.Next() {
		var item SuggestCandidate
		item.Source = "product"
		if err := rows.Scan(&item.ID, &item.Name, &item.Blurb, &item.Website, &item.Keywords, &item.Category, &item.OwnerID); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) listSuggestOffers(ctx context.Context, limit int) ([]SuggestCandidate, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT o.id::text, p.name, '', COALESCE(p.tagline, ''),
       COALESCE(NULLIF(o.landing_url, ''), p.website_url, ''),
       COALESCE(p.keywords, '{}'), COALESCE(p.primary_category, ''),
       COALESCE(o.commission_unit, ''), COALESCE(o.commission_type, ''),
       o.commission_value, COALESCE(NULLIF(o.commission_currency, ''), 'USD'),
       COALESCE(o.cookie_duration_days, 0)
FROM offers o
JOIN products p ON p.id = o.product_id
WHERE o.status = 'listed'
ORDER BY o.created_at DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSuggest(rows, "offer")
}

func (d *DB) listSuggestPrograms(ctx context.Context, limit int) ([]SuggestCandidate, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT id::text, name, COALESCE(company, ''), COALESCE(description, ''),
       COALESCE(NULLIF(website_url, ''), signup_url, ''),
       COALESCE(keywords, '{}'), COALESCE(primary_category, ''),
       COALESCE(commission_unit, ''), COALESCE(commission_type, ''),
       COALESCE(commission_value, 0), COALESCE(NULLIF(commission_currency, ''), 'USD'),
       COALESCE(cookie_duration_days, 0)
FROM affiliate_programs
WHERE status = 'active'
ORDER BY program_quality_score DESC, name
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSuggest(rows, "program")
}

func scanSuggest(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}, source string) ([]SuggestCandidate, error) {
	out := []SuggestCandidate{}
	for rows.Next() {
		var item SuggestCandidate
		item.Source = source
		if err := rows.Scan(&item.ID, &item.Name, &item.Company, &item.Blurb, &item.Website, &item.Keywords, &item.Category, &item.CommissionUnit, &item.CommissionType, &item.CommissionValue, &item.Currency, &item.CookieDays); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ProductListedFlags is the owner-page listing state for this maker's products.
func (d *DB) ProductListedFlags(ctx context.Context, makerID string) (map[string]bool, error) {
	rows, err := d.Pool.Query(ctx, `SELECT id::text, listed FROM products WHERE maker_id=$1`, makerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		var listed bool
		if err := rows.Scan(&id, &listed); err != nil {
			return nil, err
		}
		out[id] = listed
	}
	return out, rows.Err()
}

// SetProductListed turns find_partners listing on or off. Only the owner of
// the product's maker can change it. A missing or foreign product is false.
func (d *DB) SetProductListed(ctx context.Context, ownerID, productID string, listed bool) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `
UPDATE products AS p
SET listed=$3, updated_at=NOW()
FROM owners AS o
WHERE p.id=$2::uuid AND o.id=$1::uuid AND o.maker_id IS NOT NULL AND p.maker_id = o.maker_id`, ownerID, productID, listed)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
