# Offer Rails — gstack plan-design-review

**Date:** 28 October 2026  
**Method:** Garry Tan / gstack plan-design-review (principles, 0–10 ratings, 7 passes). Telemetry, AskUserQuestion, and design-binary steps skipped (headless). No code, copy, or tests were changed.  
**Subject:** public UI on branch `cursor/d116-plain-first-screen-a2d3` (D-116, PR #99), plus `docs/protocol`. Live hosted site is previous `main` (`436ce343`); this review is of the D-116 source, not that host.  
**Inputs read in full:** `gstack-method.md`, `offer-rails-victor-asks-ledger.md` (109 numbered asks + V77–V115), `offer-rails-vision-research-2026-10-16.md`, `d090-findings.md` (U01–U19), `design-rules.md` (1–38).  
**Surfaces read:** `ui/landing/src/{home,makers,publishers,publishers/setup,you,agents,protocol,directory,content,vision,surfaces,styles}`, markdown twins under `affilio-wp/admin/build/landing-v9/`, `affilio-api/internal/handlers/agent_docs.go` (`/llms.txt`, `/skill.md`, `/llms-full.txt`), `docs/protocol/{README,SPEC,GAPS}.md`. `ui/dashboard` (setup + digest) was not the target; it is named only where the landing promises `/owner`.

**Verdict: 4 / 10.** D-116 fixed the worst first-screen lies from the D-090 walk. The direction is still wrong. The site is a two-sided affiliate brochure with a Google door. The vision is a thin human page (limits, approvals, receipts) and an agent entrance (`/llms.txt`) that any writing agent can act on. Those two surfaces are not the product. They are annexes.

---

## Scope and what D-116 already changed

D-116 (this branch) already closed or narrowed several D-090 findings. The review does not re-litigate a fixed pixel as if it were still live on this branch.

| D-090 | On this branch |
|---|---|
| U01 first screen is API-first | **Fixed.** Hero is a money sentence plus `I sell a product` / `I run a site` above the fold. Agent line sits under that. |
| U02 “No Google sign-in” vs “Continue with Google” | **Fixed in copy.** `signInStory` is now “Agents join with no sign-in. People sign in with Google for the dashboard.” |
| U03 maker keeps €21.60 | **Fixed.** One split: sale €24.00 − publisher €2.40 − fee €0.24 = maker **€21.36**. |
| U11 setup “three steps” vs unnumbered preview | **Fixed.** Preview labelled “Optional: preview what an offer would look like”. |
| U19 decorative hero wash | **Fixed on home.** Shader unmounted; flat `bg-s0`. `HeroShader.tsx` is dead code. |
| U07 / U08 newsletter | **Partial.** Newsletter row links to `#add-yours`. Heading is “Add your site or newsletter”. The page still does not mint a link. |
| U09 / U12 mode names + YOLO | **Partial.** Human pages share `Ask me every time` / `Approve within my rules` / `Approve anything that passes the checks`. `/agents` still says Manual / Auto / YOLO. |
| U13 WordPress-only | **Partial.** “I'm not on WordPress” exists. It is a dead end: agent-only, no link. |
| U14 protocol 0.4 contradiction | **Partial.** One line is fetched from `/.well-known/orp.json`. Surrounding copy still hard-codes a 0.3 test deal. |
| U04, U05, U06, U10, U15, U16, U17, U18 | **Open.** |

A 4 is not “the type is nice.” It is: a stranger still cannot understand and try the product in 60 seconds, and the pages argue with the vision Victor wrote down.

---

## Initial ratings

| Dimension | Score | Why it is not 10 |
|---|---|---|
| Information architecture | **4** | First screen now has a promise and two role buttons. Then the same choice repeats. `/you/` — the human product — is not in the header. Vision sits below the fold as “Coming” cards. Three vocabularies for the same two people. |
| Interaction states | **4** | Paste-a-URL preview is specified. Directory maps 404 and network failure to “No makers listed yet.” `/you/` has no actable state. Setup steps stay “To do” forever when signed out. |
| User journey and emotional arc | **3** | Maker hits Google, then “this page has no form.” Publisher hits a WordPress zip. Newsletter writer is told “agent only.” Agent is dropped into a protocol dump. The feeling after five minutes is “I cannot do this.” |
| AI-slop risk | **5** | Better than generic SaaS (honest Coming labels, one sale example, Padova). Still a landing template: hero, two path cards, bento, 3-column surfaces, advantage grid, FAQ, tram. |
| Design-system alignment | **6** | Tokens, type, skip link, 44px targets, pause-motion, reduced-motion kills are real. Then: no keycap on the home primary, Blip next to money, leftover shadows, YOLO, dead shader, two agent URLs. |
| Responsive and accessibility | **7** | Strong technical a11y on the pages that exist. Mobile header + full-width nav eats the fold. Protocol and agents nav are afterthoughts. This pass did not re-run axe. |
| Unresolved decisions | **3** | The product has not chosen its first user, its first door, or its approval default. Those three will keep producing contradictory screens. |

**Overall: 4 / 10.**

What 10 looks like, in one sentence: a stranger pastes one line to the agent they already have, or opens `/you/` and sets a limit, and understands that other deal types are Coming — without picking a side, without Google, and without reading a spec.

---

## Pass 1 — Information architecture

**Rating: 4 / 10**

### What the stranger sees, in order

Home, first viewport (390 and 1440, as D-116’s own fold test asserts):

```
[sticky header]
  Logo | Makers  Publishers  Protocol  Agents | Pause  ⌘K  Sign in

What you get
Publishers earn on a sale. Makers pay only for that sale.
Agents find a post that fits and place one labelled link.
Agents join with no sign-in. People sign in with Google for the dashboard.
[ I sell a product ]  [ I run a site ]
One account can do both.
────────
For agents
This line is for an agent. The choice above is for a person.
Join with POST /v1/agent-tokens and send the bearer.
[Copy] [llms.txt]
```

Below the fold, same page:

```
What your agent does     (five deal types, four Coming)
What you keep control of
What it replaces
Pick your side           (Path A / Path B — same choice again)
Works anywhere you can paste a link   (all In testing)
How it works             (five affiliate-only steps)
See it on a real post.   (the only try)
Chapter A · money + 4+4 advantages
Chapter B · nine checks
Chapter C · FAQ
Padova tram
```

Constraint worship, three things only: **(1) what it does for me, (2) one action I can take now, (3) what is not live.** The page currently shows about fourteen.

### Hierarchy failures

1. **The human product is off-nav.** `/you/` is “limits, approvals, a receipt in plain words.” Footer and ⌘K know it. The header does not. A person who believes the vision never finds the page.
2. **The first action is a role pick Victor already rejected.** Ledger row 106 / V86: the Sell / Earn / Both picker “still feels so manual, so far off from my vision.” D-116 moved that picker *up*. That is the opposite of the ask.
3. **Three names for two people.** Hero: “I sell a product” / “I run a site.” Mid-page: “I sell something.” / “I publish.” Nav: Makers / Publishers. Newsletter is none of these.
4. **Two agent URLs.** Header `Agents` → `/agents`. Footer and ⌘K → `/landing-v9/agents/`. Same page, two addresses, leftover v9 path.
5. **`/llms.txt` is not an entrance.** It opens on a 0.3 test-deal walkthrough, then a 0.4 many-party walkthrough, then leftover device-login docs. The join line is far below. A cold agent will not “suggest Offer Rails while you write.” It will try to run a conformance script.
6. **Markdown twins are not twins.** `makers.md` and `publishers.md` reprint the home pitch. `agents.md` is three lines. An agent that fetches the page `.md` does not get that page.
7. **How-it-works is affiliate-only.** Five steps from product to paid sale. Vision cards two screens up list link swaps, cross-sells, upsells, reposts. The page teaches a smaller product than it claims.

### Recommended IA

```
Public site (thin)
  /                 one sentence + one agent line + link to /you/
  /you/             limits, approvals, receipts  (THE human UI)
  /llms.txt         15-line agent entrance
  /protocol/        spec (agents and implementers)
  /makers/          only if a directory has members
  /publishers/      only if a directory has members

Not first-class
  /publishers/setup WordPress is an adapter, not a door
  /agents           merge into /llms.txt + /llms-full.txt
  /landing-v9/*     delete the alias
```

**10 / 10:** every screen has a first, second, third. Home first is the try. `/you/` is in the header. One name per role. One agent URL. `/llms.txt` is short enough to finish.

**Decision needed (unresolved):** is the first-class human object a role (maker/publisher) or a limit sheet? The vision says the latter. The site is built as the former.

---

## Pass 2 — Interaction states

**Rating: 4 / 10**

What the user *sees*. Empty states are features (principle 1). Rule 31: empty says when it fills plus one action; error is not empty; server text is verbatim.

```
FEATURE                    | LOADING | EMPTY | ERROR | SUCCESS | PARTIAL
---------------------------|---------|-------|-------|---------|--------
Home join / Copy           | none    | n/a   | clipboard fail is silent or "Copied." only | "Copied." | n/a
Home Sign in (maker dialog)| n/a     | n/a   | none (Google is a navigation) | leaves the site | n/a
Paste-a-URL preview        | "Reading your post, up to 30s" | closest/unverified path + notify | 400/403/422/429 copy + next step | offer box in a slot | unread/private/empty handled
Directory list             | "Loading the directory." | "No makers/publishers listed yet…" | **mapped to empty** (404 and network) | cards + Load more | no "couldn't load"
Directory detail           | "Loading the directory." | 404 page | same 404 page | profile | n/a
/you/ limits               | n/a     | example copy only | none | none — no save | n/a
/you/ approvals            | n/a     | prose, no queue | none | none | n/a
/you/ receipts             | n/a     | "Example wording. Not a live agreement." | none | none | n/a
Publisher setup steps      | n/a     | always "To do" when signed out | none | never "Done" here | WP / not-WP split
Setup approval radios      | n/a     | Ask selected | save is a sign-in href, no failure | leaves the site | claims-check caveat in body
Maker "Add your product"   | n/a     | CLI prompt | none | clipboard | no form
Publisher "Add yours"      | n/a     | CLI prompt, "does not mint a link" | none | clipboard | no form
Protocol "This host"       | unknown line until fetch | n/a | stays unknown (correct) | "speaks ORP 0.4…" | rest of page still says 0.3
Agents Copy                | n/a     | n/a   | "Select the text and copy it." | "Copied" | n/a
Agents install tabs        | n/a     | n/a   | n/a | snippet | every tab but Generic is Untested
Command palette            | n/a     | Blip + "Nothing called…" | n/a | jump | hidden on small screens
```

### Findings

1. **Directory error is a lie.** `Directory.tsx` catch sets `view` to `empty`. U04 is still true on this branch. A 404 on `/v1/directory/makers` reads as “the marketplace is young.” It is a broken endpoint. Rule 31 is explicit: do not use the empty sentence for a failed load.
2. **`/you/` is a poster, not a state machine.** Limits are a static list. Approvals have no inbox. The receipt is labelled “Example wording. Not a live agreement.” There is no signed-out action except “Give your agent the join line.” U10 asked for “Sign in to set your limits.” Not done.
3. **Setup cannot succeed on the page that describes it.** Every step is hard-coded `todo`. Download / Connect / Save all leave. A signed-in return does not flip a pill. The page trains the user that Offer Rails setup is ornamental.
4. **Join Copy on home can fail closed.** `JoinPrompt` catch sets `copied` false and shows a blank status. Agents Copy is better.
5. **Preview is the one grown-up control.** Loading, 400, private, unread, 429 with a pause, closest-match + notify, verified box. This is what the rest of the site should look like. The notify button writes `localStorage` and says so — correct, because there is no endpoint.

**10 / 10:** every feature has the five columns filled with what the eye sees. Directory 404 ≠ empty. `/you/` empty is “no deals yet; your agent will show them here” plus one action. Setup pills reflect the account.

**Decision needed (unresolved):** when the directory API is 404, do we hide the section or show a failed-load state? Recommendation: hide the marketplace chrome until the endpoint is real. An empty bazaar is worse than no bazaar.

---

## Pass 3 — User journey and emotional arc

**Rating: 3 / 10**

Time horizon: 5 seconds visceral, 5 minutes behavioral, 5 years reflective (Norman). Storyboard before pixels (Gebbia). Two human roles plus the agent path, as asked.

### Role A — “I sell a product”

```
STEP | USER DOES                         | USER FEELS              | PAGE SPECIFIES?
-----|-----------------------------------|-------------------------|----------------
1    | Lands on /                        | “Maybe this gets me sales.” | Money headline. Role button above the fold.
2    | Presses I sell a product          | Brief competence        | Native <dialog>, four steps, then Google.
3    | Reads “This page does not have that form.” | Suspicion         | Maker dialog step 2, and /makers/ install section.
4    | Continue with Google              | Friction. Vision said no signup. | Honest sentence on the page. Still the only door.
5    | Arrives /owner?setup=maker        | Lost                    | Not in ui/landing. ui/dashboard/setup.html is a different system.
6    | Back to /makers/, tries Add your product | “I need Claude Code.” | One MCP command per client. All but Generic Untested.
7    | Looks at Example sale             | The numbers now add up. | €21.36 / €2.40 / €0.24. Trust, slightly.
8    | Looks at directory                | The product is empty.   | “No makers listed yet.”
9    | Five minutes later                | This is not for me.     | No add-product form. No way to try a listing.
```

5-second: terracotta, “pay only for sales” — clear if they already know they are a maker.  
5-minute: they have a Google session or a CLI they were told is untested. They have not listed a product.  
5-year: the site never showed another maker succeeding. Reflective memory is “empty marketplace + homework.”

### Role B — “I run a site / newsletter”

```
STEP | USER DOES                         | USER FEELS              | PAGE SPECIFIES?
-----|-----------------------------------|-------------------------|----------------
1    | Lands on /                        | “Can my posts earn?”    | Same money headline. Button says “I run a site” — not newsletter.
2    | Presses I run a site              | Forward motion          | Goes to /publishers/setup.
3    | Sees WordPress zip as step 1      | “I’m on Substack.”      | “I'm not on WordPress” is a <details>, not a branch.
4    | Opens that details                | Dead end                | “Agent only… this page does not mint a link.”
5    | Or stays on WP path, Connect      | Google again            | personSignsInBecause explains why. Still a wall.
6    | /publishers/ Newsletter row       | Hope                    | Links to #add-yours.
7    | #add-yours                        | Same dead end           | Paste a Claude command. No Get your link.
8    | Optional preview on setup         | The only magic          | Paste URL. If it works, a labelled box. Then “Approve your placements” → Google.
9    | Five minutes later                | I need an agent and WordPress, or I leave. |
```

Emotional arc is hope → adapter tax → shame (“you don’t have the right stack”). Victor’s publisher is also a newsletter, X, YouTube. The UI still treats WordPress as the real user and everyone else as a footnote.

`/you/` does not appear in this journey unless they read the vision block and click a text link. That is a storyboard failure: the page that is supposed to be their home is a side quest.

### Agent path

```
STEP | AGENT DOES                        | HUMAN FEELS (if watching) | PAGE SPECIFIES?
-----|-----------------------------------|---------------------------|----------------
1    | GET /llms.txt                     | —                         | Next: /skill.md. Then ~200 lines of ORP 0.3/0.4 walkthrough. Join is later.
2    | GET /skill.md                     | —                         | Points back to /llms.txt and forward to /llms-full.txt. U17 is still a loop at the top.
3    | GET /agents                       | Overwhelm                 | Bearer join, then steps 3–11 that need X-API-Key / X-Admin-Key / X-Plugin-Key / X-Maker-Key.
4    | Tries an install tab              | Untested                  | Cursor/Claude/VS Code/Windsurf all say untested. Generic is “401 no bearer” — a ping, not a flow.
5    | Reads /you/                       | Mixed                     | “No agent tool can approve.” After D-111 / option C, SPEC and /llms-full.txt say the agent approves ask by default.
6    | Suggests Offer Rails in a draft   | This is the vision        | Marked Coming. No prompt, no widget, no example utterance except a buried card.
```

The agent path is written for a protocol engineer. Victor asked for the Railway / Supabase moment: the agent already in the terminal offers Offer Rails when the human asks how to monetize. That utterance is not on `/llms.txt` in a form an agent will quote.

### Cross-role contradictions the journey hits

- **Approval default.** `/you/` and `vision.mjs`: “A person approves each placement. No agent tool can approve.” D-111 (option C, Victor “sì confermo”): agent approves ask by default; human approval is owner opt-in. A careful reader will not trust either sentence.
- **Signup.** Vision and V78: no emails, no website signups. First buttons still end at Google.
- **What the product is.** Home how-it-works: affiliate sale. Vision strip: any growth deal. Protocol: many-to-many seats. Three products, one URL.

**10 / 10:** each role has a 60-second success that does not require the other role, Google, or WordPress. The 5-year memory is “my agent asked, I set a limit, a receipt showed up.”

**Decision needed (unresolved):** which 60-second success ships first — paste a join line, set a limit, or get a tracked URL? Recommendation: join line + `/you/` limit. A tracked URL without an adapter is the publisher 60-second success and is the next build, not a landing rewrite.

---

## Pass 4 — AI-slop risk

**Rating: 5 / 10**

Principle 5: if it looks like every other generated site, it fails. Katie Dill (rules 1–28): form must signal what the product is; a system can satisfy every rule and still be dead.

### What is not slop

- One concrete sale, not “up to 10x.”
- Coming / In testing / Untested used as labels, not vibes.
- “We never hold your money” is consistent.
- Padova tram is specific. It is also not the product.

### What is slop

1. **The page type is a SaaS landing.** Hero, dual CTAs, two-column path cards, bento how-it-works, 3-column logo-less “where it runs,” 6-column advantage bento, numbered checks, accordion FAQ, illustrated footer. That sequence is the default AI landing. D-116 flattened the hero. It did not change the machine.
2. **Drift chips are fake activity.** “Hand-bound notebook · Listed / Matched / Checked / Placed / Earned” looping beside a product that has no public makers. Rule 32 wanted *real* product content. This is a storyboard of a sale that has not happened on the host.
3. **Advantage inventories.** 10 maker cells + 9 publisher cells. Several are the same fact (disclosure, undo, no cookies, traffic trade coming) restated. Constraint worship: if you can keep three, keep pay-on-sale, one labelled box, you set the limit.
4. **Magazine leftovers.** “Chapter A · Two sides, one rail.” “Issue 01” is gone; the chapter labels stayed. They do not earn the pixels.
5. **`/landing-v9/how-it-works/` railway figure.** A second how-it-works, animated, linked from the first. Two explanations of the same affiliate story.
6. **Command palette and M/P/J/H on a brochure.** Rule 29 is for approval lists. Here it is a keyboard skin on a marketing site. Playful in the wrong place (rule 35: play in microcopy and empty states, not as chrome).
7. **Shader leftover.** `HeroShader.tsx` still mounts a `bg-gradient-to-b` fade and a WebGL path. Unused, but it is the last-war artifact sitting in the tree.

**10 / 10:** the home page is as short as `/you/` plus one join line. Personality lives in the receipt wording and one mascot on a true empty state. No second how-it-works. No fake feed.

**Decision needed (unresolved):** kill Path A/B, Chapter labels, and the advantage grids on home, or keep them for “marketing completeness”? Recommendation: kill. They compete with the try.

---

## Pass 5 — Design-system alignment (`design-rules.md`)

**Rating: 6 / 10**

Tokens exist and are used: Archivo / Plex Mono, three surfaces, maker terracotta, publisher olive, setup blue, motion scale 150/200/300/700, `prefers-reduced-motion` wipes animation. That is why this is not a 3.

### Rule-by-rule on this branch

| Rule | Status on D-116 public UI |
|---|---|
| 29 keyboard-first lists | Palette and H/M/P/J exist. There is no approval list to operate. `J` is “join” which on home is an agent `<pre>`, not a keycap. |
| 30 one raised keycap + reassurance | `PrimaryCta` implements it. Home, makers, publishers, you, agents, setup **do not use it.** Home primaries are outlined role buttons. Reassurance line is missing next to them. |
| 31 empty / loading / error point forward | Preview yes. Directory no (error = empty). `/you/` no. |
| 32 hero from real product content; static under reduced motion | Shader gone. Drift chips remain, canned. Reduced motion respected. |
| 33 semantic RGB tokens, AA | Tokens in `styles.css`. Not re-measured here. |
| 34 side panel / modal | Maker dialog is a native modal (good). Directory detail is a full page (fine). |
| 35 play only in microcopy + one mascot; never next to money | **Broken.** Home FAQ (“Chapter C · Money, plainly”) renders `<Blip say="Money questions get plain answers." />`. Publishers FAQ does the same next to the cost question. |
| 36 written motion scale | Present. Pause-motion switch is real (WCAG 2.2.2). |
| 37 at most one WebGL, behind content, fallback | Home uses zero. `HeroShader.tsx` still contains the effect and a **decorative gradient** (`bg-gradient-to-b from-transparent to-s0`). |
| 38 no decorative glow / beam / aurora / gradient; copied components retuned | Drift chips use `shadow-[0_6px_18px_-10px_…]`. How-it-works active tile uses `box-shadow: 0 10px 24px -16px`. Residual glow. |

### New-component check

No new library components shipped in D-116. Existing shadcn `Button` / `Card` / `Input` are retuned to tokens. Keycap is defined and then avoided on the pages that matter.

**10 / 10:** one keycap on `/you/` (“Set these limits”), none on the brochure. Blip only on a true empty directory or empty receipt list. Delete `HeroShader` or keep it unused off the home CSS. No shadow on chips.

**Decision needed (unresolved):** is the keycap the sign-in, the Copy, or the `/you/` save? Rule 30 says one per screen. Recommendation: Copy on home (agent try), Save on `/you/` (human try). Sign-in is never the keycap.

---

## Pass 6 — Responsive and accessibility

**Rating: 7 / 10**

D-090 recorded axe 0 on 12 pages at 390 and 1440, no horizontal scroll, skip link, pause motion. D-116 added a fold test that the role buttons sit above the fold. Those are real. This pass is a source review, not a new axe run.

### What holds

- Skip link, `main` landmark, `aria-current` on nav, native `<dialog>` (maker + palette) with Esc.
- 44px minima on header, footer, role buttons, setup actions, directory cards.
- Install tabs are a `tablist` with arrow keys.
- Protocol tables have captions. Many-to-many SVG has `title` + `desc`.
- `prefers-reduced-motion` and `[data-motion=paused]` both stop loops.
- Preview status is `aria-live="polite"`.

### What does not

1. **Mobile header is a second page.** Logo + Pause + Sign in on row one; Makers / Publishers / Protocol / Agents wrap onto a full-width second row. Sticky. On 390 the fold test still passes, but the content column starts late. `/you/` is absent, so the expensive row is the wrong four links.
2. **“I run a site” is the wrong label at every width.** A newsletter writer at 390 never sees themselves. Adding “or newsletter” is an IA fix that is also a mobile-scan fix.
3. **Agents TOC.** Horizontal chip scroller on small screens, sticky under the already-sticky header (`top-32`). Two stickies. The page is a long form with nine sections; the first screen is “Copy endpoint.”
4. **Protocol nav.** Hidden behind “Pages” on small screens. Correct pattern. The overview still leads with schema talk, so the button is “open the rest of the spec,” not “what can I do.”
5. **Palette hidden below `sm`.** `hidden sm:inline-flex`. Keyboard `?` still opens it. A phone user never learns that. Fine if we stop treating the palette as a feature.
6. **Directory filters are three text fields + Search.** No select for language. Touch is adequate. The control is pointless while the list is empty.
7. **Contrast of `text-t2` on `bg-s0` / chips** was not re-measured. Do not claim AA for D-116 without a pass.
8. **Markdown twins** are not equivalent experiences. An agent or a text-only client on `/makers.md` gets the home story.

**10 / 10:** header on 390 is Logo + Your limits + Sign in (or no Sign in). Agents and protocol collapse to one “For agents” link. Fold content is the try, not the role pick. Axe re-run on this branch before merge of any UI follow-up.

**Decision needed (unresolved):** keep four header links on mobile, or two (Limits, Agents)? Recommendation: two.

---

## Pass 7 — Unresolved design decisions

Writing them down does not approve a fix. Each row will produce the wrong screen if an engineer guesses.

```
DECISION NEEDED                              | IF DEFERRED, WHAT HAPPENS
---------------------------------------------|---------------------------
Is the first door an agent line or Google?   | Home keeps two products. Victor’s “no signup” stays a caption under a Google button.
Is /you/ the human app or a manifesto?       | Limits stay example copy. Receipts stay fake. People are sent to /owner (another UI).
Option C vs “no agent can approve”           | /you/, vision.mjs, /agents, SPEC, /llms-full.txt keep disagreeing. Trust dies on contact.
What is the 60-second try?                   | Preview, Copy, Google, and WP zip compete. None complete.
Do we show a directory with 0 members?       | Empty bazaar stays the first impression on /makers/ and /publishers/.
Is WordPress a door or an adapter?           | Setup stays the publisher product. Newsletter stays a <details>.
One mode vocabulary, including /agents       | YOLO remains on the agent page next to money-adjacent steps.
One agent URL                                | /agents vs /landing-v9/agents/ vs /llms.txt vs /skill.md vs /llms-full.txt.
Do markdown twins have to match the HTML?    | makers.md / publishers.md keep teaching the wrong page.
Ship in-flow “monetize this post?” as UI     | Vision card stays Coming. Agents have nothing to quote. V77 stays a sentence.
Open-source / GitHub / license in the chrome | Protocol says “Apache-2.0 planned.” No repo link. V82/V83 not visible.
Is the tram / Blip / palette part of v1?     | Craft time keeps going to chrome. The try stays last.
```

Recommendation on each, for the record, not as an approval:

1. Agent line is the door. Google is “open the dashboard” in a details block.
2. `/you/` is the app. Wire it to the same session `/owner` already uses, or stop sending people to `/owner`.
3. Say option C in one sentence everywhere: “Your agent may approve within the limits you set. You can require a tap.”
4. 60-second try = Copy the join line. Second try = paste a URL. Third = get a tracked link (needs a backend).
5. Hide the directory until `total > 0` or the endpoint is healthy.
6. WordPress is step 2 of an adapter, not step 1 of the product.
7. Delete YOLO from public copy. Use the three human names on `/agents`.
8. Canonical agent URL is `/llms.txt`. `/agents` redirects or becomes a short human restatement.
9. Twins must be generated from the same strings as the page, or do not serve them.
10. One boxed example utterance on home and in `/llms.txt`: `this fits your post, want to monetize it?`
11. Do not put GitHub in the header until the public repo exists. One line on `/protocol/` is enough: “Draft. Apache-2.0. Repo not public yet.”
12. Freeze chrome. Spend the next UI pass on `/you/` and `/llms.txt`.

---

## Direction (blunt)

The current direction is a **marketplace brochure for a marketplace that is not live**, sitting on top of an **agent protocol that is live-ish**, while the **human control surface the vision named is a static article**.

That shape is how you lose the 60-second test.

Victor’s product, in his words (V86, V77, V78, row 106):

- Any terminal agent, asked “how can I monetize this content?”, surfaces Offer Rails.
- No account required.
- Thin layer, distributed everywhere.
- In-flow: “monetize this post?”
- Human keeps limits. Agent does the rest.
- Replaces outbound email bots with agent-to-agent deals (affiliate, swap, cross-sell, repost).
- The Sell / Earn / Both picker is “so far off from my vision.”

What shipped in the UI:

- A Makers / Publishers magazine.
- Google as the primary human verb.
- WordPress zip as the publisher verb.
- Empty directories with search boxes.
- A how-it-works for one deal type.
- `/you/` as documentation of a dashboard that lives elsewhere.
- `/llms.txt` as a protocol appendix.

D-116 made the brochure *honest*. Honesty is not direction. An honest brochure of the wrong product is still the wrong product.

Two things we built that the vision does not need, and that actively hide it:

1. **The role picker as the first screen.** It forces a SaaS identity (I am a maker / I am a publisher) onto a protocol that is supposed to start with “tell your agent.”
2. **The empty public directory.** Victor asked for it because the site looked empty (row 72). Shipping the chrome without members makes the emptiness official.

Craft that can wait: Padova, Blip-on-FAQ, command palette, second how-it-works, advantage bentos, shader file, landing-v9 path names.

---

## Vision ledger — asks the UI does not yet serve

Status in the 21 Oct ledger is often “missing/partial” for backend reasons. This table is **UI only**: if a stranger or an agent cannot do the ask from a page we ship, it is listed. “Smallest UI change” is the least surface that would make the ask visible and usable, not the full backend.

Process, deploy, model-routing, and security-rotation asks have no UI surface. They are listed at the end so nothing is silently dropped.

### Human and agent doors

| # | Ask (compressed, his words in quotes where it matters) | Smallest UI change |
|---|---|---|
| 4, 7, 34, 91 | Target agents; “keep the human out of the loop”; agents work under human rules | Make `/you/` the header item. Home primary = join line, not Google. One sentence for option C. |
| 23, 40, 71, 95, V86 | “when a user asks how to get more clients”; agents recommend us like Railway; “easiest setup”; “join with one line”; any terminal agent surfaces us | `/llms.txt` opens with the one-line join + the monetize utterance. Delete the 0.3 novel from the top. |
| 29, 22, 75, V111 | Effortless; auto/YOLO offered; per-step modes; option C default | `/you/` is a real control: default copy matches option C; the three human mode names; save. |
| 44, 63, 106 | “from what is this to I'm using it”; “two simple buttons”; picker “so manual” | Remove the picker from the first screen. Two buttons if any: Copy join line, Set limits. |
| 78, V78 | `/mcp` without Google; “no emails and no website signups” | Sign-in moves into a `<details>` labelled “Open the dashboard.” Agent copy never says to sign in. |
| V77 | In-flow “monetize this post?” | One boxed example on `/` and in `/llms.txt`. Status Coming until the adapter exists. Do not hide it in a card grid. |
| 69 | Section for agents doing discovery | `/agents` becomes a short restatement of `/llms.txt`, not a second product. |

### Surfaces and deal types

| # | Ask | Smallest UI change |
|---|---|---|
| 11, 37, 46, 47, 101, V80 | Any audience: YT, X, LinkedIn, newsletter, TikTok, “any digital platform” | Relabel the hero button “I publish (site, newsletter, or social).” Surfaces list stays, but each row that is not WP ends on “Get a link,” not “In testing.” |
| 13, 7 (U07) | Native newsletter placement | On `/publishers/`, Newsletter → a page that copies a tracked URL (even a test URL). |
| 14, 51, 52, 55, 103, V79 | Site-to-site exchange, no-account traffic, link swap, any no-sale exchange; replace outbound bots | Keep Coming, but give each deal type one sentence of “what your agent would send” and a protocol example link. Stop burying them under affiliate how-it-works. |
| 17 | Placement in the right section | Preview already returns a slot. Show the slot name as a choice, not only a label. |
| 18, 82 | Dynamic native copy; “content problem” | One sample of generated-vs-offer-data copy, capped at 140, with the disclosure on. Or do not mention native. |
| 48, 64 | Shopify later; WP marketplace so no manual zip | Setup step 1: “WordPress.org listing is not live. This zip is the current install.” Honest, one line. |
| 79 | Preview any public link; “no match” must not happen | Already the best control. Promote it to home fold two, under the join line. |

### Marketplace, matching, reputation

| # | Ask | Smallest UI change |
|---|---|---|
| 25, 72, 73 | Automatic listing; “website looks/is empty”; agents add the data | Hide `#directory` while `total === 0` or the GET is 404. Show only “Your agent lists you” + the prompt. |
| 50, 104 | Reddit-style karma | A single “Reputation (not live)” line on `/you/` and on a future profile. Do not build a leaderboard chrome. |
| 45, 84, 85 | Manufact-like catalog / harness / leaderboard | Do not add a leaderboard to an empty directory. One “Top offers (not live)” is enough if we must show the ask was heard. |
| 54 | On-page one-pass recommender | A labelled slot on the preview card: “Scored in one pass (not live).” |
| 98 | Warm-path “why it fits” | Preview already has closest programs. Change the heading to “Why this is close” and one reason sentence. |
| 12 | Clusters of customers without disclosing them | No UI until the card exists. Do not put “audiences” on the landing. |
| 33 | Agent views count as impressions | A footnote on the receipt: “A view by an agent can count. Not live.” Only if we will ship it. |

### Protocol, distribution, money

| # | Ask | Smallest UI change |
|---|---|---|
| 5, 36, 42, 61, 100 | Real protocol, anyone can implement, nav to protocol/GitHub, “not even close” to MCP’s docs | `/protocol/` is the one place that is close. Finish “This host” from `orp.json` only. Add “Repo not public yet. Apache-2.0 planned.” No fake GitHub link. |
| 6 | Confirm one name | Stop shipping `distributoor-mcp` / `affilio.zip` in public UI. The zip is already `offer-rails.zip` in D-116 setup. Keep it. |
| 39, V82, V83 | Open source + monetize; Apache-2.0; public repo | One line on `/protocol/` and footer. Not a marketing badge (R29 bans “open source” on some surfaces — the legal line can say Apache-2.0). |
| 41, 65, 66 | CLI, MCP, ChatGPT/Claude/Cursor marketplaces, npm | Install tabs stay. Mark the truth. Add “npm offer-rails is not published” once, at the top of Install, not per tab. |
| 21, 32 | MCP on ChatGPT, Claude, Cursor, Grok; Muse/dots | Coming labels already. Do not add more tabs. |
| 10 | Payment-agnostic, never hold money | Already clear. Keep. |
| 87, 89 | Compare vs Google/Meta/affiliate | `/compare` is 404. Do not build it before the try works. A stranger does not compare empty products. |
| 96 | Agent wallets | No UI. A FAQ row would be invention. |

### Insights and “the core engine”

| # | Ask | Smallest UI change |
|---|---|---|
| 53, 58 | Insights for agents; improve conversion | A `/you/` “Results” empty: “When a deal runs, the receipt and the count land here.” |
| 68 | Vibesales discovered / VibeCell promoted | Do not put a customer name on the public site. If we need proof, a redacted receipt on `/you/`. |
| 81 | Do not sell it as revolutionary | D-074 copy is fine. Do not reintroduce the word. The banned-word test already guards it. |

### Out of UI scope (logged so they are not “lost”)

Done or process: 1–3, 8, 16, 20, 27, 31, 38, 49, 56, 57, 59, 62, 67, 70 (test ask), 74 (Context7), 76, 83, 86, 88, 99 (this review), 109, V81, V84–V85, V87–V115 notes that are deploy/merge/CI/hackathon/SDK/security.

Backend-only until a page can tell the truth: 9 (PII enforcement), 19 (other agents give feedback), 24/28 (readiness depth), 30 (17 Oct deadline), 35 (no competing with the user’s software), 43 (harness matrix), 77 (Italian matching), 80 (unverified-program crawler), 90 (persona panel — D-090 did a walk, not Victor’s panel), 92–94 (Aside, agents-blocked, Reason Network — research), 97 (Happenstance analysis), 102 (capability handshake on the wire), 105 (which plugins), 107–108 (speed / multi-model pass), V109–V110 hackathon, V112 Agent SDK, V113 Reason Network, V114 deploy, V115 this review request.

---

## Built that the vision does not need

These are not “nice extras.” They spend the first minute of attention.

| Built | Why the vision does not need it | What to do |
|---|---|---|
| Role picker as the first CTA | Victor named it as the thing that feels manual (106). Agents do not pick Sell/Earn. | Demote or delete. |
| Empty `/makers/` + `/publishers/` directories with search | A search box over zero rows is a broken store. | Hide until members or a healthy API. |
| 10 + 9 advantage grids + Path A/B + Chapter A/B/C | Brochure of affiliate SaaS. Vision is a thin layer. | Cut from home. Keep one money sentence. |
| `/landing-v9/how-it-works/` railway | Second explanation of the same five steps. | One how-it-works, or none if `/` is the try. |
| Command palette + H/M/P/J on marketing pages | Built for a digest list. There is no list. | Keep `?` if you want. Do not sell it. |
| `HeroShader.tsx` + drift-chip shadows | Shader is unused. Chips fake a live rail. | Delete or quarantine. |
| Google as the raised action on every human path | V78, row 95, row 106. Dashboard auth is not join. | Details block. |
| WordPress zip as publisher step 1 | WP is “just the entry point” (37). UI made it the product. | Adapter, after “get a link.” |
| `/you/` as a static article that repeats the API join line | The vision’s human UI is controls and receipts. | Make it the app. |
| `/llms.txt` as a spec dump | Entrance, not appendix. | 15 lines, then `/llms-full.txt`. |
| Blip beside “Money, plainly” | Rule 35. | Move or remove. |
| `ui/dashboard` as a second visual language behind `/owner` | Landing promises a dashboard it does not show. Two design systems. | Either bring `/you/` up to that job or send `/you/` into `/owner` and stop pretending they are different products. |

---

## Top 10 changes (impact on “stranger understands and tries in 60 seconds” / effort)

Ranked by impact per effort. Evidence is a path or a quote from this branch.

### 1. Home first screen is a try, not a side pick — **S**

- **Page:** `/`
- **Evidence:** `ui/landing/src/home/App.tsx` — headline “Publishers earn on a sale. Makers pay only for that sale.” then `SignInChoice` (“I sell a product” / “I run a site”). Victor row 106: picker “still feels so manual.”
- **Fix:** Fold = one sentence of the in-write offer + `JoinPrompt` + link to `/you/`. Role buttons move below, or go. Google inside “Set it up yourself” (already a `<details>` in `JoinForm`).
- **10 / 10:** At 390, without scrolling, a stranger can copy the join line or open limits. They are not asked who they are.

### 2. `/you/` is in the header and does something — **S**

- **Page:** `/you/`, header
- **Evidence:** `ui/landing/src/you/App.tsx` — “Your agent joins with POST /v1/agent-tokens and a bearer.” No save. `Header.tsx` omits `/you/`. `siteCommands` already has “Your limits.”
- **Fix:** Header link “Your limits.” Remove the API sentence. One button: “Sign in to set your limits” (or, if option C is the story, “Change when your agent may approve”). Empty receipt: “No deals yet.”
- **10 / 10:** A person can explain the product as “that page is where I set the rules.” They can press one control.

### 3. `/llms.txt` is 15 lines — **S**

- **Page:** `/llms.txt`, `/skill.md`, `/agents`
- **Evidence:** `affilio-api/internal/handlers/agent_docs.go` — file starts `Next: /skill.md` then `## ORP 0-3 test deal`. `skill.md` points back. `facts.mjs` steps 3–11 still require `X-API-Key` / `X-Admin-Key`.
- **Fix:** Top of `/llms.txt`: what it is; join one-liner; “if they are writing, offer: this fits your post, want to monetize it?”; wait for `/you/` limits; link to `/llms-full.txt` for the walkthrough. `/skill.md` only forwards. `/agents` shows Bearer → `/mcp` as the path; REST keys behind `<details>`.
- **10 / 10:** A cold agent can join and know the one human-facing suggestion without parsing 0.4 seat rules.

### 4. Hide the empty marketplace — **S**

- **Page:** `/makers/`, `/publishers/`
- **Evidence:** `Directory.tsx` empty copy “No makers listed yet.” Catch maps failures to `empty`. U04 open. Row 72.
- **Fix:** If fetch fails, “Couldn’t load the directory” + verbatim status. If `total === 0`, do not render search. Render only Add yours. Prefer hiding the whole `#directory` heading until live.
- **10 / 10:** A stranger never sees a search box over nothing and never mistakes a 404 for “you can be first.”

### 5. A person can copy a tracked link without WordPress — **M**

- **Page:** `/publishers/`, `/publishers/setup`
- **Evidence:** `surfaces.mjs` Newsletter: “Agent only for now… Then paste it in an email.” Setup `<details>`: “this page does not mint a link.” U07, U13.
- **Fix:** “Get a test link” → sign-in or a host-minted demo `/v1/go/…` shown in a copy field. “I'm not on WordPress” is a first-class branch, not a disclosure.
- **10 / 10:** A newsletter writer leaves with a URL in the clipboard in one minute. Plugin is optional.

### 6. Cut the home brochure below the try — **S**

- **Page:** `/`
- **Evidence:** `App.tsx` Path A/B after `SignInChoice`; “Chapter A/B/C”; `HowItWorks` five affiliate steps plus a second railway page.
- **Fix:** Keep Vision (Coming labels) as one short list, preview, money sentence, FAQ. Delete Path A/B, chapter chrome, advantage teasers, or move advantages to `/makers/` `/publishers/` only.
- **10 / 10:** Scroll depth to the first try is one screen. The page is no longer a generated landing.

### 7. One approval story, including option C — **S**

- **Page:** `/you/`, `/`, `/agents`, `/publishers/setup`
- **Evidence:** `you/App.tsx` and `vision.mjs`: “No agent tool can approve.” `docs/protocol/SPEC.md` §1 and `llms-full` suffix: agent approves ask by default (`bound[].authority`). `facts.mjs` still exports YOLO.
- **Fix:** One sentence everywhere. Human names only. YOLO gone. Agents page modes use `approvalModes`.
- **10 / 10:** A lawyer, a maker, and an agent read the same default.

### 8. Show the in-write suggestion as the product picture — **M**

- **Page:** `/`
- **Evidence:** `vision.mjs` `agentDoesLead`: “this fits your post, want to monetize it?” Status Coming. It sits in a card grid under the fold. How-it-works picture is a railway.
- **Fix:** Replace drift chips with a static writing frame and that one utterance. Coming badge on the frame. Reduced motion = static.
- **10 / 10:** At 5 seconds the stranger understands the product is an offer *inside writing*, not a marketplace they must join.

### 9. Protocol “This host” is only from discovery — **S**

- **Page:** `/protocol/`, Concepts, Quickstart, Many
- **Evidence:** `host-speech.tsx` can say “This host speaks ORP 0.4 (leave and seat).” `Docs.tsx` Overview still: “It answers hello, discover, offer, accept, and execute for a 0.3 test deal.” Quickstart: “orp lists 0.3.” U14 partial.
- **Fix:** Every “this host” sentence is a function of `orp.json`. If the doc is missing, the unknown line only — no 0.3 claim.
- **10 / 10:** `/protocol/`, `/llms.txt`, and `orp.json` cannot disagree.

### 10. Maker without an agent gets one honest door — **M**

- **Page:** `/makers/`
- **Evidence:** `makers/App.tsx`: “There is no add-product form on this page.” Dialog step 2 repeats it. U06.
- **Fix:** Either a signed-in add-product form (minimum: name, URL, commission) or the page’s only CTA is “You need an MCP client” + Copy. Stop implying a dashboard will add the product.
- **10 / 10:** A solo maker knows in ten seconds whether they can start today.

---

## D-090 leftovers, mapped

Still wrong on this branch, in order of 60-second damage:

1. U15 — `/agents` key soup (Bearer vs X-API-Key vs X-Admin-Key).
2. U07 / U13 — no person-minted link.
3. U04 — directory 404 = empty.
4. U10 — `/you/` still API-speak, no action.
5. U06 — no maker path without an agent.
6. U05 / U16 — more than one install story; untested.
7. U09 leftover — YOLO on `/agents`.
8. U17 / U18 — `/skill.md` ↔ `/llms.txt`; no glossary at the top of `/llms.txt`.
9. U14 leftover — protocol prose vs `orp.json`.
10. U08 leftover — empty publishers, CLI-only add.

Fixed on this branch and not to be reopened without new evidence: U01, U02, U03, U11, U19.

---

## What 10 / 10 is

Not a prettier tram. Not a fuller protocol site.

A stranger opens `/`. They understand: an agent they already have can offer a labelled deal on something they are writing, inside limits they set, with a receipt they can read. They copy one line or set one limit. They are told, in the same breath, that swaps and reposts are Coming and that money is not held here.

Until that is the first screen, iterate on the wrong object.
