package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const approvalByMode = "A new site stores placement_publish as ask, so a person approves each placement until that step is auto. Auto commits only when the domain is verified and claim checks are live. A site key can set that publish step to auto. Ask me every time, Approve within my rules, and Approve anything that passes the checks are a separate control for the decide step. An agent tool can only tighten a site or offer mode. While claims are not checked, Approve within my rules and Approve anything that passes the checks still wait for a person."

const autoCommit = "only when the domain is verified and claim checks are live"

func TestConnectStatesApprovalByMode(t *testing.T) {
	if !strings.Contains(skillMD, approvalByMode) {
		t.Fatal("skill fallback does not state approval by mode")
	}
	root := repoRoot(t)
	for _, rel := range []string{
		"affilio-wp/admin/build/landing-v9/skill.md",
		"affilio-wp/admin/build/landing-v9/you.md",
	} {
		body := readRepo(t, root, rel)
		if !strings.Contains(body, approvalByMode) {
			t.Fatalf("%s does not state approval by mode", rel)
		}
	}
	if skillMD != readRepo(t, root, "affilio-wp/admin/build/landing-v9/skill.md") {
		t.Fatal("skill.md drifted from the fallback")
	}
	you := readRepo(t, root, "affilio-wp/admin/build/landing-v9/you.md")
	for _, label := range []string{"Ask me every time", "Approve within my rules", "Approve anything that passes the checks"} {
		if strings.Count(you, label) < 2 {
			t.Fatalf("you.md label %q is not in both the limits cards and the approvals paragraph", label)
		}
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
		if !strings.Contains(item.body, approvalByMode) {
			t.Fatalf("%s does not state approval by mode", item.name)
		}
	}
	for _, kept := range []string{
		"Under Ask me every time the decide step waits for you. A site key can set the publish step to auto.",
		"Once you set it. While claims are not checked, this still waits for a person.",
		"Only you can turn this on. While claims are not checked, this also waits for a person.",
		"Your limits come first. A new site waits for a person because the publish step starts as ask, and that stays when claim checks are on.",
		autoCommit,
	} {
		if !strings.Contains(you, kept) || !strings.Contains(built, kept) || !strings.Contains(plain, kept) {
			t.Fatalf("mode line dropped: %s", kept)
		}
	}
}

func TestLlmsFullStatesApprovalByMode(t *testing.T) {
	root := repoRoot(t)
	disk := readRepo(t, root, "affilio-wp/admin/build/landing-v9/llms-full.txt")
	if disk != llmsFullDocument() {
		t.Fatal("llms-full.txt drifted from the full document")
	}
	if !strings.Contains(disk, approvalByMode) || !strings.Contains(disk, autoCommit) {
		t.Fatal("llms-full.txt does not state approval by mode")
	}
	if !strings.Contains(disk, "The hosted MCP wrapper adds owner_approval_url whenever prepare_placement or get_placement returns a placement id, including a committed auto prepare. Send that URL to the owner when the status is awaiting_approval.") || strings.Contains(disk, "returns a proposal and owner_approval_url") || strings.Contains(disk, "when prepare waits") {
		t.Fatal("llms-full.txt does not say when owner_approval_url is added")
	}
	if !strings.Contains(disk, "A new site waits for a person to approve each placement before it is published.") || !strings.Contains(disk, "a person approves this step") || !strings.Contains(disk, "## ORP 0-3 test deal") || !strings.Contains(disk, "Placing an offer needs approval for now") {
		t.Fatal("llms-full.txt dropped a manual-scoped or step-modes sentence")
	}
	prepare, commit := placementToolLines(t, disk)
	assertPlacementPublishText(t, "llms prepare", prepare)
	assertPlacementPublishText(t, "llms commit", commit)
}

func TestNoSentenceSaysAgentToolsCannotChangeTheRule(t *testing.T) {
	root := repoRoot(t)
	bodies := []struct {
		name string
		body string
	}{
		{"skill", skillMD},
		{"llms", llmsFullDocument()},
		{"limits fallback", string(youLimitsPlain)},
		{"mcp instructions", hostedMCPInstructions},
		{"you.md", readRepo(t, root, "affilio-wp/admin/build/landing-v9/you.md")},
		{"you.html", readRepo(t, root, "affilio-wp/admin/build/you.html")},
		{"revenue tools", readRepo(t, root, "affilio-api/internal/dcp/revenue_tools.go")},
		{"hosted mcp source", readRepo(t, root, "affilio-api/internal/handlers/hosted_mcp.go")},
		{"workspace", readRepo(t, root, "affilio-api/internal/handlers/workspace.go")},
		{"vision", readRepo(t, root, "ui/landing/src/vision.mjs")},
		{"you app", readRepo(t, root, "ui/landing/src/you/App.tsx")},
		{"stamp", readRepo(t, root, "ui/landing/scripts/stamp-hosted.mjs")},
	}
	for _, item := range bodies {
		lower := strings.ToLower(item.body)
		if strings.Contains(lower, "owner's rule decides") || strings.Contains(lower, "only tighten the mode") || strings.Contains(lower, "cannot change the rule") || strings.Contains(lower, "cannot change that rule") {
			t.Fatalf("%s states an unscoped mode rule", item.name)
		}
		if strings.Contains(lower, "placement_publish") && !strings.Contains(item.body, autoCommit) {
			t.Fatalf("%s names placement_publish without %q", item.name, autoCommit)
		}
	}
}

func TestPrepareAndCommitNamePlacementPublish(t *testing.T) {
	root := repoRoot(t)
	tools := readRepo(t, root, "affilio-api/internal/dcp/revenue_tools.go")
	prepare := descriptionAfter(t, tools, `Name:        "prepare_placement"`)
	commit := descriptionAfter(t, tools, `placementTool("commit_placement", "`)
	assertPlacementPublishText(t, "prepare_placement", prepare)
	assertPlacementPublishText(t, "commit_placement", commit)
	if !strings.Contains(commit, "confirm=true cannot replace owner approval") {
		t.Fatal("commit text dropped confirm=true cannot replace owner approval")
	}
	if !strings.Contains(hostedMCPInstructions, "The wrapper adds owner_approval_url whenever prepare_placement or get_placement returns a placement id, including a committed auto prepare. Send that URL to the owner when the status is awaiting_approval.") || !strings.Contains(hostedMCPInstructions, "Agents cannot approve a placement.") || !strings.Contains(hostedMCPInstructions, autoCommit) || strings.Contains(hostedMCPInstructions, "When prepare waits") {
		t.Fatal("mcp instructions do not attach owner_approval_url for every placement id")
	}
}

func TestYouAppIntroMatchesPublishStep(t *testing.T) {
	root := repoRoot(t)
	const intro = "A new site waits for a person because the publish step starts as ask, and that stays when claim checks are on."
	app := readRepo(t, root, "ui/landing/src/you/App.tsx")
	if !strings.Contains(app, intro) || strings.Contains(app, "starts on the first one") {
		t.Fatal("you app intro does not match the publish step")
	}
	matches, err := filepath.Glob(filepath.Join(root, "affilio-wp/admin/build/landing-v9/assets/you-*.js"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("you bundle: %v %v", matches, err)
	}
	built, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(built), intro) || strings.Contains(string(built), "starts on the first one") {
		t.Fatalf("%s intro does not match the publish step", filepath.Base(matches[0]))
	}
}

func assertPlacementPublishText(t *testing.T, label, body string) {
	t.Helper()
	if !strings.Contains(body, "placement_publish") {
		t.Fatalf("%s does not name placement_publish", label)
	}
	for _, mode := range []string{"Ask me every time", "Approve within my rules", "Approve anything that passes the checks", "manual", "yolo"} {
		if strings.Contains(body, mode) {
			t.Fatalf("%s names owner mode %q", label, mode)
		}
	}
}

func placementToolLines(t *testing.T, body string) (string, string) {
	t.Helper()
	var prepare, commit string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "- prepare_placement:") {
			prepare = line
		}
		if strings.HasPrefix(line, "- commit_placement:") {
			commit = line
		}
	}
	if prepare == "" || commit == "" {
		t.Fatal("llms-full.txt dropped a prepare or commit line")
	}
	return prepare, commit
}

func descriptionAfter(t *testing.T, src, marker string) string {
	t.Helper()
	i := strings.Index(src, marker)
	if i < 0 {
		t.Fatalf("missing %s", marker)
	}
	rest := src[i+len(marker):]
	if strings.HasPrefix(marker, `placementTool("commit_placement", "`) {
		end := strings.Index(rest, `"`)
		if end < 0 {
			t.Fatal("unterminated commit description")
		}
		return rest[:end]
	}
	const key = `Description: "`
	start := strings.Index(rest, key)
	if start < 0 {
		t.Fatalf("missing description after %s", marker)
	}
	rest = rest[start+len(key):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatal("unterminated prepare description")
	}
	return rest[:end]
}
