// distributoor-mcp is the DCP MCP server: it exposes the Distributoor maker
// verbs as MCP tools over stdio, so any agent client (Claude Code, Cursor,
// Codex, Copilot, ...) can take a product from zero to listed and earning.
//
// Configuration via environment:
//
//	DISTRIBUTOOR_API_URL   base URL of the Distributoor network (default http://localhost:8080)
//	DISTRIBUTOOR_API_KEY   maker api key (obtain one with the register_maker tool)
//	DISTRIBUTOOR_SITE_KEY  publisher site api key (enables the publisher-side tools)
package main

import (
	"bufio"
	"fmt"
	"os"

	"affilio-api/internal/dcp"
)

func main() {
	apiURL := os.Getenv("DISTRIBUTOOR_API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	client := dcp.NewClient(apiURL, os.Getenv("DISTRIBUTOOR_API_KEY"))
	client.SiteKey = os.Getenv("DISTRIBUTOOR_SITE_KEY")

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
		fmt.Fprintf(os.Stderr, "distributoor-mcp: stdin error: %v\n", err)
		os.Exit(1)
	}
}
