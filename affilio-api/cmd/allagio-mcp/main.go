// Command allagio-mcp is the DCP MCP server. It exposes the Allagio maker
// verbs as MCP tools over stdio, so any agent client can take a product
// from zero to listed and earning.
//
// Configuration via environment. ALLAGIO_* is read first. OFFERRAILS_* and
// then DISTRIBUTOOR_* remain so a host that still sets an old variable keeps
// working. The localhost default is applied only after every API URL name is
// blank:
//
//	ALLAGIO_API_URL   base URL of Allagio
//	ALLAGIO_API_KEY   maker api key (obtain one with the register_maker tool)
//	ALLAGIO_SITE_KEY  publisher site api key (enables the publisher-side tools)
package main

import "affilio-api/internal/dcp"

func main() {
	dcp.RunStdio()
}
