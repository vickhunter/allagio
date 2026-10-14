package handlers

import (
	"net/url"
	"strings"

	"affilio-api/internal/database"
)

func mergeAttributionSource(returnTo, explicit string) string {
	u, err := url.Parse(returnTo)
	if err != nil || u == nil {
		return returnTo
	}
	raw := strings.TrimSpace(explicit)
	if raw == "" {
		raw = u.Query().Get("source")
	}
	if raw == "" {
		return returnTo
	}
	q := u.Query()
	q.Set("source", database.NormalizeAttributionSource(raw))
	u.RawQuery = q.Encode()
	return u.RequestURI()
}

func attributionSourceFromReturn(returnTo string) string {
	u, err := url.Parse(returnTo)
	if err != nil || u == nil || u.Query().Get("source") == "" {
		return "direct"
	}
	return database.NormalizeAttributionSource(u.Query().Get("source"))
}
