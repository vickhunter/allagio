// Command offer-rails-mcp is the DCP MCP server. It exposes the Offer Rails
// maker verbs as MCP tools over stdio, so any agent client (Claude Code,
// Cursor, Codex, Copilot, ...) can take a product from zero to listed and
// earning. The source directory stays cmd/distributoor-mcp.
//
// Configuration via environment. The OFFERRAILS_* name is read first. The
// DISTRIBUTOOR_* name is the fallback:
//
//	OFFERRAILS_API_URL   base URL of Offer Rails (default http://localhost:8080)
//	OFFERRAILS_API_KEY   maker api key (obtain one with the register_maker tool)
//	OFFERRAILS_SITE_KEY  publisher site api key (enables the publisher-side tools)
package main

import (
	"bufio"
	"fmt"
	"os"

	"affilio-api/internal/dcp"
)

func main() {
	apiURL := dcp.Getenv(dcp.EnvAPIURL, dcp.LegacyEnvAPIURL)
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	client := dcp.NewClient(apiURL, dcp.Getenv(dcp.EnvAPIKey, dcp.LegacyEnvAPIKey))
	client.SiteKey = dcp.Getenv(dcp.EnvSiteKey, dcp.LegacyEnvSiteKey)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	writer := bufio.NewWriter(os.Stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if resp := dcp.HandleMessage(client, line); resp != nil {
			writer.Write(resp)
			writer.WriteByte('\n')
			writer.Flush()
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: stdin error: %v\n", dcp.ServerName, err)
		os.Exit(1)
	}
}
