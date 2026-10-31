Offer Rails · Your limits

Your limits, your approvals, and a plain-word receipt of what your agent agreed to. This page is not a signup form.

Your limits, your approvals, a receipt in plain words.

Limits

Approvals

Receipts

Ask me every time

Approve within my rules

Approve anything that passes the checks

This page is not a signup form. It is the limits, the approvals, and a plain-word receipt of what your agent agreed to.

Sign in to set your limits

Your limits come first. A new site waits for a person because the publish step starts as ask, and that stays when claim checks are on.

Under Ask me every time the decide step waits for you. A site key can set the publish step to auto.

Once you set it. While claims are not checked, this still waits for a person.

Only you can turn this on. While claims are not checked, this also waits for a person.

A new site stores the publish step as ask, so prepare waits for a person. A stored auto still waits for a person until the domain is verified and claim checks are live, and only then does prepare commit. A site key can set that publish step to auto. Ask me every time, Approve within my rules, and Approve anything that passes the checks are a separate control for the decide step. An agent tool can only tighten a site or offer mode. While claims are not checked, Approve within my rules and Approve anything that passes the checks still wait for a person. Your agent can prepare the placement and show you the terms.

No deals yet.

Give your agent the join line

## Connect a hosted assistant

POST /v1/agent-tokens with {}.
Send the access_token as Authorization: Bearer to /mcp.
The bearer is not an X-API-Key or X-Admin-Key.
POST /orp/join with {}.
Read tools/list on /mcp.
No sign-in link.
A new site stores the publish step as ask, so prepare waits for a person. A stored auto still waits for a person until the domain is verified and claim checks are live, and only then does prepare commit. A site key can set that publish step to auto. Ask me every time, Approve within my rules, and Approve anything that passes the checks are a separate control for the decide step. An agent tool can only tighten a site or offer mode. While claims are not checked, Approve within my rules and Approve anything that passes the checks still wait for a person.
On an ORP 0.4 deal, GET /orp/deals/{deal} returns approved_by.
Each entry has step, party, and agent or owner.
A 0.3 deal does not include approved_by.
When that read fails, an ORP 0.4 deal stays HTTP 200 and sets approvals_unavailable to true.
approved_by is omitted.
A 0.3 deal does not include approvals_unavailable.
The deal steps are in /llms-full.txt.
