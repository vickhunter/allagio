# Offer Rails

This file is a pointer. It is not a second copy of the rules.

Full text: [/llms-full.txt](https://offer-rails.onrender.com/llms-full.txt)

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
The deal steps are in /llms-full.txt.
