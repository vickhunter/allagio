package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"affilio-api/internal/database"
	"affilio-api/internal/services"

	"github.com/gin-gonic/gin"
)

const internalFitReason = "strong contextual fit. Performance-adjusted (+4 from 9 historical outcomes)."

func TestGenerateContentRefusesReasonFallback(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, internalFitReason)

	rec := postGenerate(t, db, siteID, nil, map[string]interface{}{
		"recommendation_id": recID,
		"insertion_type":    "inline_mention",
		"site_language":     "it_IT",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 when there is no reader-facing copy, got %d body %s", rec.Code, rec.Body.String())
	}
	assertNoInternalCopy(t, rec.Body.String())
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if payload["error"] != "no reader-facing copy" {
		t.Fatalf("expected a clear error, got %#v", payload)
	}
}

func TestGenerateContentRejectsBlurbThatIsOnlyTheReason(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, internalFitReason)
	gen := scriptedGenerator(t, []string{internalFitReason})

	rec := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 when the model repeats the internal reason, got %d body %s", rec.Code, rec.Body.String())
	}
	assertNoInternalCopy(t, rec.Body.String())
}

func TestGenerateContentRejectsScoreMatchReasonInsteadOfStripping(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	reason := services.MatchCuratedPrograms("This article covers email marketing for small teams.", []services.CuratedProgramInput{{
		ID:   "prog-email",
		Name: "email marketing",
	}}, nil)
	if len(reason) != 1 {
		t.Fatalf("scoreMatch matches: %#v", reason)
	}
	tuned := services.ApplyRecommendationTuning(reason, []services.RecommendationTuneProfile{{
		ProgramID:  "prog-email",
		Multiplier: 1.05,
		SampleSize: 9,
	}})
	if len(tuned) != 1 || !strings.Contains(tuned[0].Reason, "Performance-adjusted (") {
		t.Fatalf("tuning reason: %#v", tuned)
	}
	siteID, recID := seedGenerateRecommendation(t, db, tuned[0].Reason)
	gen := scriptedGenerator(t, []string{
		reason[0].Reason,
		"The article discusses \u201cemail marketing\u201d, making it a for this recommendation.",
		"Teams can compare tools here. Performance-adjusted.",
		"Mailchimp helps teams schedule campaigns and see which emails get replies.",
	})

	for _, blurb := range []string{
		reason[0].Reason,
		"The article discusses \u201cemail marketing\u201d, making it a for this recommendation.",
		"Teams can compare tools here. Performance-adjusted.",
	} {
		rec := postGenerate(t, db, siteID, gen, map[string]interface{}{
			"recommendation_id": recID,
			"site_language":     "en",
			"regenerate":        true,
		})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("blurb %q: expected 422, got %d body %s", blurb, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if strings.Contains(body, "making it a for") || strings.Contains(body, "email marketing") || strings.Contains(strings.ToLower(body), "performance-adjusted") {
			t.Fatalf("blurb %q published internal copy: %s", blurb, body)
		}
		if !strings.Contains(body, "no reader-facing copy") {
			t.Fatalf("blurb %q missing the error: %s", blurb, body)
		}
	}

	clean := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
		"regenerate":        true,
	})
	if clean.Code != http.StatusOK || !strings.Contains(clean.Body.String(), "Mailchimp helps teams schedule campaigns") {
		t.Fatalf("clean blurb rejected: %d %s", clean.Code, clean.Body.String())
	}
}

func TestGenerateContentRejectsHiddenReaderCopy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	reason := services.MatchCuratedPrograms("This article covers email marketing for small teams.", []services.CuratedProgramInput{{
		ID:   "prog-hidden",
		Name: "email marketing",
	}}, nil)
	if len(reason) != 1 {
		t.Fatalf("scoreMatch matches: %#v", reason)
	}
	siteID, recID := seedGenerateRecommendation(t, db, reason[0].Reason)
	blurbs := []string{
		"Readers can see a strong contex\u200Btual fit for this comparison of tools.",
		"Readers can see a strong contextual \ufb01t for this comparison of tools.",
		"Mailchimp helps teams schedule \u0441ampaigns and see which emails get replies.",
		"Mailchimp helps teams schedule campa\u3164igns and see which emails get replies.",
		"Readers can see a <em>contextual</em> fit for this comparison of email tools.",
		"Readers can see a contex&#116;ual fit for this comparison of email tools.",
		"Mailchimp helps teams schedule campaigns and see which emails get replies.",
	}
	gen := scriptedGenerator(t, blurbs)
	for i, blurb := range blurbs {
		rec := postGenerate(t, db, siteID, gen, map[string]interface{}{
			"recommendation_id": recID,
			"site_language":     "en",
			"regenerate":        true,
		})
		want := http.StatusUnprocessableEntity
		if i == len(blurbs)-1 {
			want = http.StatusOK
		}
		if rec.Code != want {
			t.Fatalf("blurb %q: got %d body %s", blurb, rec.Code, rec.Body.String())
		}
		if want == http.StatusUnprocessableEntity && !strings.Contains(rec.Body.String(), "no reader-facing copy") {
			t.Fatalf("blurb %q missing the error: %s", blurb, rec.Body.String())
		}
	}
}

func TestGenerateContentRejectsUnsafeProgramNameAndLink(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	reason := services.MatchCuratedPrograms("This article covers email marketing for small teams.", []services.CuratedProgramInput{{
		ID:   "prog-name",
		Name: "email marketing",
	}}, nil)
	if len(reason) != 1 {
		t.Fatalf("scoreMatch matches: %#v", reason)
	}
	siteID, recID := seedGenerateRecommendation(t, db, reason[0].Reason)
	gen := scriptedGenerator(t, []string{
		"Mailchimp helps teams schedule campaigns and see which emails get replies.",
	})
	setProgram := func(name, link string) {
		t.Helper()
		if _, err := db.Pool.Exec(context.Background(), `UPDATE affiliate_programs SET name=$1, signup_url=$2 WHERE slug='semrush-reader-copy'`, name, link); err != nil {
			t.Fatal(err)
		}
	}
	expect := func(name, link string, want int) {
		t.Helper()
		setProgram(name, link)
		rec := postGenerate(t, db, siteID, gen, map[string]interface{}{
			"recommendation_id": recID,
			"site_language":     "en",
			"regenerate":        true,
		})
		if rec.Code != want {
			t.Fatalf("name %q link %q: got %d body %s", name, link, rec.Code, rec.Body.String())
		}
		if want == http.StatusUnprocessableEntity && !strings.Contains(rec.Body.String(), "no reader-facing copy") {
			t.Fatalf("name %q link %q missing the error: %s", name, link, rec.Body.String())
		}
	}
	expect("Semrush contex\u034Ftual Program", "https://example.com/aff", http.StatusUnprocessableEntity)
	expect("A strong contextual fit for readers", "https://example.com/aff", http.StatusUnprocessableEntity)
	expect("Semrush Affiliate Program", "https://example.com/aff\u200Atrack", http.StatusUnprocessableEntity)
	expect("Semrush Affiliate Program", "https://example.com/the-article-discusses-email-marketing-making", http.StatusOK)
	expect("Cœur für Straße", "https://example.com/aff", http.StatusOK)
}

func TestGenerateContentRegeneratesUnsafeStoredAnchor(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, internalFitReason)
	gen := scriptedGenerator(t, []string{
		"Mailchimp helps teams schedule campaigns and see which emails get replies.",
		"Later copy for readers who compare a few tools before they buy.",
	})
	first := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
	})
	if first.Code != http.StatusOK {
		t.Fatalf("preview generate failed: %d %s", first.Code, first.Body.String())
	}
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT generated_html FROM recommendations WHERE id=$1`, recID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	poisoned := strings.Replace(stored, "Semrush Affiliate Program", "Semrush i\u0307 Program", 1)
	if poisoned == stored {
		t.Fatalf("stored preview had no anchor to poison: %s", stored)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE recommendations SET generated_html=$2 WHERE id=$1`, recID, poisoned); err != nil {
		t.Fatal(err)
	}
	second := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
	})
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), "Later copy for readers") {
		t.Fatalf("unsafe preview was not regenerated: %d %s", second.Code, second.Body.String())
	}
	if strings.Contains(second.Body.String(), "i\u0307") {
		t.Fatalf("published the stored combining dot: %s", second.Body.String())
	}
}

func TestGenerateContentRegeneratesPoisonedStoredHTML(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	reason := services.MatchCuratedPrograms("This article covers email marketing for small teams.", []services.CuratedProgramInput{{
		ID:   "prog-stored",
		Name: "email marketing",
	}}, nil)
	if len(reason) != 1 {
		t.Fatalf("scoreMatch matches: %#v", reason)
	}
	siteID, recID := seedGenerateRecommendation(t, db, reason[0].Reason)
	const clean = "Mailchimp helps teams schedule campaigns and see which emails get replies."
	const later = "Later copy for readers who compare a few tools before they buy."
	gen := scriptedGenerator(t, []string{clean, later})
	first := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
	})
	if first.Code != http.StatusOK {
		t.Fatalf("preview generate failed: %d %s", first.Code, first.Body.String())
	}
	poisons := []string{
		"Readers can see a strong contex&#116;ual fit for this comparison of tools.",
		"Readers can see a strong contextual&#32;fit for this comparison of tools.",
		"Readers can see a strong contextual&#x20;fit for this comparison of tools.",
		"Readers can see a strong contextual&#45;fit for this comparison of tools.",
		"Readers can see a strong contextual&nbsp; fit for this comparison of tools.",
		"Readers can see a strong contex<em>tual fit</em> for this comparison of tools.",
		"Readers can see a strong contex<span></span>tual fit for this comparison of tools.",
		"Readers can see a strong contextual<b></b>fit for this comparison of tools.",
		"Readers can see a strong contextual<!--x-->fit for this comparison of tools.",
		"the <em>article</em> discusses email marketing",
	}
	for _, poison := range poisons {
		storePoisonedPreview(t, db, recID, poison, clean)
		second := postGenerate(t, db, siteID, gen, map[string]interface{}{
			"recommendation_id": recID,
			"site_language":     "en",
		})
		if second.Code != http.StatusOK {
			t.Fatalf("poison %q: got %d %s", poison, second.Code, second.Body.String())
		}
		var payload map[string]interface{}
		if err := json.Unmarshal(second.Body.Bytes(), &payload); err != nil {
			t.Fatalf("poison %q: decode %v", poison, err)
		}
		htmlOut, _ := payload["html"].(string)
		textOut, _ := payload["generated_text"].(string)
		if textOut != later || !strings.Contains(htmlOut, later) {
			t.Fatalf("poison %q was reused or inserted: text %q html %s", poison, textOut, htmlOut)
		}
		if strings.Contains(htmlOut, poison) || strings.Contains(htmlOut, "contextual fit") || strings.Contains(strings.ToLower(htmlOut), "article discusses email marketing") {
			t.Fatalf("poison %q was inserted: %s", poison, htmlOut)
		}
	}
}

func TestGenerateContentReusesCleanAmpersandAndHyphenatedSlug(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, internalFitReason)
	const link = "https://example.com/the-article-discusses-email-marketing-making"
	if _, err := db.Pool.Exec(context.Background(), `UPDATE affiliate_programs SET name=$1, signup_url=$2 WHERE slug='semrush-reader-copy'`, "H&M", link); err != nil {
		t.Fatal(err)
	}
	const firstBlurb = "Mailchimp helps teams schedule campaigns and see which emails get replies."
	const secondBlurb = "Later copy for readers who compare a few tools before they buy."
	gen := scriptedGenerator(t, []string{firstBlurb, secondBlurb})
	preview := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
	})
	if preview.Code != http.StatusOK {
		t.Fatalf("preview generate failed: %d %s", preview.Code, preview.Body.String())
	}
	reused := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
	})
	if reused.Code != http.StatusOK {
		t.Fatalf("clean preview rejected: %d %s", reused.Code, reused.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(reused.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	htmlOut, _ := payload["html"].(string)
	textOut, _ := payload["generated_text"].(string)
	if textOut != firstBlurb || strings.Contains(textOut, secondBlurb) {
		t.Fatalf("clean preview was not reused: %s", reused.Body.String())
	}
	if !strings.Contains(htmlOut, "H&amp;M") || !strings.Contains(htmlOut, link) || !strings.Contains(htmlOut, firstBlurb) {
		t.Fatalf("reused html lost the escaped name or slug: %s", htmlOut)
	}
}

func storePoisonedPreview(t *testing.T, db *database.DB, recID, poison, clean string) {
	t.Helper()
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT generated_html FROM recommendations WHERE id=$1`, recID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var preview storedReaderPreview
	if err := json.Unmarshal([]byte(stored), &preview); err != nil {
		t.Fatal(err)
	}
	const sep = " — "
	idx := strings.LastIndex(preview.HTML, sep)
	if idx < 0 || !strings.HasSuffix(preview.HTML, "</p>") {
		t.Fatalf("stored html has no blurb to poison: %s", preview.HTML)
	}
	preview.HTML = preview.HTML[:idx+len(sep)] + poison + "</p>"
	preview.Text = clean
	if strings.Contains(preview.Text, poison) {
		t.Fatalf("text field was not kept clean: %s", preview.Text)
	}
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE recommendations SET generated_html=$2 WHERE id=$1`, recID, string(encoded)); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateContentRejectsOverlapAcrossAnchorAndBlurb(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	reason := services.MatchCuratedPrograms("This article covers email marketing for small teams.", []services.CuratedProgramInput{{
		ID:   "prog-boundary",
		Name: "email marketing",
	}}, nil)
	if len(reason) != 1 || !strings.Contains(reason[0].Reason, "email marketing") || !strings.Contains(reason[0].Reason, "contextual fit") {
		t.Fatalf("scoreMatch reason: %#v", reason)
	}
	siteID, recID := seedGenerateRecommendation(t, db, reason[0].Reason)
	cases := []struct {
		name  string
		blurb string
	}{
		{"email marketing", "making it a useful comparison for readers who want a tool."},
		{"contextual", "fit for readers comparing a few tools today."},
	}
	for _, tc := range cases {
		if _, err := db.Pool.Exec(context.Background(), `UPDATE affiliate_programs SET name=$1, signup_url=$2 WHERE slug='semrush-reader-copy'`, tc.name, "https://example.com/aff"); err != nil {
			t.Fatal(err)
		}
		rec := postGenerate(t, db, siteID, scriptedGenerator(t, []string{tc.blurb}), map[string]interface{}{
			"recommendation_id": recID,
			"site_language":     "en",
			"regenerate":        true,
		})
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "no reader-facing copy") {
			t.Fatalf("name %q blurb %q: got %d %s", tc.name, tc.blurb, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), tc.blurb) {
			t.Fatalf("name %q published the blurb: %s", tc.name, rec.Body.String())
		}
	}
}

func TestGenerateContentRegeneratesPoisonedPlacementReason(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, internalFitReason)
	const clean = "Mailchimp helps teams schedule campaigns and see which emails get replies."
	const later = "Later copy for readers who compare a few tools before they buy."
	const poisonedReason = "This is a strong contextual fit for the reader."
	gen := scriptedGenerator(t, []string{clean, later})
	first := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
	})
	if first.Code != http.StatusOK {
		t.Fatalf("preview generate failed: %d %s", first.Code, first.Body.String())
	}
	var stored string
	if err := db.Pool.QueryRow(context.Background(), `SELECT generated_html FROM recommendations WHERE id=$1`, recID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var preview storedReaderPreview
	if err := json.Unmarshal([]byte(stored), &preview); err != nil {
		t.Fatal(err)
	}
	preview.PlacementReason = poisonedReason
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(context.Background(), `UPDATE recommendations SET generated_html=$2 WHERE id=$1`, recID, string(encoded)); err != nil {
		t.Fatal(err)
	}
	second := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
	})
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), later) {
		t.Fatalf("poisoned placement reason was not regenerated: %d %s", second.Code, second.Body.String())
	}
	if strings.Contains(strings.ToLower(second.Body.String()), "contextual fit") {
		t.Fatalf("published the stored placement reason: %s", second.Body.String())
	}
}

func TestGenerateContentReusesPreviewForTheSameLanguage(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	siteID, recID := seedGenerateRecommendation(t, db, internalFitReason)
	gen := scriptedGenerator(t, []string{
		"Prima frase per i lettori che confrontano strumenti SEO.",
		"Seconda frase diversa che non deve sostituire l'anteprima.",
	})

	first := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "it-IT",
	})
	if first.Code != http.StatusOK {
		t.Fatalf("preview generate failed: %d %s", first.Code, first.Body.String())
	}
	if !strings.Contains(first.Body.String(), "Prima frase per i lettori") {
		t.Fatalf("preview missing the first sentence: %s", first.Body.String())
	}

	second := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "it_IT",
	})
	if second.Code != http.StatusOK {
		t.Fatalf("confirm generate failed: %d %s", second.Code, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), "Prima frase per i lettori") {
		t.Fatalf("confirm did not reuse the previewed copy: %s", second.Body.String())
	}
	if strings.Contains(second.Body.String(), "Seconda frase diversa") {
		t.Fatalf("confirm regenerated different text: %s", second.Body.String())
	}
	assertNoInternalCopy(t, second.Body.String())

	third := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "it_IT",
		"regenerate":        true,
	})
	if third.Code != http.StatusOK || !strings.Contains(third.Body.String(), "Seconda frase diversa") {
		t.Fatalf("explicit regenerate did not replace the stored preview: %d %s", third.Code, third.Body.String())
	}

	fourth := postGenerate(t, db, siteID, gen, map[string]interface{}{
		"recommendation_id": recID,
		"site_language":     "en",
	})
	if fourth.Code == http.StatusOK && strings.Contains(fourth.Body.String(), "Seconda frase diversa") && !strings.Contains(fourth.Body.String(), "Prima frase") {
		return
	}
	// A different language must not silently reuse the Italian preview.
	if strings.Contains(fourth.Body.String(), "Prima frase per i lettori") && !strings.Contains(fourth.Body.String(), "Seconda frase diversa") {
		t.Fatalf("english confirm reused the italian preview: %s", fourth.Body.String())
	}
}

func seedGenerateRecommendation(t *testing.T, db *database.DB, reason string) (string, string) {
	t.Helper()
	siteID := createTestSite(t, db, "test-key", "example.com", "Example Site")
	articleID, err := db.UpsertArticle(context.Background(), siteID, 1, "Test", "https://example.com/post", "hash")
	if err != nil {
		t.Fatalf("create article: %v", err)
	}
	programID := insertProgram(t, db, "Semrush Affiliate Program", "Semrush", "semrush-reader-copy")
	if err := db.CreateRecommendation(context.Background(), articleID, &programID, nil, 80, reason, "inline_mention", "end_of_article"); err != nil {
		t.Fatalf("create recommendation: %v", err)
	}
	return siteID, lookupRecommendationID(t, db, articleID)
}

func postGenerate(t *testing.T, db *database.DB, siteID string, gen *services.AIContentGenerator, payload map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	handler := &GenerateHandler{DB: db, Generator: gen}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/generate/content", func(c *gin.Context) {
		setSiteID(c, siteID)
		handler.GenerateContent(c)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/generate/content", newJSONRequest(t, http.MethodPost, "/generate/content", payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

func scriptedGenerator(t *testing.T, blurbs []string) *services.AIContentGenerator {
	t.Helper()
	call := 0
	return &services.AIContentGenerator{
		APIKey: "test-key",
		Model:  "test-model",
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				blurb := blurbs[len(blurbs)-1]
				if call < len(blurbs) {
					blurb = blurbs[call]
				}
				call++
				inner, err := json.Marshal(map[string]string{
					"blurb":              blurb,
					"suggested_position": "end_of_article",
				})
				if err != nil {
					t.Fatalf("marshal blurb: %v", err)
				}
				outer, err := json.Marshal(map[string]interface{}{
					"content": []map[string]string{{
						"type": "text",
						"text": string(inner),
					}},
				})
				if err != nil {
					t.Fatalf("marshal claude body: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(string(outer))),
				}, nil
			}),
		},
	}
}

func assertNoInternalCopy(t *testing.T, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, needle := range []string{"strong contextual fit", "performance-adjusted", "historical outcomes"} {
		if strings.Contains(lower, needle) {
			t.Fatalf("response published internal scoring %q: %s", needle, body)
		}
	}
}
