package database

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestJSONList(t *testing.T) {
	var nilSlice []any
	nilRaw, err := json.Marshal(nilSlice)
	if err != nil {
		t.Fatal(err)
	}
	emptyRaw, err := json.Marshal([]any{})
	if err != nil {
		t.Fatal(err)
	}
	mandate := json.RawMessage(`[{"party":"orp:127.0.0.1/a"}]`)
	cases := []struct {
		name string
		in   json.RawMessage
		want string
	}{
		{"nil raw", nil, "[]"},
		{"empty raw", json.RawMessage{}, "[]"},
		{"json null from a nil slice", nilRaw, "[]"},
		{"empty list", emptyRaw, "[]"},
		{"mandate list", mandate, string(mandate)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(JSONList(tc.in)); got != tc.want {
				t.Fatalf("JSONList(%q) = %s", tc.in, got)
			}
		})
	}
}

func TestNullJSONNullableColumns(t *testing.T) {
	// waiting and parties stay SQL NULL. terms, reply, and message are objects
	// and are not list-normalised. bound and obligations go through JSONList.
	cases := []struct {
		name    string
		in      json.RawMessage
		wantNil bool
		want    string
	}{
		{"nil waiting", nil, true, ""},
		{"null parties", json.RawMessage("null"), true, ""},
		{"waiting object", json.RawMessage(`{"party":"orp:127.0.0.1/a","step":"accept"}`), false, `{"party":"orp:127.0.0.1/a","step":"accept"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nullJSON(tc.in)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("got %#v", got)
				}
				return
			}
			raw, ok := got.(json.RawMessage)
			if !ok || string(raw) != tc.want {
				t.Fatalf("got %#v", got)
			}
		})
	}
}

func TestRepeatClassExactWindow(t *testing.T) {
	start := time.Date(2026, 10, 27, 12, 0, 0, int(100*time.Millisecond), time.UTC)
	click := func(at string) json.RawMessage {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"obligation": "place", "class": "human", "at": at})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	stored := click(start.Format(time.RFC3339Nano))
	if !strings.Contains(start.Format(time.RFC3339Nano), ".") {
		t.Fatal("fixture lost its fraction")
	}
	if got := repeatClass([]json.RawMessage{stored}, "place", "human", start.Add(4900*time.Millisecond)); got != "repeat" {
		t.Fatalf("4.9s %s", got)
	}
	if got := repeatClass([]json.RawMessage{stored}, "place", "human", start.Add(ORPRepeatWindow)); got != "human" {
		t.Fatalf("exact %s", got)
	}
	if got := repeatClass([]json.RawMessage{stored}, "place", "human", start.Add(-time.Second)); got != "human" {
		t.Fatalf("backward %s", got)
	}
	legacy := click("2026-10-27T12:00:00Z")
	whole := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	if got := repeatClass([]json.RawMessage{legacy}, "place", "human", whole.Add(4900*time.Millisecond)); got != "repeat" {
		t.Fatalf("legacy 4.9s %s", got)
	}
	if got := repeatClass([]json.RawMessage{legacy}, "place", "human", whole.Add(ORPRepeatWindow)); got != "human" {
		t.Fatalf("legacy exact %s", got)
	}
}
