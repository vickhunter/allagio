# Offer Rails setup

One entry point installs MCP and skills, detects framework/payment dependencies
and uses the builder's existing coding agent for attribution wiring, tests and
readiness assessment. It requires Node 20+ and no separate model API key.
`npx offer-rails` is the command. `affilio`, `distributoor`, and `distributoor-setup` run the same installer.

This package is prepared for release and **is not published to npm yet**. The
hosted pilot also needs its sign-in configuration before online setup works.

## Run today

From the product repository:

```sh
node /path/to/affilio/packages/distributoor-setup/bin/setup.mjs --api-url https://your-hosted-pilot.example
```

For a portable release, run `npm pack` here, then use the archive in any product:

```sh
npx --yes --package /path/to/offer-rails-0.1.0.tgz offer-rails --api-url https://your-hosted-pilot.example
```

`affilio` and `distributoor` are aliases for that same archive. After publication, the intended command is `npx offer-rails --api-url
https://your-hosted-pilot.example`. It is not a working public installation
command until the package is published.

## Clients

| Client | Setup behavior |
| --- | --- |
| Codex CLI | Installs project skills, uses temporary MCP URL configuration, completes browser maker OAuth, then starts an interactive integration session. Global configuration stays unchanged. Future sessions need the printed launch command or an owner-configured persistent connection. |
| Cursor | Merges `.cursor/mcp.json`, installs `.agents/skills`, then directs the builder to invoke `distributoor-onboard` in Cursor. Cursor handles browser authentication. |
| Claude Code | Merges `.mcp.json`, installs `.claude/skills`, then starts Claude when installed in an interactive terminal. Claude handles MCP approval and browser authentication. |
| Other agents | `--client skill` installs portable `.agents/skills`; use the agent's native hosted MCP connection. |

`--client auto` prefers existing project client directories, then an installed
Codex or Claude CLI. Use `--client` for projects with several clients.
`--website` supplies a known deployed product URL; `--project` selects the target
repository. `--install-only` prepares files without launching an agent.
Non-interactive runs also stop after installation.

`--offline` prepares files without checking the server. Authentication and actual
product integration remain pending. Existing unrelated MCP entries are preserved;
conflicting Offer Rails entries, malformed JSON, custom skills and symbolic-link
write paths cause a clear error. The installer uploads no source or secrets.
Once launched, the user's agent follows its provider's data processing settings.

The skill reuses products, preserves checkout metadata, directs the owner to
connect Stripe in its owner form and reports separate evidence for code tests,
deployed readiness, payment configuration and an actual test purchase/refund.

## Verify

`npm test` checks config preservation, retry behavior, skill conflicts, path
safety, OAuth metadata validation and offline CLI execution. `npm pack` bundles
the canonical skills from `distributoor-plugin/skills/`; no runtime packages are
required. Full integration by a real builder agent is still a pilot check.
