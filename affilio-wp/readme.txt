=== Affilio ===
Contributors: affilio
Stable tag: 0.1.27
License: GPLv2 or later

AI affiliate copilot for WordPress. Connect from Settings, then review recommendations before anything is inserted.

== Changelog ==

= 0.1.27 =
* The disconnected Affilio Dashboard card is drawn by the dashboard script. The server sends that same card only inside noscript, for a browser with JavaScript off.
* Settings loads the admin stylesheet, so the connection status keeps its spacing.
* "Key saved (…)" can be translated. Italian (it_IT) uses "Chiave salvata (…)".

= 0.1.26 =
* POST /wp-json/affilio/v1/insert is removed. The dashboard inserts a generated recommendation through the editor route and does not send HTML to be written into the post.
* The editor insert ignores a caller html field. An Italian post (`it_IT`) gets the approved Italian disclosure. Other locales get the English disclosure. Block content stores that disclosure as a paragraph block.

= 0.1.25 =
* The "Vedi il prodotto" link is an inline-block at least 44px tall, so it is a tap target on a 390px-wide screen.

= 0.1.24 =
* The five-minute box sync also checks published posts and pages whose content contains an Affilio box, even when that box is missing from the local index. A draft is not scanned. A placement revoked on the server is removed and acknowledged.

= 0.1.23 =
* If the five-minute box sync is not scheduled, the next load schedules it again. A second load does not add another event. A placement revoked on the server is removed from the post when that sync runs.
* A go-link redirect accepts an https address, or this site's own home URL. An http or javascript address is not used, including as a remembered fallback.
* Opening the scan page with ?affilio_scan_continue=1 does not submit the next batch and does not mint a nonce. The next batch runs only from the POST that created the job.

= 0.1.22 =
* Prepare and the publication receipt send the article's decoded WordPress title and post id.
* When a new box is removed because the receipt never reached the server, the plugin asks the server to put that placement back to pending. A failed call is retried on the next prepare, before another placement is created.
* An API URL that is only the site origin is saved and called as that origin plus /v1. A trailing slash is removed.

= 0.1.21 =
* A refused receipt returns the server status and the server reason. Connect stores plugin_key when the register response includes it.
* A second box on the same post is refused with "Keep one box on this post." A box added in that request is removed. An upgrade does not delete a box that was already in the post.

= 0.1.20 =
* An upgrade no longer fatals the front page. Go-link detection reads the request path before WordPress has built the query, and the version upgrade runs on init.
* Box sync is scheduled on activation and upgrade, and cleared on deactivation. A failed plugin acknowledgement is retried with backoff.
* Choosing "leave removed" settles as removed and sends the acknowledgement. `/go/<id>` without a trailing slash is not canonically cached. Publish matches the article URL or post id the API returns.

= 0.1.19 =
* An inserted box links to this site's `/go/<placement id>` with `rel="sponsored nofollow"` and the label "(link sponsorizzato)".
* Revoke, restore, and upgrade send a plugin acknowledgement only after the saved post is re-read. A second failed re-read sends `failed` and shows an admin notice.
* `/go/<placement id>` proxies the API with the site key and falls back when the API is down. Legacy `/v1/go/` links are rewritten only while the post is rendered.
* Settings stores a plugin key for box acknowledgements. The field renders empty, and an empty save keeps the saved key.

= 0.1.18 =
* Inserting or removing an Affilio block does not create a WordPress post revision. The revision skip runs last, so another plugin cannot turn revisions back on for that write.
* If a refused receipt cannot be rolled back, even after one retry, the admin sees that the box is live without a receipt.

= 0.1.14 =
* A post counts as already disclosed only when it matches the receipt rule: the approved disclosure, "commissione", or "link" within 60 characters of "affiliat".
* If publication is refused, the insert is removed and the admin sees the reason in Italian. That automatic revert does not tell the service the placement was removed.
* An insert is a paragraph block marked with the class `affilio-placement-<placement id>`. A disclosure the plugin adds uses `affilio-disclosure-<placement id>`.
* Placement list: "Rimuovi inserimento" asks you to confirm the post and the exact link, then removes that block. The notice says whether the service was told.
* WP-CLI: `wp affilio revert <post_id> [--placement=<id>] [--dry-run]`. The post is edited first. The command prints the post change and the API result separately, and exits 0 when the local revert succeeded.

== Removing an insertion ==

From the placement list, "Rimuovi inserimento" shows the post title and the link (anchor text and go URL). Annulla leaves the post alone. Rimuovi edits the post, then reports the removal. If the service is unreachable, the post stays reverted and the report is retried.

From WP-CLI, when it is loaded:

`wp affilio revert 42`
`wp affilio revert 42 --placement=<placement id>`
`wp affilio revert 42 --dry-run`

`--dry-run` prints the post change and does not call the API. A local revert exits 0 even when the API result is "API: non raggiungibile, riprovo più tardi". Nothing matched locally exits non-zero.

= 0.1.13 =
* A placement receipt accepts a disclosure already written in the post. The setting does not skip Affilio's disclosure unless that post text actually discloses.

= 0.1.12 =
* Distribution matches listed offers that have no payment provider, and explains an empty result in Italian.
* An article keeps a single disclosure. A post that already discloses, or the setting "I miei articoli contengono già una disclosure", adds none.
* An approved recommendation is inserted as a Gutenberg paragraph with an Italian tracked link.
* Opportunities, prepare, and publish require a connected publisher account.
