# D-032 UI: setup page + digest (static; fixtures, mock, or the real PR #21 API)

- `setup.html`: page after device-code sign-in. Max 4 prefilled questions, Continue.
- `digest.html`: all projects and sites on one page, account totals, undo/redo per item, kill switch.
- `assets/digest-item.js`: reusable `<digest-item>` custom element.
- **v7 design system: `DESIGN.md`** (tokens, motion, keycap, keyboard map, states, panels and modals, voice, BLIP, landing-hero spec). `assets/ui-kit.js`: palette (⌘K or Ctrl+K), help (?), panel and modal with focus trap, key dispatcher, BLIP. `assets/digest-ui.js`: J/K/Enter/R/A, item side panel, and approval-modes modal on the digest.
- `assets/offer-rails.css`: tokens copied from the owner workspace (`src-67388fc/.../workspace.css`), Archivo + IBM Plex Mono subsets (OFL).
- `fixtures/`: JSON data in. `features/`: BDD scenarios. `tests/run_tests.py`: Playwright walk at 1440 and 390 + axe.
- Interface: `INTERFACE.md`. Backend parity (PR #21 @ fb493f0): `PARITY.md`. Self review: `REVIEW.md`.
- Real API: `?api=real&api_base=/backend` (adapter `assets/contract.js`, client `assets/api.js`). Contract test: `python3 tests/contract_test.py` (no browser, no network; checks `fixtures/api` against `PR21-body-fb493f0.md` + `tests/contract_spec.json`, both from fb493f0).

Run: `python3 tests/run_tests.py` (serves this folder on a free port, system Chrome headless, aborts `**/v1/go/**`). Screenshots go to `/workspace/builds/offer-rails/proof/d032-ui/v7-design/` (before and after pairs: `python3 tests/design_shots.py before|after <repo>`) (earlier versions in their own folders). `ONLY=<regex>` runs a subset of tests (both widths). The API is mocked with Playwright routes (`MockAPI` in the test file, bodies from `fixtures/api/`); `?mock=1` gives the same mock in a normal browser.
Preview by hand: `python3 -m http.server 8732` then `/setup.html?fixture=publisher`, `/digest.html?fixture=many`.
