package entry

import "testing"

func TestClickSourceUsesReferrerHost(t *testing.T) {
	if got := ClickSource(""); got != "direct" {
		t.Fatalf("empty: %s", got)
	}
	if got := ClickSource("not a url"); got != "direct" {
		t.Fatalf("plain: %s", got)
	}
	if got := ClickSource("https://News.Example/story"); got != "news.example" {
		t.Fatalf("host: %s", got)
	}
}
