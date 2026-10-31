# allagio

Local CLI and stdio MCP server for Allagio. The package name and the command are `allagio`. The `offer-rails` command is a legacy alias of the same program.

`ALLAGIO_URL` is the API origin. `OFFER_RAILS_URL` and `OFFER_RAILS_API_URL` are still read when `ALLAGIO_URL` is unset. When those are unset, the CLI uses the same default as `config.DefaultPublicBaseURL` in the Go API (`cli/lib/public-base.mjs`). Use https. http is accepted only for `localhost`, `127.0.0.1`, and `::1`.

The npm package is coming soon. Nothing is registered. Until then, run the program from this repository.

`login` stores a project-scoped key in `./.offer-rails/config.json` (file mode 0600). It prints whether a person approved the device code or an agent alone created the account. The key itself is not printed.

`scan` reads the project on this machine. It does not read `.env` files, key or certificate files, or paths ignored by git. The API receives the listing card, the pass/fail signals, and the score. The CLI prints that JSON and asks before the first upload. Say yes once. Page text and source stay local.

`list`, `status`, `digest`, `undo`, and `kill` use that project key. `list` submits a draft offer and prints the status the server returns. It prints `listed` when the server lists the offer. It prints `in review` only when a check fails, then prints `check:` with the server's `review_reason`, the message, and the next step. The CLI does not set an approval mode. `list` and `digest` print `mode` and `mode_preselected` from the server. For a new site, `mode` is manual and `mode_preselected` is yolo. A new site waits for a person to approve each placement before it is published. That starting choice stays when claim checks are on. `kill` turns the kill switch on. When a digest is cut at the page size, `digest` prints `truncated: true`, the total, and that the list was cut. While claims are not checked yet, `list`, `status`, and `digest` print `claims: not checked yet`.

These actions are not available here: loosen the approval mode, raise a cap or lower minimum relevance, turn the kill switch off, create an account-wide key, generate or rotate or revoke a plugin key, and restore a placement. The command prints a sign-in link to `/owner`.

## CLI

<!-- paste:begin -->
```bash
node cli/bin/offer-rails.mjs login
node cli/bin/offer-rails.mjs scan .
node cli/bin/offer-rails.mjs list
node cli/bin/offer-rails.mjs digest
node cli/bin/offer-rails.mjs undo
node cli/bin/offer-rails.mjs kill
```
<!-- paste:end -->

`scan` prints the payload and asks `Send this listing card? [y/N]` the first time. An agent-only account, with no person approving a device code, is `node cli/bin/offer-rails.mjs login --agent --role maker --name "Example" --email you@example.com`.

## Cursor

Copy this into `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "allagio": {
      "command": "node",
      "args": ["cli/bin/offer-rails.mjs", "mcp"]
    }
  }
}
```

## Claude Code

```bash
node cli/bin/offer-rails.mjs mcp
```

JSON form, for `.mcp.json` or `~/.claude.json`:

```json
{
  "mcpServers": {
    "allagio": {
      "command": "node",
      "args": ["cli/bin/offer-rails.mjs", "mcp"]
    }
  }
}
```

`node cli/bin/offer-rails.mjs mcp` speaks MCP over stdio. The npm package is coming soon. The tools are the same commands. `scan` uploads only when `confirm` is true. `restore_placement` and the other session-only tools return a sign-in link and do not call the API.
