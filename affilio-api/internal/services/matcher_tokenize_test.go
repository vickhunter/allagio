package services

import "strings"
import "testing"

func TestTokenizeDropsItalianStopwordsAndNbsp(t *testing.T) {
	got := tokenize("Vedi&nbsp;il&nbsp;prodotto per la email inbox")
	joined := " " + strings.Join(got, " ") + " "
	for _, banned := range []string{"il", "la", "per", "nbsp", "lo", "gli", "una", "che", "non", "come", "del", "della"} {
		if strings.Contains(joined, " "+banned+" ") {
			t.Fatalf("tokenize kept %q in %v", banned, got)
		}
	}
	for _, want := range []string{"vedi", "prodotto", "email", "inbox"} {
		if !strings.Contains(joined, " "+want+" ") {
			t.Fatalf("tokenize dropped %q from %v", want, got)
		}
	}
	raw := tokenize("prezzo\u00a0visibile")
	rawJoined := " " + strings.Join(raw, " ") + " "
	if strings.Contains(rawJoined, " nbsp ") || !strings.Contains(rawJoined, " prezzo ") || !strings.Contains(rawJoined, " visibile ") {
		t.Fatalf("U+00A0 was not treated as whitespace: %v", raw)
	}
}
