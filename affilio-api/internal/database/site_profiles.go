package database

import "context"

// SiteProfileData is the raw material for a publisher's topical profile:
// recent article titles plus the categories of the site's recommendations.
type SiteProfileData struct {
	Titles     []string
	Categories []string
}

func (d *DB) GetSiteProfileData(ctx context.Context, siteID string, titleLimit int) (*SiteProfileData, error) {
	out := &SiteProfileData{}
	rows, err := d.Pool.Query(ctx, `
SELECT COALESCE(title,'') FROM articles
WHERE site_id=$1
ORDER BY created_at DESC
LIMIT $2`, siteID, titleLimit)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			rows.Close()
			return nil, err
		}
		if title != "" {
			out.Titles = append(out.Titles, title)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = d.Pool.Query(ctx, `
SELECT COALESCE(ap.primary_category,''), COUNT(*)
FROM recommendations r
JOIN articles a ON a.id = r.article_id
JOIN affiliate_programs ap ON ap.id = r.program_id
WHERE a.site_id=$1
GROUP BY 1`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		for i := 0; i < count; i++ {
			out.Categories = append(out.Categories, category)
		}
	}
	return out, rows.Err()
}

// MatchableSite is a publisher candidate for offer→site ranking.
type MatchableSite struct {
	SiteID string
	Domain string
	Name   string
	Titles []string
	Joined bool
}

// ListMatchableSites returns sites with scanned content, with recent article
// titles for profiling and whether each already runs a campaign for the offer.
func (d *DB) ListMatchableSites(ctx context.Context, offerID string, siteLimit, titleLimit int) ([]MatchableSite, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT s.id, s.domain, COALESCE(s.name,''),
       ARRAY(SELECT COALESCE(a.title,'') FROM articles a WHERE a.site_id = s.id ORDER BY a.created_at DESC LIMIT $3),
       EXISTS(SELECT 1 FROM campaigns c WHERE c.site_id = s.id AND c.offer_id = $1 AND c.status='active')
FROM sites s
WHERE EXISTS (SELECT 1 FROM articles a WHERE a.site_id = s.id)
ORDER BY s.created_at DESC
LIMIT $2`, offerID, siteLimit, titleLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MatchableSite{}
	for rows.Next() {
		var site MatchableSite
		if err := rows.Scan(&site.SiteID, &site.Domain, &site.Name, &site.Titles, &site.Joined); err != nil {
			return nil, err
		}
		out = append(out, site)
	}
	return out, rows.Err()
}
