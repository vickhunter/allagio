package entry

import "time"

// PluginStates are the only values plugin_state may take. The field is never
// null and never omitted on a placement.
var PluginStates = []string{"pending", "placed", "removed", "offline", "failed"}

// VerifiedPluginWait is the only value that may be stored as placed or removed.
// The site key's plugin-ack is the only caller. Revoke, restore, retry, pause,
// and the kill switch do not use it.
func VerifiedPluginWait(action string) (string, bool) {
	switch action {
	case "restored":
		return "placed", true
	case "removed":
		return "removed", true
	default:
		return "", false
	}
}

// PluginState reports the plugin's view of one placement.
// A brand-new placement, a restore, and a removal stay pending until the site
// key acknowledges them. Any of those with no ack after the window is offline.
// Callers pass COALESCE(plugin_wait_at, created_at), so a nil timestamp is the
// only pending value that has not started the window. failed beats the window.
// placed and removed still win: a later verified ack clears the failure.
func PluginState(wait string, waitAt *time.Time, failed bool, now time.Time, window time.Duration) string {
	switch wait {
	case "placed":
		return "placed"
	case "removed":
		return "removed"
	}
	if failed && (wait == "" || wait == "restore" || wait == "remove") {
		return "failed"
	}
	if waitAt != nil && !waitAt.UTC().After(now.UTC()) && now.UTC().Sub(waitAt.UTC()) > window {
		return "offline"
	}
	return "pending"
}

// PluginRetryDoc is the retry section for /llms.txt and the mode docs.
func PluginRetryDoc() string {
	return "## Plugin retry\n\n" +
		"`POST /v1/placements/:id/retry` repeats only the action that failed. The site key's `failed` acknowledgement stores `failed_action` as `removed` or `restored`. A session or a site key for that placement may call retry. The other side receives 403. Any state other than `failed` returns 409 `not_failed`.\n\n" +
		"A failed removal re-queues only the removal. `plugin_state` becomes `pending`, then `removed` on `action: removed`. It does not re-run the hard checks, it does not become `placed`, and it does not reactivate the go link.\n\n" +
		"A failed restore re-runs every hard check. A failure returns 422 with `check_id` and `check_text` and leaves the placement failed. Success re-queues the restore: `plugin_state` becomes `pending`, then `placed` on `action: restored`.\n\n" +
		"A failed restore is not live. `failed_action` `restored` leaves `GET /v1/go/<placement_id>` untracked: it redirects to the offer's plain URL and records no click. Digest totals, site totals, and Live now exclude it, including the went-live count and earnings attributed to its clicks. Retry leaves it untracked. The link records a hit and the counts include it again only after `action: restored` confirms `placed`.\n\n" +
		"`GET /v1/go/<placement_id>` always returns 302. A ref is attached and a hit is recorded only when `placement_counts_as_live` is true. Revoked, pending undo, paused (`all`, `site`, or `offer`), kill switch, offline, failed, and removed placements redirect to the offer's plain URL with no ref and no hit. When the offer is not live, the redirect is the placement `post_url`, or the site home when `post_url` is missing. An unknown placement id redirects to the site home when a matching site key calls `?via=plugin`, and otherwise to the Offer Rails home page. Every go-link response sends `Cache-Control: no-store` and `X-Robots-Tag: noindex, nofollow`. The placement payload includes `fallback_url`, the offer's plain URL.\n\n" +
		"`GET /v1/go/<code>` stays supported. Existing posts, feeds, and caches keep that link, and nothing rewrites a saved post. It never returns a 4xx or a 5xx. A code with `?placement=` follows the placement rules. Without that query, a hit is recorded only when `placement_counts_as_live` matches a placement on the campaign's site. Every other resolved code redirects to the offer's plain URL with no ref and no hit. When the offer is not live, that redirect is the site home. An unknown code redirects to the Offer Rails home page. A rate limit answers with the same untracked 302 and never a 429.\n\n" +
		"Reader identity is a hash, never a raw IP address. Truncate first. IPv4 keeps the /24: zero the last octet and write dotted decimal with no leading zeros (`203.0.113.60` becomes `203.0.113.0`). IPv6 keeps the /48 and is written in lowercase RFC 5952 form (`2001:db8:abcd:1234::1` becomes `2001:db8:abcd::`). An IPv4-mapped IPv6 address is truncated as IPv4. The hash is the lowercase hex HMAC-SHA256 of the truncated address, keyed by the daily salt, 64 hex characters.\n\n" +
		"The plugin salt is per site and rotates at 00:00 UTC. Derive it as the lowercase hex SHA-256 of `siteSecret + \"|go-link|\" + YYYY-MM-DD` for that UTC date, then send the client hash in `X-OR-Client-Hash` with `X-OR-User-Agent` and `X-OR-Referer` on `GET /v1/go/<placement_id>?via=plugin`. The server stores that hash and does not see the address or the site salt. Dedup and bot filtering use the hash plus the user agent, scoped to that site. Those headers are ignored without a site key for that placement. Do not send `X-OR-Client-IP`.\n\n" +
		"A direct request is truncated and hashed in memory with the server salt. That salt is the lowercase hex SHA-256 of `API_KEY_SECRET + \"|go-link|\" + YYYY-MM-DD` in UTC, so it also changes at 00:00 UTC. The server refuses to start when `API_KEY_SECRET` is unset or `dev-secret`. The raw address is not stored and is scrubbed from request logs, access logs, and error reports. Click `ip_hash`, `user_agent`, and `referrer` are cleared 30 days after `clicked_at`.\n\n" +
		"A placement counts as live only when `plugin_state` is `placed` and its go link is redirecting. `placed` comes from a verified plugin acknowledgement. The go link is redirecting when the placement is still committed, the offer is listed, the campaign is active, and Pause and the kill switch are off for that site and that offer. Pending, failed, offline, removed, revoked, and paused placements are out of Live now, site totals, digest went-live, and account totals. That rule is the `placement_counts_as_live` view.\n\n" +
		"Leave a failed restore removed with `POST /v1/placements/:id/revoke` and `{\"reason\":\"undo\"}` while `plugin_state` is `failed` and `failed_action` is `restored`. That request sets `plugin_state` to `pending`, clears `failed_action`, and stops the go link. The placement does not count as live. It becomes `removed` only when the site key acknowledges `action: removed`. With no acknowledgement it becomes `offline` after 10 minutes. `action: failed` there sets `failed` with `failed_action` `removed`. `placed` and `removed` are stored only by that verified plugin acknowledgement. Revoke, restore, retry, pause, and the kill switch do not set them.\n"
}
