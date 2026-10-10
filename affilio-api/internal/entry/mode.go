package entry

// Approval modes from D-031, with D-032 install-path defaults.
const (
	ModeManual = "manual"
	ModeAuto   = "auto"
	ModeYolo   = "yolo"
)

// DefaultMode is the setup preselection. CLI, Cursor, custom connector, and
// Grok start on yolo. ChatGPT and Claude directory installs start on manual.
func DefaultMode(installPath string) string {
	switch installPath {
	case "cli", "cursor", "connector", "grok":
		return ModeYolo
	case "chatgpt_dir", "claude_dir":
		return ModeManual
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

func ValidMode(mode string) bool {
	switch mode {
	case ModeManual, ModeAuto, ModeYolo:
		return true
	default:
		return false
	}
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
