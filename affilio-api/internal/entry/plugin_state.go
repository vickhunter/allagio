package entry

import "time"

// PluginStates are the only values plugin_state may take. The field is never
// null and never omitted on a placement.
var PluginStates = []string{"pending", "placed", "removed", "offline", "failed"}

// VerifiedPluginWait is the only value that may be stored as placed or removed.
// The plugin key's plugin-ack is the only caller. Revoke, restore, retry, pause,
// and the kill switch do not use it.
func VerifiedPluginWait(action string) (string, bool) {
	switch action {
	case "placed", "restored":
		return "placed", true
	case "removed":
		return "removed", true
	default:
		return "", false
	}
}

// PluginAckNeedsCommit reports that this acknowledgement would record an insert
// or a re-insert. Those are allowed only after the placement is committed.
// A removal acknowledgement, including a failed removal, stays available after
// undo, when the placement is revoked and plugin_wait is remove.
func PluginAckNeedsCommit(action, wait, status string) bool {
	switch action {
	case "placed", "restored":
		return status != "committed"
	case "failed":
		if wait == "remove" {
			return false
		}
		if wait == "" || wait == "restore" {
			return status != "committed"
		}
	}
	return false
}

// PluginState reports the plugin's view of one placement.
// A brand-new placement, a restore, and a removal stay pending until the site
// key acknowledges them. Any of those with no ack after the window is offline.
// Callers pass plugin_wait_at. A nil timestamp has not started the window.
// failed beats the window.
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
		"`POST /v1/placements/:id/retry` repeats only the action that failed. The plugin key's `failed` acknowledgement stores `failed_action` as `removed` or `restored`. A session or a site key for that placement may call retry. The other side receives 403. Any state other than `failed` returns 409 `not_failed`.\n\n" +
		"A failed removal re-queues only the removal. `plugin_state` becomes `pending`, then `removed` on `action: removed`. It does not re-run the hard checks, it does not become `placed`, and it does not reactivate the go link.\n\n" +
		"A failed restore re-runs every hard check. A failure returns 422 with `check_id` and `check_text` and leaves the placement failed. Success re-queues the restore: `plugin_state` becomes `pending`, then `placed` on `action: restored`.\n\n" +
		"A failed restore is not live. `failed_action` `restored` leaves `GET /v1/go/<placement_id>` untracked: it redirects to the offer's plain URL and records no click. Digest totals, site totals, and Live now exclude it, including the went-live count and earnings attributed to its clicks. Retry leaves it untracked. The link records a hit and the counts include it again only after `action: placed` or `action: restored` confirms `placed`.\n\n" +
		"`GET /v1/go/<placement_id>` returns 302 when the placement exists. A ref is attached and a hit is recorded from the first redirect when the tracked link is valid, with or without plugin-ack, on any surface. `placement_counts_as_live` is one such placement. An `approved` placement records one too, while it is inside its window, the offer is available, and the site has an active campaign for that offer. `plugin_wait` does not have to be `placed`, and `campaign_id` may still be empty. The click uses that campaign. A `committed` placement records one the same way when `plugin_wait` is empty or `restore` and the restore has not failed. `POST /v1/placements/:id/plugin-ack` stays an optional verification signal: it sets `plugin_wait` to `placed` and does not gate the click. Revoked, pending undo, paused (`all`, `site`, or `offer`), kill switch, a failed restore, a pending removal, and removed placements redirect to the offer's plain URL with no ref and no hit. When the offer is not live, the redirect is the placement `post_url`, or the site home when `post_url` is missing. An unknown placement id returns a 404 page and does not redirect, so the request port stays. Undo and redo before the plugin acknowledges a removal leaves a box that is still showing in `placed`, and that go link keeps counting clicks. Every go-link response sends `Cache-Control: no-store` and `X-Robots-Tag: noindex, nofollow`. The placement payload includes `fallback_url`, the offer's plain URL.\n\n" +
		"`GET /v1/go/<code>` stays supported. Existing posts, feeds, and caches keep that link, and nothing rewrites a saved post. It never returns a 4xx or a 5xx. A code with `?placement=` follows the placement rules. Without that query, a hit is recorded only when exactly one placement on the campaign is clickable: it is in `placement_counts_as_live`, it is `approved` and still inside its window, or it is `committed` before plugin-ack, and it is not revoked or removed. Every other resolved code redirects to the offer's plain URL with no ref and no hit. When the offer is not live, that redirect is the site home. An unknown code redirects to the Allagio home page. A rate limit answers with the same untracked 302 and never a 429.\n\n" +
		"Reader identity is a hash, never a raw IP address. Truncate first. IPv4 keeps the /24: zero the last octet and write dotted decimal with no leading zeros (`203.0.113.60` becomes `203.0.113.0`). IPv6 keeps the /48 and is written in lowercase RFC 5952 form (`2001:db8:abcd:1234::1` becomes `2001:db8:abcd::`). An IPv4-mapped IPv6 address is truncated as IPv4. The hash is the lowercase hex HMAC-SHA256 of the truncated address, keyed by the daily salt, 64 hex characters.\n\n" +
		"The plugin salt is per site and rotates at 00:00 UTC. Derive it as the lowercase hex SHA-256 of `siteSecret + \"|go-link|\" + YYYY-MM-DD` for that UTC date, then send the client hash in `X-OR-Client-Hash` with `X-OR-User-Agent` and `X-OR-Referer` on `GET /v1/go/<placement_id>?via=plugin`. The server stores that hash and does not see the address or the site salt. Dedup and bot filtering use the hash plus the user agent, scoped to that site. Those headers are ignored without a site key for that placement. Do not send `X-OR-Client-IP`.\n\n" +
		"A direct request is truncated and hashed in memory with the server salt. That salt is the lowercase hex SHA-256 of `API_KEY_SECRET + \"|go-link|\" + YYYY-MM-DD` in UTC, so it also changes at 00:00 UTC. The server refuses to start when `API_KEY_SECRET` is unset or `dev-secret`. The raw address is not stored and is scrubbed from request logs, access logs, and error reports. Click `ip_hash`, `user_agent`, and `referrer` are cleared 30 days after `clicked_at`.\n\n" +
		"A placement counts as live only when `plugin_state` is `placed` and its go link is redirecting. `placed` comes from a verified plugin acknowledgement. The go link is redirecting when the placement is still committed, the offer is available, the campaign is active, and Pause and the kill switch are off for that site and that offer. Pending, failed, offline, removed, revoked, and paused placements are out of Live now, site totals, digest went-live, and account totals. That rule is the `placement_counts_as_live` view.\n\n" +
		"Leave a failed restore removed with `POST /v1/placements/:id/revoke` and `{\"reason\":\"undo\"}` while `plugin_state` is `failed` and `failed_action` is `restored`. That request sets `plugin_state` to `pending`, clears `failed_action`, and stops the go link. The placement does not count as live. It becomes `removed` only when the plugin key acknowledges `action: removed`. With no acknowledgement it becomes `offline` after 10 minutes. `action: failed` there sets `failed` with `failed_action` `removed`. `placed` and `removed` are stored only by that verified plugin acknowledgement. Revoke, restore, retry, pause, and the kill switch do not set them.\n\n" +
		"A committed placement does not expire. The 7-day window applies only to a placement that was prepared and never committed. The offline timer starts when the plugin is asked to act (`plugin_wait_at`), not when the placement is created. A null `plugin_wait_at` stays `pending`.\n\n" +
		"The plugin refuses a placement locally, before it sends a receipt, with `POST /v1/placements/:id/refuse` and the site `X-API-Key`. The body is `{\"reason\":\"...\"}` and `reason` is required. The placement returns to `pending` with `publication_status` `refused`, the same open state as a refused receipt. The tracked link stays when a receipt already exists. That row no longer counts as the box, so the next prepare on the post returns a new box and does not return 409 `one_per_post`. Calling refuse again returns 200 and says already refused. A missing id is 404. When the update changes no row, the response is the current state: a receipt that landed as publisher_reported stays committed and returns 409 `receipt_recorded`, and does not claim the placement was refused. A receipt that already reported publication stays `committed` and this route returns 409 `receipt_recorded`. A maker key is not accepted.\n\n" +
		"`POST /v1/placements/:id/plugin-ack` authenticates with `X-Plugin-Key`. `X-API-Key`, a project key, an account key, an agent key, a maker key, and a session receive 403. `action` is `placed`, `removed`, `restored`, or `failed`. The body always includes `rel_sponsored`, the boolean the plugin read from the post. `action: placed` and `action: restored` require `rel_sponsored: true`. Disclosure words do not satisfy it. A false value returns 422 `rel_sponsored` and does not set `placed`. `action: placed` is the first insert and the upgrade ack for a box that is already in the post. `action: restored` is the ack after the plugin puts the block back. `failed` also sends `reason`.\n\n" +
		"`placed`, `restored`, and a `failed` acknowledgement of an insert or a re-insert return 409 `not_committed` until the placement status is `committed`. Approval alone is not enough. The row is left unchanged. A removal acknowledgement (`removed`, or `failed` while the plugin was asked to remove) stays available after undo. `nothing_to_ack` is only for an action that is not waiting.\n\n" +
		"`POST /v1/placements/:id/receipt` uses the site `X-API-Key` after the placement is committed. The body fields are `surface_url`, `published_text`, and `evidence`. `surface_url` is the article URL. `published_text` is the published HTML and must include the approved copy, the disclosure, and the tracked link. `evidence` is the publisher's note that this copy is on the page. `post_title` and `post_id` are optional.\n\n" +
		"A receipt's published text may contain `/go/<placement_id>` or `/v1/go/<placement_id>`. The stored receipt link stays `/v1/go/<placement_id>`. A HEAD check of the offer URL treats 403 and 405 as up. A network error, 404, 410, or 5xx is down. A live `GET /v1/go/<placement_id>` still redirects to the offer URL with the ref and records one click when that probe is down. It does not send the reader to the article.\n\n" +
		"A bare `/v1/go/<code>` records a hit only when exactly one placement on that campaign is clickable: it is in `placement_counts_as_live`, it is `approved` and still inside its window, or it is `committed` before plugin-ack, and it is not revoked or removed. Zero or two or more redirect to the plain offer URL and record no hit.\n\n" +
		"Saving setup never overwrites a mode that is already stored. While the claim check is a stub, the response `mode` is `manual` with `approval_reason` `claim_check_unavailable` and `approval_message`, and the stored mode stays `auto` or `yolo`. Pause and resume return `effective_mode`, `approval_reason`, and `approval_message` the same way. Each account-digest project includes `total_items`, the count of every item, while `items` stays capped at 50.\n"
}
