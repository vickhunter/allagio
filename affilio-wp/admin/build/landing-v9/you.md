Offer Rails · Your limits

Your limits, your approvals, and a plain-word receipt of what your agent agreed to. This page is not a signup form.

Your limits, your approvals, a receipt in plain words.

This page is not a signup form. It is the limits, the approvals, and a plain-word receipt of what your agent agreed to.

Your agent joins with POST /v1/agent-tokens and a bearer. No sign-in link.

Limits

Your limits come first. A new site waits for a person because the publish step starts as ask, and that stays when claim checks are on.

Ask me every time

Under Ask me every time the decide step waits for you. A site key can set the publish step to auto.

Approve within my rules

Once you set it. While claims are not checked, this still waits for a person.

Approve anything that passes the checks

Only you can turn this on. While claims are not checked, this also waits for a person.

Approvals

A new site stores placement_publish as ask, so a person approves each placement until that step is auto. Auto commits only when the domain is verified and claim checks are live. A site key can set that publish step to auto. Ask me every time, Approve within my rules, and Approve anything that passes the checks are a separate control for the decide step. An agent tool can only tighten a site or offer mode. While claims are not checked, Approve within my rules and Approve anything that passes the checks still wait for a person.

Receipts

Example wording. Not a live agreement.

Your agent agreed: link to X in your post Y, 8% per sale, ends 30 Nov.

## Connect a hosted assistant

POST /v1/agent-tokens with {}.
Send the access_token as Authorization: Bearer to /mcp.
The bearer is not an X-API-Key or X-Admin-Key.
POST /orp/join with {}.
Read tools/list on /mcp.
No sign-in link.
A new site stores placement_publish as ask, so a person approves each placement until that step is auto. Auto commits only when the domain is verified and claim checks are live. A site key can set that publish step to auto. Ask me every time, Approve within my rules, and Approve anything that passes the checks are a separate control for the decide step. An agent tool can only tighten a site or offer mode. While claims are not checked, Approve within my rules and Approve anything that passes the checks still wait for a person.
On an ORP 0.4 deal, GET /orp/deals/{deal} returns approved_by.
Each entry has step, party, and agent or owner.
A 0.3 deal does not include approved_by.
When that read fails, an ORP 0.4 deal stays HTTP 200 and sets approvals_unavailable to true.
approved_by is omitted.
A 0.3 deal does not include approvals_unavailable.
The deal steps are in /llms-full.txt.
