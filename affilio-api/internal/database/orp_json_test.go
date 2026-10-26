package database

import (
	"encoding/json"
	"testing"
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
