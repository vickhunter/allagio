# Meng To (@MengTo): design patterns and skills, applied to landing v8

Sources (16 Oct 2026): X profile + last ~25 posts via the x API (user id 5797422); github.com/MengTo/Skills; aurachat.io/learn/tips-for-prompting; his LinkedIn posts "avoid AI slop on landing pages" and "Prompt builder guide"; YouTube "One-shotting beautiful landing pages with code references".

## What he recommends
1. **Never start from a blank prompt.** Start from an image reference, a site URL or existing code, plus a taste skill / DESIGN.md, with design rules in AGENTS.md. ("Don't use GPT to start a design. Always use an image reference or site url. Never prompt without a taste skill or DESIGN.md.")
2. **Spell out the typography.** Family, weight, size, line height, tracking. His prompt-builder default: headings 40-60px with tight negative tracking (around -0.06em), subheads 28-36px, body 14-16px at 1.5 line height.
3. **Bento grids:** mixed col/row spans with one larger featured cell.
4. **Build the hero first, then go section by section.** Remix templates and components (variants beat rerolls), and reference full code, not just screenshots.
5. **Shaders for landing pages** (10 Oct post, the source of our rule 37): a curated free shader collection. "Shaders can be super lightweight." Turn them off for older browsers/computers, and let agents handle the fallback.
6. **Anatomy skill for visual explainers** (11 Oct): all SVG, animated. That is the skill behind our how-it-works railway figure.
7. **Skills repo:** video→super prompt, HTML→interaction prompts, stitched full-page capture, daily UI inspiration loop, design-first UI prompting (goal → format → layout → type → color → constraints, plus negative prompts), and moods like `light-mode-paper-technical` and `orange-clean-paper-saas`.
8. **Logos and imagery:** don't accept text-only or cheesy AI logos. Pick a reference style first.

## How v8 applies them
- References first: built from our existing DESIGN.md tokens (d032-ui), the how-it-works railway figure (4a93a13) and the PostHog/Amp inspo, not a blank prompt. All three versions share one token file (`app/src/styles.css`).
- Typography written down: Archivo Rail 700 display with -0.035em tracking (a bit looser than his -0.06em because Archivo is already tight), 500-weight sans body at 1.6-1.7 line height, Plex Mono only for numbers, IDs and eyebrows.
- A uses his bento recipe: a 6-column grid with one featured 4×2 cell ("One account. Both sides.").
- B follows his shader advice under rule 37: one effect only (a Three.js fragment shader, warm field plus faint rail lines) behind real product content. It falls back to static CSS for reduced motion or no WebGL, and pauses when offscreen.
- Hero first: each version's hero was built and screenshot-reviewed before the sections under it.
- Not used: Aura/Scrolltide paid templates, AI image generation, his Three.js templates (paid/unknown licence).
