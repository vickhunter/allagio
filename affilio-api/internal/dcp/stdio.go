package dcp

import (
	"bufio"
	"fmt"
	"os"
)

// RunStdio serves one MCP session on stdin and stdout. cmd/offer-rails-mcp is
// the entrypoint. cmd/distributoor-mcp calls this same function so an existing
// config that starts the old binary keeps working.
func RunStdio() {
	apiURL := Getenv(EnvAPIURL, LegacyEnvAPIURL, OlderEnvAPIURL)
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	client := NewClient(apiURL, Getenv(EnvAPIKey, LegacyEnvAPIKey, OlderEnvAPIKey))
	client.SiteKey = Getenv(EnvSiteKey, LegacyEnvSiteKey, OlderEnvSiteKey)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	writer := bufio.NewWriter(os.Stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if resp := HandleMessage(client, line); resp != nil {
			writer.Write(resp)
			writer.WriteByte('\n')
			writer.Flush()
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: stdin error: %v\n", ServerName, err)
		os.Exit(1)
	}
}
