package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDisclosureMatcherUsesTheSharedPhraseList(t *testing.T) {
	approved := "Affiliate disclosure: I may earn a commission if you buy through this link."
	var phrases struct {
		Positives []string `json:"positives"`
		Negatives []string `json:"negatives"`
	}
	raw, err := os.ReadFile(disclosureFixturePath(t))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &phrases); err != nil {
		t.Fatal(err)
	}
	if len(phrases.Positives) == 0 || len(phrases.Negatives) == 0 {
		t.Fatal("shared disclosure fixture is empty")
	}
	for _, phrase := range phrases.Positives {
		if !publishedDisclosureOK("<p><em>"+phrase+"</em></p>", approved) {
			t.Fatalf("positive phrase was not a disclosure: %q", phrase)
		}
	}
	for _, phrase := range phrases.Negatives {
		if publishedDisclosureOK("<p>"+phrase+"</p>", approved) {
			t.Fatalf("negative phrase counted as a disclosure: %q", phrase)
		}
	}
}

func disclosureFixturePath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the disclosure matcher test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "affilio-wp", "tests", "fixtures", "disclosure-match.json"))
}
