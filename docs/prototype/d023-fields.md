# D-023 prototype fields

Static screens at `/prototype/share`, `/prototype/audience`, and `/prototype/payments`. Nothing here calls an API. "Example data" is fixture copy. The example toggle in the prototype nav is not a product field and never leaves the browser.

Privacy rule: nothing that identifies a person leaves the owner's site. Groups smaller than 50 are never shared. Discovery can find an offer, and it goes live only after the owner claims and signs it. We match people to offers. We do not check payments, and we never hold money.

Precision is one of three: an exact value, a range, or a bucket. Cards leave only when Signed by owner is Claimed and signed and Sharing is on.

## What my agent shares

| Field | Type | Precision | Allowed values | Filled by | Leaves the site? |
| --- | --- | --- | --- | --- | --- |
| Sharing on/off | switch | exact | on, off | Human | No. Off keeps both cards here. |
| Signed by owner | status | exact | Not claimed yet, Claimed and signed | Human, via Claim and sign | The status stays here. Not claimed yet is a draft and is not shared. Claimed and signed is live, and the cards leave only then, and only while sharing is on. |
| Project | short text | exact | Product name. No person. | Agent, from the product already registered | Yes, only when claimed and sharing is on |
| Category | short text | exact | A topic such as "Developer tools" | Agent | Yes, only when claimed and sharing is on |
| Commission terms | short text | exact | Percent or flat amount of the first purchase | Agent, from the offer the human already set | Yes, only when claimed and sharing is on |
| Revenue range | range | range | Under $1k, $1k–10k, $10k–100k, $100k+ a month | Agent | Yes, only when claimed and sharing is on |
| Exact revenue | money | exact | — | — | **Drop.** Do not add it. |
| Customer names, emails, handles, customer lists, IPs | identity | exact | — | — | **Drop.** Listed on the screen as never shared. |

## Where my audience lives

| Field | Type | Precision | Allowed values | Filled by | Leaves the site? |
| --- | --- | --- | --- | --- | --- |
| Signed by owner | status | exact | Not claimed yet, Claimed and signed | Human, via Claim and sign on the share screen | Same gate as the offer card. The audience card leaves only when claimed and sharing is on. |
| Surface | multi-select | exact | Blog, X, YouTube, TikTok, Pinterest, LinkedIn, Newsletter, App | Human | Yes, on the audience card, only when claimed and sharing is on |
| Size bucket | enum | bucket | <1k, 1-10k, 10-100k, 100k+ | Human | Yes, on the audience card, only when claimed and sharing is on |
| Topic | short text, 80 characters | exact | A subject. No names. | Human | Yes, on the audience card, only when claimed and sharing is on |
| Segment count | range | range | Ranges such as 1k-10k or 10k-100k. Never an exact reader count. | Agent | Yes, only when claimed and sharing is on and the group has at least 50 people |
| Too small to share | status | exact | Shown on the owner's card. No count is shown. | Agent | **No.** The owner can see the row. It does not leave. |
| Minimum group size | product rule | exact | 50 | Not entered by a person | No. This is the rule. Groups smaller than 50 are never shared. |
| Exact follower count | number | exact | — | — | **Drop.** A range or a bucket is enough. |
| Groups under 50 | count | exact | — | Agent marks them Too small to share | **Drop.** They are not sent. |
| Profile URL, handle, or email of the publisher | identity | exact | — | — | **Drop.** The surface name is enough. |

## Who pays whom

These rows are for the owner to read. They do not leave the site. Sending them would make us a payment checker.

| Field | Type | Precision | Allowed values | Filled by | Leaves the site? |
| --- | --- | --- | --- | --- | --- |
| Payment provider | enum | exact | Stripe, Polar, Lemon Squeezy, Paddle, other | Agent, read on the owner's site | **No.** Keep the name here. **Drop** it from anything that is sent. |
| Sale | short text | exact | "First purchase" plus a coarse time such as "This month" | Agent | **No.** Owner only. |
| How the sale is known | enum | exact | Confirmed by provider, Reported by seller | Agent | **No.** Owner only. |
| Buyer, email, exact amount, charge id, provider account id | identity or money | exact | — | — | **Drop.** |
| Balance we hold | money | exact | — | — | **Drop.** We never hold money. Payouts go from seller to publisher. |
