package entry

import (
	"testing"
	"time"
)

func TestPluginState(t *testing.T) {
	now := time.Date(2026, 10, 10, 4, 0, 0, 0, time.UTC)
	window := 10 * time.Minute
	fresh := now.Add(-time.Minute)
	exact := now.Add(-window)
	stale := now.Add(-11 * time.Minute)
	future := now.Add(time.Minute)

	cases := []struct {
		name   string
		wait   string
		waitAt *time.Time
		want   string
	}{
		{name: "brand new", want: "pending"},
		{name: "fresh undo", wait: "remove", waitAt: &fresh, want: "pending"},
		{name: "undo at the window edge", wait: "remove", waitAt: &exact, want: "pending"},
		{name: "undo after the window", wait: "remove", waitAt: &stale, want: "offline"},
		{name: "undo with no timestamp", wait: "remove", want: "pending"},
		{name: "future undo stays pending", wait: "remove", waitAt: &future, want: "pending"},
		{name: "acked removal", wait: "removed", waitAt: &stale, want: "removed"},
		{name: "restore waits for the plugin", wait: "restore", waitAt: &stale, want: "pending"},
		{name: "acked placement", wait: "placed", waitAt: &stale, want: "placed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PluginState(tc.wait, tc.waitAt, now, window); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestPluginStateNeverEmpty(t *testing.T) {
	now := time.Date(2026, 10, 10, 4, 0, 0, 0, time.UTC)
	allowed := map[string]bool{}
	for _, state := range PluginStates {
		if state == "" {
			t.Fatal("plugin state list contains an empty value")
		}
		allowed[state] = true
	}
	if len(allowed) != 4 {
		t.Fatalf("plugin states: %v", PluginStates)
	}
	stamps := []*time.Time{nil, ptrTime(now.Add(-time.Hour)), ptrTime(now.Add(time.Hour))}
	for _, wait := range []string{"", "remove", "restore", "removed", "placed", "unknown"} {
		for _, at := range stamps {
			got := PluginState(wait, at, now, 10*time.Minute)
			if !allowed[got] {
				t.Fatalf("wait %q produced %q", wait, got)
			}
		}
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
