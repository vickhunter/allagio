// Command offer-rails-mcp is the DCP MCP server. It exposes the Allagio
// maker verbs as MCP tools over stdio, so any agent client can take a product
// from zero to listed and earning.
//
// Configuration via environment. The OFFERRAILS_* name is read first. The
// DISTRIBUTOOR_* name is the fallback. The localhost default is applied only
// after both API URL names are blank:
//
//	OFFERRAILS_API_URL   base URL of Allagio
//	OFFERRAILS_API_KEY   maker api key (obtain one with the register_maker tool)
//	OFFERRAILS_SITE_KEY  publisher site api key (enables the publisher-side tools)
package main

import "affilio-api/internal/dcp"

func main() {
	dcp.RunStdio()
}
