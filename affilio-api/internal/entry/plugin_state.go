package entry

import "time"

// PluginStates are the only values plugin_state may take. The field is never
// null and never omitted on a placement.
var PluginStates = []string{"pending", "placed", "removed", "offline", "failed"}

// PluginState reports the plugin's view of one placement.
// A brand-new placement and a restore stay pending until the site key
// acknowledges them, which stores placed. A revoke stays pending until the
// site key acknowledges the removal, or until the window passes with no ack.
// failed means the plugin reported that its post re-read failed. That beats
// the offline window, because a failure acknowledgement is an acknowledgement.
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
	switch wait {
	case "remove":
		if waitAt != nil && !waitAt.UTC().After(now.UTC()) && now.UTC().Sub(waitAt.UTC()) > window {
			return "offline"
		}
		return "pending"
	default:
		return "pending"
	}
}

// PluginRetryDoc is the retry section for /llms.txt and the mode docs.
func PluginRetryDoc() string {
	return "## Plugin retry\n\n" +
		"`POST /v1/placements/:id/retry` is the publisher retry. A session or a site key for that placement may call it. The other side receives 403. It is allowed only while `plugin_state` is `failed`. Any other state returns 409 `not_failed`. It re-runs every hard check. A failure returns 422 with `check_id` and `check_text` and leaves the placement failed. Success sets the state to `pending` and re-queues the pending action, so the next acknowledgement moves it to `placed` or `removed`.\n"
}
