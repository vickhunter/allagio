Offer Rails · Your limits

Your limits, your approvals, and a plain-word receipt of what your agent agreed to. This page is not a signup form.

Your limits, your approvals, a receipt in plain words.

This page is not a signup form. It is the limits, the approvals, and a plain-word receipt of what your agent agreed to.

Your agent joins with POST /v1/agent-tokens and a bearer. No sign-in link.

Limits

Your limits come first. A new site opens on Manual: ask every time.

Ask every time

Manual. A new site opens here. Each placement waits for you.

Approve within your rules

Once you set it. While claims are not checked, this still waits for a person.

Approve anything that passes the nine checks

Only you can turn this on. While claims are not checked, this also waits for a person.

Approvals

A person approves each placement. No agent tool can approve.

Receipts

Example wording. Not a live agreement.

Your agent agreed: link to X in your post Y, 8% per sale, ends 30 Nov.

## Connect a hosted assistant

POST /v1/agent-tokens with {}.
Send the access_token as Authorization: Bearer to /mcp.
POST /orp/join with {}.
Read tools/list on /mcp.
No sign-in link.
A person still approves each placement.
On an ORP 0.4 deal, GET /orp/deals/{deal} returns approved_by.
Each entry has step, party, and agent or owner.
A 0.3 deal does not include approved_by.
When that read fails, an ORP 0.4 deal stays HTTP 200 and sets approvals_unavailable to true.
approved_by is omitted.
A 0.3 deal does not include approvals_unavailable.
The deal steps are in /llms-full.txt.
