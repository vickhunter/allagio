package entry

import "time"

// PluginStates are the only values plugin_state may take. The field is never
// null and never omitted on a placement.
var PluginStates = []string{"pending", "placed", "removed", "offline"}

// PluginState reports the plugin's view of one placement.
// A brand-new placement and a restore stay pending until the site key
// acknowledges them, which stores placed. A revoke stays pending until the
// site key acknowledges the removal, or until the window passes with no ack.
func PluginState(wait string, waitAt *time.Time, now time.Time, window time.Duration) string {
	switch wait {
	case "placed":
		return "placed"
	case "removed":
		return "removed"
	case "remove":
		if waitAt != nil && !waitAt.UTC().After(now.UTC()) && now.UTC().Sub(waitAt.UTC()) > window {
			return "offline"
		}
		return "pending"
	default:
		return "pending"
	}
}
