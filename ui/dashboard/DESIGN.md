# DESIGN.md: the Offer Rails design system (reusable template)

Version 1, 10 Oct 2026. These are the defaults for every project, starting with Offer Rails D-032 (setup, digest, approval modes).
To start a new project, copy this file, keep the rules, and change only the hues and the voice examples. Tokens live in
`assets/offer-rails.css` (`:root`), and the shared behaviour lives in `assets/ui-kit.js` (palette, help, panel, modal, key map, BLIP).

Sources: `/workspace/shared/resources/inspo/README.md` §4 (8 ideas from a PostHog and Amp study, approved by Victor 10 Oct 2026),
lessons 2, 3 and 5 of `/workspace/shared/resources/inspo/ampcode/offer-rails-lessons.md`, and `/workspace/shared/design-rules.md`.
We use them **for inspiration only**: no logo, mascot, font, illustration or copy from PostHog or Amp is used here. Our fonts are Archivo and IBM Plex Mono
(OFL subsets from the owner workspace) over system fallbacks. BLIP is our own character, and every sentence is our own.

**Brand value: calm control.** The owner can always see what is live, and can always take it back. Every choice below serves that value.

---

## 1. Tokens

Colours are RGB triplets, so one token gives any alpha: `rgb(var(--rgb-surface-2) / .6)`. Components use the semantic aliases
(`--paper`, `--panel`, `--tint`, `--ink`, `--muted`, `--line`, `--accent`, `--fail`…). They never use raw hex.

### 1.1 Surfaces (3 levels plus an inverse) and warm neutrals
| token | rgb | use |
|---|---|---|
| `--rgb-surface-0` | 247 247 239 | page background (`--paper`) |
| `--rgb-surface-1` | 253 253 247 | cards, panels, modals (`--panel`) |
| `--rgb-surface-2` | 236 239 226 | inset and quiet areas, skeleton bars, hover (`--tint`) |
| `--rgb-surface-inv` | 35 39 32 | toast, the inverted hover, shadows (as alpha) |
| `--rgb-text-1` | 35 39 32 | body text and data (`--ink`) |
| `--rgb-text-2` | 92 98 87 | secondary text, captions, eyebrows (`--muted`) |
| `--rgb-line` / `--rgb-line-strong` | 221 224 211 / 184 189 172 | 1px borders; field borders |

The neutrals are olive-warm, never cool slate: a warm page reads as *calm*.

### 1.2 One accent per concept
Each accent is reused on that concept's tab, badge, focus ring and empty state. Nowhere else.
| concept | text / fill | edge (keycap side) | soft (background) | where |
|---|---|---|---|---|
| **setup** | 35 82 201 | 26 61 150 | 232 237 250 | setup page, Continue keycap, setup focus ring, Continue setup pill |
| **digest** | 168 64 44 | n/a | 248 234 228 | digest page, digest focus ring, BLIP. `--rgb-digest-brand 198 79 56` is decorative only (no text) |
| **approvals** | 63 92 53 | 42 64 35 | 230 238 223 | approval modes, "live" state, the Approve keycap (`.keycap.approve`) |
| fail (state, not a concept) | 168 53 43 | n/a | 249 236 234 (+ line 229 196 191) | errors, refused, destructive confirms |
| caution (state) | 107 69 0 | n/a | 251 242 220 (+ line 217 191 128) | "needs a look", in-flight retries |

### 1.3 Contrast (WCAG AA: 4.5 for body text). Every pair below is in use and passes
| text colour | on surface-0 | on surface-1 | on surface-2 | other |
|---|---|---|---|---|
| text-1 | 14.11 | 14.88 | 13.03 | surface-1 on inverse 14.88 |
| text-2 | 5.84 | 6.16 | 5.39 | the v6 grey (105 112 99) was 4.39 on surface-2, so it was retired |
| setup | 6.25 | 6.59 | 5.77 | on setup-soft 5.74; white on setup 6.73 |
| digest | 5.68 | 5.99 | 5.24 | on digest-soft 5.21 |
| approvals | 6.98 | 7.36 | 6.45 | on approvals-soft 6.32; white on approvals 7.52 |
| fail | 6.08 | 6.41 | 5.61 | on fail-soft 5.68 |
| caution | 7.88 | 8.31 | 7.27 | on caution-soft 7.61 |

A rule for new tokens: compute the ratio before using a colour for text, and add the row here. Axe runs on every screen in the test suite
(after entrance animations settle) and must report 0 issues.

### 1.4 Type, radius and focus
- Headings use `--font-rail` (Archivo subset). Eyebrows, flags, `kbd` and IDs use `--font-flag` (IBM Plex Mono subset). Body uses `--font-sans` (system). Numbers use `tabular-nums`. H1 is tight (−1.5px). Never another company's brand font.
- Radius: 8px for cards, buttons and panels; 4px for chips and `kbd`; 20px for pills.
- Focus: a **3px solid ring** in the page's accent (`--focus`) with a 3px offset on every focusable element, including list rows
  (`article.digest-item` with tabindex −1). It is never removed.

## 2. Motion scale
| token | value | for |
|---|---|---|
| `--dur-color` | 150ms | colour, background and border changes (hover, selected row) |
| `--dur-move` | 200ms | movement: keycap lift, small transforms |
| `--dur-panel` | 300ms | panels, modals, sheets, palette |
| `--dur-fade` | 700ms | first-load fade of a whole screen, once |
| `--ease-out` | cubic-bezier(0,0,.2,1) | every **entrance** |
| `--ease-in` | cubic-bezier(.4,0,1,1) | every **exit** and the keycap press |
| `--ease-menu` | cubic-bezier(.16,1,.3,1) | the palette (a menu) |

- Entrances: panel slides from the right with a fade; modal pops in from scale .96; mobile sheet rises 24px from the bottom; the palette pops in from .96 in 200ms with `--ease-menu`.
- Nothing loops except BLIP's blink and the skeleton shimmer, and both stop under reduced motion.
- **`prefers-reduced-motion: reduce`**: every duration becomes 1ms, BLIP stops blinking (animation none), the skeleton stops
  shimmering, the keycap face stays put on hover and press (the colour still changes), and the rail dot is static. This is tested
  (`reduced_motion`), with snapshots `v7-reduced-motion-*`.
- Review new motion frame by frame (design-rules, Motion) before shipping.

## 3. The raised keycap primary button
```html
<button class="keycap" id="continue"><span class="keycap-face">Continue</span></button>
<span class="reassure">Nothing goes live without passing every check above. You can change any of this later from the dashboard.</span>
```
- Two layers: the outer `.keycap` is the *edge* (accent-edge colour, which shows as the side of the key). The inner `.keycap-face` is the
  accent face with white text. The face sits at `translateY(-4px)` at rest, `-6px` on hover and `-2px` while pressed. It lifts with `--dur-move`
  ease-out and presses with ease-in.
- **One per screen**, for the one primary action. Everything else is a flat `.btn` (secondary or quiet).
- **Always with a reassurance line** (`.reassure`) right next to it, saying what will *not* happen or how to take it back.
- Disabled: a grey face resting at −2px (it looks already pressed and doesn't react), and the reason sits beside it (`aria-describedby`).
- Variants: `.keycap` (setup blue) for *Continue*, and `.keycap.approve` (approvals green) for *Approve*. In D-032 only *Continue* uses it,
  because these screens contain no approval queue (see §10).
- Change the label by writing to `.keycap-face`, never to the button's `textContent`.

## 4. Keyboard map
Keys never fire while the user is typing in an input, textarea, select or contenteditable element, and never with Ctrl, Cmd or Alt
(except ⌘K / Ctrl+K). While a panel, modal or palette is open, only that overlay's keys work.

| key | where | does | guard |
|---|---|---|---|
| ⌘K / Ctrl+K | everywhere | opens or closes the command palette | always works, including from a field |
| ? | everywhere | keyboard help (lists every shortcut, and why one is unavailable now) | not while typing |
| J / K | lists | next / previous item (focus ring moves, row scrolls into view) | skips hidden rows |
| Enter | a focused list row | opens the item's side panel | on a button or link, Enter keeps its normal meaning |
| R | a focused list row | **undoes**: presses the row's own *Undo* or *Leave it removed* button | only when that button is visible and enabled, so never Redo, never a listing, never a failed removal; otherwise it says why |
| A | a focused list row | **approves** | only where an Approve action exists for this session. On D-032 nothing waits for approval, so A only says "Approving happens in the dashboard" and makes no call |
| Esc | overlay | closes it and returns focus to where it was | n/a |
| ↑ ↓ Enter | palette | move / run | disabled rows say why and don't run |

Keys never add power: each one presses a button the user could press, under the same rules (session vs key, per-currency, server confirmation).

## 5. Empty, loading and error states
**Empty states point to the next step.** Every empty state has 1) a plain line saying *when* things will appear, using the real date and time zone
when the server sends one, and 2) **exactly one** action. BLIP may sit here.
> All clear. Your next digest arrives on Sat 17 Oct at 10:00 (Europe/Rome), or sooner if something goes live. · [Copy the scan command]

**Setup can always be skipped** (lesson 2). *Skip for now* remembers the first unanswered question. The dashboard then shows one
*Continue setup →* pill ("You stopped at question 3 of 4. Your agent keeps working with what it has."), and the pill reopens setup at that question.

**Loading = skeletons in the real layout.** They use the same classes and grid as the loaded screen (totals tiles, project cards, item rows,
question cards), so nothing jumps. They carry `aria-busy` and an sr-only "Loading …" line, and shimmer only with motion allowed. Tested: the skeleton and
real totals have the same width, offset and column count.

**Errors say what happened, what didn't change, and what to do next.**
- A load failure says "Nothing changed on your sites" and offers one *Try again* that brings back the skeleton.
- Server refusals (409 and 422) are shown **verbatim** with the server's `message`, inline, next to the thing they refer to. Never as a toast.
- Undo and the kill switch say exactly what was reverted and what wasn't (lesson 5):
  - "Undone. Clicks are off now. Not yet reverted: the box stays in the post until ‹site› takes it out, and cached copies of the page may show it a little longer."
  - "Paused 1 site and 1 project. They ask you before anything new goes live, and tracked links stop counting clicks. Not changed: boxes already in posts stay there until you undo them."
- No optimistic states: a box shows as placed or removed only after the plugin confirms.

## 6. Panels and modals
- **Side panel** = one item's detail over the list (`.side-panel`, 460px, right edge, own scroll, the list stays put underneath). At ≤600px it is
  a **full-screen sheet**. It opens from Enter on a row or the row's *Details* button. It holds facts and the same actions as the row, and never money.
- **Modal** = settings and modes (`.modal`, centred horizontally at 12vh from the top, max 560px, **blurred backdrop** `backdrop-filter: blur(6px)` over a 32% inverse scrim (panels use an 18% scrim without blur, so the list stays readable)).
  At ≤600px it becomes a bottom sheet.
- **Palette** = a modal-style dialog at the top third, with one input, grouped commands and keys in `kbd` on the right.
- All overlays: `role=dialog` with `aria-modal`, a title, a Close button that shows `Esc`, **focus trapped** (Tab and Shift+Tab cycle inside),
  the rest of the page `inert`, Esc or a scrim click closes, and **focus returns** to the element that opened it. Only one overlay at a time.
  Tested on panel, modal, help and palette.

## 7. Voice
- **Playful only in microcopy**: empty states, BLIP's label, footers and help hints. One light touch at most per screen.
- **Plain and serious for data, money and states**: numbers, currencies, "Live", "Removed", errors, refusals, confirmations. No jokes, no
  exclamation marks, and no mascot near them.
- Say what happened, then what didn't, then what to do. Use the user's words ("your post", "their site"), not ours.
- Server messages are quoted as they are. We never rephrase a 409 or 422.

## 8. BLIP (the mascot)
BLIP is the team mascot: a blinking text cursor. Here it is drawn as an I-beam bar in the digest accent, with two dot eyes. It is an inline SVG
(`OR.ui.blip()`, about 40×56, `role=img`, label "BLIP, the Offer Rails cursor, waiting").
- **Empty states only.** Never next to money, totals, errors, refusals or confirms. Never more than one per screen.
- The blink (opacity 1 → .2 on a 1.1s step cycle, like a text cursor) is off under reduced motion.
- It is original artwork, so a new project may recolour it with its accent but must not swap in another company's character.

## 9. Landing hero (spec for later; there is no landing page in this repo)
Idea 4. Not built here, because this repo has only the D-032 dashboard screens. For the future Offer Rails page:
- **Real content in ambient motion**: 8–12 *real example decisions* ("Box approved on ‹post›: matched your rules", "Listing undone in 1 tap")
  as pills around the headline at three depths (far ones are smaller and blurred, near ones sharp), drifting slowly. *Or* one dash-offset
  "rail" line along which an offer travels into an approval check. **Use one loop only.**
- One keycap CTA ("Get started") with a reassurance line ("Nothing goes live until you approve it. Undo is one tap.").
- Headline and numbers serious. Jokes only in footers and counters.
- Reduced motion: the pills are static and the rail is drawn complete. First-load fade 700ms, once.
- Built with our tokens: surface-0 background and the digest accent as the brand colour. Nothing like PostHog's desktop or Amp's dark hero.

## 10. Where D-032 applies this, and what it doesn't yet
- Setup: keycap *Continue* plus reassurance, *Skip for now*, resume at a question, a skeleton, and a palette (save, skip, go to question 1–4).
- Digest: J/K/Enter/R/A, the palette, help, the side panel, the modes modal (the *Change* link opens it; changing the mode still happens in the dashboard),
  a skeleton, the "All clear." empty state with BLIP, the Continue setup pill, and the lesson-5 outcome copy.
- Not applied: the *Approve* keycap and a working A key (no approval queue or list endpoint in PR #21 fb493f0; `POST /decide` exists, but
  nothing on these screens lists items awaiting a decision); the real next-digest time in API mode (no schedule field, so the copy falls back to
  "as soon as something goes live"); a settings modal beyond approval modes (no settings read); and the hero (§9).
