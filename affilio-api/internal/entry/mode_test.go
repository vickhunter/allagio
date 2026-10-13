package entry

import "testing"

func TestDefaultModeByInstallPath(t *testing.T) {
	yolo := []string{"cli", "cursor", "connector", "grok"}
	for _, path := range yolo {
		if got := DefaultMode(path); got != ModeYolo {
			t.Fatalf("%s default = %s", path, got)
		}
	}
	manual := []string{"chatgpt_dir", "claude_dir", "", "direct", "facebook"}
	for _, path := range manual {
		if got := DefaultMode(path); got != ModeManual {
			t.Fatalf("%s default = %s", path, got)
		}
	}
}

func TestDefaultInstallPathFollowsSource(t *testing.T) {
	same := []string{"cli", "cursor", "connector", "grok"}
	for _, source := range same {
		if got := DefaultInstallPath(source); got != source || !ValidInstallPath(got) {
			t.Fatalf("%s install path = %s", source, got)
		}
	}
	if got := DefaultInstallPath("mcp"); got != "connector" || !ValidInstallPath(got) {
		t.Fatalf("mcp install path = %s", got)
	}
	if got := DefaultInstallPath("chatgpt"); got != "chatgpt_dir" || !ValidInstallPath(got) {
		t.Fatalf("chatgpt install path = %s", got)
	}
	if got := DefaultInstallPath("claude"); got != "claude_dir" || !ValidInstallPath(got) {
		t.Fatalf("claude install path = %s", got)
	}
	for _, source := range []string{"direct", "", "community", "x"} {
		if got := DefaultInstallPath(source); got != "cli" || !ValidInstallPath(got) {
			t.Fatalf("%s install path = %s", source, got)
		}
	}
}

func TestAgentMayOnlyTightenMode(t *testing.T) {
	if !AgentMaySetMode(ModeYolo, ModeAuto) || !AgentMaySetMode(ModeAuto, ModeManual) || !AgentMaySetMode(ModeYolo, ModeManual) {
		t.Fatal("tightening was refused")
	}
	if !AgentMaySetMode(ModeAuto, ModeAuto) || !AgentMaySetMode(ModeManual, ModeManual) {
		t.Fatal("keeping the mode was refused")
	}
	if AgentMaySetMode(ModeManual, ModeAuto) || AgentMaySetMode(ModeAuto, ModeYolo) || AgentMaySetMode(ModeManual, ModeYolo) {
		t.Fatal("loosening was allowed")
	}
	if AgentMaySetMode("nope", ModeManual) || AgentMaySetMode(ModeYolo, "ask") {
		t.Fatal("unknown mode was allowed")
	}
}
