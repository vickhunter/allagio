# Affilio MVP Checklist

Last Updated: February 06, 2026

## Core Flow
- [x] Scan full site + single post
- [x] Detect affiliate links (rules + optional Claude)
- [x] Recommend programs (user first, curated second)
- [x] Insert with FTC disclosure once per post
- [x] Track clicks (beacon + API)
- [x] Analytics endpoints (articles + recommendations)
- [x] Analytics dashboard tables

## Quality + Safety
- [x] [P2] Time-range analytics (7d/30d)
- [x] [P2] Content generation prompts + per-article context
- [x] [P3] Discovery promotion flow (queue -> curated program)
- [x] [P3] End-to-end tests for insert + tracking
- [x] [P4] API key rotation + audit
- [x] [P4] Semantic/intent matching upgrade
- [x] [P4] Advanced analytics reporting (beyond 7d/30d)
- [x] Recommendation dedupe (user over curated)
- [x] [P4] Conversion/revenue attribution analytics
- [x] [P4] Conversion funnel reporting (click -> inserted -> dismissed)
- [x] [P4] Analytics export (CSV/JSON download)
- [x] [P4] Multi-touch attribution model options
- [x] [P4] Cohort analytics by article publish date
- [x] [P4] Recommendation quality feedback loop (accept/dismiss outcomes)
- [x] [P4] Retention/churn cohort tracking
- [x] [P4] Recommendation quality model tuning loop
- [x] [P4] WP onboarding + connection status UX (key setup, banners, advanced-mode settings)
- [x] [P4] Program discovery enrichment automation
- [x] [P4] Browser-level E2E tests for WP flows
- [x] [P0] Validate and restrict recommendation status transitions (`pending -> approved/dismissed -> inserted`)
- [x] [P0] Harden site lifecycle endpoints (`/sites/register`, `/sites` delete) with proper auth/rate limiting and remove destructive delete by API key alone
- [x] [P0][Now] Recommendation insert preview UX v1 (show generated copy + planned placement + rationale before insert)
- [x] [P0] Structured insertion plan response v1 (`generated_text`, `suggested_position`, `placement_reason`)
- [x] [P0] Smart insertion placement strategy v1 (after intro / mid article / before conclusion / end fallback)
- [x] [P0] Language + tone alignment guidance in generation prompt v1 (match article language + tone with context excerpt)
- [x] [P1] Inline edit/regenerate flow before insert v1 (human-in-the-loop copy control)
- [x] [P1] Opportunity-first dashboard UX v1 (prioritized “Quick Wins” cards + one-click review jump)
- [x] [P1] Explainable recommendations (clear reason + expected impact + placement why)
- [x] [P1] Localized UX/copy defaults v1 for non-technical bloggers (IT/EN ready for connection/settings/sidebar/dashboard core flows)
- [x] [P2] Learning insights UX (show what recommendation tuning learned and why)

## Security + Launch Blockers
- [x] [P0] Validate and restrict recommendation status transitions (`pending -> approved/dismissed -> inserted`)
- [x] [P0] Harden site lifecycle endpoints (`/sites/register`, `/sites` delete) with proper auth/rate limiting and remove destructive delete by API key alone
- [x] [P0] Add CORS allowlist enforcement using `ALLOWED_ORIGINS` for browser-based plugin calls
- [x] [P0] Enforce per-post capability check in WP insert route (`current_user_can('edit_post', $post_id)`)
- [x] [P1] Add click anti-abuse controls (basic rate limit + duplicate suppression heuristics) for public tracking route
- [x] [P1] Reduce API key exposure in WP editor/sidebar (role gating or short-lived scoped token exchange)
- [x] [P1] Key security hardening: hash/encrypt stored API keys and add previous-key expiry window after rotation
- [x] [P1] Add audit logs + alerts for auth failures and admin-key endpoint usage

## Testing
- [x] Service unit tests (classifier/generator/matcher)
- [x] Handler integration tests (scan/generate/detect/recs)
- [x] [P4] CI wiring for tests
- [x] [P4] Test quality gates (coverage threshold + required checks)

## Ops
- [x] Test DB compose + make target
- [x] Status command (`make status`)
- [x] [P4] Production deployment notes
- [x] [P4] Branch protection requiring `affilio-api-tests`
