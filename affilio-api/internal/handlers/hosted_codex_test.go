package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestHostedCodexOAuthLogin(t *testing.T) {
	binary := os.Getenv("AFFILIO_CODEX_BINARY")
	if binary == "" {
		t.Skip("AFFILIO_CODEX_BINARY not set; optional installed-client check")
	}
	db, cleanup := setupTestDB(t)
	defer cleanup()
	h, r, _ := hostedTestServer(t, db)
	o, cookie := testOwnerSession(t, h, "codex-publisher", "codex-publisher@example.com")
	if rec := ownerForm(t, h, r, cookie, "/owner/accounts", url.Values{"csrf": {o.CSRF}, "role": {"publisher"}, "domain": {"publisher.example.com"}}); rec.Code != 303 {
		t.Fatal(rec.Body.String())
	}
	nameToken, _ := randomToken()
	name := "distributoor-qa-" + strings.ToLower(nameToken[:12])
	configuration := fmt.Sprintf("mcp_servers.%s.url=%q", name, h.resource())
	args := []string{"-c", configuration, "-c", `mcp_oauth_credentials_store="keyring"`, "mcp", "login", name, "--no-browser", "--scopes", "publisher", "--oauth-client-registration", "dcr"}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = cmd.Stdout
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := exec.CommandContext(cleanupCtx, binary, "-c", configuration, "-c", `mcp_oauth_credentials_store="keyring"`, "mcp", "logout", name).Run(); err != nil {
			t.Errorf("could not remove temporary Codex keyring entry: %v", err)
		}
	})
	lines := make(chan string, 32)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	var authorizationURL string
	var output strings.Builder
	pattern := regexp.MustCompile(`https?://[^\s]+/oauth/authorize[^\s]*`)
	for authorizationURL == "" {
		select {
		case line, ok := <-lines:
			if !ok {
				t.Fatalf("Codex did not start authorization: %s", output.String())
			}
			output.WriteString(line + "\n")
			authorizationURL = pattern.FindString(line)
		case <-ctx.Done():
			t.Fatal("Codex authorization timed out")
		}
	}
	u, err := url.Parse(authorizationURL)
	if err != nil || u.Scheme+"://"+u.Host != h.BaseURL {
		t.Fatal("unexpected authorization server")
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", authorizationURL, nil)
	req.AddCookie(cookie)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	requestID := regexp.MustCompile(`name="request_id" value="([^"]+)"`).FindStringSubmatch(string(body))
	if response.StatusCode != 200 || len(requestID) != 2 {
		t.Fatalf("Codex consent failed: %d %s", response.StatusCode, body)
	}
	consent := ownerForm(t, h, r, cookie, "/owner/authorize", url.Values{"csrf": {o.CSRF}, "request_id": {requestID[1]}})
	if consent.Code != 303 {
		t.Fatal(consent.Body.String())
	}
	callback, err := url.Parse(consent.Header().Get("Location"))
	if err != nil || !loopbackHost(callback.Hostname()) {
		t.Fatal("unexpected Codex callback")
	}
	if _, err = io.WriteString(stdin, callback.String()+"\n"); err != nil {
		t.Fatal("could not deliver Codex callback")
	}
	for line := range lines {
		output.WriteString(line + "\n")
	}
	if err = cmd.Wait(); err != nil || !strings.Contains(output.String(), "Successfully logged in") {
		t.Fatalf("Codex login failed: %v %s", err, output.String())
	}
	t.Log("Installed Codex CLI completed DCR, discovery, PKCE and OAuth token exchange against the local hosted server with a fixture owner session. Cleanup checks removal of its temporary keyring entry; no persistent MCP configuration or real OIDC provider was changed.")
}
