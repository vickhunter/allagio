package database

import (
	"context"
	"time"
)

type Maker struct {
	ID        string
	Email     string
	Name      string
	Status    string
	CreatedAt time.Time
}

type Product struct {
	ID              string
	MakerID         string
	Name            string
	Tagline         string
	WebsiteURL      string
	DocsURL         string
	PricingURL      string
	ProductType     string
	PaymentProvider string
	PrimaryCategory string
	Tags            []string
	Keywords        []string
	ReadinessScore  int
	ReadinessStatus string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Offer struct {
	ID                 string
	ProductID          string
	Status             string
	CommissionType     string
	CommissionValue    float64
	CommissionUnit     string
	CommissionCurrency string
	CookieDurationDays int
	RefundWindowDays   int
	LandingURL         string
	RefParam           string
	Terms              string
	ProgramID          *string
	ReviewReason       string
	ReviewMessage      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type OfferReviewItem struct {
	Offer
	ProductName     string
	ProductTagline  string
	WebsiteURL      string
	PrimaryCategory string
	ReadinessScore  int
	ReadinessStatus string
	MakerEmail      string
	MakerName       string
}

func (d *DB) CreateMaker(ctx context.Context, apiKeyHash, apiKeyPrefix, email, name string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `INSERT INTO makers (api_key_hash, api_key_prefix, email, name) VALUES ($1,$2,$3,$4) RETURNING id`,
		apiKeyHash, apiKeyPrefix, email, name).Scan(&id)
	return id, err
}

func (d *DB) GetMakerByAPIKeyHash(ctx context.Context, keyHash string) (*Maker, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT id, email, COALESCE(name,''), status, created_at
FROM makers
WHERE (api_key_hash=$1 OR (api_key_prev_hash=$1 AND api_key_prev_expires_at > NOW())) AND status='active'`, keyHash)
	var m Maker
	if err := row.Scan(&m.ID, &m.Email, &m.Name, &m.Status, &m.CreatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

func (d *DB) IsMakerTrusted(ctx context.Context, makerID string) (bool, error) {
	var trusted bool
	err := d.Pool.QueryRow(ctx, `SELECT COALESCE(trusted,false) FROM makers WHERE id=$1`, makerID).Scan(&trusted)
	return trusted, err
}

func (d *DB) SetMakerTrusted(ctx context.Context, makerID string, trusted bool) (bool, error) {
	res, err := d.Pool.Exec(ctx, `UPDATE makers SET trusted=$2 WHERE id=$1`, makerID, trusted)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

func (d *DB) RotateMakerAPIKey(ctx context.Context, makerID, newKeyHash, newKeyPrefix string, gracePeriod time.Duration) error {
	_, err := d.Pool.Exec(ctx, `
UPDATE makers
SET api_key_prev_hash=api_key_hash,
    api_key_prev_expires_at=NOW()+$3::interval,
    api_key_hash=$2,
    api_key_prefix=$4,
    api_key_rotated_at=NOW()
WHERE id=$1`, makerID, newKeyHash, gracePeriod, newKeyPrefix)
	return err
}

func (d *DB) CreateProduct(ctx context.Context, p *Product) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
INSERT INTO products (maker_id, name, tagline, website_url, docs_url, pricing_url, product_type, payment_provider, primary_category, tags, keywords)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id`, p.MakerID, p.Name, p.Tagline, p.WebsiteURL, p.DocsURL, p.PricingURL, p.ProductType, p.PaymentProvider, p.PrimaryCategory, p.Tags, p.Keywords).Scan(&id)
	return id, err
}

const productColumns = `id, maker_id, name, COALESCE(tagline,''), website_url, COALESCE(docs_url,''), COALESCE(pricing_url,''), product_type, COALESCE(payment_provider,''), COALESCE(primary_category,''), COALESCE(tags,'{}'), COALESCE(keywords,'{}'), readiness_score, readiness_status, created_at, updated_at`

func scanProduct(row interface{ Scan(...interface{}) error }) (*Product, error) {
	var p Product
	if err := row.Scan(&p.ID, &p.MakerID, &p.Name, &p.Tagline, &p.WebsiteURL, &p.DocsURL, &p.PricingURL, &p.ProductType, &p.PaymentProvider, &p.PrimaryCategory, &p.Tags, &p.Keywords, &p.ReadinessScore, &p.ReadinessStatus, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DB) ListProducts(ctx context.Context, makerID string) ([]Product, error) {
	rows, err := d.Pool.Query(ctx, `SELECT `+productColumns+` FROM products WHERE maker_id=$1 ORDER BY created_at DESC`, makerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (d *DB) GetProduct(ctx context.Context, makerID, productID string) (*Product, error) {
	row := d.Pool.QueryRow(ctx, `SELECT `+productColumns+` FROM products WHERE id=$1 AND maker_id=$2`, productID, makerID)
	return scanProduct(row)
}

func (d *DB) UpdateProduct(ctx context.Context, p *Product) error {
	_, err := d.Pool.Exec(ctx, `
UPDATE products
SET name=$3, tagline=$4, website_url=$5, docs_url=$6, pricing_url=$7,
    product_type=$8, payment_provider=$9, primary_category=$10, tags=$11, keywords=$12,
    updated_at=NOW()
WHERE id=$1 AND maker_id=$2`, p.ID, p.MakerID, p.Name, p.Tagline, p.WebsiteURL, p.DocsURL, p.PricingURL, p.ProductType, p.PaymentProvider, p.PrimaryCategory, p.Tags, p.Keywords)
	return err
}

func (d *DB) CreateOffer(ctx context.Context, o *Offer) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
INSERT INTO offers (product_id, commission_type, commission_value, commission_unit, commission_currency, cookie_duration_days, refund_window_days, landing_url, ref_param, terms)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id`, o.ProductID, o.CommissionType, o.CommissionValue, o.CommissionUnit, o.CommissionCurrency, o.CookieDurationDays, o.RefundWindowDays, o.LandingURL, o.RefParam, o.Terms).Scan(&id)
	return id, err
}

const offerColumns = `o.id, o.product_id, o.status, o.commission_type, o.commission_value, COALESCE(o.commission_unit,''), o.commission_currency, o.cookie_duration_days, o.refund_window_days, o.landing_url, o.ref_param, COALESCE(o.terms,''), o.program_id, COALESCE(o.review_reason,''), COALESCE(o.review_message,''), o.created_at, o.updated_at`

func scanOffer(row interface{ Scan(...interface{}) error }) (*Offer, error) {
	var o Offer
	if err := row.Scan(&o.ID, &o.ProductID, &o.Status, &o.CommissionType, &o.CommissionValue, &o.CommissionUnit, &o.CommissionCurrency, &o.CookieDurationDays, &o.RefundWindowDays, &o.LandingURL, &o.RefParam, &o.Terms, &o.ProgramID, &o.ReviewReason, &o.ReviewMessage, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return nil, err
	}
	return &o, nil
}

func (d *DB) ListOffers(ctx context.Context, makerID string) ([]Offer, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT `+offerColumns+`
FROM offers o
JOIN products p ON p.id = o.product_id
WHERE p.maker_id=$1
ORDER BY o.created_at DESC`, makerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Offer{}
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

func (d *DB) GetOfferForMaker(ctx context.Context, makerID, offerID string) (*Offer, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT `+offerColumns+`
FROM offers o
JOIN products p ON p.id = o.product_id
WHERE o.id=$1 AND p.maker_id=$2`, offerID, makerID)
	return scanOffer(row)
}

func (d *DB) GetOfferByID(ctx context.Context, offerID string) (*Offer, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT `+offerColumns+`
FROM offers o
WHERE o.id=$1`, offerID)
	return scanOffer(row)
}

func (d *DB) UpdateOfferStatus(ctx context.Context, offerID, status string) error {
	if status == "draft" {
		_, err := d.Pool.Exec(ctx, `UPDATE offers SET status=$2, review_reason='', review_message='', updated_at=NOW() WHERE id=$1`, offerID, status)
		return err
	}
	_, err := d.Pool.Exec(ctx, `UPDATE offers SET status=$2, updated_at=NOW() WHERE id=$1`, offerID, status)
	return err
}

// UpdateOfferReview stores the advisory check outcome on a listed offer so
// later reads report the same flag the submit response did.
func (d *DB) UpdateOfferReview(ctx context.Context, offerID, reason, message string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE offers SET review_reason=$2, review_message=$3, updated_at=NOW() WHERE id=$1`, offerID, reason, message)
	return err
}

// CountListedOffers counts offers this maker already has live in the catalog.
func (d *DB) CountListedOffers(ctx context.Context, makerID string) (int, error) {
	var n int
	err := d.Pool.QueryRow(ctx, `
SELECT COUNT(*)
FROM offers o
JOIN products p ON p.id = o.product_id
WHERE p.maker_id=$1 AND o.status='listed'`, makerID).Scan(&n)
	return n, err
}

func (d *DB) ListSubmittedOffers(ctx context.Context, limit, offset int) ([]OfferReviewItem, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT `+offerColumns+`,
       p.name, COALESCE(p.tagline,''), p.website_url, COALESCE(p.primary_category,''),
       p.readiness_score, p.readiness_status,
       m.email, COALESCE(m.name,'')
FROM offers o
JOIN products p ON p.id = o.product_id
JOIN makers m ON m.id = p.maker_id
WHERE o.status='submitted'
ORDER BY o.updated_at ASC
LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OfferReviewItem{}
	for rows.Next() {
		var item OfferReviewItem
		if err := rows.Scan(
			&item.ID, &item.Offer.ProductID, &item.Offer.Status, &item.CommissionType, &item.CommissionValue, &item.CommissionUnit, &item.CommissionCurrency,
			&item.CookieDurationDays, &item.RefundWindowDays, &item.LandingURL, &item.RefParam, &item.Terms, &item.Offer.ProgramID, &item.Offer.ReviewReason, &item.Offer.ReviewMessage, &item.Offer.CreatedAt, &item.Offer.UpdatedAt,
			&item.ProductName, &item.ProductTagline, &item.WebsiteURL, &item.PrimaryCategory,
			&item.ReadinessScore, &item.ReadinessStatus,
			&item.MakerEmail, &item.MakerName,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

type ReadinessAssessment struct {
	ID         string
	ProductID  string
	Status     string
	Score      int
	ChecksJSON []byte
	AISummary  string
	CreatedAt  time.Time
	FinishedAt *time.Time
}

func (d *DB) InsertReadinessAssessment(ctx context.Context, productID string, score int, checksJSON []byte, aiSummary string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
INSERT INTO readiness_assessments (product_id, status, score, checks, ai_summary, finished_at)
VALUES ($1,'complete',$2,$3,$4,NOW())
RETURNING id`, productID, score, checksJSON, aiSummary).Scan(&id)
	return id, err
}

func (d *DB) ListReadinessAssessments(ctx context.Context, makerID, productID string, limit int) ([]ReadinessAssessment, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT a.id, a.product_id, a.status, COALESCE(a.score,0), a.checks, COALESCE(a.ai_summary,''), a.created_at, a.finished_at
FROM readiness_assessments a
JOIN products p ON p.id = a.product_id
WHERE a.product_id=$1 AND p.maker_id=$2
ORDER BY a.created_at DESC
LIMIT $3`, productID, makerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReadinessAssessment{}
	for rows.Next() {
		var a ReadinessAssessment
		if err := rows.Scan(&a.ID, &a.ProductID, &a.Status, &a.Score, &a.ChecksJSON, &a.AISummary, &a.CreatedAt, &a.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (d *DB) UpdateProductReadiness(ctx context.Context, productID string, score int, status string) error {
	_, err := d.Pool.Exec(ctx, `
UPDATE products SET readiness_score=$2, readiness_status=$3, updated_at=NOW() WHERE id=$1`, productID, score, status)
	return err
}

// FillProductTaxonomy fills matcher fields from AI extraction, but only where
// the maker left them empty — maker-provided taxonomy always wins.
func (d *DB) FillProductTaxonomy(ctx context.Context, productID, category string, keywords, tags []string) error {
	_, err := d.Pool.Exec(ctx, `
UPDATE products
SET primary_category = CASE WHEN COALESCE(primary_category,'')='' THEN $2 ELSE primary_category END,
    keywords = CASE WHEN COALESCE(array_length(keywords,1),0)=0 THEN $3 ELSE keywords END,
    tags = CASE WHEN COALESCE(array_length(tags,1),0)=0 THEN $4 ELSE tags END,
    updated_at=NOW()
WHERE id=$1`, productID, category, keywords, tags)
	return err
}

// PromoteOffer lists a submitted offer in the curated catalog: it upserts an
// affiliate_programs row sourced from the offer's product and links the offer
// to it. From that point the existing matcher/recommendation pipeline serves it.
func (d *DB) PromoteOffer(ctx context.Context, offerID, slug string) (string, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var programID string
	err = tx.QueryRow(ctx, `
INSERT INTO affiliate_programs (
    name, company, slug, description, website_url,
    commission_type, commission_value, commission_currency, cookie_duration_days,
    referral_url, signup_url, primary_category, tags, keywords,
    status, is_verified, source, offer_id
)
SELECT p.name,
       COALESCE(NULLIF(m.name,''), p.name),
       $2,
       COALESCE(p.tagline,''),
       p.website_url,
       o.commission_type, o.commission_value, o.commission_currency, o.cookie_duration_days,
       o.landing_url, o.landing_url,
       COALESCE(NULLIF(p.primary_category,''), 'software'),
       COALESCE(p.tags,'{}'), COALESCE(p.keywords,'{}'),
       'active', false, 'distributoor', o.id
FROM offers o
JOIN products p ON p.id = o.product_id
JOIN makers m ON m.id = p.maker_id
WHERE o.id=$1
ON CONFLICT (slug) DO UPDATE SET
    name=EXCLUDED.name,
    description=EXCLUDED.description,
    website_url=EXCLUDED.website_url,
    commission_type=EXCLUDED.commission_type,
    commission_value=EXCLUDED.commission_value,
    referral_url=EXCLUDED.referral_url,
    primary_category=EXCLUDED.primary_category,
    tags=EXCLUDED.tags,
    keywords=EXCLUDED.keywords,
    updated_at=NOW()
RETURNING id`, offerID, slug).Scan(&programID)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE offers SET status='listed', program_id=$2, updated_at=NOW() WHERE id=$1`, offerID, programID); err != nil {
		return "", err
	}
	return programID, tx.Commit(ctx)
}
