package database

import "strings"

func NormalizeAttributionSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "chatgpt", "claude", "cursor", "grok", "x", "community", "direct", "cli", "mcp":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "direct"
	}
}
