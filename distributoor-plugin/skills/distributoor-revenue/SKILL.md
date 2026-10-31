---
name: distributoor-revenue
description: Find relevant revenue opportunities and prepare owner-approved placements using the publisher's existing agent and publishing tools.
---

Use the Allagio MCP connection (`distributoor`). Keep the owner's existing runtime and publishing tools.

1. Use `find_opportunities` with the article's actual context. Accept `no_match`; commission never compensates for an irrelevant recommendation. Use approved personal affiliate links through the existing Allagio workflow when available; this native-offer POC does not enroll the owner in external programs.
2. Verify product claims with the owner. Call `prepare_placement` with exact copy and an HTTPS article URL on the publisher's registered domain.
3. Show the owner copy, disclosure, destination, commission, attribution window, refund hold, direct merchant payout terms and service fee. The pilot operator records their approval through the separate admin endpoint. Never request or handle the admin key, webhook secret or encryption key in an agent conversation. `confirm=true` does not authorize publication.
4. Poll `list_agent_tasks` using the owner's existing scheduler. Continue only for an approved, unexpired proposal. Call `commit_placement` once; retries return the same receipt.
5. Use the owner's separately authorized publishing tool to publish exact approved copy, link and disclosure. A committed proposal is not a published article. Changes to copy, surface or terms require a new proposal.
6. Call `record_placement_receipt` with the article URL, published text and evidence reference. The server labels this evidence `publisher_reported`; do not present it as independent verification.
7. Read `get_balance` and `get_payout_history`. Distinguish held, cleared, reserved, maker-reported payment and publisher-confirmed receipt. An API conversion is not a bank payment. Synthetic events are demo evidence only.

Use a scoped, expiring publisher key as `OFFERRAILS_SITE_KEY`. `DISTRIBUTOOR_SITE_KEY` still works when `OFFERRAILS_SITE_KEY` is unset. It can read revenue and manage approved placement operations. It cannot approve itself, confirm a bank receipt, change owner content settings or join through the legacy API. Owner revocation stops new attribution through the placement link; already accepted clicks remain obligations.

The first surface is owned articles. X, video, podcasts and newsletters need separate surface ownership, publishing and attribution tests. Cursor, OpenClaw, Grok Bot, Muse and dots compatibility must be tested with the actual client before being advertised. Stdio is the verified transport in this POC; the hosted endpoint has no new OAuth implementation.
