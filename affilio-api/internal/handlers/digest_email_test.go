package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"affilio-api/internal/database"
)

type fakeMailer struct{ sent []string }

func (f *fakeMailer) Enabled() bool { return true }
func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	f.sent = append(f.sent, to+"|"+subject+"|"+body)
	return nil
}

func insertDigestOwner(t *testing.T, db *database.DB, email string, optOut bool, sentAt *time.Time) string {
	t.Helper()
	ctx := context.Background()
	var id string
	if err := db.Pool.QueryRow(ctx, `INSERT INTO owners(issuer,subject,email,name,source,digest_opt_out,digest_sent_at)
VALUES('issuer',$1,$2,$3,'direct',$4,$5) RETURNING id::text`,
		"subj-"+email, email, "Owner "+email, optOut, sentAt).Scan(&id); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	return id
}

func TestDigestEmailRunnerSendsOnlyDueOwners(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()

	due := insertDigestOwner(t, db, "due@example.com", false, nil)
	recent := time.Now().Add(-time.Hour)
	insertDigestOwner(t, db, "recent@example.com", false, &recent)
	stale := time.Now().Add(-8 * 24 * time.Hour)
	overdue := insertDigestOwner(t, db, "overdue@example.com", false, &stale)
	insertDigestOwner(t, db, "optout@example.com", true, nil)
	insertDigestOwner(t, db, "", false, nil)

	mailer := &fakeMailer{}
	r := &DigestEmailRunner{DB: db, Mailer: mailer, BaseURL: "https://allagio.dev"}
	r.runOnce(ctx)

	if len(mailer.sent) != 2 {
		t.Fatalf("expected 2 digest emails, got %d: %v", len(mailer.sent), mailer.sent)
	}
	got := map[string]bool{}
	for _, s := range mailer.sent {
		to := strings.SplitN(s, "|", 2)[0]
		got[to] = true
		if !strings.Contains(s, "/owner") || !strings.Contains(s, "weekly digest") {
			t.Fatalf("digest body missing workspace link or copy: %s", s)
		}
	}
	if !got["due@example.com"] || !got["overdue@example.com"] {
		t.Fatalf("unexpected recipients: %v", mailer.sent)
	}
	for _, id := range []string{due, overdue} {
		_, sentAt, err := db.OwnerDigestPrefs(ctx, id)
		if err != nil {
			t.Fatalf("prefs: %v", err)
		}
		if sentAt == nil {
			t.Fatalf("owner %s not marked sent", id)
		}
	}
}

func TestDigestEmailBodyListsProjectsAndTotals(t *testing.T) {
	digest := database.AccountDigest{
		WentLive:          3,
		Clicks:            42,
		PublisherEarnings: []database.MoneyAmount{{Currency: "EUR", Amount: 12.5}},
		Projects: []database.AccountProject{
			{Kind: "site", Name: "blog.example", WentLive: 2, Clicks: 30},
			{Kind: "offer", Name: "Widget", WentLive: 1, Clicks: 12, Earnings: []database.MoneyAmount{{Currency: "EUR", Amount: 5}}},
		},
	}
	subject, body := digestEmailBody(database.Owner{Email: "o@example.com", Name: "O"}, digest, "https://allagio.dev/")
	if !strings.Contains(subject, "digest") {
		t.Fatalf("subject: %s", subject)
	}
	for _, want := range []string{"Hi O", "3 item(s)", "42", "12.50 EUR", "Site blog.example", "Offer Widget", "5.00 EUR", "https://allagio.dev/owner"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}
