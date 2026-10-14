# offer-rails

Local CLI and stdio MCP server for Offer Rails. The package name and the MCP server id are `offer-rails`.

`OFFER_RAILS_API_URL` defaults to `https://offer-rails.onrender.com`. Use https. http is accepted only for `localhost`, `127.0.0.1`, and `::1`.

`login` stores a project-scoped key in `./.offer-rails/config.json` (file mode 0600). It prints whether a person approved the device code or an agent alone created the account. The key itself is not printed.

`scan` reads the project on this machine. It does not read `.env` files, key or certificate files, or paths ignored by git. The API receives the listing card, the pass/fail signals, and the score. The CLI prints that JSON and asks before the first upload. Say yes once. Page text and source stay local.

`list`, `status`, `digest`, `undo`, and `kill` use that project key. `list` submits a draft offer and prints the status the server returns. It prints `listed` when the server lists the offer. It prints `in review` only when a check fails, then prints `check:` with the server's `review_reason`, the message, and the next step. The CLI does not set an approval mode. `list` and `digest` print `mode` and `mode_preselected` from the server. For a new site, `mode` is manual and `mode_preselected` is yolo. `kill` turns the kill switch on. When a digest is cut at the page size, `digest` prints `truncated: true`, the total, and that the list was cut. While claims are not checked yet, `list`, `status`, and `digest` print `claims: not checked yet`.

These actions are not available here: loosen the approval mode, raise a cap or lower minimum relevance, turn the kill switch off, create an account-wide key, generate or rotate or revoke a plugin key, and restore a placement. The command prints a sign-in link to `/owner`.

## CLI

<!-- paste:begin -->
```bash
npx offer-rails login
npx offer-rails scan .
npx offer-rails list
npx offer-rails digest
npx offer-rails undo
npx offer-rails kill
```
<!-- paste:end -->

`scan` prints the payload and asks `Send this listing card? [y/N]` the first time. An agent-only account, with no person approving a device code, is `npx offer-rails login --agent --role maker --name "Example" --email you@example.com`.

## Cursor

Copy this into `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "offer-rails": {
      "command": "npx",
      "args": ["-y", "offer-rails", "mcp"]
    }
  }
}
```

## Claude Code

```bash
claude mcp add offer-rails -- npx offer-rails mcp
```

JSON form, for `.mcp.json` or `~/.claude.json`:

```json
{
  "mcpServers": {
    "offer-rails": {
      "command": "npx",
      "args": ["offer-rails", "mcp"]
    }
  }
}
```

`npx offer-rails mcp` speaks MCP over stdio. The tools are the same commands. `scan` uploads only when `confirm` is true. `restore_placement` and the other session-only tools return a sign-in link and do not call the API.
