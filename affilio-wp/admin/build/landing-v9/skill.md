# Offer Rails

This file is a pointer. It is not a second copy of the rules.

Full text: [/llms-full.txt](https://offer-rails.onrender.com/llms-full.txt)

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
