package database

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDialWithFallbackBoundsIPv6(t *testing.T) {
	var firstDeadline time.Duration
	var networks []string
	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		networks = append(networks, network)
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if network == "tcp6" {
				firstDeadline = remaining
			}
		}
		if network == "tcp6" {
			return nil, context.DeadlineExceeded
		}
		return nil, errors.New("ipv4 reached")
	}
	parent, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := dialWithFallback(parent, dial, "db.internal:5432", 2*time.Second)
	if err == nil || err.Error() != "ipv4 reached" {
		t.Fatalf("fallback error = %v", err)
	}
	if len(networks) != 2 || networks[0] != "tcp6" || networks[1] != "tcp" {
		t.Fatalf("dial order = %v", networks)
	}
	if firstDeadline < 1500*time.Millisecond || firstDeadline > 2500*time.Millisecond {
		t.Fatalf("ipv6 deadline remaining = %s, want about 2s", firstDeadline)
	}
}
