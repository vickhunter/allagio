package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// DemoPlacementRow is one placement on a site flagged demo.
// The demo pages read only these rows.
type DemoPlacementRow struct {
	ID         string
	Status     string
	Copy       string
	ApprovedBy string
	Domain     string
	Clicks     int
	PaidClicks int
}

// ListDemoPlacements returns placements whose site is demo, newest pending first.
func (d *DB) ListDemoPlacements(ctx context.Context) ([]DemoPlacementRow, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT pp.id::text, pp.status, COALESCE(pp.copy,''), COALESCE(pp.approved_by_kind,''), s.domain,
       (SELECT COUNT(*) FROM clicks c WHERE c.placement_id = pp.id),
       (SELECT COUNT(*) FROM clicks c WHERE c.placement_id = pp.id AND c.click_class = 'browser')
FROM placement_proposals pp
JOIN sites s ON s.id = pp.site_id
WHERE s.demo
ORDER BY CASE WHEN pp.status = 'pending' THEN 0 ELSE 1 END, pp.created_at DESC
LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DemoPlacementRow{}
	for rows.Next() {
		var row DemoPlacementRow
		if err = rows.Scan(&row.ID, &row.Status, &row.Copy, &row.ApprovedBy, &row.Domain, &row.Clicks, &row.PaidClicks); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// DemoSponsoredPlacementID is the go-link id for the newest approved or
// committed placement on a demo site. An empty id means the demo post uses
// the sample link that records nothing.
func (d *DB) DemoSponsoredPlacementID(ctx context.Context) (string, error) {
	var id string
	err := d.Pool.QueryRow(ctx, `
SELECT pp.id::text
FROM placement_proposals pp
JOIN sites s ON s.id = pp.site_id
JOIN offers o ON o.id = pp.offer_id
WHERE s.demo
  AND o.status = 'listed'
  AND pp.status IN ('approved', 'committed')
  AND pp.expires_at > NOW()
ORDER BY COALESCE(pp.approved_at, pp.created_at) DESC
LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return id, err
}
