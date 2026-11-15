package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"affilio-api/internal/database"
	"affilio-api/internal/services"
)

// DigestEmailRunner mails each owner a weekly activity digest: what went
// live, clicks and (test-mode) earnings, and where to act. It runs hourly;
// OwnersDueForDigest applies the weekly spacing, so a missed send simply
// stays due on the next tick. Disabled entirely when the Mailer is not
// configured.
type DigestEmailRunner struct {
	DB       *database.DB
	Mailer   services.MailSender
	BaseURL  string
	Interval time.Duration
	Limit    int
}

func (r *DigestEmailRunner) RunWorker(ctx context.Context) {
	if r.Mailer == nil || !r.Mailer.Enabled() {
		return
	}
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		r.runOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *DigestEmailRunner) runOnce(ctx context.Context) {
	interval := r.Interval
	if interval <= 0 {
		interval = 7 * 24 * time.Hour
	}
	limit := r.Limit
	if limit <= 0 {
		limit = 50
	}
	owners, err := r.DB.OwnersDueForDigest(ctx, time.Now().Add(-interval), limit)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("weekly digest: due-owner query failed: %v", err)
		}
		return
	}
	for _, o := range owners {
		digest, err := r.DB.AccountDigest(ctx, o.ID)
		if err != nil {
			log.Printf("weekly digest: digest for owner %s failed: %v", o.ID, err)
			continue
		}
		subject, body := digestEmailBody(o, digest, r.BaseURL)
		if err = r.Mailer.Send(ctx, o.Email, subject, body); err != nil {
			log.Printf("weekly digest: send to owner %s failed: %v", o.ID, err)
			continue
		}
		if err = r.DB.MarkDigestSent(ctx, o.ID); err != nil {
			log.Printf("weekly digest: mark sent for owner %s failed: %v", o.ID, err)
		}
	}
}

func digestEmailBody(o database.Owner, digest database.AccountDigest, baseURL string) (string, string) {
	var b strings.Builder
	name := strings.TrimSpace(o.Name)
	if name == "" {
		name = o.Email
	}
	fmt.Fprintf(&b, "Hi %s,\n\nYour weekly Allagio digest.\n\n", name)
	fmt.Fprintf(&b, "Live now: %d item(s)\nClicks: %d\n", digest.WentLive, digest.Clicks)
	writeAmounts(&b, "Publisher earnings", digest.PublisherEarnings)
	writeAmounts(&b, "Maker earnings", digest.MakerEarnings)
	for _, p := range digest.Projects {
		fmt.Fprintf(&b, "\n%s %s — %d live, %d clicks", projectKindLabel(p.Kind), p.Name, p.WentLive, p.Clicks)
		if len(p.Earnings) > 0 {
			var parts []string
			for _, e := range p.Earnings {
				parts = append(parts, fmt.Sprintf("%.2f %s", e.Amount, e.Currency))
			}
			fmt.Fprintf(&b, ", %s", strings.Join(parts, ", "))
		}
		b.WriteString("\n")
	}
	base := strings.TrimRight(baseURL, "/")
	fmt.Fprintf(&b, "\nOpen your workspace: %s/owner\n", base)
	fmt.Fprintf(&b, "\nEarnings figures reflect test-mode payments only. You are getting this because an agent registered an account for this address. Turn the digest off any time at %s/owner/settings.\n", base)
	return "Your Allagio weekly digest", b.String()
}

func projectKindLabel(kind string) string {
	switch kind {
	case "site":
		return "Site"
	case "offer":
		return "Offer"
	default:
		return kind
	}
}

func writeAmounts(b *strings.Builder, label string, amounts []database.MoneyAmount) {
	if len(amounts) == 0 {
		return
	}
	var parts []string
	for _, e := range amounts {
		parts = append(parts, fmt.Sprintf("%.2f %s", e.Amount, e.Currency))
	}
	fmt.Fprintf(b, "%s: %s\n", label, strings.Join(parts, ", "))
}
