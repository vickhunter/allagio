# Contributing

Small, focused PRs are welcome. The house rules (also in `AGENTS.md`/`CLAUDE.md`):

- **Minimal, additive changes.** Don't remove existing functionality unless the
  PR is specifically about that.
- **Update `STATUS.md`** with any feature or behavior change — prepend a dated
  entry at the top.
- **UI changes live in `affilio-wp/admin/build/`** (compiled JS; no separate
  source tree for the dashboard). Landing pages are stamped from `ui/landing/`
  into `affilio-wp/admin/build/` — regenerate, don't hand-edit stamped HTML.
- **Doc twins must stay in sync.** `agent_docs.go` consts mirror
  `ui/landing/public/llms.txt` and `affilio-wp/admin/build/landing-v9/*.txt|md`;
  the drift tests (`TestColdAgentEntrance`, `TestMarkdownTwinsMatchHTMLPages`)
  will fail otherwise.
- **Migrations** go in `affilio-api/internal/migrations/` in numeric order; bump
  `applyMigrationsThrough` + `migrationFiles()` in `handlers_test.go`.
- **Never commit credentials.** Env config is in `config.go`; secrets come from
  environment, not files.

## Tests

```bash
cd affilio-api
make test-db-up   # Postgres on :5433
make test         # handler suite
```

Go tests run with `GOTOOLCHAIN=local` and the `AFFILIO_TEST_DATABASE_URL` env var.

Every PR gets an automated Devin Review — findings are triaged before merge;
CI runs the Go suite (note: GitHub-hosted CI on this repo is temporarily red for
billing reasons; a green local suite is the accepted signal).
