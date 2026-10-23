# D-098 proof after rebase onto main

Rebase onto `27e0a6334b9603aa8e1d344b79353bd153e60d35` (`D-093b: placement_publish defaults to ask`, PR #73) is clean: `origin/main` is an ancestor of this branch. Conflicts in `STATUS.md`, the agents bundle, and the vision commit were resolved without dropping the landing or vision work and without dropping the ask-default. `placement_publish` with no saved step-mode row still reads as `ask` in `/llms.txt` and in the agents copy. `joinSignInNote` is not on the page.

Documented code head: `7f359cb79c248a3c7e2a13e30627116dc555f45e`

Landing `node --test`: 27 pass, 0 fail.

Axe serious+critical: 0 on `/`, `/makers/`, `/publishers/`, and `/you/` at 390x844 and 1440x900. Violations of any impact: 0.

Rendered R29 grep: 0. Pattern: open source / opensource / open-source, ad/ads, pilot, fully automatic, revolutionary, best-converting. Pages: those four routes plus `/llms.txt`. Personal data (email, IP, phone): 0.

Copy button at scroll 0: 390 top 625 bottom 669, header bottom 113, viewport 844. 1440 top 666 bottom 710, header bottom 65, viewport 900. Vision sections do not overlap.

Shots in this folder: `hero-390.png`, `hero-1440.png`, `vision-does-390.png`, `vision-does-1440.png`, `vision-keep-390.png`, `vision-keep-1440.png`, `vision-replaces-390.png`, `vision-replaces-1440.png`, `you-390.png`, `you-1440.png`.

Preview (not production, not the hosted Offer Rails deploy): https://thoroughly-dispatch-carry-publish.trycloudflare.com

It serves the stamped landing from this branch on the agent box, port 4173, through a Cloudflare quick tunnel. No uptime guarantee. It stays up while that tunnel process is running on this agent VM, started 2026-10-23 07:18 UTC. Paths: `/`, `/makers/`, `/publishers/`, `/you/`, `/llms.txt`.

PR #72 stays a draft. Not merged. Not deployed to hosted or production.
