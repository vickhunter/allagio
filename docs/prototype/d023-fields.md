# D-023 prototype fields

Static screens at `/prototype/share`, `/prototype/audience`, and `/prototype/payments`. Nothing here calls an API. "Example data" is fixture copy. The example toggle in the prototype nav is not a product field and never leaves the browser.

Privacy rule: nothing that identifies a person leaves the owner's site. Groups smaller than 20 people are not shared. We match people to offers. We do not check payments, and we never hold money.

## What my agent shares

| Field | Type | Allowed values | Filled by | Leaves the site? |
| --- | --- | --- | --- | --- |
| Sharing on/off | switch | on, off | Human | No. Off keeps both cards here. On is the only permission that lets the two cards leave. |
| Project | short text | Product name. No person. | Agent, from the product already registered | Yes, only while sharing is on |
| Category | short text | A topic such as "Developer tools" | Agent | Yes, only while sharing is on |
| Commission terms | short text | Percent or flat amount of the first purchase | Agent, from the offer the human already set | Yes, only while sharing is on |
| Revenue range | bucket | Under $1k, $1k–10k, $10k–100k, $100k+ a month | Agent | Yes, only while sharing is on |
| Exact revenue | money | — | — | **Drop.** Do not add it. |
| Customer names, emails, handles, customer lists, IPs | identity | — | — | **Drop.** Listed on the screen as never shared. |

## Where my audience lives

| Field | Type | Allowed values | Filled by | Leaves the site? |
| --- | --- | --- | --- | --- |
| Surface | multi-select | Blog, X, YouTube, TikTok, Pinterest, LinkedIn, Newsletter, App | Human | Yes, on the audience card, only while sharing is on |
| Size bucket | enum | <1k, 1-10k, 10-100k, 100k+ | Human | Yes, on the audience card, only while sharing is on |
| Topic | short text, 80 characters | A subject. No names. | Human | Yes, on the audience card, only while sharing is on |
| Segment count | bucket | 20–40, 40–100, 100–400 readers, or hidden | Agent | Yes, only while sharing is on and the group has at least 20 people |
| Exact follower count | number | — | — | **Drop.** The bucket is enough. |
| Groups under 20 | count | — | Agent suppresses them | **Drop.** They are not put on the card. |
| Profile URL, handle, or email of the publisher | identity | — | — | **Drop.** The surface name is enough. |

## Who pays whom

These rows are for the owner to read. They do not leave the site. Sending them would make us a payment checker.

| Field | Type | Allowed values | Filled by | Leaves the site? |
| --- | --- | --- | --- | --- |
| Payment provider | enum | Stripe, Polar, Lemon Squeezy, Paddle, other | Agent, read on the owner's site | **No.** Keep the name here. **Drop** it from anything that is sent. |
| Sale | short text | "First purchase" plus a coarse time such as "This month" | Agent | **No.** Owner only. |
| How the sale is known | enum | Confirmed by provider, Reported by seller | Agent | **No.** Owner only. |
| Buyer, email, exact amount, charge id, provider account id | identity or money | — | — | **Drop.** |
| Balance we hold | money | — | — | **Drop.** We never hold money. Payouts go from seller to publisher. |
