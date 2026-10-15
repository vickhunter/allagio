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
