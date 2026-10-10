package entry

import "time"

// PluginState is what the digest reports for one placement.
// pending means the plugin still has to remove or re-insert the block.
// removed means the plugin confirmed the block is gone.
// offline means the plugin has been silent longer than the contact window.
// current means the plugin has checked in and nothing is waiting.
func PluginState(wait string, waitAt, contact *time.Time, now time.Time, window time.Duration) string {
	fresh := freshStamp(contact, now, window)
	waitLive := (wait == "remove" || wait == "restore") && freshStamp(waitAt, now, window)
	switch wait {
	case "remove", "restore":
		if waitLive || fresh {
			return "pending"
		}
		return "offline"
	case "removed":
		return "removed"
	default:
		if fresh {
			return "current"
		}
		return "offline"
	}
}

func freshStamp(at *time.Time, now time.Time, window time.Duration) bool {
	if at == nil || window < 0 {
		return false
	}
	stamp := at.UTC()
	current := now.UTC()
	if stamp.After(current) {
		return false
	}
	return current.Sub(stamp) <= window
}
