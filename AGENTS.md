# Agent Instructions (Affilio)

## Project Goal
Build Affilio: a WordPress plugin + Go backend that scans posts, detects affiliate links, recommends curated + user‑specific programs, and enables one‑click insertion with a Yoast‑style score.

## Core Rules
- Keep the MVP tight: scan → detect → recommend → insert → track.
- Prioritize user‑specific affiliate links first; then curated catalog.
- Any insert must include FTC disclosure once per post.
- Recommendations must include a reason and relevance score; max 5 per article.

## Repository Layout
- `affilio-api/` (Go backend)
- `affilio-wp/` (WordPress plugin)

## Implementation Conventions
- Backend endpoints use `X-API-Key` for sites and `X-Admin-Key` for discovery queue.
- Use PostgreSQL migrations in `affilio-api/internal/migrations/`.
- Keep all new UI changes in `affilio-wp/admin/build/` (compiled JS for now).

## Work Style
- Do not remove existing functionality unless explicitly requested.
- Keep changes minimal and additive.
- If you add a new endpoint, add a matching UI change or document it in STATUS.md.
- Always update `STATUS.md` when adding features or changing behavior.

## Current Gaps (see STATUS.md)
- Claude integration is stubbed.
- Content generation is placeholder.
- No tests or analytics endpoints.

## Notes
- The discovery queue is admin‑only; never expose admin key to public frontend.
- Use safe defaults and avoid auto‑insert beyond approved inline mentions.
