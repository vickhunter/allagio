package database

import "testing"

func TestRefundProrationDoesNotOverflow(t *testing.T) {
	const largestMinor = 9999999999
	if got := proratedMinor(largestMinor, largestMinor, largestMinor); got != largestMinor {
		t.Fatalf("full refund overflow: %d", got)
	}
	if got := proratedMinor(980, 2450, 4900); got != 490 {
		t.Fatalf("half refund: %d", got)
	}
}
