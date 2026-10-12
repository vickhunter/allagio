# offer-rails

Local CLI and stdio MCP server for Offer Rails. The package name and the MCP server id are `offer-rails`.

`OFFER_RAILS_API_URL` defaults to `https://offer-rails.onrender.com`. Use https. http is accepted only for `localhost`, `127.0.0.1`, and `::1`.

`login` stores a project-scoped key in `./.offer-rails/config.json` (file mode 0600). It prints whether a person approved the device code or an agent alone created the account. The key itself is not printed.

`scan` reads the project on this machine. It does not read `.env` files, key or certificate files, or paths ignored by git. The API receives the listing card, the pass/fail signals, and the score. The CLI prints that JSON and asks before the first upload. Say yes once. Page text and source stay local.

`list`, `status`, `digest`, `undo`, and `kill` use that project key. Listings and placements go live with no extra confirm. The server still runs the hard checks. `kill` turns the kill switch on.

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
