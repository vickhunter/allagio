package handlers

import (
	"context"
	"log"
	"strings"
	"time"

	"affilio-api/internal/services"
)

const (
	ownerLoginRetryMin = time.Second
	ownerLoginRetryMax = 30 * time.Second
)

// DiscoverOwnerLogin retries issuer discovery until it succeeds or ctx ends.
// A failure is logged, without the client secret, and does not stop the process.
// Sign-in stays unavailable until SetOwnerLogin runs.
func DiscoverOwnerLogin(ctx context.Context, h *HostedHandler, secret string, discover func() (services.OwnerLogin, error)) {
	discoverOwnerLogin(ctx, h, secret, discover, waitOwnerLoginBackoff)
}

func discoverOwnerLogin(ctx context.Context, h *HostedHandler, secret string, discover func() (services.OwnerLogin, error), wait func(context.Context, time.Duration) bool) {
	delay := ownerLoginRetryMin
	for {
		if ctx.Err() != nil {
			return
		}
		login, err := discover()
		if err == nil {
			h.SetOwnerLogin(login)
			return
		}
		log.Printf("owner sign-in discovery failed: %s", redactSecret(err.Error(), secret))
		if !wait(ctx, delay) {
			return
		}
		if delay < ownerLoginRetryMax/2 {
			delay *= 2
		} else {
			delay = ownerLoginRetryMax
		}
	}
}

func waitOwnerLoginBackoff(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func redactSecret(message, secret string) string {
	if secret == "" {
		return message
	}
	return strings.ReplaceAll(message, secret, "redacted")
}
