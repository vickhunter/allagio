package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Site struct {
	ID           string `json:"id"`
	APIKeyHash   string `json:"-"`
	APIKeyPrefix string `json:"-"`
	Domain       string `json:"domain"`
	Name         string `json:"name"`
}

type SiteKeyRotation struct {
	SiteID        string `json:"site_id"`
	OldKey        string `json:"old_key"`
	NewKey        string `json:"new_key"`
	RotatedAt     string `json:"rotated_at"`
	RotationActor string `json:"actor"`
}

type Article struct {
	ID                 string `json:"id"`
	SiteID             string `json:"site_id"`
	WPPostID           int64  `json:"wp_post_id"`
	Title              string `json:"title"`
	URL                string `json:"url"`
	MonetizationScore  int    `json:"monetization_score"`
	Potential          string `json:"potential"`
	ExistingLinksJSON  []byte `json:"-"`
	AnalysisJSON       []byte `json:"-"`
	DisclosureInserted bool   `json:"disclosure_inserted"`
}

type Recommendation struct {
	ID                string  `json:"id"`
	ArticleID         string  `json:"article_id"`
	ProgramID         *string `json:"program_id"`
	UserProgramID     *string `json:"user_program_id"`
	RelevanceScore    int     `json:"relevance_score"`
	Reason            string  `json:"reason"`
	ReasonSummary     string  `json:"reason_summary,omitempty"`
	ExpectedImpact    string  `json:"expected_impact,omitempty"`
	PlacementWhy      string  `json:"placement_why,omitempty"`
	InsertionType     *string `json:"insertion_type"`
	SuggestedPosition *string `json:"suggested_position"`
	Status            string  `json:"status"`
	ProgramName       string  `json:"program_name"`
	ProgramType       string  `json:"program_type"`
}

type RecommendationContent struct {
	ProgramName       string
	ProgramType       string
	Company           string
	Reason            string
	InsertionType     string
	SuggestedPosition string
	Link              string
	ArticleTitle      string
	ArticleURL        string
}

type UserProgram struct {
	ID     string `json:"id"`
	SiteID string `json:"site_id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Link   string `json:"affiliate_link"`
}

type CuratedProgram struct {
	ID       string
	Name     string
	Company  string
	Slug     string
	Keywords []string
	Category string
}

type ArticleClickCount struct {
	ArticleID string `json:"article_id"`
	WPPostID  int64  `json:"wp_post_id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Clicks    int    `json:"clicks"`
}

type RecommendationClickCount struct {
	RecommendationID string `json:"recommendation_id"`
	ArticleID        string `json:"article_id"`
	ProgramName      string `json:"program_name"`
	ProgramType      string `json:"program_type"`
	Clicks           int    `json:"clicks"`
}

type ClickSummary struct {
	TotalClicks           int `json:"total_clicks"`
	UniqueArticles        int `json:"unique_articles"`
	UniqueRecommendations int `json:"unique_recommendations"`
}

type DailyClickCount struct {
	Day    string `json:"day"`
	Clicks int    `json:"clicks"`
}

type AttributionProgramRaw struct {
	ProgramName    string
	ProgramType    string
	Credits        float64
	CommissionType string
	CommissionUnit string
	Commission     float64
}

type AttributionProgram struct {
	ProgramName         string  `json:"program_name"`
	ProgramType         string  `json:"program_type"`
	AttributedClicks    float64 `json:"attributed_clicks"`
	ClickSharePercent   float64 `json:"click_share_percent"`
	EstimatedRevenueUSD float64 `json:"estimated_revenue_usd"`
	CommissionType      string  `json:"commission_type"`
	CommissionUnit      string  `json:"commission_unit"`
}

type FunnelMetrics struct {
	Clicks             int     `json:"clicks"`
	Inserted           int     `json:"inserted"`
	Dismissed          int     `json:"dismissed"`
	Approved           int     `json:"approved"`
	Pending            int     `json:"pending"`
	InsertToClickRate  float64 `json:"insert_to_click_rate"`
	DismissRate        float64 `json:"dismiss_rate"`
	TrackedSuggestions int     `json:"tracked_suggestions"`
}

type CohortMetrics struct {
	CohortDate       string  `json:"cohort_date"`
	Articles         int     `json:"articles"`
	Inserted         int     `json:"inserted"`
	Clicks           int     `json:"clicks"`
	ClicksPerArticle float64 `json:"clicks_per_article"`
}

type RetentionCohort struct {
	CohortWeek  string  `json:"cohort_week"`
	Articles    int     `json:"articles"`
	Week0Active int     `json:"week0_active"`
	Week1Active int     `json:"week1_active"`
	Week2Active int     `json:"week2_active"`
	Week3Active int     `json:"week3_active"`
	Week1Rate   float64 `json:"week1_rate"`
	Week2Rate   float64 `json:"week2_rate"`
	Week3Rate   float64 `json:"week3_rate"`
	Week1Churn  float64 `json:"week1_churn"`
	Week2Churn  float64 `json:"week2_churn"`
	Week3Churn  float64 `json:"week3_churn"`
}

type RecommendationQualityRaw struct {
	ProgramName string
	ProgramType string
	Total       int
	Approved    int
	Dismissed   int
	Inserted    int
	Pending     int
	Clicks      int
}

type RecommendationQuality struct {
	ProgramName  string  `json:"program_name"`
	ProgramType  string  `json:"program_type"`
	Total        int     `json:"total"`
	Approved     int     `json:"approved"`
	Dismissed    int     `json:"dismissed"`
	Inserted     int     `json:"inserted"`
	Pending      int     `json:"pending"`
	Clicks       int     `json:"clicks"`
	ApprovalRate float64 `json:"approval_rate"`
	DismissRate  float64 `json:"dismiss_rate"`
	InsertRate   float64 `json:"insert_rate"`
	ClickRate    float64 `json:"click_rate"`
}

type AuditEvent struct {
	ID        string                 `json:"id"`
	EventType string                 `json:"event_type"`
	Severity  string                 `json:"severity"`
	Actor     string                 `json:"actor"`
	IPAddress string                 `json:"ip_address"`
	Endpoint  string                 `json:"endpoint"`
	Method    string                 `json:"method"`
	Details   map[string]interface{} `json:"details"`
	CreatedAt string                 `json:"created_at"`
}

type RecommendationTuneProfile struct {
	ProgramID     string  `json:"program_id"`
	ProgramName   string  `json:"program_name"`
	IsUserProgram bool    `json:"is_user_program"`
	SampleSize    int     `json:"sample_size"`
	ApprovalRate  float64 `json:"approval_rate"`
	InsertRate    float64 `json:"insert_rate"`
	ClickRate     float64 `json:"click_rate"`
	Multiplier    float64 `json:"multiplier"`
}

// GetSiteByKeyHash looks up a site by the SHA-256 hash of an API key (current key only).
func (d *DB) GetSiteByKeyHash(ctx context.Context, keyHash, keyPrefix string) (*Site, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT id, api_key_hash, COALESCE(api_key_prefix,''), domain, COALESCE(name,'')
FROM sites
WHERE api_key_prefix=$1 AND api_key_hash=$2`, keyPrefix, keyHash)
	var s Site
	if err := row.Scan(&s.ID, &s.APIKeyHash, &s.APIKeyPrefix, &s.Domain, &s.Name); err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSiteByAnyKeyHash looks up a site by hash of current or previous (non-expired) key.
func (d *DB) GetSiteByAnyKeyHash(ctx context.Context, keyHash, keyPrefix string) (*Site, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT id, api_key_hash, COALESCE(api_key_prefix,''), domain, COALESCE(name,'')
FROM sites
WHERE (api_key_prefix=$1 AND api_key_hash=$2)
   OR (api_key_prev_prefix=$1 AND api_key_prev_hash=$2 AND (api_key_previous_expires_at IS NULL OR api_key_previous_expires_at > NOW()))`,
		keyPrefix, keyHash)
	var s Site
	if err := row.Scan(&s.ID, &s.APIKeyHash, &s.APIKeyPrefix, &s.Domain, &s.Name); err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *DB) GetSiteByID(ctx context.Context, siteID string) (*Site, error) {
	row := d.Pool.QueryRow(ctx, `SELECT id, COALESCE(api_key_hash,''), COALESCE(api_key_prefix,''), domain, COALESCE(name,'') FROM sites WHERE id=$1`, siteID)
	var s Site
	if err := row.Scan(&s.ID, &s.APIKeyHash, &s.APIKeyPrefix, &s.Domain, &s.Name); err != nil {
		return nil, err
	}
	return &s, nil
}

func (d *DB) DeleteSiteByID(ctx context.Context, siteID string) (bool, error) {
	res, err := d.Pool.Exec(ctx, `DELETE FROM sites WHERE id=$1`, siteID)
	if err != nil {
		return false, err
	}
	return res.RowsAffected() > 0, nil
}

func (d *DB) CreateSite(ctx context.Context, keyHash, keyPrefix, domain, name string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `INSERT INTO sites (api_key_hash, api_key_prefix, domain, name) VALUES ($1,$2,$3,$4) RETURNING id`, keyHash, keyPrefix, domain, name).Scan(&id)
	return id, err
}

func (d *DB) RotateSiteAPIKey(ctx context.Context, siteID, newKeyHash, newKeyPrefix string, gracePeriod time.Duration) error {
	graceInterval := fmt.Sprintf("%d hours", int(gracePeriod.Hours()))
	_, err := d.Pool.Exec(ctx, `
UPDATE sites
SET api_key_prev_hash=api_key_hash,
    api_key_prev_prefix=api_key_prefix,
    api_key_hash=$2,
    api_key_prefix=$3,
    api_key_rotated_at=NOW(),
    api_key_previous_expires_at=NOW() + ($4 || ' hours')::interval
WHERE id=$1`, siteID, newKeyHash, newKeyPrefix, graceInterval)
	return err
}

func (d *DB) UpsertArticle(ctx context.Context, siteID string, wpPostID int64, title, url, contentHash string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
INSERT INTO articles (site_id, wp_post_id, title, url, content_hash, updated_at)
VALUES ($1,$2,$3,$4,$5,NOW())
ON CONFLICT (site_id, wp_post_id)
DO UPDATE SET title=EXCLUDED.title, url=EXCLUDED.url, content_hash=EXCLUDED.content_hash, updated_at=NOW()
RETURNING id`, siteID, wpPostID, title, url, contentHash).Scan(&id)
	return id, err
}

func (d *DB) UpdateArticleAnalysis(ctx context.Context, articleID string, analysisJSON []byte, score int, existingLinksJSON []byte) error {
	_, err := d.Pool.Exec(ctx, `
UPDATE articles
SET analysis=$1, monetization_score=$2, existing_affiliate_links=$3, analyzed_at=NOW(), last_analyzed_at=NOW()
WHERE id=$4`, analysisJSON, score, existingLinksJSON, articleID)
	return err
}

func (d *DB) InsertLinkDetection(ctx context.Context, articleID string, url, domain string, isAffiliate bool, confidence float64, method string) error {
	_, err := d.Pool.Exec(ctx, `
INSERT INTO link_detections (article_id, url, domain, is_affiliate, confidence, method)
VALUES ($1,$2,$3,$4,$5,$6)`, articleID, url, domain, isAffiliate, confidence, method)
	return err
}

func (d *DB) UpsertUserProgram(ctx context.Context, siteID, name, domain, programGuess, affiliateLink string) (string, error) {
	var programID string
	err := d.Pool.QueryRow(ctx, `
INSERT INTO user_programs (site_id, name, domain, program_guess)
VALUES ($1,$2,$3,$4)
ON CONFLICT (site_id, domain)
DO UPDATE SET name=EXCLUDED.name
RETURNING id`, siteID, name, domain, programGuess).Scan(&programID)
	if err != nil {
		return "", err
	}
	_, _ = d.Pool.Exec(ctx, `
INSERT INTO user_program_links (user_program_id, affiliate_link)
VALUES ($1,$2)
ON CONFLICT DO NOTHING`, programID, affiliateLink)
	return programID, nil
}

func (d *DB) EnqueueDiscovery(ctx context.Context, domain, sampleURL string) error {
	_, err := d.Pool.Exec(ctx, `
INSERT INTO program_discovery_queue (domain, sample_url, sites_count)
VALUES ($1,$2,1)
ON CONFLICT (domain)
DO UPDATE SET sites_count = program_discovery_queue.sites_count + 1`, domain, sampleURL)
	return err
}

func (d *DB) CreateScanJob(ctx context.Context, siteID string, total int) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
INSERT INTO scan_jobs (site_id, status, total, processed, started_at)
VALUES ($1,'scanning',$2,0,NOW())
RETURNING id`, siteID, total).Scan(&id)
	return id, err
}

func (d *DB) UpdateScanJobProgress(ctx context.Context, jobID string, processed int, status string, errText *string) error {
	_, err := d.Pool.Exec(ctx, `
UPDATE scan_jobs
SET processed=$1, status=$2, finished_at=CASE WHEN $2 IN ('complete','failed') THEN NOW() ELSE finished_at END, error=$3
WHERE id=$4`, processed, status, errText, jobID)
	return err
}

func (d *DB) LatestScanJob(ctx context.Context, siteID string) (map[string]interface{}, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT id, status, total, processed, started_at, finished_at, error
FROM scan_jobs
WHERE site_id=$1
ORDER BY started_at DESC
LIMIT 1`, siteID)
	var id, status string
	var total, processed int
	var startedAt, finishedAt *time.Time
	var errText *string
	if err := row.Scan(&id, &status, &total, &processed, &startedAt, &finishedAt, &errText); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":          id,
		"status":      status,
		"total":       total,
		"processed":   processed,
		"started_at":  formatTimePtr(startedAt),
		"finished_at": formatTimePtr(finishedAt),
		"error":       errText,
	}, nil
}

func (d *DB) ListArticles(ctx context.Context, siteID string) ([]Article, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT id, site_id, wp_post_id, COALESCE(title,''), COALESCE(url,''), monetization_score, COALESCE(analysis,'{}'::jsonb)::text, COALESCE(existing_affiliate_links,'[]'::jsonb)::text, disclosure_inserted
FROM articles
WHERE site_id=$1
ORDER BY monetization_score ASC, updated_at DESC`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		var analysisText, linksText string
		if err := rows.Scan(&a.ID, &a.SiteID, &a.WPPostID, &a.Title, &a.URL, &a.MonetizationScore, &analysisText, &linksText, &a.DisclosureInserted); err != nil {
			return nil, err
		}
		a.AnalysisJSON = []byte(analysisText)
		a.ExistingLinksJSON = []byte(linksText)
		out = append(out, a)
	}
	return out, nil
}

func (d *DB) GetArticle(ctx context.Context, siteID, articleID string) (*Article, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT id, site_id, wp_post_id, COALESCE(title,''), COALESCE(url,''), monetization_score, COALESCE(analysis,'{}'::jsonb)::text, COALESCE(existing_affiliate_links,'[]'::jsonb)::text, disclosure_inserted
FROM articles
WHERE site_id=$1 AND id=$2`, siteID, articleID)
	var a Article
	var analysisText, linksText string
	if err := row.Scan(&a.ID, &a.SiteID, &a.WPPostID, &a.Title, &a.URL, &a.MonetizationScore, &analysisText, &linksText, &a.DisclosureInserted); err != nil {
		return nil, err
	}
	a.AnalysisJSON = []byte(analysisText)
	a.ExistingLinksJSON = []byte(linksText)
	return &a, nil
}

func (d *DB) ListRecommendations(ctx context.Context, siteID string, articleID *string) ([]Recommendation, error) {
	query := `
SELECT r.id, r.article_id, r.program_id, r.user_program_id, r.relevance_score, r.reason, r.insertion_type, r.suggested_position, r.status,
       COALESCE(ap.name, up.name, '') AS program_name,
       CASE WHEN r.user_program_id IS NOT NULL THEN 'user' ELSE 'curated' END AS program_type
FROM recommendations r
JOIN articles a ON a.id = r.article_id
LEFT JOIN affiliate_programs ap ON ap.id = r.program_id
LEFT JOIN user_programs up ON up.id = r.user_program_id
WHERE a.site_id=$1`
	args := []interface{}{siteID}
	if articleID != nil {
		query += " AND r.article_id=$2"
		args = append(args, *articleID)
	}
	rows, err := d.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Recommendation
	for rows.Next() {
		var r Recommendation
		if err := rows.Scan(&r.ID, &r.ArticleID, &r.ProgramID, &r.UserProgramID, &r.RelevanceScore, &r.Reason, &r.InsertionType, &r.SuggestedPosition, &r.Status, &r.ProgramName, &r.ProgramType); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (d *DB) GetRecommendation(ctx context.Context, siteID, recID string) (*Recommendation, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT r.id, r.article_id, r.program_id, r.user_program_id, r.relevance_score, r.reason, r.insertion_type, r.suggested_position, r.status,
       COALESCE(ap.name, up.name, '') AS program_name,
       CASE WHEN r.user_program_id IS NOT NULL THEN 'user' ELSE 'curated' END AS program_type
FROM recommendations r
JOIN articles a ON a.id = r.article_id
LEFT JOIN affiliate_programs ap ON ap.id = r.program_id
LEFT JOIN user_programs up ON up.id = r.user_program_id
WHERE a.site_id=$1 AND r.id=$2`, siteID, recID)
	var r Recommendation
	if err := row.Scan(&r.ID, &r.ArticleID, &r.ProgramID, &r.UserProgramID, &r.RelevanceScore, &r.Reason, &r.InsertionType, &r.SuggestedPosition, &r.Status, &r.ProgramName, &r.ProgramType); err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *DB) GetRecommendationContent(ctx context.Context, siteID, recID string) (*RecommendationContent, error) {
	row := d.Pool.QueryRow(ctx, `
SELECT
       COALESCE(ap.name, up.name, '') AS program_name,
       CASE WHEN r.user_program_id IS NOT NULL THEN 'user' ELSE 'curated' END AS program_type,
       COALESCE(ap.company,'') AS company,
       COALESCE(r.reason,'') AS reason,
       COALESCE(r.insertion_type,'') AS insertion_type,
       COALESCE(r.suggested_position,'') AS suggested_position,
       COALESCE(saa.affiliate_link, upl.affiliate_link, ap.referral_url, ap.signup_url, ap.website_url, '') AS link,
       COALESCE(a.title,'') AS article_title,
       COALESCE(a.url,'') AS article_url
FROM recommendations r
JOIN articles a ON a.id = r.article_id
LEFT JOIN affiliate_programs ap ON ap.id = r.program_id
LEFT JOIN site_affiliate_accounts saa ON saa.program_id = r.program_id AND saa.site_id = a.site_id
LEFT JOIN user_programs up ON up.id = r.user_program_id
LEFT JOIN LATERAL (
    SELECT affiliate_link FROM user_program_links WHERE user_program_id = up.id ORDER BY created_at DESC LIMIT 1
) upl ON true
WHERE a.site_id=$1 AND r.id=$2`, siteID, recID)
	var rc RecommendationContent
	if err := row.Scan(&rc.ProgramName, &rc.ProgramType, &rc.Company, &rc.Reason, &rc.InsertionType, &rc.SuggestedPosition, &rc.Link, &rc.ArticleTitle, &rc.ArticleURL); err != nil {
		return nil, err
	}
	return &rc, nil
}

func (d *DB) CreateRecommendation(ctx context.Context, articleID string, programID *string, userProgramID *string, relevance int, reason string, insertionType string, position string) error {
	_, err := d.Pool.Exec(ctx, `
INSERT INTO recommendations (article_id, program_id, user_program_id, relevance_score, reason, insertion_type, suggested_position)
VALUES ($1,$2,$3,$4,$5,$6,$7)`, articleID, programID, userProgramID, relevance, reason, insertionType, position)
	return err
}

func (d *DB) ListUserPrograms(ctx context.Context, siteID string) ([]UserProgram, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT up.id, up.site_id, up.name, COALESCE(up.domain,''), COALESCE(upl.affiliate_link,'')
FROM user_programs up
LEFT JOIN LATERAL (
    SELECT affiliate_link FROM user_program_links WHERE user_program_id = up.id ORDER BY created_at DESC LIMIT 1
) upl ON true
WHERE up.site_id=$1
ORDER BY up.created_at DESC`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserProgram
	for rows.Next() {
		var up UserProgram
		if err := rows.Scan(&up.ID, &up.SiteID, &up.Name, &up.Domain, &up.Link); err != nil {
			return nil, err
		}
		out = append(out, up)
	}
	return out, nil
}

func (d *DB) CreateUserProgram(ctx context.Context, siteID, name, domain, link string) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
INSERT INTO user_programs (site_id, name, domain, program_guess)
VALUES ($1,$2,$3,$4)
RETURNING id`, siteID, name, domain, name).Scan(&id)
	if err != nil {
		return "", err
	}
	if link != "" {
		_, _ = d.Pool.Exec(ctx, `INSERT INTO user_program_links (user_program_id, affiliate_link) VALUES ($1,$2)`, id, link)
	}
	return id, nil
}

func (d *DB) DeleteUserProgram(ctx context.Context, siteID, id string) error {
	ct, err := d.Pool.Exec(ctx, `DELETE FROM user_programs WHERE id=$1 AND site_id=$2`, id, siteID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (d *DB) ListCuratedProgramsForMatching(ctx context.Context, limit int) ([]CuratedProgram, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT id, name, company, slug, COALESCE(keywords,'{}')::text[], COALESCE(primary_category,'')
FROM affiliate_programs
WHERE status='active'
ORDER BY program_quality_score DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CuratedProgram
	for rows.Next() {
		var p CuratedProgram
		var keywords []string
		if err := rows.Scan(&p.ID, &p.Name, &p.Company, &p.Slug, &keywords, &p.Category); err != nil {
			return nil, err
		}
		p.Keywords = keywords
		out = append(out, p)
	}
	return out, nil
}

func (d *DB) ListArticleClickCounts(ctx context.Context, siteID string, days int) ([]ArticleClickCount, error) {
	if days < 0 {
		days = 0
	}
	rows, err := d.Pool.Query(ctx, `
SELECT a.id, a.wp_post_id, COALESCE(a.title,''), COALESCE(a.url,''), COUNT(c.id) AS clicks
FROM articles a
LEFT JOIN clicks c ON c.article_id = a.id
    AND ($2 = 0 OR c.clicked_at >= NOW() - ($2 * INTERVAL '1 day'))
WHERE a.site_id=$1
GROUP BY a.id, a.wp_post_id, a.title, a.url
ORDER BY clicks DESC`, siteID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ArticleClickCount
	for rows.Next() {
		var row ArticleClickCount
		if err := rows.Scan(&row.ArticleID, &row.WPPostID, &row.Title, &row.URL, &row.Clicks); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (d *DB) ListRecommendationClickCounts(ctx context.Context, siteID string, days int) ([]RecommendationClickCount, error) {
	if days < 0 {
		days = 0
	}
	rows, err := d.Pool.Query(ctx, `
SELECT r.id,
       r.article_id,
       COALESCE(ap.name, up.name, '') AS program_name,
       CASE WHEN r.user_program_id IS NOT NULL THEN 'user' ELSE 'curated' END AS program_type,
       COUNT(c.id) AS clicks
FROM recommendations r
JOIN articles a ON a.id = r.article_id
LEFT JOIN clicks c ON c.recommendation_id = r.id
    AND ($2 = 0 OR c.clicked_at >= NOW() - ($2 * INTERVAL '1 day'))
LEFT JOIN affiliate_programs ap ON ap.id = r.program_id
LEFT JOIN user_programs up ON up.id = r.user_program_id
WHERE a.site_id=$1
GROUP BY r.id, r.article_id, ap.name, up.name, r.user_program_id
ORDER BY clicks DESC`, siteID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecommendationClickCount
	for rows.Next() {
		var row RecommendationClickCount
		if err := rows.Scan(&row.RecommendationID, &row.ArticleID, &row.ProgramName, &row.ProgramType, &row.Clicks); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (d *DB) GetClickSummary(ctx context.Context, siteID string, days int) (*ClickSummary, error) {
	if days < 0 {
		days = 0
	}
	row := d.Pool.QueryRow(ctx, `
SELECT
    COUNT(c.id) AS total_clicks,
    COUNT(DISTINCT c.article_id) AS unique_articles,
    COUNT(DISTINCT c.recommendation_id) AS unique_recommendations
FROM clicks c
WHERE c.site_id=$1
  AND ($2 = 0 OR c.clicked_at >= NOW() - ($2 * INTERVAL '1 day'))`, siteID, days)
	var out ClickSummary
	if err := row.Scan(&out.TotalClicks, &out.UniqueArticles, &out.UniqueRecommendations); err != nil {
		return nil, err
	}
	return &out, nil
}

func (d *DB) ListDailyClickCounts(ctx context.Context, siteID string, days int) ([]DailyClickCount, error) {
	if days <= 0 {
		days = 30
	}
	rows, err := d.Pool.Query(ctx, `
SELECT TO_CHAR(day, 'YYYY-MM-DD') AS day, clicks
FROM (
    SELECT DATE_TRUNC('day', c.clicked_at)::date AS day, COUNT(*)::int AS clicks
    FROM clicks c
    WHERE c.site_id=$1
      AND c.clicked_at >= NOW() - ($2 * INTERVAL '1 day')
    GROUP BY 1
) q
ORDER BY day ASC`, siteID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DailyClickCount, 0)
	for rows.Next() {
		var row DailyClickCount
		if err := rows.Scan(&row.Day, &row.Clicks); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (d *DB) ListAttributionProgramRaw(ctx context.Context, siteID string, days int, model string) ([]AttributionProgramRaw, error) {
	if days < 0 {
		days = 0
	}
	query := `
SELECT
    COALESCE(ap.name, up.name, 'Unknown program') AS program_name,
    CASE WHEN r.user_program_id IS NOT NULL THEN 'user' ELSE 'curated' END AS program_type,
    SUM(c.credit)::float8 AS credits,
    COALESCE(ap.commission_type, 'unknown') AS commission_type,
    COALESCE(ap.commission_unit, '') AS commission_unit,
    COALESCE(ap.commission_value, 0)::float8 AS commission_value
FROM (%s) c
LEFT JOIN recommendations r ON r.id = c.recommendation_id
LEFT JOIN affiliate_programs ap ON ap.id = COALESCE(c.program_id, r.program_id)
LEFT JOIN user_programs up ON up.id = r.user_program_id
GROUP BY 1,2,4,5,6
ORDER BY credits DESC
LIMIT 25`
	rows, err := d.Pool.Query(ctx, fmt.Sprintf(query, attributionModelSQL(model)), siteID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AttributionProgramRaw, 0)
	for rows.Next() {
		var row AttributionProgramRaw
		if err := rows.Scan(&row.ProgramName, &row.ProgramType, &row.Credits, &row.CommissionType, &row.CommissionUnit, &row.Commission); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func attributionModelSQL(model string) string {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "first_click":
		return `
SELECT
    c.site_id,
    c.article_id,
    c.recommendation_id,
    c.program_id,
    c.clicked_at,
    1.0::float8 AS credit
FROM (
    SELECT DISTINCT ON (COALESCE(cl.ip_hash,'anon:'||cl.id::text) || ':' || COALESCE(cl.article_id::text,'none'))
        cl.*
    FROM clicks cl
    WHERE cl.site_id=$1
      AND ($2 = 0 OR cl.clicked_at >= NOW() - ($2 * INTERVAL '1 day'))
    ORDER BY (COALESCE(cl.ip_hash,'anon:'||cl.id::text) || ':' || COALESCE(cl.article_id::text,'none')), cl.clicked_at ASC
) c`
	case "linear_session":
		return `
WITH session_touches AS (
    SELECT DISTINCT
        COALESCE(cl.ip_hash,'anon:'||cl.id::text) || ':' || COALESCE(cl.article_id::text,'none') AS session_key,
        cl.site_id,
        cl.article_id,
        cl.recommendation_id,
        cl.program_id,
        MIN(cl.clicked_at) AS clicked_at
    FROM clicks cl
    WHERE cl.site_id=$1
      AND ($2 = 0 OR cl.clicked_at >= NOW() - ($2 * INTERVAL '1 day'))
    GROUP BY 1,2,3,4,5
),
session_counts AS (
    SELECT session_key, COUNT(*)::float8 AS touch_count
    FROM session_touches
    GROUP BY session_key
)
SELECT
    st.site_id,
    st.article_id,
    st.recommendation_id,
    st.program_id,
    st.clicked_at,
    (1.0 / NULLIF(sc.touch_count, 0))::float8 AS credit
FROM session_touches st
JOIN session_counts sc ON sc.session_key = st.session_key`
	default:
		return `
SELECT
    cl.site_id,
    cl.article_id,
    cl.recommendation_id,
    cl.program_id,
    cl.clicked_at,
    1.0::float8 AS credit
FROM clicks cl
WHERE cl.site_id=$1
  AND ($2 = 0 OR cl.clicked_at >= NOW() - ($2 * INTERVAL '1 day'))`
	}
}

func (d *DB) GetFunnelMetrics(ctx context.Context, siteID string, days int) (*FunnelMetrics, error) {
	if days < 0 {
		days = 0
	}
	row := d.Pool.QueryRow(ctx, `
SELECT
  (SELECT COUNT(*)::int
   FROM clicks c
   WHERE c.site_id=$1
     AND ($2 = 0 OR c.clicked_at >= NOW() - ($2 * INTERVAL '1 day'))) AS clicks,
  (SELECT COUNT(*)::int
   FROM recommendations r
   JOIN articles a ON a.id=r.article_id
   WHERE a.site_id=$1
     AND r.status='inserted'
     AND ($2 = 0 OR r.inserted_at >= NOW() - ($2 * INTERVAL '1 day'))) AS inserted,
  (SELECT COUNT(*)::int
   FROM recommendations r
   JOIN articles a ON a.id=r.article_id
   WHERE a.site_id=$1
     AND r.status='dismissed'
     AND ($2 = 0 OR r.created_at >= NOW() - ($2 * INTERVAL '1 day'))) AS dismissed,
  (SELECT COUNT(*)::int
   FROM recommendations r
   JOIN articles a ON a.id=r.article_id
   WHERE a.site_id=$1
     AND r.status='approved'
     AND ($2 = 0 OR r.approved_at >= NOW() - ($2 * INTERVAL '1 day'))) AS approved,
  (SELECT COUNT(*)::int
   FROM recommendations r
   JOIN articles a ON a.id=r.article_id
   WHERE a.site_id=$1
     AND r.status='pending'
     AND ($2 = 0 OR r.created_at >= NOW() - ($2 * INTERVAL '1 day'))) AS pending`, siteID, days)
	var out FunnelMetrics
	if err := row.Scan(&out.Clicks, &out.Inserted, &out.Dismissed, &out.Approved, &out.Pending); err != nil {
		return nil, err
	}
	out.TrackedSuggestions = out.Inserted + out.Dismissed + out.Approved + out.Pending
	if out.Inserted > 0 {
		out.InsertToClickRate = float64(out.Clicks) / float64(out.Inserted) * 100
	}
	if out.TrackedSuggestions > 0 {
		out.DismissRate = float64(out.Dismissed) / float64(out.TrackedSuggestions) * 100
	}
	return &out, nil
}

func (d *DB) ListCohortMetrics(ctx context.Context, siteID string, days int) ([]CohortMetrics, error) {
	if days <= 0 {
		days = 90
	}
	rows, err := d.Pool.Query(ctx, `
WITH article_cohorts AS (
    SELECT a.id, DATE_TRUNC('day', a.created_at)::date AS cohort_date
    FROM articles a
    WHERE a.site_id=$1
      AND a.created_at >= NOW() - ($2 * INTERVAL '1 day')
),
recommendation_stats AS (
    SELECT r.article_id, COUNT(*) FILTER (WHERE r.status='inserted')::int AS inserted
    FROM recommendations r
    JOIN articles a ON a.id=r.article_id
    WHERE a.site_id=$1
    GROUP BY r.article_id
),
click_stats AS (
    SELECT c.article_id, COUNT(*)::int AS clicks
    FROM clicks c
    WHERE c.site_id=$1
      AND c.clicked_at >= NOW() - ($2 * INTERVAL '1 day')
    GROUP BY c.article_id
)
SELECT
    TO_CHAR(ac.cohort_date, 'YYYY-MM-DD') AS cohort_date,
    COUNT(ac.id)::int AS articles,
    COALESCE(SUM(rs.inserted),0)::int AS inserted,
    COALESCE(SUM(cs.clicks),0)::int AS clicks,
    CASE WHEN COUNT(ac.id) > 0 THEN COALESCE(SUM(cs.clicks),0)::float8 / COUNT(ac.id)::float8 ELSE 0 END AS clicks_per_article
FROM article_cohorts ac
LEFT JOIN recommendation_stats rs ON rs.article_id = ac.id
LEFT JOIN click_stats cs ON cs.article_id = ac.id
GROUP BY ac.cohort_date
ORDER BY ac.cohort_date DESC`, siteID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]CohortMetrics, 0)
	for rows.Next() {
		var row CohortMetrics
		if err := rows.Scan(&row.CohortDate, &row.Articles, &row.Inserted, &row.Clicks, &row.ClicksPerArticle); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (d *DB) ListRecommendationQualityRaw(ctx context.Context, siteID string, days int) ([]RecommendationQualityRaw, error) {
	if days < 0 {
		days = 0
	}
	rows, err := d.Pool.Query(ctx, `
WITH recommendation_clicks AS (
    SELECT c.recommendation_id, COUNT(*)::int AS clicks
    FROM clicks c
    WHERE c.site_id=$1
      AND ($2 = 0 OR c.clicked_at >= NOW() - ($2 * INTERVAL '1 day'))
    GROUP BY c.recommendation_id
)
SELECT
    COALESCE(ap.name, up.name, 'Unknown program') AS program_name,
    CASE WHEN r.user_program_id IS NOT NULL THEN 'user' ELSE 'curated' END AS program_type,
    COUNT(*)::int AS total,
    COUNT(*) FILTER (WHERE r.status='approved')::int AS approved,
    COUNT(*) FILTER (WHERE r.status='dismissed')::int AS dismissed,
    COUNT(*) FILTER (WHERE r.status='inserted')::int AS inserted,
    COUNT(*) FILTER (WHERE r.status='pending')::int AS pending,
    COALESCE(SUM(rc.clicks),0)::int AS clicks
FROM recommendations r
JOIN articles a ON a.id = r.article_id
LEFT JOIN affiliate_programs ap ON ap.id = r.program_id
LEFT JOIN user_programs up ON up.id = r.user_program_id
LEFT JOIN recommendation_clicks rc ON rc.recommendation_id = r.id
WHERE a.site_id=$1
  AND ($2 = 0 OR r.created_at >= NOW() - ($2 * INTERVAL '1 day'))
GROUP BY 1,2
ORDER BY total DESC, clicks DESC
LIMIT 50`, siteID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RecommendationQualityRaw, 0)
	for rows.Next() {
		var row RecommendationQualityRaw
		if err := rows.Scan(&row.ProgramName, &row.ProgramType, &row.Total, &row.Approved, &row.Dismissed, &row.Inserted, &row.Pending, &row.Clicks); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (d *DB) ListRecommendationTuneProfiles(ctx context.Context, siteID string, days int) ([]RecommendationTuneProfile, error) {
	if days <= 0 {
		days = 90
	}
	rows, err := d.Pool.Query(ctx, `
WITH base AS (
    SELECT
        COALESCE(r.user_program_id::text, r.program_id::text) AS program_id,
        COALESCE(up.name, ap.name, 'Unknown program') AS program_name,
        CASE WHEN r.user_program_id IS NOT NULL THEN TRUE ELSE FALSE END AS is_user_program,
        COUNT(*)::int AS total,
        COUNT(*) FILTER (WHERE r.status='approved')::int AS approved,
        COUNT(*) FILTER (WHERE r.status='inserted')::int AS inserted,
        COUNT(c.id)::int AS clicks
    FROM recommendations r
    JOIN articles a ON a.id = r.article_id
    LEFT JOIN affiliate_programs ap ON ap.id = r.program_id
    LEFT JOIN user_programs up ON up.id = r.user_program_id
    LEFT JOIN clicks c ON c.recommendation_id = r.id
    WHERE a.site_id=$1
      AND r.created_at >= NOW() - ($2 * INTERVAL '1 day')
      AND COALESCE(r.user_program_id::text, r.program_id::text) IS NOT NULL
    GROUP BY 1,2,3
),
rated AS (
    SELECT
        program_id,
        program_name,
        is_user_program,
        total,
        approved::float8 / GREATEST(total,1)::float8 AS approval_rate,
        inserted::float8 / GREATEST(total,1)::float8 AS insert_rate,
        clicks::float8 / GREATEST(total,1)::float8 AS click_rate
    FROM base
    WHERE total >= 3
)
SELECT
    program_id,
    program_name,
    is_user_program,
    total,
    approval_rate,
    insert_rate,
    click_rate,
    LEAST(1.2, GREATEST(0.8, 0.8 + (approval_rate*0.25 + insert_rate*0.35 + click_rate*0.40))) AS multiplier
FROM rated
ORDER BY total DESC`, siteID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RecommendationTuneProfile, 0)
	for rows.Next() {
		var row RecommendationTuneProfile
		if err := rows.Scan(
			&row.ProgramID,
			&row.ProgramName,
			&row.IsUserProgram,
			&row.SampleSize,
			&row.ApprovalRate,
			&row.InsertRate,
			&row.ClickRate,
			&row.Multiplier,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (d *DB) ListRetentionCohorts(ctx context.Context, siteID string, days int) ([]RetentionCohort, error) {
	if days <= 0 {
		days = 180
	}
	rows, err := d.Pool.Query(ctx, `
WITH base_articles AS (
    SELECT
        a.id AS article_id,
        DATE_TRUNC('week', a.created_at)::date AS cohort_week,
        a.created_at
    FROM articles a
    WHERE a.site_id=$1
      AND a.created_at >= NOW() - ($2 * INTERVAL '1 day')
),
activity AS (
    SELECT
        ba.article_id,
        ba.cohort_week,
        FLOOR(EXTRACT(EPOCH FROM (c.clicked_at - ba.created_at)) / 604800)::int AS week_offset
    FROM base_articles ba
    JOIN clicks c ON c.article_id = ba.article_id
    WHERE c.site_id=$1
      AND c.clicked_at >= ba.created_at
      AND c.clicked_at < ba.created_at + INTERVAL '28 days'
)
SELECT
    TO_CHAR(ba.cohort_week, 'YYYY-MM-DD') AS cohort_week,
    COUNT(DISTINCT ba.article_id)::int AS articles,
    COUNT(DISTINCT CASE WHEN ac.week_offset=0 THEN ac.article_id END)::int AS week0_active,
    COUNT(DISTINCT CASE WHEN ac.week_offset=1 THEN ac.article_id END)::int AS week1_active,
    COUNT(DISTINCT CASE WHEN ac.week_offset=2 THEN ac.article_id END)::int AS week2_active,
    COUNT(DISTINCT CASE WHEN ac.week_offset=3 THEN ac.article_id END)::int AS week3_active
FROM base_articles ba
LEFT JOIN activity ac ON ac.article_id = ba.article_id
GROUP BY ba.cohort_week
ORDER BY ba.cohort_week DESC`, siteID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RetentionCohort, 0)
	for rows.Next() {
		var row RetentionCohort
		if err := rows.Scan(
			&row.CohortWeek,
			&row.Articles,
			&row.Week0Active,
			&row.Week1Active,
			&row.Week2Active,
			&row.Week3Active,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

func (d *DB) UpdateRecommendationStatus(ctx context.Context, siteID, recID, status string) error {
	status = strings.ToLower(strings.TrimSpace(status))
	if !isRecommendationStatus(status) {
		return ErrInvalidStatus
	}
	var current string
	err := d.Pool.QueryRow(ctx, `
SELECT r.status
FROM recommendations r
JOIN articles a ON a.id = r.article_id
WHERE a.site_id=$1 AND r.id=$2`, siteID, recID).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if !canTransitionRecommendationStatus(current, status) {
		return ErrInvalidTransition
	}
	_, err = d.Pool.Exec(ctx, `
UPDATE recommendations r
SET status=$1::text,
    approved_at=CASE WHEN $1::text='approved' AND approved_at IS NULL THEN NOW() ELSE approved_at END,
    inserted_at=CASE WHEN $1::text='inserted' AND inserted_at IS NULL THEN NOW() ELSE inserted_at END
FROM articles a
WHERE r.article_id=a.id AND a.site_id=$2 AND r.id=$3`, status, siteID, recID)
	return err
}

func NormalizeDomain(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = strings.TrimPrefix(input, "http://")
	input = strings.TrimPrefix(input, "https://")
	input = strings.TrimPrefix(input, "www.")
	input = strings.Split(input, "/")[0]
	return input
}

func ToJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

var ErrNotFound = errors.New("not found")
var ErrInvalidStatus = errors.New("invalid recommendation status")
var ErrInvalidTransition = errors.New("invalid recommendation status transition")

func isRecommendationStatus(status string) bool {
	switch status {
	case "pending", "approved", "dismissed", "inserted":
		return true
	default:
		return false
	}
}

func canTransitionRecommendationStatus(current, next string) bool {
	if current == next {
		return true
	}
	switch current {
	case "pending":
		return next == "approved" || next == "dismissed"
	case "approved":
		return next == "dismissed" || next == "inserted"
	default:
		return false
	}
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func (d *DB) InsertAuditEvent(ctx context.Context, ev AuditEvent) error {
	detailsJSON := ToJSON(ev.Details)
	_, err := d.Pool.Exec(ctx, `
INSERT INTO audit_events (event_type, severity, actor, ip_address, endpoint, method, details)
VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		ev.EventType, ev.Severity, ev.Actor, ev.IPAddress, ev.Endpoint, ev.Method, detailsJSON)
	return err
}

func (d *DB) ListAuditEvents(ctx context.Context, eventType, severity string, limit, offset int) ([]AuditEvent, int, error) {
	where := "1=1"
	args := []interface{}{}
	argN := 1
	if eventType != "" {
		where += fmt.Sprintf(" AND event_type=$%d", argN)
		args = append(args, eventType)
		argN++
	}
	if severity != "" {
		where += fmt.Sprintf(" AND severity=$%d", argN)
		args = append(args, severity)
		argN++
	}

	var total int
	countQ := "SELECT COUNT(*) FROM audit_events WHERE " + where
	if err := d.Pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQ := fmt.Sprintf("SELECT id, event_type, severity, COALESCE(actor,''), COALESCE(ip_address,''), COALESCE(endpoint,''), COALESCE(method,''), COALESCE(details::text,'{}'), created_at FROM audit_events WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", where, argN, argN+1)
	args = append(args, limit, offset)

	rows, err := d.Pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var ev AuditEvent
		var detailsStr string
		var createdAt time.Time
		if err := rows.Scan(&ev.ID, &ev.EventType, &ev.Severity, &ev.Actor, &ev.IPAddress, &ev.Endpoint, &ev.Method, &detailsStr, &createdAt); err != nil {
			continue
		}
		ev.CreatedAt = createdAt.Format(time.RFC3339)
		_ = json.Unmarshal([]byte(detailsStr), &ev.Details)
		events = append(events, ev)
	}
	return events, total, nil
}

func (d *DB) CountRecentAuthFailures(ctx context.Context, ip string, minutes int) (int, error) {
	var count int
	err := d.Pool.QueryRow(ctx, `
SELECT COUNT(*) FROM audit_events
WHERE ip_address=$1 AND event_type IN ('auth_failure_api_key','auth_failure_admin_key')
AND created_at > NOW() - ($2 || ' minutes')::interval`, ip, fmt.Sprintf("%d", minutes)).Scan(&count)
	return count, err
}
