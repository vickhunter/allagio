package entry

// Approval modes from D-031, with D-032 install-path defaults.
const (
	ModeManual = "manual"
	ModeAuto   = "auto"
	ModeYolo   = "yolo"
)

// DefaultMode is the setup preselection from owners.source. cli, cursor, grok,
// mcp, and connector start on yolo. Every other source starts on manual.
func DefaultMode(source string) string {
	switch source {
	case "cli", "cursor", "grok", "mcp", "connector":
		return ModeYolo
	default:
		return ModeManual
	}
}

func ValidInstallPath(path string) bool {
	switch path {
	case "cli", "cursor", "connector", "grok", "chatgpt_dir", "claude_dir":
		return true
	default:
		return false
	}
}

// DefaultInstallPath is the install path used when setup is read or saved
// without one. It follows owners.source. mcp has no path of its own and uses
// connector. chatgpt and claude use their directory installs. Anything else,
// including direct, uses cli. The path does not select the mode.
func DefaultInstallPath(source string) string {
	switch source {
	case "cli", "cursor", "connector", "grok":
		return source
	case "mcp":
		return "connector"
	case "chatgpt":
		return "chatgpt_dir"
	case "claude":
		return "claude_dir"
	default:
		return "cli"
	}
}

func ValidMode(mode string) bool {
	switch mode {
	case ModeManual, ModeAuto, ModeYolo:
		return true
	default:
		return false
	}
}

// AvailableModes is the mode list the setup page and the digest show.
func AvailableModes() []string {
	return []string{ModeManual, ModeAuto, ModeYolo}
}

// modeRank orders modes from strictest to loosest.
func modeRank(mode string) int {
	switch mode {
	case ModeManual:
		return 0
	case ModeAuto:
		return 1
	case ModeYolo:
		return 2
	default:
		return -1
	}
}

// AgentMaySetMode is true when an agent tool keeps the mode or tightens it
// (yolo → auto → manual). Loosening, including turning yolo on, is refused.
func AgentMaySetMode(from, to string) bool {
	fromRank, toRank := modeRank(from), modeRank(to)
	return fromRank >= 0 && toRank >= 0 && toRank <= fromRank
}
