package entry

import (
	"testing"
	"time"
)

func TestPluginState(t *testing.T) {
	now := time.Date(2026, 10, 10, 4, 0, 0, 0, time.UTC)
	window := 10 * time.Minute
	fresh := now.Add(-time.Minute)
	stale := now.Add(-11 * time.Minute)
	future := now.Add(time.Minute)

	cases := []struct {
		name    string
		wait    string
		waitAt  *time.Time
		contact *time.Time
		want    string
	}{
		{name: "no contact", want: "offline"},
		{name: "fresh undo", wait: "remove", waitAt: &fresh, want: "pending"},
		{name: "stale undo", wait: "remove", waitAt: &stale, want: "offline"},
		{name: "stale undo with a fresh poll", wait: "remove", waitAt: &stale, contact: &fresh, want: "pending"},
		{name: "acked removal stays removed", wait: "removed", waitAt: &stale, contact: &stale, want: "removed"},
		{name: "fresh restore", wait: "restore", waitAt: &fresh, want: "pending"},
		{name: "acked restore", contact: &fresh, want: "current"},
		{name: "silent after restore", contact: &stale, want: "offline"},
		{name: "future contact is not a check-in", contact: &future, want: "offline"},
		{name: "future wait is not pending", wait: "remove", waitAt: &future, want: "offline"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PluginState(tc.wait, tc.waitAt, tc.contact, now, window); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
