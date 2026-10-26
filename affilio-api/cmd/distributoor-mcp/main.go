// Command distributoor-mcp is the legacy binary name for the Offer Rails stdio
// server. It runs the same process as cmd/offer-rails-mcp so a saved config
// that still starts distributoor-mcp keeps working.
package main

import "affilio-api/internal/dcp"

func main() {
	dcp.RunStdio()
}
