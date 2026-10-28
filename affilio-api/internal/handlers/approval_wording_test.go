package handlers

import (
	"strings"
	"testing"
)

const approvalByMode = "Ask me every time, a person approves each placement. In Approve within my rules and Approve anything that passes the checks, the owner's rule decides, and agent tools still cannot change that rule."

func TestConnectStatesApprovalByMode(t *testing.T) {
	if !strings.Contains(skillMD, approvalByMode) || strings.Contains(skillMD, "A person still approves each placement.") {
		t.Fatal("skill fallback does not state approval by mode")
	}
	root := repoRoot(t)
	for _, rel := range []string{
		"affilio-wp/admin/build/landing-v9/skill.md",
		"affilio-wp/admin/build/landing-v9/you.md",
	} {
		body := readRepo(t, root, rel)
		if !strings.Contains(body, approvalByMode) || strings.Contains(body, "A person still approves each placement.") {
			t.Fatalf("%s does not state approval by mode", rel)
		}
		assertApprovalLinesAreManualScoped(t, rel, body)
	}
	if skillMD != readRepo(t, root, "affilio-wp/admin/build/landing-v9/skill.md") {
		t.Fatal("skill.md drifted from the fallback")
	}
}

func TestYouLimitsStatesApprovalByMode(t *testing.T) {
	root := repoRoot(t)
	you := readRepo(t, root, "affilio-wp/admin/build/landing-v9/you.md")
	built := readRepo(t, root, "affilio-wp/admin/build/you.html")
	plain := string(youLimitsPlain)
	for _, item := range []struct {
		name string
		body string
	}{
		{"you.md", you},
		{"you.html", built},
		{"limits fallback", plain},
	} {
		if !strings.Contains(item.body, approvalByMode) || strings.Contains(item.body, "No agent tool can approve") {
			t.Fatalf("%s does not state approval by mode", item.name)
		}
	}
	for _, kept := range []string{
		"Manual. A new site opens here. Each placement waits for you.",
		"Once you set it. While claims are not checked, this still waits for a person.",
		"Only you can turn this on. While claims are not checked, this also waits for a person.",
	} {
		if !strings.Contains(you, kept) || !strings.Contains(built, kept) || !strings.Contains(plain, kept) {
			t.Fatalf("mode line dropped: %s", kept)
		}
	}
	assertApprovalLinesAreManualScoped(t, "you.md", strings.Split(you, "## Connect a hosted assistant")[0])
}

func TestLlmsFullStatesApprovalByMode(t *testing.T) {
	root := repoRoot(t)
	disk := readRepo(t, root, "affilio-wp/admin/build/landing-v9/llms-full.txt")
	fallback := llmsFullDocument()
	if disk != fallback {
		t.Fatal("llms-full.txt drifted from the full document")
	}
	for _, flat := range []string{
		"No agent tool can approve",
		"a person approves before anything is published",
		"a person approves it before it is published",
		"wait for a person to approve before anything is published",
		"a person still has to approve",
		"after a person approves",
		"A person still approves each placement.",
	} {
		if strings.Contains(disk, flat) {
			t.Fatalf("llms-full.txt still says %q", flat)
		}
	}
	if !strings.Contains(disk, approvalByMode) || !strings.Contains(disk, "Approve within my rules") || !strings.Contains(disk, "Approve anything that passes the checks") {
		t.Fatal("llms-full.txt does not name the modes")
	}
	if !strings.Contains(disk, "A new site waits for a person to approve each placement before it is published.") || !strings.Contains(disk, "a person approves this step") || !strings.Contains(disk, "## ORP 0-3 test deal") || !strings.Contains(disk, "Placing an offer needs approval for now") {
		t.Fatal("llms-full.txt dropped a manual-scoped or step-modes sentence")
	}
	assertApprovalLinesAreManualScoped(t, "llms-full.txt", disk)
}

func assertApprovalLinesAreManualScoped(t *testing.T, label, body string) {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "approves each placement") && !strings.Contains(line, "Ask me every time") {
			t.Fatalf("%s states a flat approval: %s", label, line)
		}
	}
}
