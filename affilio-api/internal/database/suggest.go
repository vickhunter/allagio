package database

import "context"

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
