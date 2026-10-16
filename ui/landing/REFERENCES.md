# Offer Rails landing v9: references (research only, no code)
Compiled 16 Oct 2026 by Grok Bot. Sources: X API (search_posts_all, get_posts_by_ids), WebSearch, plus curl checks (HTTP 200 on 16 Oct unless noted). Read-only: nothing posted, no sign-ups, no purchases.
Also read: `/workspace/shared/design-rules.md` (rule 37) and `/workspace/shared/resources/inspo/README.md`. The Amp and PostHog notes below are summarised from that README; no new capture was made.

## 1. City-skyline footer trend
Most of these posts are from Sep to Oct 2026. Several people call it "a new trend for footers".

| Post | What it is | How it's made |
|---|---|---|
| https://x.com/BidyutKundu12/status/2098286734245237145 (11 Sep, 84 likes) | An illustrated London-landmarks skyline in the footer of https://opensourceui.in | A static illustration, line/flat style |
| https://x.com/tomaruxai/status/2098826837808033824 (12 Sep) | The NCR skyline with a Delhi Metro train gliding across it in a loop. Built with Google Antigravity | One static 1376×768 elevation backdrop plus a separate 4,924px train sprite, moved only with GPU-composited CSS transforms using container-query units (100cqw). No canvas, no video. (The live site, ncrstartups.in, did not respond, so it is dropped.) |
| https://x.com/tadeodonegana/status/2107681890434159071 (7 Oct, 47 likes) | A pixel-art scene of his hometown with ambient animation and a reveal on scroll | An AI agent plus the "Taste Engine" MCP, prompted to find "premium pixel-art or illustrated footers" and then redesign his footer |
| https://x.com/krishna_dxx/status/2109604969980805300 (12 Oct) | The Jaipur skyline drawn in a blue-pottery motif | An illustration |
| https://x.com/uzair_hasnain_/status/2105373711398551672 (30 Sep) | Pixel horses living in the footer, with hover, hold and click interactions | Pixel sprites |
| https://x.com/ramacharya06/status/2105224382054551731 (30 Sep) | A related trend: a giant name in the footer that flickers pixel by pixel, inspired by the footer on https://www.langchain.com | — |
| Framer "Pixel Skyline Footer" by Karim Saif: https://www.framer.com/marketplace/components/pixel-skyline-footer/ | A procedural pixel skyline with a Tetris mini-game | **Paid ($3), so not usable.** Reference only |
| Tools: pppixelate (fffuel), https://www.uwarp.design/pppixelate. Illustrator pixel workflow and SVG seam fix: https://getillustrations.com/blog/vector-pixel-art-in-adobe-illustrator-tutorial-how-to-create-it-export-it-and-fix-svg-gaps/ | | |

### Top 3 approaches for Padova
1. **Layered SVG elevation with a CSS-only ambient detail** (the tomaruxai technique). Draw a flat or line skyline of the Basilica del Santo (domes and minarets), Palazzo della Ragione (keel roof), Prato della Valle (the oval of statues and the canal) and the Specola tower. Animate one small element with `transform` only, for example a tram on the Padova SIR1 line or a bicycle crossing, or a slow cloud drift. Effort: low to medium (1–2 days, with the illustration as most of the work). Fit: **best**. It's light, crisp, the static fallback is free (it's simply the SVG without the moving layer), and it doesn't conflict with rule 37 because there's no shader.
2. **Pixel-art scene** (tadeodonegana / pppixelate). An 8–12 colour palette, drawn on a grid and exported as SVG or PNG, with a few twinkling windows or a pigeon on Il Santo. Effort: medium. Fit: good and playful, but it risks reading as a "dev portfolio" look rather than a serious brand. Fix the SVG seams, or ship a PNG.
3. **Three.js / 3D diorama** (MengTo's threeui and Hyper3D style). Effort: high, and heavy. Fit: weak. It competes with the hero's single permitted effect (rule 37's spirit), costs performance, and needs a raster fallback anyway.

**Recommendation:** go with #1. Make an inline SVG with `aria-hidden="true"` and a visually hidden caption like "Padova skyline: Basilica del Santo, Palazzo della Ragione, Prato della Valle, La Specola". Animate a single transform loop of at least 30s. Use `@media (prefers-reduced-motion: reduce){animation:none}` and pause it with IntersectionObserver when it's off-screen. Draw the illustration ourselves or commission it, and never trace copyrighted photos.

## 2. Hero references (hero section = big top visual plus headline)
- **Shaders** (MengTo, 10 Oct, 1,276 likes): https://x.com/MengTo/status/2108924473265275003. Repo: https://github.com/shader-effects-inc/shaders, site: https://shaders.com. This is the source of rule 37: free parts only, one effect per page, and a static fallback.
- **ThreeUI** (MengTo): https://threeui.com. Over 450 three.js scenes; the post says 150 are free and open source: https://x.com/MengTo/status/2103155074705019158.
- **Seijaku**, a three.js house-tour landing page (MengTo): https://seijaku.mengto.here.now. Post: https://x.com/MengTo/status/2099898247959691523. Scroll-driven camera, under 1 MB excluding images.
- **ASCII architecture studio** (viktoroddy): https://architecture-ascii-studio-main-2.vercel.app/. The building is made of text, the camera dives in as you scroll, and a cube follows the cursor. Post: https://x.com/viktoroddy/status/2109463155915251954. The prompt is at https://motionsites.ai/?prompt=ascii-studio.
- **motionsites.ai** (viktoroddy's prompt library, which claims over 500 prompts). Example: the 3D fox 404 page at https://motionsites.ai/3d-websites?prompt=lost-little-explorer (post: https://x.com/viktoroddy/status/2110919540771291589). Some content may be paywalled; I didn't sign up.
- Other relevant posts that aren't verified sites: MengTo's three.js plus Mobbin MCP tutorial, https://x.com/MengTo/status/2103491024287084744, and his exploded-view product sites, https://x.com/MengTo/status/2092275643623109037. viktoroddy's "scroll and the ground opens up", https://x.com/viktoroddy/status/2109984169845821651.

**Recommendation:** use a Shaders free effect behind the real product UI (an offer card or digest crop) with a static fallback image. That's the one effect for the page. The three.js and ASCII heroes are inspiration for *scroll storytelling below the hero*, done in CSS only.

## 3. Amp and PostHog motion (from inspo/README.md)
- **Amp:** a dark hero with drifting example-prompt pills at several depths (blur, scale and fade), a logo shimmer, and an orbit diagram of event sources. App transitions are quick and hovers are subtle. To copy: show real example offers drifting around the headline, rendered static under reduced motion.
- **PostHog:** 0.7s ease-out first-load fades, 0.2s transform hover lifts, 0.15s colour transitions, accordions that animate `grid-template-rows`, a window pop-in (scale .92→1), an SVG dash-offset "data flow" line, a marquee, an auto-advancing carousel with a progress bar and pause button, rough-notation underlines, and the keycap CTA. It respects prefers-reduced-motion.
- **Recommended motion scale:** 150ms colour, 200ms transform, 300ms panels, 700ms first-load only. One rough-notation underline in the hero, and a keycap CTA.

## Conflicts to resolve
1. **Rule 37 says one effect per landing page.** A Shaders hero plus Amp-style drifting chips plus an animated footer could count as three. Proposal: the Shaders effect is "the effect", and the chips and footer are CSS motion, not shaders. Victor should confirm whether rule 37 means only shader effects or all ambient effects. If it means all, pick either the chips or the shader for the hero.
2. Amp's drifting chips and a shader behind the hero would fight visually. Choose one.
3. The README says not to copy PostHog's OS gimmick or its assets. Steal the patterns only.
4. Paid items are excluded: the Framer skyline component ($3), Shaders Pro, and the paid part of ThreeUI.
