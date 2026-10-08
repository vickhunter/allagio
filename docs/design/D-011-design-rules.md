# D-011 design-rules pass

Source: Katie Dill on Lenny's Podcast, 2026-10-07, [How to scale intent, quality, and artistry with AI](https://youtu.be/GLvFTMtw4Jk). The notes are 28 bullets, counted in order below. They are a design philosophy, not a visual spec. The offer-rail mockup (engineering-sheet paper, hairline rules, Archivo, IBM Plex Mono, rail blue) is the visual direction for the owner journey only.

Surfaces checked at 390×844 and 1440×900:

- **Landing** — `affilio-wp/admin/build/home.html`, served as a Go template. Audited from a static render (`audit-landing-390.png`, `audit-landing-1440.png`). Not restyled.
- **Owner workspace** — `affilio-wp/admin/build/workspace.html`, `workspace.js`, `workspace.css`. The Go host parses `workspace.html` and injects the page body. This PR does not change that host.
- **WordPress** — Distribution, Marketplace, and Earnings in `affilio-wp/admin/build/`. They stay on WordPress admin chrome (`#dcdcde`, `#2271b1`, 13px), not the engineering sheet.

Verdicts: **pass** means the surface already met the rule or this PR made it true. **gap** means it does not, and the line says whether this PR fixed it or deferred it. **n-a** means the bullet is a process rule with no UI surface.

## Brand and point of view

| # | Rule | Landing | Owner | WordPress |
| --- | --- | --- | --- | --- |
| 1 | Write the point of view before designing | n-a | n-a | n-a |
| 2 | Layout, imagery, and copy signal what the product is | pass | pass | gap |
| 3 | One brand value in every detail | gap | gap | gap |
| 4 | Match the form to the signal (trust, security, play) | pass | pass | pass |

1. Process rule. No new brand essay was written. The existing line — a distribution channel for what you build, a revenue channel for what you publish — is what the rail and the landing both use.
2. Landing: the distribution scene plus "You built it. Give it a distribution channel." Owner: the six stations name the job (product, ready, Stripe test, offer, submitted, listed) instead of a generic dashboard. WordPress: Distribution copy is clear, but the screen is still a settings panel. A full opportunity-first redesign is deferred; it is a separate brief and would fight wp-admin.
3. Landing is ivory and moss. The workspace body is still terracotta on warm paper. The rail is rail blue on `#F4F3EF`. Those are three palettes. Unifying them is deferred so this PR does not restyle screens outside the offer journey.
4. The rail uses hairlines, one blue action, and a red remediation next to the failed check. Landing stays editorial and calm. WordPress errors use the admin red panel, which reads as a trusted admin warning.

## Quality bar and craft

| # | Rule | Landing | Owner | WordPress |
| --- | --- | --- | --- | --- |
| 5 | Same standard as handmade work | gap | pass | pass |
| 6 | Craft one level deeper than the user can name | gap | pass | gap |
| 7 | Small anticipatory details | pass | pass | pass |
| 8 | A bit of life and personality | gap | gap | gap |
| 9 | Troubleshooting built into the flow | n-a | pass | pass |
| 10 | Do not ship polish that does not solve the problem | pass | pass | pass |
| 11 | Own maintenance of what you ship | n-a | pass | pass |

5. The landing scene was not re-cut for this pass. The rail and the WordPress error placement were built to the same bar as the surrounding screens: real copy, focus, and a single next step.
6. Owner: station flags say Done / Now / Later, and a failing gate check quotes the remediation. Landing and WordPress did not get a second layer of detail beyond the error text. Deferred with the broader restyle.
7. Landing already shows a live marker on the scene. The owner shows "Assessment running. Reading the live pages." and "Checking live pages…" while a check is in flight. WordPress names the failed request and offers Retry.
8. No mascot and no new idle motion. Deferred. The landing scene is the existing personality, and this PR does not add decoration to the rail.
9. Landing has no failure flow. Owner: a failed readiness check, a submit rejection, a missing product list, a returned draft, and an in-flight assessment each have a visible next step. WordPress: Distribution, Marketplace, and Earnings failures render next to the control, and Marketplace no longer uses `window.alert`.
10. The landing was left alone. The rail exists to answer "what do I do next," not to reskin the app.
11. Rail, fonts, and WordPress behavior live in the existing UI files. Archivo and IBM Plex Mono are SIL OFL and recorded in `affilio-wp/admin/build/THIRD_PARTY_NOTICES.md`.

## Design systems for AI building

| # | Rule | Landing | Owner | WordPress |
| --- | --- | --- | --- | --- |
| 12 | Treat the system, not the screen, as the object | gap | gap | gap |
| 13 | Opinionated enough to feel handmade | pass | pass | gap |
| 14 | Ship full templates and flows | pass | pass | pass |
| 15 | Deliver design docs at the moment of use | n-a | n-a | n-a |
| 16 | Following every rule is the baseline, not the goal | n-a | n-a | n-a |

12. The rail styles are scoped to `.offer-rail`. There is no shared token file across landing, owner, and wp-admin. Deferred: a token pass would touch Go-served landing CSS and every admin screen.
13. Landing keeps its editorial type and scene. The rail refuses a second primary action. WordPress still inherits wp-admin, on purpose, so it does not pretend to be the engineering sheet.
14. The owner flow now runs product → readiness → Stripe test → offer → submit → listed, including the failure and empty states. WordPress Distribution, Marketplace, and Earnings have connected, empty, error, and retry states.
15. This file is a review note. It is not injected into a generator at build time.
16. Process reminder. The goal of this pass is one obvious next step and an error the maker can see.

## Editing and UX review

| # | Rule | Landing | Owner | WordPress |
| --- | --- | --- | --- | --- |
| 17 | Assign an editor | n-a | n-a | n-a |
| 18 | Review by using the full journey | pass | pass | pass |
| 19 | Ask whether it is fully formed | pass | gap | pass |
| 20 | Unexpected details and a continuous theme | gap | gap | gap |
| 21 | Justify every choice to the pixel | gap | pass | pass |

17. Process rule. No human editor was assigned in this change.
18. Playwright walks the owner journey at both widths, and opens Distribution, Marketplace, and Earnings at both widths. Landing was screenshotted at both widths.
19. The owner journey still depends on two facts the API does not return: Stripe connection (including test versus live) and the operator's review note. A new browser cannot see a Stripe confirmation or a returned draft until those fields exist. Landing and WordPress are complete for the states this PR owns.
20. The landing scene and the rail do not share a theme. Deferred with rule 3.
21. Rail colors are the mockup's paper `#F4F3EF`, ink `#201F1B`, hairline `#E3E1D8`, blue `#2352C9`, done `#1F7A46`, and fail `#B23A2F` / `#F9ECEA` / `#E5C4BF`. WordPress errors use the existing admin danger colors, not a new palette. Landing colors were not re-chosen.

## Motion

| # | Rule | Landing | Owner | WordPress |
| --- | --- | --- | --- | --- |
| 22 | Scrutinize animation frame by frame | gap | n-a | n-a |
| 23 | Use more viewpoints, then edit hard | n-a | n-a | n-a |

22. No new animation. The landing scene already pauses, respects reduced motion, and falls back when WebGL is missing. It was not re-timed frame by frame. Deferred.
23. Process rule for how the work was produced, not a property of a screen.

## Creativity and differentiation

| # | Rule | Landing | Owner | WordPress |
| --- | --- | --- | --- | --- |
| 24 | Do not default to chat, charts, or a CLI | pass | pass | pass |
| 25 | Prompt with beliefs and source material | n-a | n-a | n-a |
| 26 | Push one step further, then critique | gap | pass | pass |
| 27 | Humans set the direction, AI refines | n-a | n-a | n-a |
| 28 | Leave room for a strange idea | gap | gap | gap |

24. The mockup's KPI strip was not added. The owner surface is a rail. Landing keeps the distribution scene. WordPress stays a task screen.
25. The brief, the 28 notes, and the mockup were the source. That is process, not a screen.
26. The owner and WordPress paths are locked by an automated walk that fails if a second primary action appears or an error sits off screen. Landing was only audited.
27. The station list and the "one next action" rule came from the brief. Implementation followed that direction.
28. No experimental interaction was added. Deferred so the maker's next step stays obvious.

## Counts

| Surface | pass | gap | n-a |
| --- | --- | --- | --- |
| Landing | 9 | 10 | 9 |
| Owner workspace | 14 | 6 | 8 |
| WordPress | 12 | 8 | 8 |

## Fixed in this PR

- Owner offer rail with stations product → ready → Stripe test → offer → submitted → listed, exactly one primary next action, and done stations marked Done.
- Readiness and submit errors render beside the check or the button, with `role="alert"`, `aria-describedby`, and focus plus scroll into view.
- Empty, running, failed, returned-draft, and network states on the products page.
- Distribution, Marketplace, and Earnings: one primary action, inline errors, and a Retry on load failure. Marketplace no longer calls `window.alert`.
- Rail fonts are SIL OFL Archivo and IBM Plex Mono, embedded in `workspace.css`.

## Deferred, in order

1. Expose Stripe `connected`, `configuration_verified` (or `configuration_verified_at`), and `live_mode` on the owner product JSON, and allow `font-src 'self'` (or the embedded font) in the hosted content-security policy. Until then a new browser cannot know Stripe state, and the hosted page will not paint the rail fonts.
2. Persist `review_note` on an offer the operator returns to draft. This browser can only notice the return after it has seen the offer submitted.
3. One palette across landing, workspace, and the rail. Three palettes are still on screen.
4. Landing scene motion and a personality pass (mascot or equivalent). Out of the offer journey.
5. WordPress opportunity-first redesign. Distribution, Marketplace, and Earnings keep wp-admin.
6. A shared design system and in-tool design docs. Process rules 1, 15, 16, 17, 23, 25, and 27 have no screen to change.
