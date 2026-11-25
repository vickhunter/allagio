import{G as e,J as t,K as n,Y as r,d as i,l as a,t as o}from"./Page-8rG-TjS9.js";var s=r(t(),1),c={version:`0.3`,versions:[`0.3`,`0.4`],license:`Apache-2.0 planned`,schemas:[{file:`0.3/constraints.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/constraints.schema.json`,slug:`constraints`,href:`/protocol/spec/0.3/constraints/`,title:`Constraints`,description:`An owner's private limits for its agent, kept by the host and never sent to a counterparty. Inside them the agent acts alone. Outside an allowance the host escalates to the owner. A deny rule or a step set to off refuses outright. Anything missing defaults to the safe side: every step that can bind the owner (list, negotiate, accept, execute, amend, settle, share) asks; only revoke is auto. With no budget, any spend asks.`,kind:`schema`,type:``,required:[`steps`],anyOf:[],sections:[{name:`Constraints`,description:`An owner's private limits for its agent, kept by the host and never sent to a counterparty. Inside them the agent acts alone. Outside an allowance the host escalates to the owner. A deny rule or a step set to off refuses outright. Anything missing defaults to the safe side: every step that can bind the owner (list, negotiate, accept, execute, amend, settle, share) asks; only revoke is auto. With no budget, any spend asks.`,closed:!0,fields:[{name:`steps`,required:!0,type:`Steps`,description:``},{name:`budget`,required:!1,type:`object, closed`,description:`Caps on money the owner pays. A deal that would cross one escalates with over_budget. Pay per click or per conversion counts at its cap; with no cap it always escalates.`},{name:`approve_over`,required:!1,type:`Money`,description:`A person approves any single deal worth more than this (over_threshold), in either direction.`},{name:`kinds`,required:!1,type:`array of DealKind`,description:`Deal kinds the agent may do alone. Others escalate (kind_outside).`},{name:`categories`,required:!1,type:`object, closed`,description:``},{name:`spaces`,required:!1,type:`object, closed`,description:``},{name:`counterparties`,required:!1,type:`object, closed`,description:``},{name:`brand`,required:!1,type:`object, closed`,description:`Hard content rules. Terms that break them are refused with brand_rule, never escalated.`},{name:`max_duration`,required:!1,type:`Duration`,description:`Longer deals escalate (duration_over).`},{name:`expires`,required:!1,type:`Timestamp`,description:`After this every step asks (mandate_expired).`}]}],valid:[{name:`constraints.json`,json:`{
  "steps": {
    "list": "ask",
    "negotiate": "auto",
    "accept": "auto",
    "execute": "auto",
    "amend": "ask",
    "settle": "ask",
    "share": "off",
    "revoke": "auto"
  },
  "budget": {
    "per_deal": {
      "amount_minor": 30000,
      "currency": "EUR"
    },
    "per_month": {
      "amount_minor": 100000,
      "currency": "EUR"
    },
    "max_rate_bps": 3000
  },
  "approve_over": {
    "amount_minor": 50000,
    "currency": "EUR"
  },
  "kinds": [
    "affiliate",
    "sponsored",
    "swap",
    "repost"
  ],
  "categories": {
    "allow": [
      "software",
      "business"
    ],
    "deny": [
      "gambling",
      "finance.crypto"
    ]
  },
  "spaces": {
    "allow": [
      "sp_news_1",
      "sp_x_1"
    ]
  },
  "counterparties": {
    "min_reputation": 400,
    "new": "ask"
  },
  "brand": {
    "must_not_include": [
      "guaranteed income"
    ],
    "disclosure": {
      "label": "Partner mention",
      "lang": "en"
    }
  },
  "max_duration": "P90D",
  "expires": "2027-06-30T00:00:00Z"
}`}],invalid:[{name:`invalid/constraints-unknown-step.json`,why:`Steps are the eight named steps.`,json:`{
  "steps": {
    "publish_anything": "auto"
  }
}`}]},{file:`0.3/defs.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/defs.schema.json`,slug:`defs`,href:`/protocol/spec/0.3/defs/`,title:`ORP 0.3 shared definitions`,description:`Types shared by every ORP 0.3 message. Core objects are closed (additionalProperties false); extension data goes in the envelope's ext field.`,kind:`schema`,type:``,required:[],anyOf:[],sections:[{name:`Version`,description:`Protocol version, MAJOR.MINOR.`,closed:!1,fields:[]},{name:`PartyId`,description:`Pseudonymous party id: orp:<host>/<opaque>. The opaque part is random and never encodes a name, email, handle or domain of the owner.`,closed:!1,fields:[]},{name:`MessageId`,description:`Sender-chosen id, unique per sender. Doubles as the idempotency key.`,closed:!1,fields:[]},{name:`LocalId`,description:`Host-scoped id for deals, spaces, offers, escalations, consents and disputes.`,closed:!1,fields:[]},{name:`ObligationId`,description:``,closed:!1,fields:[]},{name:`Hash`,description:`sha256- followed by the unpadded base64url SHA-256 digest of the RFC 8785 (JCS) canonical JSON.`,closed:!1,fields:[]},{name:`Timestamp`,description:``,closed:!1,fields:[]},{name:`Duration`,description:`ISO 8601 duration, for example P7D or PT12H.`,closed:!1,fields:[]},{name:`Url`,description:``,closed:!1,fields:[]},{name:`ExtName`,description:`Reverse-DNS extension name, for example com.example.boost.`,closed:!1,fields:[]},{name:`Text`,description:``,closed:!1,fields:[]},{name:`Money`,description:`Integer amount in the currency's minor unit (cents for EUR and USD).`,closed:!0,fields:[{name:`amount_minor`,required:!0,type:`integer, min 0, max 100000000000`,description:``},{name:`currency`,required:!0,type:`string, pattern`,description:``}]},{name:`MessageType`,description:``,closed:!1,fields:[]},{name:`SpaceKind`,description:`Core kinds are family.kind. Extension kinds use reverse-DNS with at least three labels.`,closed:!1,fields:[]},{name:`Platform`,description:``,closed:!1,fields:[]},{name:`DealKind`,description:`A hint for discovery; the obligations are what bind. affiliate: paid per conversion on a tracked link. sponsored: paid per deal or placement. swap: space for space. traffic_swap: tracked clicks for tracked clicks. link_exchange: links on each other's pages. repost: a repost, retweet or quote. comment: a comment or comment space. cross_sell: one product offered to another's customers, including upsells at checkout or upgrade. data: aggregate data exchange. partnership: anything longer-running.`,closed:!1,fields:[]},{name:`ProofMethod`,description:`link: human clicks on the deal's tracking link. proof_url: a public URL the verifier fetched. platform_check: the platform's public interface (oEmbed, public API, feed, archive). attestation: the counterparty confirms. witness: a host-controlled observer, such as a seed inbox. provider: a payment provider event.`,closed:!1,fields:[]},{name:`Step`,description:`A step an owner can set to auto, ask or off.`,closed:!1,fields:[]},{name:`StepMode`,description:``,closed:!1,fields:[]},{name:`Steps`,description:`The mode of each step. A missing step uses the default: ask, except revoke, which is auto.`,closed:!0,fields:[{name:`list`,required:!1,type:`StepMode`,description:``},{name:`negotiate`,required:!1,type:`StepMode`,description:``},{name:`accept`,required:!1,type:`StepMode`,description:``},{name:`execute`,required:!1,type:`StepMode`,description:``},{name:`amend`,required:!1,type:`StepMode`,description:``},{name:`settle`,required:!1,type:`StepMode`,description:``},{name:`share`,required:!1,type:`StepMode`,description:``},{name:`revoke`,required:!1,type:`StepMode`,description:``}]},{name:`Agent`,description:`The software acting for the party. Never a person.`,closed:!0,fields:[{name:`name`,required:!0,type:`string, pattern`,description:`1 to 64 characters: letters, digits, underscore, dot, space, or hyphen. Parentheses are not allowed.`},{name:`version`,required:!1,type:`string, max length 32`,description:``}]},{name:`EscalationReason`,description:``,closed:!1,fields:[]},{name:`SizeBucket`,description:``,closed:!1,fields:[]},{name:`ShareBucket`,description:``,closed:!1,fields:[]},{name:`Category`,description:`Lowercase dotted category, for example software.devtools.`,closed:!1,fields:[]},{name:`Disclosure`,description:`The label readers see next to the placement, in the reader's language.`,closed:!0,fields:[{name:`label`,required:!0,type:`string, min length 2, max length 80`,description:``},{name:`lang`,required:!1,type:`string, pattern`,description:``}]},{name:`Audience`,description:`Aggregate audience description. Buckets only. Any segment smaller than k_min people is omitted.`,closed:!0,fields:[{name:`size`,required:!1,type:`SizeBucket`,description:``},{name:`regions`,required:!1,type:`array of object, closed`,description:``},{name:`k_min`,required:!1,type:`integer, min 50`,description:``},{name:`source`,required:!1,type:`"self_report" | "platform" | "host_measured"`,description:``}]},{name:`Verification`,description:``,closed:!0,fields:[{name:`method`,required:!0,type:`"domain" | "dns" | "platform_oauth" | "bio_token" | "challenge_post"`,description:``},{name:`at`,required:!0,type:`Timestamp`,description:``},{name:`by`,required:!1,type:`PartyId`,description:``}]},{name:`Space`,description:`A place an owner controls where content can appear. The locator may be withheld until a deal is active.`,closed:!0,fields:[{name:`id`,required:!0,type:`LocalId`,description:``},{name:`kind`,required:!0,type:`SpaceKind`,description:``},{name:`platform`,required:!0,type:`Platform`,description:``},{name:`title`,required:!1,type:`string, min length 1, max length 120`,description:``},{name:`locator`,required:!1,type:`Url`,description:``},{name:`topics`,required:!1,type:`array of Category`,description:``},{name:`languages`,required:!1,type:`array of string, pattern`,description:``},{name:`audience`,required:!1,type:`Audience`,description:``},{name:`accepts`,required:!1,type:`array of DealKind`,description:``},{name:`disclosure`,required:!1,type:`Disclosure`,description:``},{name:`verified`,required:!1,type:`array of Verification`,description:``},{name:`rules`,required:!1,type:`object, closed`,description:``},{name:`adapter`,required:!1,type:`string, pattern`,description:``}]},{name:`SpaceRef`,description:`Either one named space, or any space of these kinds owned by the obligor.`,closed:!0,fields:[{name:`id`,required:!1,type:`LocalId`,description:``},{name:`kinds`,required:!1,type:`array of SpaceKind`,description:``}]},{name:`Role`,description:`offerer is the party that opened the deal; acceptor is the other party. Roles never swap during a deal.`,closed:!1,fields:[]},{name:`Content`,description:``,closed:!0,fields:[{name:`link`,required:!1,type:`boolean`,description:`Must carry the deal's tracking link.`},{name:`code`,required:!1,type:`boolean`,description:`Must carry the deal's tracking code.`},{name:`direct`,required:!1,type:`boolean`,description:`Link straight to the destination instead of the tracking link, as in a link exchange. The link must carry rel="sponsored" or rel="nofollow". Proven by proof_url and rechecked while keep lasts; clicks are not counted.`},{name:`brief`,required:!1,type:`string, min length 1, max length 1000`,description:``},{name:`copy`,required:!1,type:`string, min length 1, max length 2000`,description:`Exact text, when the counterparty fixes it.`},{name:`max_chars`,required:!1,type:`integer, min 1, max 100000`,description:``},{name:`must_include`,required:!1,type:`array of string, min length 1, max length 200`,description:``},{name:`must_not_include`,required:!1,type:`array of string, min length 1, max length 200`,description:``}]},{name:`ProofList`,description:`Proofs the counterparty accepts for this obligation; any one is enough.`,closed:!1,fields:[]},{name:`Deadline`,description:`A timestamp, or a duration counted from the moment the deal becomes active.`,closed:!1,fields:[]},{name:`PlaceObligation`,description:`Put content, usually the tracking link or code, somewhere. Without space, any surface the obligor controls counts: the tracking link works everywhere.`,closed:!0,fields:[{name:`id`,required:!0,type:`ObligationId`,description:``},{name:`by`,required:!0,type:`Role`,description:``},{name:`action`,required:!0,type:`const "place"`,description:``},{name:`space`,required:!1,type:`SpaceRef`,description:``},{name:`content`,required:!1,type:`Content`,description:``},{name:`destination`,required:!1,type:`Url`,description:``},{name:`count`,required:!1,type:`integer, min 1, max 100`,description:``},{name:`clicks`,required:!1,type:`integer, min 1, max 10000000`,description:`Done once the tracking link has counted this many human clicks, as in a traffic swap. Without it, the first human click proves the placement.`},{name:`due`,required:!1,type:`Deadline`,description:``},{name:`keep`,required:!1,type:`Duration`,description:``},{name:`proof`,required:!0,type:`ProofList`,description:``}]},{name:`RepostObligation`,description:`Repost or share a public item. No click is needed; proof is a receipt.`,closed:!0,fields:[{name:`id`,required:!0,type:`ObligationId`,description:``},{name:`by`,required:!0,type:`Role`,description:``},{name:`action`,required:!0,type:`const "repost"`,description:``},{name:`target`,required:!0,type:`Url`,description:``},{name:`space`,required:!1,type:`SpaceRef`,description:``},{name:`quote`,required:!1,type:`boolean`,description:``},{name:`due`,required:!1,type:`Deadline`,description:``},{name:`keep`,required:!1,type:`Duration`,description:``},{name:`proof`,required:!0,type:`ProofList`,description:``}]},{name:`CommentObligation`,description:`Comment on a public item, or let the counterparty use a comment space.`,closed:!0,fields:[{name:`id`,required:!0,type:`ObligationId`,description:``},{name:`by`,required:!0,type:`Role`,description:``},{name:`action`,required:!0,type:`const "comment"`,description:``},{name:`target`,required:!0,type:`Url`,description:``},{name:`content`,required:!1,type:`Content`,description:``},{name:`due`,required:!1,type:`Deadline`,description:``},{name:`keep`,required:!1,type:`Duration`,description:``},{name:`proof`,required:!0,type:`ProofList`,description:``}]},{name:`PayObligation`,description:`Pay money: a fixed amount, per click, per conversion (fixed or rate), or per placement. Settlement happens on a payment rail, never inside ORP.`,closed:!0,fields:[{name:`id`,required:!0,type:`ObligationId`,description:``},{name:`by`,required:!0,type:`Role`,description:``},{name:`action`,required:!0,type:`const "pay"`,description:``},{name:`per`,required:!0,type:`"deal" | "placement" | "click" | "conversion"`,description:``},{name:`amount`,required:!1,type:`Money`,description:``},{name:`rate_bps`,required:!1,type:`integer, min 1, max 10000`,description:`Share of the sale in basis points, for per conversion.`},{name:`cap`,required:!1,type:`Money`,description:`The most this obligation can cost over the whole deal.`},{name:`window`,required:!1,type:`Duration`,description:`For per conversion: how long after a counted click a sale still counts.`},{name:`hold`,required:!1,type:`Duration`,description:`Holdback: how long earned money waits before it becomes payable, so a refund inside the hold reverses it first. Absent: P30D for per conversion, none otherwise.`},{name:`after`,required:!1,type:`ObligationId`,description:`The obligation this pay is tied to. Per deal or placement: paid once it is verified. Per click or conversion: counted on its tracking link only. Absent: every tracking link in the deal.`},{name:`due`,required:!1,type:`Deadline`,description:``},{name:`rail`,required:!1,type:`string, pattern`,description:`stripe, lemonsqueezy, polar, manual, or an extension name.`},{name:`proof`,required:!1,type:`ProofList`,description:``}]},{name:`CreditObligation`,description:`Give non-cash credits, for traffic exchange or reposts. Credits are host-scoped and never convert to money inside ORP.`,closed:!0,fields:[{name:`id`,required:!0,type:`ObligationId`,description:``},{name:`by`,required:!0,type:`Role`,description:``},{name:`action`,required:!0,type:`const "credit"`,description:``},{name:`credits`,required:!0,type:`integer, min 1, max 1000000`,description:``},{name:`after`,required:!1,type:`ObligationId`,description:``}]},{name:`ShareObligation`,description:`Share an aggregate, non-personal dataset. Personal data never crosses between agents.`,closed:!0,fields:[{name:`id`,required:!0,type:`ObligationId`,description:``},{name:`by`,required:!0,type:`Role`,description:``},{name:`action`,required:!0,type:`const "share"`,description:``},{name:`data`,required:!0,type:`object, closed`,description:``},{name:`due`,required:!1,type:`Deadline`,description:``},{name:`proof`,required:!0,type:`ProofList`,description:``}]},{name:`ExtensionObligation`,description:`An obligation defined by an extension. Hosts that do not know the action must refuse the terms.`,closed:!1,fields:[{name:`id`,required:!0,type:`ObligationId`,description:``},{name:`by`,required:!0,type:`Role`,description:``},{name:`action`,required:!0,type:`string, pattern`,description:``},{name:`proof`,required:!1,type:`ProofList`,description:``}]},{name:`Obligation`,description:``,closed:!1,fields:[]},{name:`Terms`,description:`What each side will do. terms_hash is the JCS SHA-256 of this object.`,closed:!0,fields:[{name:`title`,required:!0,type:`string, min length 1, max length 120`,description:``},{name:`kind`,required:!1,type:`DealKind`,description:``},{name:`obligations`,required:!0,type:`array of Obligation`,description:``},{name:`categories`,required:!1,type:`array of Category`,description:``},{name:`disclosure`,required:!1,type:`Disclosure`,description:``},{name:`starts`,required:!1,type:`Timestamp`,description:``},{name:`ends`,required:!1,type:`Timestamp`,description:``}]},{name:`Tracking`,description:`The link and optional code the host mints for one obligation when a deal becomes active. They work wherever a person can click or type them: email, social posts, video descriptions, podcast notes, a blog. No adapter or platform integration is needed. After revoke or end, status is plain: the link still redirects but no longer counts.`,closed:!0,fields:[{name:`obligation`,required:!0,type:`ObligationId`,description:``},{name:`link`,required:!0,type:`Url`,description:``},{name:`code`,required:!1,type:`string, pattern`,description:``},{name:`destination`,required:!0,type:`Url`,description:``},{name:`status`,required:!0,type:`"live" | "plain"`,description:``},{name:`param`,required:!1,type:`string, pattern`,description:`Query parameter that carries the random click id on the redirect to destination.`},{name:`alias`,required:!1,type:`string, pattern`,description:`A second parameter, such as ref, that carries the same click id for checkouts that read it.`},{name:`clicks`,required:!1,type:`integer, min 0`,description:`Counted human clicks on this entry. Bot, preview, test, and repeat clicks are excluded. A repeat is a human click less than 5 seconds after the previous counted click on this entry. A 0.3 deal snapshot omits this field.`},{name:`click_share`,required:!1,type:`array of object, closed`,description:`Each party's share summed across counted human clicks, in seat order. A percent is an integer from 0 to 100 on the click, and this sum divides that integer by 100, so 50 twice is 1.0. amount_minor stays an integer sum. Repeat clicks are not included. No address, user agent, or referrer. A 0.3 deal snapshot omits this field.`},{name:`receipt`,required:!1,type:`object, closed`,description:`Set once a counted human click verifies this entry. id is the receipt message. method is link. verified_at is when the host verified it. No address, user agent, or referrer. A 0.3 deal snapshot omits this field.`},{name:`expires`,required:!1,type:`Timestamp`,description:``}]},{name:`ClickCounts`,description:`Aggregate click counts by class. Only human clicks count toward pay or reputation. Never per-click rows, addresses, user agents or referrers.`,closed:!0,fields:[{name:`human`,required:!0,type:`integer, min 0`,description:``},{name:`bot`,required:!1,type:`integer, min 0`,description:``},{name:`preview`,required:!1,type:`integer, min 0`,description:``},{name:`test`,required:!1,type:`integer, min 0`,description:``}]},{name:`Authority`,description:`Why a party is bound. mandate: the owner's standing constraints allowed it. An agent approval is approved_by agent and is not a person's approval. The signed-in owner form records approved_by owner and a mandate. consent: a host-only message type.`,closed:!0,fields:[{name:`type`,required:!0,type:`"mandate" | "consent"`,description:``},{name:`ref`,required:!1,type:`LocalId`,description:``}]},{name:`Waiting`,description:`Who a deal or a listing is waiting for. escalation, reasons and url appear only in the waiting party's own copy.`,closed:!0,fields:[{name:`party`,required:!0,type:`PartyId`,description:``},{name:`step`,required:!1,type:`Step`,description:``},{name:`since`,required:!0,type:`Timestamp`,description:``},{name:`escalation`,required:!1,type:`LocalId`,description:``},{name:`reasons`,required:!1,type:`array of EscalationReason`,description:``},{name:`url`,required:!1,type:`Url`,description:`Where the waiting party is sent. By default the waiting party's agent approves an ask with its bearer, and the host records approved_by agent. That is not a person's approval. When the owner turns on human approval for that step, the signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing.`}]},{name:`Fit`,description:`Why a result fits, with evidence links a human can open.`,closed:!0,fields:[{name:`score`,required:!0,type:`integer, min 0, max 100`,description:``},{name:`reasons`,required:!0,type:`array of string, min length 1, max length 200`,description:``},{name:`evidence`,required:!1,type:`array of Url`,description:``}]},{name:`Trust`,description:`Aggregate trust signals. Never a list of the party's counterparties.`,closed:!0,fields:[{name:`reputation`,required:!1,type:`integer | null, min 0, max 1000`,description:``},{name:`completed_deals`,required:!1,type:`integer, min 0`,description:``},{name:`verified`,required:!1,type:`array of "person" | "domain" | "dns" | "platform_oauth" | "bio_token" | "challenge_post"`,description:`person: a person has claimed the party (passkey or signed-in session), with no identity disclosed. The others prove control of a space.`},{name:`path`,required:!1,type:`array of string, min length 1, max length 200`,description:``}]},{name:`Capabilities`,description:``,closed:!0,fields:[{name:`messages`,required:!1,type:`array of MessageType`,description:``},{name:`spaces`,required:!1,type:`array of SpaceKind`,description:``},{name:`actions`,required:!1,type:`array of string, pattern`,description:``},{name:`proofs`,required:!1,type:`array of ProofMethod`,description:``},{name:`tracking`,required:!1,type:`object, closed`,description:``},{name:`signatures`,required:!1,type:`object, closed`,description:``},{name:`inbox`,required:!1,type:`object, closed`,description:``},{name:`checks`,required:!1,type:`object, closed`,description:`What the host can verify before content goes out. claims: real compares copy with the destination page; stub holds every offer it cannot check; none never checks. A host without real claims checks never runs execute as auto.`},{name:`ext`,required:!1,type:`array of ExtName`,description:``}]},{name:`Checks`,description:`Per-offer check status, shown to the agent before it proposes copy.`,closed:!0,fields:[{name:`claims`,required:!0,type:`"checked" | "not_checked" | "failed"`,description:``},{name:`at`,required:!1,type:`Timestamp`,description:``}]}],valid:[],invalid:[]},{file:`0.3/discovery.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/discovery.schema.json`,slug:`discovery`,href:`/protocol/spec/0.3/discovery/`,title:`Discovery`,description:`The document a host serves at /.well-known/orp.json. Serving it correctly is all Level 0 asks. URLs may be absolute or a path on the document's origin.`,kind:`schema`,type:``,required:[`orp`,`host`],anyOf:[],sections:[{name:`Discovery`,description:`The document a host serves at /.well-known/orp.json. Serving it correctly is all Level 0 asks. URLs may be absolute or a path on the document's origin.`,closed:!0,fields:[{name:`orp`,required:!0,type:`array of Version`,description:`Supported protocol versions, newest first.`},{name:`host`,required:!0,type:`PartyId`,description:`The host's own party id, orp:<host>/host. Host-originated messages use it as from.`},{name:`name`,required:!1,type:`string, min length 1, max length 80`,description:``},{name:`description`,required:!1,type:`string, max length 500`,description:``},{name:`endpoint`,required:!1,type:`UrlOrPath`,description:`The HTTP binding: POST messages here. Required for Level 1 and up.`},{name:`suggest`,required:!1,type:`UrlOrPath`,description:`POST a draft here with no credentials to get deals a person could take now. Required for Level 1 and up.`},{name:`join`,required:!1,type:`UrlOrPath`,description:`POST here with no credentials and no personal data to get a party id and token. Required for Level 1 and up.`},{name:`mcp`,required:!1,type:`object, closed`,description:``},{name:`auth`,required:!1,type:`object, closed`,description:``},{name:`tracking`,required:!1,type:`object, closed`,description:``},{name:`conformance`,required:!1,type:`object, closed`,description:``},{name:`capabilities`,required:!1,type:`Capabilities`,description:``},{name:`keys`,required:!1,type:`array of object`,description:`Public keys for host signatures (Level 3), as JWKs.`},{name:`policies`,required:!1,type:`object, closed`,description:``},{name:`docs`,required:!1,type:`UrlOrPath`,description:``},{name:`legacy`,required:!1,type:`object`,description:`Pre-ORP interfaces the host still serves. Informational; ORP clients ignore it.`}]},{name:`UrlOrPath`,description:``,closed:!1,fields:[]},{name:`UrlOrPath`,description:``,closed:!1,fields:[]},{name:`UrlOrPath`,description:``,closed:!1,fields:[]},{name:`UrlOrPath`,description:``,closed:!1,fields:[]}],valid:[{name:`discovery.json`,json:`{
  "orp": [
    "0.3"
  ],
  "host": "orp:hub.example/host",
  "name": "Hub Example",
  "description": "An example ORP host.",
  "endpoint": "https://hub.example/orp",
  "suggest": "https://hub.example/orp/suggest",
  "join": "https://hub.example/orp/join",
  "mcp": {
    "url": "https://hub.example/mcp",
    "transport": "streamable-http",
    "auth": [
      "bearer",
      "oauth"
    ],
    "binding": true,
    "tools": [
      "orp_suggest",
      "orp_join",
      "orp_send",
      "orp_inbox",
      "orp_deal"
    ]
  },
  "auth": {
    "bearer": true,
    "oauth_metadata": "https://hub.example/.well-known/oauth-protected-resource/mcp"
  },
  "tracking": {
    "links": true,
    "codes": true,
    "base": "https://hub.example/go/"
  },
  "conformance": {
    "levels": [
      "L0",
      "L1",
      "L2",
      "L3"
    ]
  },
  "capabilities": {
    "messages": [
      "hello",
      "discover",
      "results",
      "space",
      "request",
      "offer",
      "counter",
      "accept",
      "decline",
      "deal",
      "execute",
      "receipt",
      "outcome",
      "payout",
      "revoke",
      "consent",
      "dispute",
      "reputation",
      "error"
    ],
    "proofs": [
      "link",
      "proof_url",
      "platform_check",
      "attestation",
      "witness",
      "provider"
    ],
    "tracking": {
      "links": true,
      "codes": true
    },
    "signatures": {
      "algs": [
        "EdDSA"
      ],
      "required": false
    },
    "inbox": {
      "poll": true
    },
    "checks": {
      "claims": "real"
    }
  },
  "keys": [
    {
      "kty": "OKP",
      "crv": "Ed25519",
      "x": "kdyMDwk4CBcDCgiyPImPfJuJr-2QdJ7l72X-4lztu1o",
      "kid": "host-2026-10",
      "alg": "EdDSA",
      "use": "sig"
    }
  ],
  "policies": {
    "terms": "https://hub.example/terms",
    "privacy": "https://hub.example/privacy",
    "abuse": "https://hub.example/abuse"
  },
  "docs": "https://hub.example/docs/orp"
}`}],invalid:[{name:`invalid/discovery-without-versions.json`,why:`Discovery must list the versions the host speaks.`,json:`{
  "host": "orp:hub.example/host",
  "name": "No versions"
}`}]},{file:`0.3/envelope.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/envelope.schema.json`,slug:`envelope`,href:`/protocol/spec/0.3/envelope/`,title:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,kind:`schema`,type:``,required:[`orp`,`type`,`id`,`ts`,`from`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]}],valid:[],invalid:[]},{file:`0.3/http/inbox.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/http/inbox.schema.json`,slug:`inbox`,href:`/protocol/spec/0.3/inbox/`,title:`InboxPage`,description:`The response to GET {endpoint}/inbox?after=<cursor>: messages for the caller's party, oldest first. Pass next back as after to continue.`,kind:`http`,type:``,required:[`messages`,`next`],anyOf:[],sections:[{name:`InboxPage`,description:`The response to GET {endpoint}/inbox?after=<cursor>: messages for the caller's party, oldest first. Pass next back as after to continue.`,closed:!0,fields:[{name:`messages`,required:!0,type:`array of message.schema.json`,description:``},{name:`next`,required:!0,type:`string, max length 200`,description:``}]}],valid:[{name:`inbox.json`,json:`{
  "messages": [
    {
      "orp": "0.3",
      "type": "deal",
      "id": "m_host_0007",
      "ts": "2026-10-21T11:30:01Z",
      "from": "orp:hub.example/host",
      "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
      "deal": "deal_swp_4KfP",
      "body": {
        "state": "awaiting",
        "offerer": "orp:hub.example/p_Vb3nR8sYc1Qe6",
        "acceptor": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "terms": {
          "title": "Newsletter mention for a launch repost",
          "kind": "swap",
          "categories": [
            "software.productivity"
          ],
          "disclosure": {
            "label": "Partner mention",
            "lang": "en"
          },
          "obligations": [
            {
              "id": "mention",
              "by": "offerer",
              "action": "place",
              "space": {
                "kinds": [
                  "newsletter.issue"
                ]
              },
              "content": {
                "link": true,
                "brief": "One paragraph on why Acme Notes suits solo founders.",
                "max_chars": 600,
                "code": true
              },
              "destination": "https://acmenotes.example/",
              "due": "P14D",
              "proof": [
                "link",
                "witness",
                "attestation"
              ]
            },
            {
              "id": "repost",
              "by": "acceptor",
              "action": "repost",
              "target": "https://x.com/indiestackweekly/status/1848000000000000000",
              "space": {
                "kinds": [
                  "social.repost"
                ]
              },
              "due": "P7D",
              "keep": "P14D",
              "proof": [
                "platform_check",
                "proof_url",
                "attestation"
              ]
            }
          ]
        },
        "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
        "round": 2,
        "bound": [
          {
            "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
            "authority": {
              "type": "mandate"
            },
            "at": "2026-10-21T11:20:01Z"
          }
        ],
        "updated": "2026-10-21T11:30:01Z",
        "waiting": {
          "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
          "step": "accept",
          "since": "2026-10-21T11:30:01Z"
        }
      }
    },
    {
      "orp": "0.3",
      "type": "deal",
      "id": "m_host_0009",
      "ts": "2026-10-21T12:10:00Z",
      "from": "orp:hub.example/host",
      "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
      "deal": "deal_swp_4KfP",
      "body": {
        "state": "active",
        "offerer": "orp:hub.example/p_Vb3nR8sYc1Qe6",
        "acceptor": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "terms": {
          "title": "Newsletter mention for a launch repost",
          "kind": "swap",
          "categories": [
            "software.productivity"
          ],
          "disclosure": {
            "label": "Partner mention",
            "lang": "en"
          },
          "obligations": [
            {
              "id": "mention",
              "by": "offerer",
              "action": "place",
              "space": {
                "kinds": [
                  "newsletter.issue"
                ]
              },
              "content": {
                "link": true,
                "brief": "One paragraph on why Acme Notes suits solo founders.",
                "max_chars": 600,
                "code": true
              },
              "destination": "https://acmenotes.example/",
              "due": "P14D",
              "proof": [
                "link",
                "witness",
                "attestation"
              ]
            },
            {
              "id": "repost",
              "by": "acceptor",
              "action": "repost",
              "target": "https://x.com/indiestackweekly/status/1848000000000000000",
              "space": {
                "kinds": [
                  "social.repost"
                ]
              },
              "due": "P7D",
              "keep": "P14D",
              "proof": [
                "platform_check",
                "proof_url",
                "attestation"
              ]
            }
          ]
        },
        "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
        "round": 2,
        "bound": [
          {
            "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
            "authority": {
              "type": "mandate"
            },
            "at": "2026-10-21T11:20:01Z"
          },
          {
            "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
            "authority": {
              "type": "consent",
              "ref": "cns_3Vt8"
            },
            "at": "2026-10-21T12:10:00Z"
          }
        ],
        "tracking": [
          {
            "obligation": "mention",
            "link": "https://hub.example/go/Nw4Hd7Pe",
            "code": "INDIE14",
            "destination": "https://acmenotes.example/",
            "status": "live"
          }
        ],
        "obligations": [
          {
            "id": "mention",
            "state": "pending",
            "due": "2026-11-04T12:10:00Z"
          },
          {
            "id": "repost",
            "state": "pending",
            "due": "2026-10-28T12:10:00Z"
          }
        ],
        "spaces": [
          {
            "id": "sp_news_1",
            "kind": "newsletter.issue",
            "platform": "newsletter",
            "title": "Indie Stack Weekly",
            "topics": [
              "software.productivity",
              "business.startups"
            ],
            "languages": [
              "en"
            ],
            "audience": {
              "size": "10k-100k",
              "regions": [
                {
                  "code": "US",
                  "share": "25-50%"
                },
                {
                  "code": "DE",
                  "share": "10-25%"
                }
              ],
              "k_min": 50,
              "source": "host_measured"
            },
            "accepts": [
              "affiliate",
              "sponsored",
              "swap"
            ],
            "disclosure": {
              "label": "Partner mention",
              "lang": "en"
            },
            "verified": [
              {
                "method": "domain",
                "at": "2026-09-02T08:00:00Z"
              }
            ],
            "rules": {
              "rel": "sponsored",
              "max_per_week": 2
            },
            "locator": "https://indiestackweekly.example/archive"
          }
        ],
        "updated": "2026-10-21T12:10:00Z"
      }
    }
  ],
  "next": "c_000042"
}`}],invalid:[]},{file:`0.3/http/join-request.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/http/join-request.schema.json`,slug:`join-request`,href:`/protocol/spec/0.3/join-request/`,title:`JoinRequest`,description:`POST to the discovery document's join URL with no credentials. There is no field for an email, phone, name, password or website, and a host must not ask for one: the party is pseudonymous from the start. An empty object is a complete request.`,kind:`http`,type:``,required:[],anyOf:[],sections:[{name:`JoinRequest`,description:`POST to the discovery document's join URL with no credentials. There is no field for an email, phone, name, password or website, and a host must not ask for one: the party is pseudonymous from the start. An empty object is a complete request.`,closed:!0,fields:[{name:`agent`,required:!1,type:`Agent`,description:``},{name:`key`,required:!1,type:`object, closed`,description:`Optional Ed25519 public key (JWK). Once set, every message from this party must be signed with it.`}]}],valid:[{name:`http/join-request.json`,json:`{
  "agent": {
    "name": "writer-assistant",
    "version": "2.1.0"
  }
}`}],invalid:[{name:`invalid/join-with-email.json`,why:`Joining takes no email, phone, name or website. There is no field for one.`,json:`{
  "agent": {
    "name": "writer-assistant"
  },
  "email": "writer@example.com"
}`}]},{file:`0.3/http/join-response.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/http/join-response.schema.json`,slug:`join-response`,href:`/protocol/spec/0.3/join-response/`,title:`JoinResponse`,description:`A new pseudonymous party, usable at once. Until an owner sets limits, every step that binds asks: the waiting party's agent approves it. The host records approved_by agent. That is not a person's approval. The signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing. The claim URL lets the owner make the party theirs (a passkey, no email), set limits and get paid.`,kind:`http`,type:``,required:[`party`,`token`,`claim`],anyOf:[],sections:[{name:`JoinResponse`,description:`A new pseudonymous party, usable at once. Until an owner sets limits, every step that binds asks: the waiting party's agent approves it. The host records approved_by agent. That is not a person's approval. The signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing. The claim URL lets the owner make the party theirs (a passkey, no email), set limits and get paid.`,closed:!0,fields:[{name:`party`,required:!0,type:`PartyId`,description:``},{name:`token`,required:!0,type:`string, min length 20, max length 200`,description:`Bearer token for this party. Shown once; keep it like a password.`},{name:`claim`,required:!0,type:`Url`,description:`Give this to your person, never to a counterparty.`},{name:`steps`,required:!1,type:`Steps`,description:`The modes in force until a person sets limits.`},{name:`limits`,required:!1,type:`object, closed`,description:``}]}],valid:[{name:`http/join-response.json`,json:`{
  "party": "orp:hub.example/p_Wr1tE9xQ2mKa5",
  "token": "orpt_3mV8kQ2xL9wR7tB4nZ6pD1sF5hJ0cY",
  "claim": "https://hub.example/claim/clm_Hk7Qz2Lw9Rt4",
  "steps": {
    "list": "ask",
    "negotiate": "ask",
    "accept": "ask",
    "execute": "ask",
    "amend": "ask",
    "settle": "ask",
    "share": "ask",
    "revoke": "auto"
  },
  "limits": {
    "open_deals": 5,
    "messages_per_minute": 30
  }
}`}],invalid:[]},{file:`0.3/http/suggest-request.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/http/suggest-request.schema.json`,slug:`suggest-request`,href:`/protocol/spec/0.3/suggest-request/`,title:`SuggestRequest`,description:`POST to the discovery document's suggest URL with no credentials. Any agent that writes for someone (a post, a thread, a newsletter) sends what it is writing and gets deals its person could take now. The host uses the text in memory to match, then forgets it: it never stores, logs, shares or trains on it.`,kind:`http`,type:``,required:[`text`],anyOf:[],sections:[{name:`SuggestRequest`,description:`POST to the discovery document's suggest URL with no credentials. Any agent that writes for someone (a post, a thread, a newsletter) sends what it is writing and gets deals its person could take now. The host uses the text in memory to match, then forgets it: it never stores, logs, shares or trains on it.`,closed:!0,fields:[{name:`text`,required:!0,type:`string, min length 1, max length 4000`,description:`The draft, or a short summary of it. Leave out names, emails and anything personal.`},{name:`platform`,required:!1,type:`Platform`,description:`Where it will be published.`},{name:`space_kind`,required:!1,type:`SpaceKind`,description:``},{name:`want`,required:!1,type:`array of "earn" | "grow"`,description:`earn: get paid (affiliate, sponsored, cross_sell). grow: get readers or reach (swap, traffic_swap, link_exchange, repost). Both when absent.`},{name:`kinds`,required:!1,type:`array of DealKind`,description:``},{name:`lang`,required:!1,type:`string, pattern`,description:`Language for the pitch.`},{name:`join`,required:!1,type:`join-request.schema.json`,description:`Optional: also join in this call. The reply then carries joined, so taking an offer needs only an accept.`},{name:`limit`,required:!1,type:`integer, min 1, max 5`,description:``}]}],valid:[{name:`http/suggest-request.json`,json:`{
  "text": "Thread draft: five tools that keep a solo founder organised. 1/ Notes that sync with invoices...",
  "platform": "x",
  "space_kind": "social.post",
  "want": [
    "earn",
    "grow"
  ],
  "lang": "en",
  "limit": 3,
  "join": {
    "agent": {
      "name": "thread-writer",
      "version": "1.4"
    }
  }
}`}],invalid:[]},{file:`0.3/http/suggest-response.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/http/suggest-response.schema.json`,slug:`suggest-response`,href:`/protocol/spec/0.3/suggest-response/`,title:`SuggestResponse`,description:`Standing offers the person could take now, or open spaces that trade with posts like this one, best first, each with a pitch to show as is. Taking an offer: POST join once (no account), then send accept with the offer id and terms_hash. Taking a space: join, then send a directed offer to the space's party.`,kind:`http`,type:``,required:[`items`,`join`],anyOf:[],sections:[{name:`SuggestResponse`,description:`Standing offers the person could take now, or open spaces that trade with posts like this one, best first, each with a pitch to show as is. Taking an offer: POST join once (no account), then send accept with the offer id and terms_hash. Taking a space: join, then send a directed offer to the space's party.`,closed:!0,fields:[{name:`items`,required:!0,type:`array of object`,description:``},{name:`empty_reason`,required:!1,type:`"no_match" | "below_threshold" | "filtered"`,description:``},{name:`joined`,required:!1,type:`join-response.schema.json`,description:`Present when the request carried join.`},{name:`join`,required:!0,type:`UrlOrPath`,description:`Where to get a party id and token with no account, email or signup.`}]}],valid:[{name:`http/suggest-response.json`,json:`{
  "items": [
    {
      "offer": {
        "id": "ofr_acme_30",
        "terms": {
          "title": "Acme Notes: 30% of first-year revenue",
          "kind": "affiliate",
          "categories": [
            "software.productivity"
          ],
          "disclosure": {
            "label": "Affiliate link",
            "lang": "en"
          },
          "obligations": [
            {
              "id": "place",
              "by": "acceptor",
              "action": "place",
              "content": {
                "link": true,
                "must_not_include": [
                  "free forever"
                ]
              },
              "destination": "https://acmenotes.example/pricing",
              "proof": [
                "link"
              ]
            },
            {
              "id": "commission",
              "by": "offerer",
              "action": "pay",
              "per": "conversion",
              "rate_bps": 3000,
              "window": "P30D",
              "rail": "stripe",
              "proof": [
                "provider"
              ]
            }
          ]
        },
        "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
        "expires": "2027-01-31T00:00:00Z"
      },
      "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
      "fit": {
        "score": 82,
        "reasons": [
          "The thread recommends note-taking tools for solo founders."
        ]
      },
      "trust": {
        "reputation": 858,
        "completed_deals": 48,
        "verified": [
          "person",
          "domain"
        ]
      },
      "pitch": "You can monetize this thread: Acme Notes pays 30% of first-year revenue for readers who subscribe. You'd add one link labelled 'Affiliate link'. Want to test it?"
    },
    {
      "space": {
        "id": "sp_news_1",
        "kind": "newsletter.issue",
        "platform": "newsletter",
        "title": "Indie Stack Weekly",
        "topics": [
          "software.productivity",
          "business.startups"
        ],
        "languages": [
          "en"
        ],
        "audience": {
          "size": "10k-100k",
          "regions": [
            {
              "code": "US",
              "share": "25-50%"
            },
            {
              "code": "DE",
              "share": "10-25%"
            }
          ],
          "k_min": 50,
          "source": "host_measured"
        },
        "accepts": [
          "affiliate",
          "sponsored",
          "swap"
        ],
        "disclosure": {
          "label": "Partner mention",
          "lang": "en"
        },
        "verified": [
          {
            "method": "domain",
            "at": "2026-09-02T08:00:00Z"
          }
        ],
        "rules": {
          "rel": "sponsored",
          "max_per_week": 2
        }
      },
      "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
      "fit": {
        "score": 64,
        "reasons": [
          "Writes for the same readers and accepts swaps."
        ]
      },
      "trust": {
        "reputation": 791,
        "completed_deals": 17,
        "verified": [
          "person",
          "domain"
        ]
      },
      "pitch": "Want more readers? Indie Stack Weekly (10k to 100k readers) trades mentions with posts like this. Want me to propose a swap?"
    }
  ],
  "join": "https://hub.example/orp/join",
  "joined": {
    "party": "orp:hub.example/p_Wr1tE9xQ2mKa5",
    "token": "orpt_3mV8kQ2xL9wR7tB4nZ6pD1sF5hJ0cY",
    "claim": "https://hub.example/claim/clm_Hk7Qz2Lw9Rt4",
    "steps": {
      "list": "ask",
      "negotiate": "ask",
      "accept": "ask",
      "execute": "ask",
      "amend": "ask",
      "settle": "ask",
      "share": "ask",
      "revoke": "auto"
    },
    "limits": {
      "open_deals": 5,
      "messages_per_minute": 30
    }
  }
}`}],invalid:[{name:`invalid/suggest-without-pitch.json`,why:`Every suggestion carries a pitch the agent can show its person.`,json:`{
  "items": [
    {
      "offer": {
        "id": "ofr_acme_30",
        "terms": {
          "title": "Acme Notes: 30% of first-year revenue",
          "kind": "affiliate",
          "categories": [
            "software.productivity"
          ],
          "disclosure": {
            "label": "Affiliate link",
            "lang": "en"
          },
          "obligations": [
            {
              "id": "place",
              "by": "acceptor",
              "action": "place",
              "content": {
                "link": true,
                "must_not_include": [
                  "free forever"
                ]
              },
              "destination": "https://acmenotes.example/pricing",
              "proof": [
                "link"
              ]
            },
            {
              "id": "commission",
              "by": "offerer",
              "action": "pay",
              "per": "conversion",
              "rate_bps": 3000,
              "window": "P30D",
              "rail": "stripe",
              "proof": [
                "provider"
              ]
            }
          ]
        },
        "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
        "expires": "2027-01-31T00:00:00Z"
      },
      "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
      "fit": {
        "score": 82,
        "reasons": [
          "The thread recommends note-taking tools for solo founders."
        ]
      },
      "trust": {
        "reputation": 858,
        "completed_deals": 48,
        "verified": [
          "person",
          "domain"
        ]
      }
    },
    {
      "space": {
        "id": "sp_news_1",
        "kind": "newsletter.issue",
        "platform": "newsletter",
        "title": "Indie Stack Weekly",
        "topics": [
          "software.productivity",
          "business.startups"
        ],
        "languages": [
          "en"
        ],
        "audience": {
          "size": "10k-100k",
          "regions": [
            {
              "code": "US",
              "share": "25-50%"
            },
            {
              "code": "DE",
              "share": "10-25%"
            }
          ],
          "k_min": 50,
          "source": "host_measured"
        },
        "accepts": [
          "affiliate",
          "sponsored",
          "swap"
        ],
        "disclosure": {
          "label": "Partner mention",
          "lang": "en"
        },
        "verified": [
          {
            "method": "domain",
            "at": "2026-09-02T08:00:00Z"
          }
        ],
        "rules": {
          "rel": "sponsored",
          "max_per_week": 2
        }
      },
      "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
      "fit": {
        "score": 64,
        "reasons": [
          "Writes for the same readers and accepts swaps."
        ]
      },
      "trust": {
        "reputation": 791,
        "completed_deals": 17,
        "verified": [
          "person",
          "domain"
        ]
      },
      "pitch": "Want more readers? Indie Stack Weekly (10k to 100k readers) trades mentions with posts like this. Want me to propose a swap?"
    }
  ],
  "join": "https://hub.example/orp/join"
}`}]},{file:`0.3/message.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/message.schema.json`,slug:`message`,href:`/protocol/spec/0.3/message/`,title:`Message`,description:`Any ORP 0.3 message: the envelope, then the body rules for the declared type. Extension types (reverse-DNS names) get envelope checks only; a receiver that does not know them answers unsupported_type.`,kind:`schema`,type:``,required:[],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`Composed schemas`,description:`Any ORP 0.3 message: the envelope, then the body rules for the declared type. Extension types (reverse-DNS names) get envelope checks only; a receiver that does not know them answers unsupported_type.`,closed:!1,fields:[{name:`envelope.schema.json`,required:!1,type:`envelope.schema.json`,description:``},{name:`hello`,required:!0,type:`messages/hello.schema.json`,description:``},{name:`discover`,required:!0,type:`messages/discover.schema.json`,description:``},{name:`results`,required:!0,type:`messages/results.schema.json`,description:``},{name:`space`,required:!0,type:`messages/space.schema.json`,description:``},{name:`request`,required:!0,type:`messages/request.schema.json`,description:``},{name:`offer`,required:!0,type:`messages/offer.schema.json`,description:``},{name:`counter`,required:!0,type:`messages/counter.schema.json`,description:``},{name:`accept`,required:!0,type:`messages/accept.schema.json`,description:``},{name:`decline`,required:!0,type:`messages/decline.schema.json`,description:``},{name:`deal`,required:!0,type:`messages/deal.schema.json`,description:``},{name:`execute`,required:!0,type:`messages/execute.schema.json`,description:``},{name:`receipt`,required:!0,type:`messages/receipt.schema.json`,description:``},{name:`outcome`,required:!0,type:`messages/outcome.schema.json`,description:``},{name:`payout`,required:!0,type:`messages/payout.schema.json`,description:``},{name:`revoke`,required:!0,type:`messages/revoke.schema.json`,description:``},{name:`consent`,required:!0,type:`messages/consent.schema.json`,description:``},{name:`dispute`,required:!0,type:`messages/dispute.schema.json`,description:``},{name:`reputation`,required:!0,type:`messages/reputation.schema.json`,description:``},{name:`error`,required:!0,type:`messages/error.schema.json`,description:``}]}],valid:[],invalid:[{name:`invalid/accept-missing-hash.json`,why:`Accept must name the exact terms by hash.`,json:`{
  "orp": "0.3",
  "type": "accept",
  "id": "m_pub_0005",
  "ts": "2026-10-21T11:30:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "spaces": [
      {
        "obligation": "mention",
        "space": "sp_news_1"
      }
    ]
  }
}`},{name:`invalid/accept-no-deal-no-offer.json`,why:`Accept needs a deal or a standing offer id.`,json:`{
  "orp": "0.3",
  "type": "accept",
  "id": "m_pub_0003",
  "ts": "2026-10-21T10:02:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo"
  }
}`},{name:`invalid/audience-k-too-small.json`,why:`Audience segments use k_min of at least 50.`,json:`{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0009",
  "ts": "2026-10-21T12:10:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "body": {
    "state": "active",
    "offerer": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "acceptor": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "newsletter.issue"
            ]
          },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600,
            "code": true
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": [
            "link",
            "witness",
            "attestation"
          ]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": {
            "kinds": [
              "social.repost"
            ]
          },
          "due": "P7D",
          "keep": "P14D",
          "proof": [
            "platform_check",
            "proof_url",
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "round": 2,
    "bound": [
      {
        "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T11:20:01Z"
      },
      {
        "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
        "authority": {
          "type": "consent",
          "ref": "cns_3Vt8"
        },
        "at": "2026-10-21T12:10:00Z"
      }
    ],
    "tracking": [
      {
        "obligation": "mention",
        "link": "https://hub.example/go/Nw4Hd7Pe",
        "code": "INDIE14",
        "destination": "https://acmenotes.example/",
        "status": "live"
      }
    ],
    "obligations": [
      {
        "id": "mention",
        "state": "pending",
        "due": "2026-11-04T12:10:00Z"
      },
      {
        "id": "repost",
        "state": "pending",
        "due": "2026-10-28T12:10:00Z"
      }
    ],
    "spaces": [
      {
        "id": "sp_news_1",
        "kind": "newsletter.issue",
        "platform": "newsletter",
        "title": "Indie Stack Weekly",
        "topics": [
          "software.productivity",
          "business.startups"
        ],
        "languages": [
          "en"
        ],
        "audience": {
          "size": "10k-100k",
          "regions": [
            {
              "code": "US",
              "share": "25-50%"
            },
            {
              "code": "DE",
              "share": "10-25%"
            }
          ],
          "k_min": 10,
          "source": "host_measured"
        },
        "accepts": [
          "affiliate",
          "sponsored",
          "swap"
        ],
        "disclosure": {
          "label": "Partner mention",
          "lang": "en"
        },
        "verified": [
          {
            "method": "domain",
            "at": "2026-09-02T08:00:00Z"
          }
        ],
        "rules": {
          "rel": "sponsored",
          "max_per_week": 2
        },
        "locator": "https://indiestackweekly.example/archive"
      }
    ],
    "updated": "2026-10-21T12:10:00Z"
  }
}`},{name:`invalid/consent-unknown-assurance.json`,why:`Consent assurance is one of four known ways a host knows the owner decided.`,json:`{
  "orp": "0.3",
  "type": "consent",
  "id": "m_host_0008",
  "ts": "2026-10-21T12:10:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "consent": "cns_3Vt8",
    "escalation": "esc_7Rk2",
    "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "decision": "approved",
    "step": "accept",
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "reasons": [
      "step_ask"
    ],
    "assurance": "agent_said_so",
    "at": "2026-10-21T12:09:58Z"
  }
}`},{name:`invalid/counter-without-deal.json`,why:`A counter belongs to an open deal.`,json:`{
  "orp": "0.3",
  "type": "counter",
  "id": "m_mkr_0003",
  "ts": "2026-10-21T11:20:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "body": {
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "newsletter.issue"
            ]
          },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600,
            "code": true
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": [
            "link",
            "witness",
            "attestation"
          ]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": {
            "kinds": [
              "social.repost"
            ]
          },
          "due": "P7D",
          "keep": "P14D",
          "proof": [
            "platform_check",
            "proof_url",
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "note": "Happy to repost. Our reposts stay up 14 days, and please add the code too."
  }
}`},{name:`invalid/discover-unknown-field.json`,why:`Core bodies are closed; extension data goes in ext.`,json:`{
  "orp": "0.3",
  "type": "discover",
  "id": "m_pub_0002",
  "ts": "2026-10-21T10:01:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "want": "offers",
    "context": {
      "text": "This week: five tools that keep a solo founder organised, from note-taking to invoicing.",
      "space": "sp_news_1"
    },
    "filters": {
      "kinds": [
        "affiliate",
        "swap"
      ],
      "categories": [
        "software.productivity"
      ],
      "min_reputation": 500
    },
    "limit": 5,
    "reader_emails": true
  }
}`},{name:`invalid/dispute-open-without-claim.json`,why:`Opening a dispute states the claim.`,json:`{
  "orp": "0.3",
  "type": "dispute",
  "id": "m_pub_0006",
  "ts": "2026-10-28T10:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "action": "open",
    "obligation": "repost",
    "statement": "The repost was removed on day 5. The deal says 14 days.",
    "evidence": [
      {
        "url": "https://x.com/acmenotes/status/1848100000000000000",
        "note": "Returns not found since 27 October."
      }
    ],
    "remedy": "redo"
  }
}`},{name:`invalid/envelope-68-shape.json`,why:`The #68 envelope (orp_version, message_type, sender, payload) is superseded by the 0.3 envelope (orp, type, from, body).`,json:`{
  "orp_version": "0.3.0",
  "message_id": "msg_01jc9004accept",
  "message_type": "accept",
  "deal_id": "deal_01jc9002swap",
  "sender": "orp:party:maker_codelens",
  "recipient": "orp:party:pub_tsweekly",
  "timestamp": "2026-10-21T09:45:00Z",
  "idempotency_key": "idem_accept_01jc9004",
  "payload": {
    "accepted_message_id": "msg_01jc9003counter",
    "accepted_seq": 2,
    "terms_hash": "sha256:4b227777d4dd1fc61c6f884f48641d02b4d121d3fd328cb08b5531fcacdabf8a"
  }
}`},{name:`invalid/envelope-missing-ts.json`,why:`ts is one of the six required envelope fields.`,json:`{
  "orp": "0.3",
  "type": "hello",
  "id": "m_pub_0001",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "agent": {
    "name": "indie-stack-agent",
    "version": "1.4.0"
  },
  "body": {
    "versions": [
      "0.3"
    ],
    "capabilities": {
      "messages": [
        "discover",
        "request",
        "offer",
        "counter",
        "accept",
        "decline",
        "execute",
        "receipt",
        "revoke",
        "dispute"
      ],
      "proofs": [
        "link",
        "proof_url",
        "attestation"
      ],
      "tracking": {
        "links": true,
        "codes": true
      }
    }
  }
}`},{name:`invalid/extension-type-not-reverse-dns.json`,why:`Extension types use reverse-DNS names.`,json:`{
  "orp": "0.3",
  "type": "boost",
  "id": "m_mkr_0010",
  "ts": "2026-10-23T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "ext": {
    "com.example.boost": {
      "tier": "gold"
    }
  },
  "body": {
    "budget": 2000
  }
}`},{name:`invalid/money-negative.json`,why:`Money is a non-negative integer in minor units.`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_host_0002",
  "ts": "2026-10-21T09:00:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "re": "m_mkr_0001",
  "body": {
    "id": "ofr_acme_30",
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true,
            "must_not_include": [
              "free forever"
            ]
          },
          "destination": "https://acmenotes.example/pricing",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "window": "P30D",
          "rail": "stripe",
          "proof": [
            "provider"
          ],
          "amount": {
            "amount_minor": -500,
            "currency": "EUR"
          }
        }
      ]
    },
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
    "limit": 500,
    "expires": "2027-01-31T00:00:00Z"
  }
}`},{name:`invalid/outcome-source-full-url.json`,why:`A click source is a registrable domain, never a page URL that could name a reader's page.`,json:`{
  "orp": "0.3",
  "type": "outcome",
  "id": "m_host_0006",
  "ts": "2026-10-28T00:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "period": {
      "from": "2026-10-21T00:00:00Z",
      "to": "2026-10-28T00:00:00Z"
    },
    "rows": [
      {
        "obligation": "place",
        "clicks": {
          "human": 3
        },
        "sources": [
          {
            "source": "https://someone.example/private/page?u=42",
            "human": 3
          }
        ]
      }
    ]
  }
}`},{name:`invalid/outcome-with-emails.json`,why:`Click counts are integers by class, never people.`,json:`{
  "orp": "0.3",
  "type": "outcome",
  "id": "m_host_0005",
  "ts": "2026-10-28T00:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "period": {
      "from": "2026-10-21T00:00:00Z",
      "to": "2026-10-28T00:00:00Z"
    },
    "rows": [
      {
        "obligation": "place",
        "clicks": {
          "human": 41,
          "bot": 3,
          "preview": 12,
          "test": 1,
          "emails": [
            "reader@example.com"
          ]
        },
        "conversions": 2,
        "refunds": 0,
        "revenue": {
          "amount_minor": 19200,
          "currency": "EUR"
        },
        "earned": {
          "amount_minor": 5760,
          "currency": "EUR"
        }
      }
    ],
    "final": false
  }
}`},{name:`invalid/outcome-with-ip.json`,why:`Outcome rows are closed: no field can carry a click address.`,json:`{
  "orp": "0.3",
  "type": "outcome",
  "id": "m_host_0005",
  "ts": "2026-10-28T00:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "period": {
      "from": "2026-10-21T00:00:00Z",
      "to": "2026-10-28T00:00:00Z"
    },
    "rows": [
      {
        "obligation": "place",
        "clicks": {
          "human": 41,
          "bot": 3,
          "preview": 12,
          "test": 1
        },
        "conversions": 2,
        "refunds": 0,
        "revenue": {
          "amount_minor": 19200,
          "currency": "EUR"
        },
        "earned": {
          "amount_minor": 5760,
          "currency": "EUR"
        },
        "ip": "203.0.113.7"
      }
    ],
    "final": false
  }
}`},{name:`invalid/party-id-email.json`,why:`A party id is orp:<host>/<opaque>, never an email address.`,json:`{
  "orp": "0.3",
  "type": "discover",
  "id": "m_pub_0002",
  "ts": "2026-10-21T10:01:00Z",
  "from": "alice@example.com",
  "body": {
    "want": "offers",
    "context": {
      "text": "This week: five tools that keep a solo founder organised, from note-taking to invoicing.",
      "space": "sp_news_1"
    },
    "filters": {
      "kinds": [
        "affiliate",
        "swap"
      ],
      "categories": [
        "software.productivity"
      ],
      "min_reputation": 500
    },
    "limit": 5
  }
}`},{name:`invalid/party-id-role-prefix-68.json`,why:`#68 party ids (orp:party:...) are not host-scoped and fail the 0.3 PartyId pattern.`,json:`{
  "orp": "0.3",
  "type": "hello",
  "id": "m_pub_0001",
  "ts": "2026-10-21T10:00:00Z",
  "from": "orp:party:maker_cloudbranch",
  "body": {
    "versions": [
      "0.3"
    ]
  }
}`},{name:`invalid/payout-answer-with-amount.json`,why:`A payee answers yes or no; it cannot restate the amount.`,json:`{
  "orp": "0.3",
  "type": "payout",
  "id": "m_pub_0202",
  "ts": "2026-12-02T09:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "payout": "po_aff_9wQz_1",
    "answer": "confirm",
    "amount": {
      "amount_minor": 9999,
      "currency": "EUR"
    }
  }
}`},{name:`invalid/receipt-bad-result.json`,why:`A receipt result is verified, failed or partial.`,json:`{
  "orp": "0.3",
  "type": "receipt",
  "id": "m_host_0010",
  "ts": "2026-10-22T09:00:05Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "re": "m_mkr_0004",
  "body": {
    "obligation": "repost",
    "result": "probably",
    "method": "platform_check",
    "at": "2026-10-22T09:00:04Z",
    "url": "https://x.com/acmenotes/status/1848100000000000000",
    "found": {
      "target": true
    },
    "next_check": "2026-10-29T09:00:00Z"
  }
}`},{name:`invalid/revoke-obligation-without-id.json`,why:`Revoking one obligation names it.`,json:`{
  "orp": "0.3",
  "type": "revoke",
  "id": "m_mkr_0006",
  "ts": "2026-12-01T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_aff_9wQz",
  "body": {
    "scope": "obligation",
    "reason": "owner_request",
    "note": "Spring pricing replaces this offer.",
    "effective": "2026-12-08T00:00:00Z"
  }
}`},{name:`invalid/share-personal-data.json`,why:`Share obligations carry aggregate data only.`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_host_0002",
  "ts": "2026-10-21T09:00:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "re": "m_mkr_0001",
  "body": {
    "id": "ofr_acme_30",
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true,
            "must_not_include": [
              "free forever"
            ]
          },
          "destination": "https://acmenotes.example/pricing",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 3000,
          "window": "P30D",
          "rail": "stripe",
          "proof": [
            "provider"
          ]
        },
        {
          "id": "emails",
          "by": "acceptor",
          "action": "share",
          "data": {
            "class": "personal",
            "fields": [
              "email"
            ],
            "k_min": 50,
            "purpose": "retargeting",
            "retention": "P30D"
          },
          "proof": [
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
    "limit": 500,
    "expires": "2027-01-31T00:00:00Z"
  }
}`},{name:`invalid/space-put-without-space.json`,why:`A put names the space it publishes.`,json:`{
  "orp": "0.3",
  "type": "space",
  "id": "m_pub_0102",
  "ts": "2026-10-21T08:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "action": "put"
  }
}`},{name:`invalid/tracking-status-broken.json`,why:`A tracking link is live or plain; it never breaks.`,json:`{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0004",
  "ts": "2026-10-21T10:02:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "re": "m_pub_0003",
  "body": {
    "state": "active",
    "offerer": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "acceptor": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "offer": "ofr_acme_30",
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true,
            "must_not_include": [
              "free forever"
            ]
          },
          "destination": "https://acmenotes.example/pricing",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 3000,
          "window": "P30D",
          "rail": "stripe",
          "proof": [
            "provider"
          ]
        }
      ]
    },
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
    "bound": [
      {
        "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T09:00:01Z"
      },
      {
        "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T10:02:01Z"
      }
    ],
    "tracking": [
      {
        "obligation": "place",
        "link": "https://hub.example/go/Zr8Kq2Lw",
        "destination": "https://acmenotes.example/pricing",
        "status": "broken"
      }
    ],
    "obligations": [
      {
        "id": "place",
        "state": "pending"
      },
      {
        "id": "commission",
        "state": "pending"
      }
    ],
    "updated": "2026-10-21T10:02:01Z"
  }
}`},{name:`invalid/traffic-swap-zero-clicks.json`,why:`A clicks target is at least one human click.`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_mkr_0011",
  "ts": "2026-10-24T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "terms": {
      "title": "Traffic swap: 500 readers each way",
      "kind": "traffic_swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "send_readers",
          "by": "offerer",
          "action": "place",
          "content": {
            "link": true
          },
          "destination": "https://indiestackweekly.example/subscribe",
          "clicks": 0,
          "due": "P30D",
          "proof": [
            "link"
          ]
        },
        {
          "id": "send_back",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true
          },
          "destination": "https://acmenotes.example/blog/launch",
          "clicks": 500,
          "due": "P30D",
          "proof": [
            "link"
          ]
        }
      ]
    },
    "terms_hash": "sha256-j2f8SES1MpLaz_7cQKk3GfIystwFTjGh0Iri5QLp38U",
    "note": "Our launch post is doing well. 500 readers each way this month?",
    "expires": "2026-10-31T00:00:00Z"
  }
}`},{name:`invalid/version-semver.json`,why:`The version is MAJOR.MINOR (0.3), not the 0.3.0 of the #68 draft.`,json:`{
  "orp": "0.3.0",
  "type": "hello",
  "id": "m_pub_0001",
  "ts": "2026-10-21T10:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "versions": [
      "0.3"
    ]
  }
}`}]},{file:`0.3/messages/accept.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/accept.schema.json`,slug:`accept`,href:`/protocol/spec/0.3/accept/`,title:`AcceptMessage`,description:`Agree to exact terms, named by terms_hash. Accept an open deal (deal set) or a standing offer (body.offer set, which opens a new deal). The host binds the sender only if the owner's constraints allow it; otherwise the deal waits for the waiting party's agent and the reply says so. The host records approved_by agent. That is not a person's approval. The signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing.`,kind:`message`,type:`accept`,required:[`type`,`body`],anyOf:[[`deal`],[]],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`AcceptMessage`,description:`Agree to exact terms, named by terms_hash. Accept an open deal (deal set) or a standing offer (body.offer set, which opens a new deal). The host binds the sender only if the owner's constraints allow it; otherwise the deal waits for the waiting party's agent and the reply says so. The host records approved_by agent. That is not a person's approval. The signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "accept"`,description:``},{name:`body`,required:!0,type:`AcceptBody`,description:``}]},{name:`AcceptBody`,description:``,closed:!0,fields:[{name:`terms_hash`,required:!0,type:`Hash`,description:``},{name:`offer`,required:!1,type:`LocalId`,description:``},{name:`spaces`,required:!1,type:`array of object, closed`,description:`Optional: which of the sender's spaces each obligation will use. Leave out to keep the choice open; the tracking link works on any of them.`},{name:`note`,required:!1,type:`Text`,description:``}]}],valid:[{name:`messages/07-accept-standing.json`,json:`{
  "orp": "0.3",
  "type": "accept",
  "id": "m_pub_0003",
  "ts": "2026-10-21T10:02:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "offer": "ofr_acme_30",
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo"
  }
}`},{name:`messages/13-accept.json`,json:`{
  "orp": "0.3",
  "type": "accept",
  "id": "m_pub_0005",
  "ts": "2026-10-21T11:30:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "spaces": [
      {
        "obligation": "mention",
        "space": "sp_news_1"
      }
    ]
  }
}`}],invalid:[{name:`invalid/accept-missing-hash.json`,why:`Accept must name the exact terms by hash.`,json:`{
  "orp": "0.3",
  "type": "accept",
  "id": "m_pub_0005",
  "ts": "2026-10-21T11:30:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "spaces": [
      {
        "obligation": "mention",
        "space": "sp_news_1"
      }
    ]
  }
}`},{name:`invalid/accept-no-deal-no-offer.json`,why:`Accept needs a deal or a standing offer id.`,json:`{
  "orp": "0.3",
  "type": "accept",
  "id": "m_pub_0003",
  "ts": "2026-10-21T10:02:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo"
  }
}`}]},{file:`0.3/messages/consent.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/consent.schema.json`,slug:`consent`,href:`/protocol/spec/0.3/consent/`,title:`ConsentMessage`,description:`The host's record that a person decided an escalation. Only a host sends it, never an agent. It names the exact terms_hash the person saw, how the host knows it was the owner (assurance), and when. The counterparty's copy omits reasons. A signed consent (envelope sig) is the verifiable human-consent record.`,kind:`message`,type:`consent`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`ConsentMessage`,description:`The host's record that a person decided an escalation. Only a host sends it, never an agent. It names the exact terms_hash the person saw, how the host knows it was the owner (assurance), and when. The counterparty's copy omits reasons. A signed consent (envelope sig) is the verifiable human-consent record.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "consent"`,description:``},{name:`body`,required:!0,type:`ConsentBody`,description:``}]},{name:`ConsentBody`,description:``,closed:!0,fields:[{name:`consent`,required:!0,type:`LocalId`,description:``},{name:`escalation`,required:!1,type:`LocalId`,description:``},{name:`party`,required:!0,type:`PartyId`,description:`The party whose person decided.`},{name:`decision`,required:!0,type:`"approved" | "refused"`,description:``},{name:`step`,required:!0,type:`Step`,description:``},{name:`terms_hash`,required:!0,type:`Hash`,description:``},{name:`reasons`,required:!1,type:`array of EscalationReason`,description:``},{name:`assurance`,required:!0,type:`"session" | "passkey" | "email_link" | "client_prompt"`,description:`session: the owner approved in a signed-in host session. passkey: the owner's passkey signed a challenge derived from terms_hash. email_link: a one-time link sent to the owner. client_prompt: an MCP client asked the person (the host cannot tell a person from an auto-approving client, so it is the weakest).`},{name:`at`,required:!0,type:`Timestamp`,description:``},{name:`expires`,required:!1,type:`Timestamp`,description:``}]}],valid:[{name:`messages/16-consent.json`,json:`{
  "orp": "0.3",
  "type": "consent",
  "id": "m_host_0008",
  "ts": "2026-10-21T12:10:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "consent": "cns_3Vt8",
    "escalation": "esc_7Rk2",
    "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "decision": "approved",
    "step": "accept",
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "reasons": [
      "step_ask"
    ],
    "assurance": "passkey",
    "at": "2026-10-21T12:09:58Z"
  }
}`}],invalid:[{name:`invalid/consent-unknown-assurance.json`,why:`Consent assurance is one of four known ways a host knows the owner decided.`,json:`{
  "orp": "0.3",
  "type": "consent",
  "id": "m_host_0008",
  "ts": "2026-10-21T12:10:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "consent": "cns_3Vt8",
    "escalation": "esc_7Rk2",
    "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "decision": "approved",
    "step": "accept",
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "reasons": [
      "step_ask"
    ],
    "assurance": "agent_said_so",
    "at": "2026-10-21T12:09:58Z"
  }
}`}]},{file:`0.3/messages/counter.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/counter.schema.json`,slug:`counter`,href:`/protocol/spec/0.3/counter/`,title:`CounterMessage`,description:`Replacement terms for an open deal. Either party may counter while the deal is proposed. Each counter replaces the terms and clears every earlier acceptance, so nobody is bound to terms they did not see.`,kind:`message`,type:`counter`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`CounterMessage`,description:`Replacement terms for an open deal. Either party may counter while the deal is proposed. Each counter replaces the terms and clears every earlier acceptance, so nobody is bound to terms they did not see.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "counter"`,description:``},{name:`body`,required:!0,type:`CounterBody`,description:``}]},{name:`CounterBody`,description:``,closed:!0,fields:[{name:`terms`,required:!0,type:`Terms`,description:``},{name:`terms_hash`,required:!1,type:`Hash`,description:``},{name:`note`,required:!1,type:`Text`,description:``},{name:`expires`,required:!1,type:`Timestamp`,description:``}]}],valid:[{name:`messages/12-counter.json`,json:`{
  "orp": "0.3",
  "type": "counter",
  "id": "m_mkr_0003",
  "ts": "2026-10-21T11:20:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "body": {
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "newsletter.issue"
            ]
          },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600,
            "code": true
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": [
            "link",
            "witness",
            "attestation"
          ]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": {
            "kinds": [
              "social.repost"
            ]
          },
          "due": "P7D",
          "keep": "P14D",
          "proof": [
            "platform_check",
            "proof_url",
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "note": "Happy to repost. Our reposts stay up 14 days, and please add the code too."
  }
}`}],invalid:[{name:`invalid/counter-without-deal.json`,why:`A counter belongs to an open deal.`,json:`{
  "orp": "0.3",
  "type": "counter",
  "id": "m_mkr_0003",
  "ts": "2026-10-21T11:20:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "body": {
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "newsletter.issue"
            ]
          },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600,
            "code": true
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": [
            "link",
            "witness",
            "attestation"
          ]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": {
            "kinds": [
              "social.repost"
            ]
          },
          "due": "P7D",
          "keep": "P14D",
          "proof": [
            "platform_check",
            "proof_url",
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "note": "Happy to repost. Our reposts stay up 14 days, and please add the code too."
  }
}`}]},{file:`0.3/messages/deal.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/deal.schema.json`,slug:`deal`,href:`/protocol/spec/0.3/deal/`,title:`DealMessage`,description:`The host's snapshot of a deal. It is the reply to every message that changes a deal, and it lands in both parties' inboxes. Each party gets its own copy: escalation ids, reasons and approval URLs appear only in the copy for the party that is waiting.`,kind:`message`,type:`deal`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`DealMessage`,description:`The host's snapshot of a deal. It is the reply to every message that changes a deal, and it lands in both parties' inboxes. Each party gets its own copy: escalation ids, reasons and approval URLs appear only in the copy for the party that is waiting.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "deal"`,description:``},{name:`body`,required:!0,type:`DealBody`,description:``}]},{name:`DealBody`,description:``,closed:!0,fields:[{name:`state`,required:!0,type:`DealState`,description:`proposed: terms on the table. awaiting: the waiting party's agent approves the ask by default. The host records approved_by agent. That is not a person's approval. The signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing. active: both bound, tracking live. completed: every obligation verified or waived. declined, expired, revoked: ended before or after activation. disputed: a dispute is open.`},{name:`offerer`,required:!0,type:`PartyId`,description:``},{name:`acceptor`,required:!0,type:`PartyId`,description:``},{name:`offer`,required:!1,type:`LocalId`,description:`The standing offer this deal came from, if any.`},{name:`terms`,required:!0,type:`Terms`,description:``},{name:`terms_hash`,required:!0,type:`Hash`,description:``},{name:`round`,required:!1,type:`integer, min 1, max 100`,description:``},{name:`bound`,required:!1,type:`array of object, closed`,description:`Who is bound to terms_hash, and by what authority.`},{name:`waiting`,required:!1,type:`Waiting`,description:``},{name:`tracking`,required:!1,type:`array of Tracking`,description:``},{name:`obligations`,required:!1,type:`array of object, closed`,description:``},{name:`spaces`,required:!1,type:`array of Space`,description:`Spaces named in the deal, with locators revealed once the deal is active.`},{name:`expires`,required:!1,type:`Timestamp`,description:``},{name:`updated`,required:!0,type:`Timestamp`,description:``}]}],valid:[{name:`messages/08-deal-active.json`,json:`{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0004",
  "ts": "2026-10-21T10:02:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "re": "m_pub_0003",
  "body": {
    "state": "active",
    "offerer": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "acceptor": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "offer": "ofr_acme_30",
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true,
            "must_not_include": [
              "free forever"
            ]
          },
          "destination": "https://acmenotes.example/pricing",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 3000,
          "window": "P30D",
          "rail": "stripe",
          "proof": [
            "provider"
          ]
        }
      ]
    },
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
    "bound": [
      {
        "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T09:00:01Z"
      },
      {
        "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T10:02:01Z"
      }
    ],
    "tracking": [
      {
        "obligation": "place",
        "link": "https://hub.example/go/Zr8Kq2Lw",
        "destination": "https://acmenotes.example/pricing",
        "status": "live",
        "param": "via",
        "alias": "ref"
      }
    ],
    "obligations": [
      {
        "id": "place",
        "state": "pending"
      },
      {
        "id": "commission",
        "state": "pending"
      }
    ],
    "updated": "2026-10-21T10:02:01Z"
  }
}`},{name:`messages/14-deal-awaiting-own-copy.json`,json:`{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0006",
  "ts": "2026-10-21T11:30:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "re": "m_pub_0005",
  "body": {
    "state": "awaiting",
    "offerer": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "acceptor": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "newsletter.issue"
            ]
          },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600,
            "code": true
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": [
            "link",
            "witness",
            "attestation"
          ]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": {
            "kinds": [
              "social.repost"
            ]
          },
          "due": "P7D",
          "keep": "P14D",
          "proof": [
            "platform_check",
            "proof_url",
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "round": 2,
    "bound": [
      {
        "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T11:20:01Z"
      }
    ],
    "updated": "2026-10-21T11:30:01Z",
    "waiting": {
      "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
      "step": "accept",
      "since": "2026-10-21T11:30:01Z",
      "escalation": "esc_7Rk2",
      "reasons": [
        "step_ask"
      ],
      "url": "https://hub.example/approve/esc_7Rk2"
    }
  }
}`},{name:`messages/15-deal-awaiting-counterparty-copy.json`,json:`{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0007",
  "ts": "2026-10-21T11:30:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "body": {
    "state": "awaiting",
    "offerer": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "acceptor": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "newsletter.issue"
            ]
          },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600,
            "code": true
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": [
            "link",
            "witness",
            "attestation"
          ]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": {
            "kinds": [
              "social.repost"
            ]
          },
          "due": "P7D",
          "keep": "P14D",
          "proof": [
            "platform_check",
            "proof_url",
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "round": 2,
    "bound": [
      {
        "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T11:20:01Z"
      }
    ],
    "updated": "2026-10-21T11:30:01Z",
    "waiting": {
      "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
      "step": "accept",
      "since": "2026-10-21T11:30:01Z"
    }
  }
}`},{name:`messages/17-deal-active-swap.json`,json:`{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0009",
  "ts": "2026-10-21T12:10:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "body": {
    "state": "active",
    "offerer": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "acceptor": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "newsletter.issue"
            ]
          },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600,
            "code": true
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": [
            "link",
            "witness",
            "attestation"
          ]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": {
            "kinds": [
              "social.repost"
            ]
          },
          "due": "P7D",
          "keep": "P14D",
          "proof": [
            "platform_check",
            "proof_url",
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "round": 2,
    "bound": [
      {
        "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T11:20:01Z"
      },
      {
        "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
        "authority": {
          "type": "consent",
          "ref": "cns_3Vt8"
        },
        "at": "2026-10-21T12:10:00Z"
      }
    ],
    "tracking": [
      {
        "obligation": "mention",
        "link": "https://hub.example/go/Nw4Hd7Pe",
        "code": "INDIE14",
        "destination": "https://acmenotes.example/",
        "status": "live"
      }
    ],
    "obligations": [
      {
        "id": "mention",
        "state": "pending",
        "due": "2026-11-04T12:10:00Z"
      },
      {
        "id": "repost",
        "state": "pending",
        "due": "2026-10-28T12:10:00Z"
      }
    ],
    "spaces": [
      {
        "id": "sp_news_1",
        "kind": "newsletter.issue",
        "platform": "newsletter",
        "title": "Indie Stack Weekly",
        "topics": [
          "software.productivity",
          "business.startups"
        ],
        "languages": [
          "en"
        ],
        "audience": {
          "size": "10k-100k",
          "regions": [
            {
              "code": "US",
              "share": "25-50%"
            },
            {
              "code": "DE",
              "share": "10-25%"
            }
          ],
          "k_min": 50,
          "source": "host_measured"
        },
        "accepts": [
          "affiliate",
          "sponsored",
          "swap"
        ],
        "disclosure": {
          "label": "Partner mention",
          "lang": "en"
        },
        "verified": [
          {
            "method": "domain",
            "at": "2026-09-02T08:00:00Z"
          }
        ],
        "rules": {
          "rel": "sponsored",
          "max_per_week": 2
        },
        "locator": "https://indiestackweekly.example/archive"
      }
    ],
    "updated": "2026-10-21T12:10:00Z"
  }
}`}],invalid:[{name:`invalid/audience-k-too-small.json`,why:`Audience segments use k_min of at least 50.`,json:`{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0009",
  "ts": "2026-10-21T12:10:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "body": {
    "state": "active",
    "offerer": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "acceptor": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "newsletter.issue"
            ]
          },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600,
            "code": true
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": [
            "link",
            "witness",
            "attestation"
          ]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": {
            "kinds": [
              "social.repost"
            ]
          },
          "due": "P7D",
          "keep": "P14D",
          "proof": [
            "platform_check",
            "proof_url",
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-XqlxlRGWPLFRQGlZNLpnwIpj-F1xr7UK1Pilom6pvCk",
    "round": 2,
    "bound": [
      {
        "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T11:20:01Z"
      },
      {
        "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
        "authority": {
          "type": "consent",
          "ref": "cns_3Vt8"
        },
        "at": "2026-10-21T12:10:00Z"
      }
    ],
    "tracking": [
      {
        "obligation": "mention",
        "link": "https://hub.example/go/Nw4Hd7Pe",
        "code": "INDIE14",
        "destination": "https://acmenotes.example/",
        "status": "live"
      }
    ],
    "obligations": [
      {
        "id": "mention",
        "state": "pending",
        "due": "2026-11-04T12:10:00Z"
      },
      {
        "id": "repost",
        "state": "pending",
        "due": "2026-10-28T12:10:00Z"
      }
    ],
    "spaces": [
      {
        "id": "sp_news_1",
        "kind": "newsletter.issue",
        "platform": "newsletter",
        "title": "Indie Stack Weekly",
        "topics": [
          "software.productivity",
          "business.startups"
        ],
        "languages": [
          "en"
        ],
        "audience": {
          "size": "10k-100k",
          "regions": [
            {
              "code": "US",
              "share": "25-50%"
            },
            {
              "code": "DE",
              "share": "10-25%"
            }
          ],
          "k_min": 10,
          "source": "host_measured"
        },
        "accepts": [
          "affiliate",
          "sponsored",
          "swap"
        ],
        "disclosure": {
          "label": "Partner mention",
          "lang": "en"
        },
        "verified": [
          {
            "method": "domain",
            "at": "2026-09-02T08:00:00Z"
          }
        ],
        "rules": {
          "rel": "sponsored",
          "max_per_week": 2
        },
        "locator": "https://indiestackweekly.example/archive"
      }
    ],
    "updated": "2026-10-21T12:10:00Z"
  }
}`},{name:`invalid/tracking-status-broken.json`,why:`A tracking link is live or plain; it never breaks.`,json:`{
  "orp": "0.3",
  "type": "deal",
  "id": "m_host_0004",
  "ts": "2026-10-21T10:02:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "re": "m_pub_0003",
  "body": {
    "state": "active",
    "offerer": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "acceptor": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "offer": "ofr_acme_30",
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true,
            "must_not_include": [
              "free forever"
            ]
          },
          "destination": "https://acmenotes.example/pricing",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 3000,
          "window": "P30D",
          "rail": "stripe",
          "proof": [
            "provider"
          ]
        }
      ]
    },
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
    "bound": [
      {
        "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T09:00:01Z"
      },
      {
        "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
        "authority": {
          "type": "mandate"
        },
        "at": "2026-10-21T10:02:01Z"
      }
    ],
    "tracking": [
      {
        "obligation": "place",
        "link": "https://hub.example/go/Zr8Kq2Lw",
        "destination": "https://acmenotes.example/pricing",
        "status": "broken"
      }
    ],
    "obligations": [
      {
        "id": "place",
        "state": "pending"
      },
      {
        "id": "commission",
        "state": "pending"
      }
    ],
    "updated": "2026-10-21T10:02:01Z"
  }
}`}]},{file:`0.3/messages/decline.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/decline.schema.json`,slug:`decline`,href:`/protocol/spec/0.3/decline/`,title:`DeclineMessage`,description:`Say no to an open deal before it is active. The reason is coarse on purpose: it must not reveal the decliner's budget or private rules.`,kind:`message`,type:`decline`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`DeclineMessage`,description:`Say no to an open deal before it is active. The reason is coarse on purpose: it must not reveal the decliner's budget or private rules.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "decline"`,description:``},{name:`body`,required:!0,type:`DeclineBody`,description:``}]},{name:`DeclineBody`,description:``,closed:!0,fields:[{name:`reason`,required:!0,type:`"not_relevant" | "price" | "timing" | "brand" | "capacity" | "policy" | "spam" | "other"`,description:``},{name:`note`,required:!1,type:`Text`,description:``}]}],valid:[{name:`messages/28-decline.json`,json:`{
  "orp": "0.3",
  "type": "decline",
  "id": "m_mkr_0008",
  "ts": "2026-10-23T08:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_spn_2Lx7",
  "body": {
    "reason": "timing",
    "note": "Booked through November."
  }
}`}],invalid:[]},{file:`0.3/messages/discover.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/discover.schema.json`,slug:`discover`,href:`/protocol/spec/0.3/discover/`,title:`DiscoverMessage`,description:`Search the host's index for offers (publisher side) or spaces (maker side). Read-only: it never commits anyone.`,kind:`message`,type:`discover`,required:[`type`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`DiscoverMessage`,description:`Search the host's index for offers (publisher side) or spaces (maker side). Read-only: it never commits anyone.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "discover"`,description:``},{name:`body`,required:!0,type:`DiscoverBody`,description:``}]},{name:`DiscoverBody`,description:``,closed:!0,fields:[{name:`want`,required:!0,type:`"offers" | "spaces"`,description:``},{name:`query`,required:!1,type:`string, min length 1, max length 500`,description:``},{name:`context`,required:!1,type:`object, closed`,description:`The caller's own public content to match against. Never someone else's data.`},{name:`filters`,required:!1,type:`object, closed`,description:``},{name:`limit`,required:!1,type:`integer, min 1, max 50`,description:``},{name:`cursor`,required:!1,type:`string, max length 200`,description:``}]}],valid:[{name:`messages/05-discover.json`,json:`{
  "orp": "0.3",
  "type": "discover",
  "id": "m_pub_0002",
  "ts": "2026-10-21T10:01:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "want": "offers",
    "context": {
      "text": "This week: five tools that keep a solo founder organised, from note-taking to invoicing.",
      "space": "sp_news_1"
    },
    "filters": {
      "kinds": [
        "affiliate",
        "swap"
      ],
      "categories": [
        "software.productivity"
      ],
      "min_reputation": 500
    },
    "limit": 5
  }
}`}],invalid:[{name:`invalid/discover-unknown-field.json`,why:`Core bodies are closed; extension data goes in ext.`,json:`{
  "orp": "0.3",
  "type": "discover",
  "id": "m_pub_0002",
  "ts": "2026-10-21T10:01:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "want": "offers",
    "context": {
      "text": "This week: five tools that keep a solo founder organised, from note-taking to invoicing.",
      "space": "sp_news_1"
    },
    "filters": {
      "kinds": [
        "affiliate",
        "swap"
      ],
      "categories": [
        "software.productivity"
      ],
      "min_reputation": 500
    },
    "limit": 5,
    "reader_emails": true
  }
}`},{name:`invalid/party-id-email.json`,why:`A party id is orp:<host>/<opaque>, never an email address.`,json:`{
  "orp": "0.3",
  "type": "discover",
  "id": "m_pub_0002",
  "ts": "2026-10-21T10:01:00Z",
  "from": "alice@example.com",
  "body": {
    "want": "offers",
    "context": {
      "text": "This week: five tools that keep a solo founder organised, from note-taking to invoicing.",
      "space": "sp_news_1"
    },
    "filters": {
      "kinds": [
        "affiliate",
        "swap"
      ],
      "categories": [
        "software.productivity"
      ],
      "min_reputation": 500
    },
    "limit": 5
  }
}`}]},{file:`0.3/messages/dispute.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/dispute.schema.json`,slug:`dispute`,href:`/protocol/spec/0.3/dispute/`,title:`DisputeMessage`,description:`Open, answer, resolve or withdraw a dispute about one obligation. The host (or a resolver it names) resolves. Evidence is public URLs and hashes, never personal data.`,kind:`message`,type:`dispute`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`DisputeMessage`,description:`Open, answer, resolve or withdraw a dispute about one obligation. The host (or a resolver it names) resolves. Evidence is public URLs and hashes, never personal data.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "dispute"`,description:``},{name:`body`,required:!0,type:`DisputeBody`,description:``}]},{name:`DisputeBody`,description:``,closed:!0,fields:[{name:`action`,required:!0,type:`"open" | "respond" | "resolve" | "withdraw"`,description:``},{name:`dispute`,required:!1,type:`LocalId`,description:``},{name:`obligation`,required:!1,type:`ObligationId`,description:``},{name:`claim`,required:!1,type:`"not_done" | "removed_early" | "disclosure_missing" | "content_mismatch" | "not_paid" | "invalid_traffic" | "bad_receipt" | "other"`,description:``},{name:`statement`,required:!1,type:`string, min length 1, max length 2000`,description:``},{name:`evidence`,required:!1,type:`array of object, closed`,description:``},{name:`remedy`,required:!1,type:`Remedy`,description:``},{name:`resolution`,required:!1,type:`object, closed`,description:``}]}],valid:[{name:`messages/24-dispute-open.json`,json:`{
  "orp": "0.3",
  "type": "dispute",
  "id": "m_pub_0006",
  "ts": "2026-10-28T10:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "action": "open",
    "obligation": "repost",
    "claim": "removed_early",
    "statement": "The repost was removed on day 5. The deal says 14 days.",
    "evidence": [
      {
        "url": "https://x.com/acmenotes/status/1848100000000000000",
        "note": "Returns not found since 27 October."
      }
    ],
    "remedy": "redo"
  }
}`},{name:`messages/25-dispute-resolve.json`,json:`{
  "orp": "0.3",
  "type": "dispute",
  "id": "m_host_0012",
  "ts": "2026-10-29T10:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "action": "resolve",
    "dispute": "dsp_5Mq1",
    "obligation": "repost",
    "resolution": {
      "outcome": "upheld",
      "remedy": "redo",
      "at_fault": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
      "note": "The platform check confirms removal on day 5. Repost and keep it up for the remaining 9 days."
    }
  }
}`}],invalid:[{name:`invalid/dispute-open-without-claim.json`,why:`Opening a dispute states the claim.`,json:`{
  "orp": "0.3",
  "type": "dispute",
  "id": "m_pub_0006",
  "ts": "2026-10-28T10:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "action": "open",
    "obligation": "repost",
    "statement": "The repost was removed on day 5. The deal says 14 days.",
    "evidence": [
      {
        "url": "https://x.com/acmenotes/status/1848100000000000000",
        "note": "Returns not found since 27 October."
      }
    ],
    "remedy": "redo"
  }
}`}]},{file:`0.3/messages/error.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/error.schema.json`,slug:`error`,href:`/protocol/spec/0.3/error/`,title:`ErrorMessage`,description:`A refusal. An error never changes state. Escalation is not an error: an ask returns a deal snapshot in state awaiting.`,kind:`message`,type:`error`,required:[`type`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`ErrorMessage`,description:`A refusal. An error never changes state. Escalation is not an error: an ask returns a deal snapshot in state awaiting.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "error"`,description:``},{name:`body`,required:!0,type:`ErrorBody`,description:``}]},{name:`ErrorBody`,description:``,closed:!0,fields:[{name:`code`,required:!0,type:`ErrorCode`,description:``},{name:`message`,required:!0,type:`string, min length 1, max length 500`,description:``},{name:`retryable`,required:!1,type:`boolean`,description:``},{name:`retry_after`,required:!1,type:`integer, min 0, max 86400`,description:`Seconds.`},{name:`supported`,required:!1,type:`array of Version`,description:``},{name:`step`,required:!1,type:`Step`,description:``},{name:`field`,required:!1,type:`string, max length 200`,description:`JSON Pointer to the offending field.`}]}],valid:[{name:`messages/29-error-step-off.json`,json:`{
  "orp": "0.3",
  "type": "error",
  "id": "m_host_0014",
  "ts": "2026-10-23T08:30:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "re": "m_mkr_0009",
  "body": {
    "code": "step_off",
    "message": "The owner has turned off the settle step for this agent.",
    "step": "settle",
    "retryable": false
  }
}`}],invalid:[]},{file:`0.3/messages/execute.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/execute.schema.json`,slug:`execute`,href:`/protocol/spec/0.3/execute/`,title:`ExecuteMessage`,description:`The obligor says an obligation is done. A claim is not proof: it moves the obligation to claimed, and only a receipt moves it to verified. For click-tracked placements the claim is optional, since human clicks on the tracking link are the proof.`,kind:`message`,type:`execute`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`ExecuteMessage`,description:`The obligor says an obligation is done. A claim is not proof: it moves the obligation to claimed, and only a receipt moves it to verified. For click-tracked placements the claim is optional, since human clicks on the tracking link are the proof.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "execute"`,description:``},{name:`body`,required:!0,type:`ExecuteBody`,description:``}]},{name:`ExecuteBody`,description:``,closed:!0,fields:[{name:`obligation`,required:!0,type:`ObligationId`,description:``},{name:`proof_url`,required:!1,type:`Url`,description:`A public URL where the result can be seen, such as the post, the repost or the comment.`},{name:`space`,required:!1,type:`LocalId`,description:``},{name:`at`,required:!1,type:`Timestamp`,description:``},{name:`note`,required:!1,type:`Text`,description:``}]}],valid:[{name:`messages/18-execute.json`,json:`{
  "orp": "0.3",
  "type": "execute",
  "id": "m_mkr_0004",
  "ts": "2026-10-22T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "body": {
    "obligation": "repost",
    "proof_url": "https://x.com/acmenotes/status/1848100000000000000",
    "at": "2026-10-22T08:58:00Z"
  }
}`}],invalid:[]},{file:`0.3/messages/hello.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/hello.schema.json`,slug:`hello`,href:`/protocol/spec/0.3/hello/`,title:`HelloMessage`,description:`Version and capability negotiation. Optional for agents, required for hosts to answer. The reply carries exactly one version: the one both sides will use.`,kind:`message`,type:`hello`,required:[`type`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`HelloMessage`,description:`Version and capability negotiation. Optional for agents, required for hosts to answer. The reply carries exactly one version: the one both sides will use.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "hello"`,description:``},{name:`body`,required:!0,type:`HelloBody`,description:``}]},{name:`HelloBody`,description:``,closed:!0,fields:[{name:`versions`,required:!0,type:`array of Version`,description:``},{name:`capabilities`,required:!1,type:`Capabilities`,description:``},{name:`party`,required:!1,type:`PartyId`,description:``},{name:`limits`,required:!1,type:`object, closed`,description:``}]}],valid:[{name:`messages/01-hello.json`,json:`{
  "orp": "0.3",
  "type": "hello",
  "id": "m_pub_0001",
  "ts": "2026-10-21T10:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "agent": {
    "name": "indie-stack-agent",
    "version": "1.4.0"
  },
  "body": {
    "versions": [
      "0.3"
    ],
    "capabilities": {
      "messages": [
        "discover",
        "request",
        "offer",
        "counter",
        "accept",
        "decline",
        "execute",
        "receipt",
        "revoke",
        "dispute"
      ],
      "proofs": [
        "link",
        "proof_url",
        "attestation"
      ],
      "tracking": {
        "links": true,
        "codes": true
      }
    }
  }
}`},{name:`messages/02-hello-reply.json`,json:`{
  "orp": "0.3",
  "type": "hello",
  "id": "m_host_0001",
  "ts": "2026-10-21T10:00:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "re": "m_pub_0001",
  "body": {
    "versions": [
      "0.3"
    ],
    "party": "orp:hub.example/p_Vb3nR8sYc1Qe6",
    "capabilities": {
      "messages": [
        "hello",
        "discover",
        "results",
        "space",
        "request",
        "offer",
        "counter",
        "accept",
        "decline",
        "deal",
        "execute",
        "receipt",
        "outcome",
        "payout",
        "revoke",
        "consent",
        "dispute",
        "reputation",
        "error"
      ],
      "proofs": [
        "link",
        "proof_url",
        "platform_check",
        "attestation",
        "witness",
        "provider"
      ],
      "tracking": {
        "links": true,
        "codes": true
      },
      "signatures": {
        "algs": [
          "EdDSA"
        ],
        "required": false
      },
      "inbox": {
        "poll": true
      },
      "checks": {
        "claims": "real"
      }
    },
    "limits": {
      "messages_per_minute": 60,
      "max_body_bytes": 65536
    }
  }
}`}],invalid:[{name:`invalid/envelope-missing-ts.json`,why:`ts is one of the six required envelope fields.`,json:`{
  "orp": "0.3",
  "type": "hello",
  "id": "m_pub_0001",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "agent": {
    "name": "indie-stack-agent",
    "version": "1.4.0"
  },
  "body": {
    "versions": [
      "0.3"
    ],
    "capabilities": {
      "messages": [
        "discover",
        "request",
        "offer",
        "counter",
        "accept",
        "decline",
        "execute",
        "receipt",
        "revoke",
        "dispute"
      ],
      "proofs": [
        "link",
        "proof_url",
        "attestation"
      ],
      "tracking": {
        "links": true,
        "codes": true
      }
    }
  }
}`},{name:`invalid/party-id-role-prefix-68.json`,why:`#68 party ids (orp:party:...) are not host-scoped and fail the 0.3 PartyId pattern.`,json:`{
  "orp": "0.3",
  "type": "hello",
  "id": "m_pub_0001",
  "ts": "2026-10-21T10:00:00Z",
  "from": "orp:party:maker_cloudbranch",
  "body": {
    "versions": [
      "0.3"
    ]
  }
}`},{name:`invalid/version-semver.json`,why:`The version is MAJOR.MINOR (0.3), not the 0.3.0 of the #68 draft.`,json:`{
  "orp": "0.3.0",
  "type": "hello",
  "id": "m_pub_0001",
  "ts": "2026-10-21T10:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "versions": [
      "0.3"
    ]
  }
}`}]},{file:`0.3/messages/offer.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/offer.schema.json`,slug:`offer`,href:`/protocol/spec/0.3/offer/`,title:`OfferMessage`,description:`Proposed terms. With to, it is directed: the host opens a deal and answers with a deal snapshot. Without to, it is a standing offer: the host lists it in discovery and answers with the offer as listed, id and terms_hash filled in. Anyone may accept a standing offer without negotiating.`,kind:`message`,type:`offer`,required:[`type`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`OfferMessage`,description:`Proposed terms. With to, it is directed: the host opens a deal and answers with a deal snapshot. Without to, it is a standing offer: the host lists it in discovery and answers with the offer as listed, id and terms_hash filled in. Anyone may accept a standing offer without negotiating.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "offer"`,description:``},{name:`body`,required:!0,type:`OfferBody`,description:``}]},{name:`OfferBody`,description:``,closed:!0,fields:[{name:`id`,required:!1,type:`LocalId`,description:`Set by the host on a listed standing offer.`},{name:`terms`,required:!0,type:`Terms`,description:``},{name:`terms_hash`,required:!1,type:`Hash`,description:`Optional from agents. When present the host checks it and refuses a mismatch with terms_mismatch.`},{name:`limit`,required:!1,type:`integer, min 1, max 100000`,description:`Standing offers only: the most deals it may open.`},{name:`waiting`,required:!1,type:`Waiting`,description:`Set by the host when listing waits for the waiting party's agent. The host records approved_by agent. That is not a person's approval. The signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing. The offer is listed once approved.`},{name:`note`,required:!1,type:`Text`,description:``},{name:`expires`,required:!1,type:`Timestamp`,description:``}]}],valid:[{name:`messages/03-offer-standing.json`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_mkr_0001",
  "ts": "2026-10-21T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "body": {
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true,
            "must_not_include": [
              "free forever"
            ]
          },
          "destination": "https://acmenotes.example/pricing",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 3000,
          "window": "P30D",
          "rail": "stripe",
          "proof": [
            "provider"
          ]
        }
      ]
    },
    "limit": 500,
    "expires": "2027-01-31T00:00:00Z"
  }
}`},{name:`messages/04-offer-listed.json`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_host_0002",
  "ts": "2026-10-21T09:00:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "re": "m_mkr_0001",
  "body": {
    "id": "ofr_acme_30",
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true,
            "must_not_include": [
              "free forever"
            ]
          },
          "destination": "https://acmenotes.example/pricing",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 3000,
          "window": "P30D",
          "rail": "stripe",
          "proof": [
            "provider"
          ]
        }
      ]
    },
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
    "limit": 500,
    "expires": "2027-01-31T00:00:00Z"
  }
}`},{name:`messages/11-offer-directed.json`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_pub_0004",
  "ts": "2026-10-21T11:05:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "re": "m_mkr_0002",
  "body": {
    "terms": {
      "title": "Newsletter mention for a launch repost",
      "kind": "swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "mention",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "newsletter.issue"
            ]
          },
          "content": {
            "link": true,
            "brief": "One paragraph on why Acme Notes suits solo founders.",
            "max_chars": 600
          },
          "destination": "https://acmenotes.example/",
          "due": "P14D",
          "proof": [
            "link",
            "witness",
            "attestation"
          ]
        },
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/indiestackweekly/status/1848000000000000000",
          "space": {
            "kinds": [
              "social.repost"
            ]
          },
          "due": "P7D",
          "keep": "P30D",
          "proof": [
            "platform_check",
            "proof_url",
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-6zjTp5mN8-GueFQpGvpswiLBxnrTUE3v927FEe-qX4k",
    "note": "A swap: a mention in the 4 November issue for a repost of our launch thread.",
    "expires": "2026-10-24T00:00:00Z"
  }
}`},{name:`messages/31-offer-traffic-swap.json`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_mkr_0011",
  "ts": "2026-10-24T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "terms": {
      "title": "Traffic swap: 500 readers each way",
      "kind": "traffic_swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "send_readers",
          "by": "offerer",
          "action": "place",
          "content": {
            "link": true
          },
          "destination": "https://indiestackweekly.example/subscribe",
          "clicks": 500,
          "due": "P30D",
          "proof": [
            "link"
          ]
        },
        {
          "id": "send_back",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true
          },
          "destination": "https://acmenotes.example/blog/launch",
          "clicks": 500,
          "due": "P30D",
          "proof": [
            "link"
          ]
        }
      ]
    },
    "terms_hash": "sha256-j2f8SES1MpLaz_7cQKk3GfIystwFTjGh0Iri5QLp38U",
    "note": "Our launch post is doing well. 500 readers each way this month?",
    "expires": "2026-10-31T00:00:00Z"
  }
}`},{name:`messages/32-offer-link-exchange.json`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_dev_0001",
  "ts": "2026-10-24T10:00:00Z",
  "from": "orp:hub.example/p_Dv4nT8sLq2Wm7",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "terms": {
      "title": "Link exchange: resource page links for 90 days",
      "kind": "link_exchange",
      "categories": [
        "software.devtools"
      ],
      "disclosure": {
        "label": "Partner link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "link_out",
          "by": "offerer",
          "action": "place",
          "space": {
            "kinds": [
              "web.page"
            ]
          },
          "content": {
            "direct": true
          },
          "destination": "https://indiestackweekly.example/tools",
          "keep": "P90D",
          "proof": [
            "proof_url"
          ]
        },
        {
          "id": "link_back",
          "by": "acceptor",
          "action": "place",
          "space": {
            "kinds": [
              "web.page"
            ]
          },
          "content": {
            "direct": true
          },
          "destination": "https://devnotes.example/",
          "keep": "P90D",
          "proof": [
            "proof_url"
          ]
        }
      ]
    },
    "terms_hash": "sha256-W3ltrm-gBbU5vnf7ZbpTKrrRTT1Pe0gsD047utSznDk",
    "note": "Your tools page and our resources page cover the same readers."
  }
}`},{name:`messages/33-offer-cross-sell.json`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_mkr_0012",
  "ts": "2026-10-24T11:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "body": {
    "terms": {
      "title": "Offer Acme Notes on your checkout thank-you page: 20% of the first payment",
      "kind": "cross_sell",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner offer",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "slot",
          "by": "acceptor",
          "action": "place",
          "space": {
            "kinds": [
              "app.slot",
              "newsletter.dedicated"
            ]
          },
          "content": {
            "link": true,
            "brief": "One line under the receipt: notes that sync with your invoices."
          },
          "destination": "https://acmenotes.example/partners",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 2000,
          "window": "P14D",
          "rail": "stripe",
          "proof": [
            "provider"
          ]
        }
      ]
    },
    "limit": 50,
    "expires": "2027-03-31T00:00:00Z"
  }
}`},{name:`messages/34-offer-paid-repost.json`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_mkr_0013",
  "ts": "2026-10-24T12:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "body": {
    "terms": {
      "title": "Quote our launch post with your take: EUR 15 per verified repost",
      "kind": "repost",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Paid partnership",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "repost",
          "by": "acceptor",
          "action": "repost",
          "target": "https://x.com/acmenotes/status/1848200000000000000",
          "quote": true,
          "due": "P3D",
          "keep": "P7D",
          "proof": [
            "platform_check",
            "proof_url"
          ]
        },
        {
          "id": "fee",
          "by": "offerer",
          "action": "pay",
          "per": "deal",
          "amount": {
            "amount_minor": 1500,
            "currency": "EUR"
          },
          "after": "repost",
          "rail": "stripe",
          "proof": [
            "provider"
          ]
        }
      ]
    },
    "limit": 20,
    "expires": "2026-11-01T00:00:00Z"
  }
}`}],invalid:[{name:`invalid/money-negative.json`,why:`Money is a non-negative integer in minor units.`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_host_0002",
  "ts": "2026-10-21T09:00:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "re": "m_mkr_0001",
  "body": {
    "id": "ofr_acme_30",
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true,
            "must_not_include": [
              "free forever"
            ]
          },
          "destination": "https://acmenotes.example/pricing",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "window": "P30D",
          "rail": "stripe",
          "proof": [
            "provider"
          ],
          "amount": {
            "amount_minor": -500,
            "currency": "EUR"
          }
        }
      ]
    },
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
    "limit": 500,
    "expires": "2027-01-31T00:00:00Z"
  }
}`},{name:`invalid/share-personal-data.json`,why:`Share obligations carry aggregate data only.`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_host_0002",
  "ts": "2026-10-21T09:00:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "re": "m_mkr_0001",
  "body": {
    "id": "ofr_acme_30",
    "terms": {
      "title": "Acme Notes: 30% of first-year revenue",
      "kind": "affiliate",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true,
            "must_not_include": [
              "free forever"
            ]
          },
          "destination": "https://acmenotes.example/pricing",
          "proof": [
            "link"
          ]
        },
        {
          "id": "commission",
          "by": "offerer",
          "action": "pay",
          "per": "conversion",
          "rate_bps": 3000,
          "window": "P30D",
          "rail": "stripe",
          "proof": [
            "provider"
          ]
        },
        {
          "id": "emails",
          "by": "acceptor",
          "action": "share",
          "data": {
            "class": "personal",
            "fields": [
              "email"
            ],
            "k_min": 50,
            "purpose": "retargeting",
            "retention": "P30D"
          },
          "proof": [
            "attestation"
          ]
        }
      ]
    },
    "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
    "limit": 500,
    "expires": "2027-01-31T00:00:00Z"
  }
}`},{name:`invalid/traffic-swap-zero-clicks.json`,why:`A clicks target is at least one human click.`,json:`{
  "orp": "0.3",
  "type": "offer",
  "id": "m_mkr_0011",
  "ts": "2026-10-24T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "terms": {
      "title": "Traffic swap: 500 readers each way",
      "kind": "traffic_swap",
      "categories": [
        "software.productivity"
      ],
      "disclosure": {
        "label": "Partner link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "send_readers",
          "by": "offerer",
          "action": "place",
          "content": {
            "link": true
          },
          "destination": "https://indiestackweekly.example/subscribe",
          "clicks": 0,
          "due": "P30D",
          "proof": [
            "link"
          ]
        },
        {
          "id": "send_back",
          "by": "acceptor",
          "action": "place",
          "content": {
            "link": true
          },
          "destination": "https://acmenotes.example/blog/launch",
          "clicks": 500,
          "due": "P30D",
          "proof": [
            "link"
          ]
        }
      ]
    },
    "terms_hash": "sha256-j2f8SES1MpLaz_7cQKk3GfIystwFTjGh0Iri5QLp38U",
    "note": "Our launch post is doing well. 500 readers each way this month?",
    "expires": "2026-10-31T00:00:00Z"
  }
}`}]},{file:`0.3/messages/outcome.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/outcome.schema.json`,slug:`outcome`,href:`/protocol/spec/0.3/outcome/`,title:`OutcomeMessage`,description:`Aggregate results for a period, sent by the host to both parties. Counts and money only. Rows are closed objects: there is no field for an address, user agent, referrer, email or order id.`,kind:`message`,type:`outcome`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`OutcomeMessage`,description:`Aggregate results for a period, sent by the host to both parties. Counts and money only. Rows are closed objects: there is no field for an address, user agent, referrer, email or order id.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "outcome"`,description:``},{name:`body`,required:!0,type:`OutcomeBody`,description:``}]},{name:`OutcomeBody`,description:``,closed:!0,fields:[{name:`period`,required:!0,type:`object, closed`,description:``},{name:`rows`,required:!0,type:`array of OutcomeRow`,description:``},{name:`final`,required:!1,type:`boolean`,description:`No later outcome will change these numbers.`}]}],valid:[{name:`messages/09-outcome.json`,json:`{
  "orp": "0.3",
  "type": "outcome",
  "id": "m_host_0005",
  "ts": "2026-10-28T00:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "period": {
      "from": "2026-10-21T00:00:00Z",
      "to": "2026-10-28T00:00:00Z"
    },
    "rows": [
      {
        "obligation": "place",
        "clicks": {
          "human": 41,
          "bot": 3,
          "preview": 12,
          "test": 1
        },
        "conversions": 2,
        "refunds": 0,
        "revenue": {
          "amount_minor": 19200,
          "currency": "EUR"
        },
        "earned": {
          "amount_minor": 5760,
          "currency": "EUR"
        },
        "attested_conversions": 0,
        "sources": [
          {
            "source": "x.com",
            "human": 27
          },
          {
            "source": "direct",
            "human": 9
          },
          {
            "source": "other",
            "human": 5
          }
        ]
      }
    ],
    "final": false
  }
}`}],invalid:[{name:`invalid/outcome-source-full-url.json`,why:`A click source is a registrable domain, never a page URL that could name a reader's page.`,json:`{
  "orp": "0.3",
  "type": "outcome",
  "id": "m_host_0006",
  "ts": "2026-10-28T00:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "period": {
      "from": "2026-10-21T00:00:00Z",
      "to": "2026-10-28T00:00:00Z"
    },
    "rows": [
      {
        "obligation": "place",
        "clicks": {
          "human": 3
        },
        "sources": [
          {
            "source": "https://someone.example/private/page?u=42",
            "human": 3
          }
        ]
      }
    ]
  }
}`},{name:`invalid/outcome-with-emails.json`,why:`Click counts are integers by class, never people.`,json:`{
  "orp": "0.3",
  "type": "outcome",
  "id": "m_host_0005",
  "ts": "2026-10-28T00:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "period": {
      "from": "2026-10-21T00:00:00Z",
      "to": "2026-10-28T00:00:00Z"
    },
    "rows": [
      {
        "obligation": "place",
        "clicks": {
          "human": 41,
          "bot": 3,
          "preview": 12,
          "test": 1,
          "emails": [
            "reader@example.com"
          ]
        },
        "conversions": 2,
        "refunds": 0,
        "revenue": {
          "amount_minor": 19200,
          "currency": "EUR"
        },
        "earned": {
          "amount_minor": 5760,
          "currency": "EUR"
        }
      }
    ],
    "final": false
  }
}`},{name:`invalid/outcome-with-ip.json`,why:`Outcome rows are closed: no field can carry a click address.`,json:`{
  "orp": "0.3",
  "type": "outcome",
  "id": "m_host_0005",
  "ts": "2026-10-28T00:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "period": {
      "from": "2026-10-21T00:00:00Z",
      "to": "2026-10-28T00:00:00Z"
    },
    "rows": [
      {
        "obligation": "place",
        "clicks": {
          "human": 41,
          "bot": 3,
          "preview": 12,
          "test": 1
        },
        "conversions": 2,
        "refunds": 0,
        "revenue": {
          "amount_minor": 19200,
          "currency": "EUR"
        },
        "earned": {
          "amount_minor": 5760,
          "currency": "EUR"
        },
        "ip": "203.0.113.7"
      }
    ],
    "final": false
  }
}`}]},{file:`0.3/messages/payout.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/payout.schema.json`,slug:`payout`,href:`/protocol/spec/0.3/payout/`,title:`PayoutMessage`,description:`The life of one payment for one pay obligation. ORP never holds or moves money: the money moves on a rail (stripe, lemonsqueezy, polar, manual, or an extension), and this message records where it is. The host sends a record to both parties at each state change. The payee answers a sent payout with confirm or not_received. A payout is paid only once the payee confirms or the rail's provider event says so.`,kind:`message`,type:`payout`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`PayoutMessage`,description:`The life of one payment for one pay obligation. ORP never holds or moves money: the money moves on a rail (stripe, lemonsqueezy, polar, manual, or an extension), and this message records where it is. The host sends a record to both parties at each state change. The payee answers a sent payout with confirm or not_received. A payout is paid only once the payee confirms or the rail's provider event says so.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "payout"`,description:``},{name:`body`,required:!0,type:`PayoutBody`,description:``}]},{name:`PayoutBody`,description:``,closed:!1,fields:[]}],valid:[{name:`messages/39-payout-held.json`,json:`{
  "orp": "0.3",
  "type": "payout",
  "id": "m_host_0201",
  "ts": "2026-10-28T00:00:05Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "payout": "po_aff_9wQz_1",
    "obligation": "commission",
    "state": "held",
    "amount": {
      "amount_minor": 5760,
      "currency": "EUR"
    },
    "rail": "stripe",
    "basis": "provider",
    "period": {
      "from": "2026-10-21T00:00:00Z",
      "to": "2026-10-28T00:00:00Z"
    },
    "hold_until": "2026-11-27T00:00:00Z",
    "at": "2026-10-28T00:00:05Z"
  }
}`},{name:`messages/40-payout-confirm.json`,json:`{
  "orp": "0.3",
  "type": "payout",
  "id": "m_pub_0201",
  "ts": "2026-12-02T09:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "to": "orp:hub.example/host",
  "deal": "deal_aff_9wQz",
  "body": {
    "payout": "po_aff_9wQz_1",
    "answer": "confirm"
  }
}`},{name:`messages/41-payout-paid-chain.json`,json:`{
  "orp": "0.3",
  "type": "payout",
  "id": "m_host_0301",
  "ts": "2026-11-16T10:00:12Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_spn_4kTq",
  "body": {
    "payout": "po_spn_4kTq_1",
    "obligation": "fee",
    "state": "paid",
    "amount": {
      "amount_minor": 1500,
      "currency": "USD"
    },
    "rail": "usdc.base",
    "ref": "0x9f2c1e5b7a3d4c6e8f0a1b2c3d4e5f60718293a4b5c6d7e8f9a0b1c2d3e4f5a6",
    "basis": "receipt",
    "at": "2026-11-16T10:00:12Z"
  }
}`}],invalid:[{name:`invalid/payout-answer-with-amount.json`,why:`A payee answers yes or no; it cannot restate the amount.`,json:`{
  "orp": "0.3",
  "type": "payout",
  "id": "m_pub_0202",
  "ts": "2026-12-02T09:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_aff_9wQz",
  "body": {
    "payout": "po_aff_9wQz_1",
    "answer": "confirm",
    "amount": {
      "amount_minor": 9999,
      "currency": "EUR"
    }
  }
}`}]},{file:`0.3/messages/receipt.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/receipt.schema.json`,slug:`receipt`,href:`/protocol/spec/0.3/receipt/`,title:`ReceiptMessage`,description:`A verifier's finding about one obligation. The host sends receipts for link, proof_url, platform_check, witness and provider checks. The counterparty sends a receipt with method attestation to confirm (or reject) work it saw itself. Only receipts count toward reputation; execute claims never do.`,kind:`message`,type:`receipt`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`ReceiptMessage`,description:`A verifier's finding about one obligation. The host sends receipts for link, proof_url, platform_check, witness and provider checks. The counterparty sends a receipt with method attestation to confirm (or reject) work it saw itself. Only receipts count toward reputation; execute claims never do.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "receipt"`,description:``},{name:`body`,required:!0,type:`ReceiptBody`,description:``}]},{name:`ReceiptBody`,description:``,closed:!0,fields:[{name:`obligation`,required:!0,type:`ObligationId`,description:``},{name:`result`,required:!0,type:`"verified" | "failed" | "partial"`,description:``},{name:`method`,required:!0,type:`ProofMethod`,description:``},{name:`at`,required:!0,type:`Timestamp`,description:``},{name:`url`,required:!1,type:`Url`,description:`What was checked.`},{name:`found`,required:!1,type:`object, closed`,description:`What the check saw at url.`},{name:`reason`,required:!1,type:`"not_found" | "link_missing" | "rel_missing" | "disclosure_missing" | "content_mismatch" | "removed" | "unreachable" | "too_early" | "too_late" | "not_public" | "other"`,description:``},{name:`settlement`,required:!1,type:`object, closed`,description:`For pay obligations: where the money moved. ORP never holds funds.`},{name:`next_check`,required:!1,type:`Timestamp`,description:`For keep periods: when the verifier looks again.`}]}],valid:[{name:`messages/19-receipt-platform-check.json`,json:`{
  "orp": "0.3",
  "type": "receipt",
  "id": "m_host_0010",
  "ts": "2026-10-22T09:00:05Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "re": "m_mkr_0004",
  "body": {
    "obligation": "repost",
    "result": "verified",
    "method": "platform_check",
    "at": "2026-10-22T09:00:04Z",
    "url": "https://x.com/acmenotes/status/1848100000000000000",
    "found": {
      "target": true
    },
    "next_check": "2026-10-29T09:00:00Z"
  }
}`},{name:`messages/20-receipt-attestation.json`,json:`{
  "orp": "0.3",
  "type": "receipt",
  "id": "m_mkr_0005",
  "ts": "2026-11-04T15:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "body": {
    "obligation": "mention",
    "result": "verified",
    "method": "attestation",
    "at": "2026-11-04T14:55:00Z",
    "url": "https://indiestackweekly.example/archive/2026-11-04",
    "found": {
      "link": true,
      "code": true,
      "disclosure": true
    }
  }
}`},{name:`messages/21-receipt-link.json`,json:`{
  "orp": "0.3",
  "type": "receipt",
  "id": "m_host_0011",
  "ts": "2026-11-04T16:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_swp_4KfP",
  "body": {
    "obligation": "mention",
    "result": "verified",
    "method": "link",
    "at": "2026-11-04T15:59:00Z",
    "found": {
      "link": true
    }
  }
}`},{name:`messages/35-receipt-link-exchange.json`,json:`{
  "orp": "0.3",
  "type": "receipt",
  "id": "m_host_0015",
  "ts": "2026-10-25T09:00:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "deal": "deal_lnk_6Tq3",
  "body": {
    "obligation": "link_out",
    "result": "verified",
    "method": "proof_url",
    "at": "2026-10-25T08:59:58Z",
    "url": "https://devnotes.example/resources",
    "found": {
      "destination": true,
      "rel": true,
      "disclosure": true
    },
    "next_check": "2026-11-24T09:00:00Z"
  }
}`},{name:`messages/36-receipt-traffic-swap.json`,json:`{
  "orp": "0.3",
  "type": "receipt",
  "id": "m_host_0016",
  "ts": "2026-11-12T17:40:00Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_trf_3Hn8",
  "body": {
    "obligation": "send_back",
    "result": "verified",
    "method": "link",
    "at": "2026-11-12T17:39:51Z",
    "found": {
      "link": true,
      "clicks": 500
    }
  }
}`}],invalid:[{name:`invalid/receipt-bad-result.json`,why:`A receipt result is verified, failed or partial.`,json:`{
  "orp": "0.3",
  "type": "receipt",
  "id": "m_host_0010",
  "ts": "2026-10-22T09:00:05Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_swp_4KfP",
  "re": "m_mkr_0004",
  "body": {
    "obligation": "repost",
    "result": "probably",
    "method": "platform_check",
    "at": "2026-10-22T09:00:04Z",
    "url": "https://x.com/acmenotes/status/1848100000000000000",
    "found": {
      "target": true
    },
    "next_check": "2026-10-29T09:00:00Z"
  }
}`}]},{file:`0.3/messages/reputation.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/reputation.schema.json`,slug:`reputation`,href:`/protocol/spec/0.3/reputation/`,title:`ReputationMessage`,description:`Ask the host for a party's reputation (body has only subject), or the host's signed answer (an attestation). Scores come only from verified receipts and resolved disputes. Components are counts; they never list counterparties.`,kind:`message`,type:`reputation`,required:[`type`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`ReputationMessage`,description:`Ask the host for a party's reputation (body has only subject), or the host's signed answer (an attestation). Scores come only from verified receipts and resolved disputes. Components are counts; they never list counterparties.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "reputation"`,description:``},{name:`body`,required:!0,type:`ReputationBody`,description:``}]},{name:`ReputationBody`,description:``,closed:!1,fields:[]}],valid:[{name:`messages/26-reputation-query.json`,json:`{
  "orp": "0.3",
  "type": "reputation",
  "id": "m_pub_0007",
  "ts": "2026-10-21T11:06:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "subject": "orp:hub.example/p_7Hq2kLm9Zt4Xw"
  }
}`},{name:`messages/27-reputation-attestation.json`,json:`{
  "orp": "0.3",
  "type": "reputation",
  "id": "m_host_0013",
  "ts": "2026-10-21T11:06:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "re": "m_pub_0007",
  "body": {
    "subject": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
    "algorithm": "orp-rep-1",
    "score": 858,
    "components": {
      "verified": 46.3,
      "failed": 2.1,
      "disputes_lost": 1,
      "counterparties": 31,
      "evidence_weight": 48.4
    },
    "computed_at": "2026-10-21T11:06:01Z",
    "valid_until": "2026-10-22T11:06:01Z"
  }
}`}],invalid:[]},{file:`0.3/messages/request.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/request.schema.json`,slug:`request`,href:`/protocol/spec/0.3/request/`,title:`RequestMessage`,description:`A directed request for terms, like a request for quote: 'do you have a slot?' or 'would you do this offer at a different rate?'. Named ask in the #68 and #71 drafts; renamed so it never reads like the ask step mode. It commits nobody. The answer is an offer (with re set to this message), a decline, or nothing.`,kind:`message`,type:`request`,required:[`type`,`body`,`to`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`RequestMessage`,description:`A directed request for terms, like a request for quote: 'do you have a slot?' or 'would you do this offer at a different rate?'. Named ask in the #68 and #71 drafts; renamed so it never reads like the ask step mode. It commits nobody. The answer is an offer (with re set to this message), a decline, or nothing.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "request"`,description:``},{name:`body`,required:!0,type:`RequestBody`,description:``}]},{name:`RequestBody`,description:``,closed:!0,fields:[{name:`offer`,required:!1,type:`LocalId`,description:`A standing offer the asker wants different terms for.`},{name:`space`,required:!1,type:`LocalId`,description:`A space from results the asker wants to use.`},{name:`kinds`,required:!1,type:`array of DealKind`,description:``},{name:`actions`,required:!1,type:`array of "place" | "repost" | "comment" | "pay" | "credit" | "share"`,description:`Obligation actions the asker wants from the other side.`},{name:`give`,required:!1,type:`array of "place" | "repost" | "comment" | "pay" | "credit" | "share"`,description:`Obligation actions the asker is willing to do in return.`},{name:`max_price`,required:!1,type:`Money`,description:`Optional. Disclosing a price is the asker's choice; constraints stay private.`},{name:`window`,required:!1,type:`object, closed`,description:``},{name:`note`,required:!1,type:`Text`,description:``},{name:`expires`,required:!1,type:`Timestamp`,description:``}]}],valid:[{name:`messages/10-request.json`,json:`{
  "orp": "0.3",
  "type": "request",
  "id": "m_mkr_0002",
  "ts": "2026-10-21T11:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "space": "sp_news_1",
    "kinds": [
      "swap",
      "sponsored"
    ],
    "actions": [
      "place"
    ],
    "give": [
      "repost",
      "pay"
    ],
    "window": {
      "from": "2026-11-02T00:00:00Z",
      "to": "2026-11-09T00:00:00Z"
    },
    "note": "Acme Notes 2.0 launches on 3 November. Is there room in an issue that week?",
    "expires": "2026-10-25T00:00:00Z"
  }
}`}],invalid:[]},{file:`0.3/messages/results.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/results.schema.json`,slug:`results`,href:`/protocol/spec/0.3/results/`,title:`ResultsMessage`,description:`The host's answer to discover. Every item says why it fits, with evidence links, and shows aggregate trust.`,kind:`message`,type:`results`,required:[`type`,`body`,`re`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`ResultsMessage`,description:`The host's answer to discover. Every item says why it fits, with evidence links, and shows aggregate trust.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "results"`,description:``},{name:`body`,required:!0,type:`ResultsBody`,description:``}]},{name:`ResultsBody`,description:``,closed:!0,fields:[{name:`items`,required:!0,type:`array of Result`,description:``},{name:`cursor`,required:!1,type:`string, max length 200`,description:``},{name:`empty_reason`,required:!1,type:`"no_match" | "below_threshold" | "filtered"`,description:``}]}],valid:[{name:`messages/06-results.json`,json:`{
  "orp": "0.3",
  "type": "results",
  "id": "m_host_0003",
  "ts": "2026-10-21T10:01:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "re": "m_pub_0002",
  "body": {
    "items": [
      {
        "offer": {
          "id": "ofr_acme_30",
          "terms": {
            "title": "Acme Notes: 30% of first-year revenue",
            "kind": "affiliate",
            "categories": [
              "software.productivity"
            ],
            "disclosure": {
              "label": "Affiliate link",
              "lang": "en"
            },
            "obligations": [
              {
                "id": "place",
                "by": "acceptor",
                "action": "place",
                "content": {
                  "link": true,
                  "must_not_include": [
                    "free forever"
                  ]
                },
                "destination": "https://acmenotes.example/pricing",
                "proof": [
                  "link"
                ]
              },
              {
                "id": "commission",
                "by": "offerer",
                "action": "pay",
                "per": "conversion",
                "rate_bps": 3000,
                "window": "P30D",
                "rail": "stripe",
                "proof": [
                  "provider"
                ]
              }
            ]
          },
          "terms_hash": "sha256-Bti4pak22vzeA0HAuX0eLXhUQy1pnbf9YGiGKYgJpNo",
          "expires": "2027-01-31T00:00:00Z"
        },
        "party": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
        "fit": {
          "score": 82,
          "reasons": [
            "Note-taking for solo founders matches the issue topic.",
            "Pays 30% for 30 days after a click."
          ],
          "evidence": [
            "https://acmenotes.example/pricing"
          ]
        },
        "trust": {
          "reputation": 858,
          "completed_deals": 48,
          "verified": [
            "person",
            "domain"
          ]
        },
        "pool": "public",
        "pitch": "Acme Notes pays 30% of first-year revenue for readers who subscribe through your link.",
        "checks": {
          "claims": "not_checked"
        }
      }
    ]
  }
}`}],invalid:[]},{file:`0.3/messages/revoke.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/revoke.schema.json`,slug:`revoke`,href:`/protocol/spec/0.3/revoke/`,title:`RevokeMessage`,description:`Stop a deal, one obligation, or a standing offer. Future obligations stop. Tracking links never break: they turn plain, still redirecting to the destination but no longer counting. Work already verified stays verified.`,kind:`message`,type:`revoke`,required:[`type`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`RevokeMessage`,description:`Stop a deal, one obligation, or a standing offer. Future obligations stop. Tracking links never break: they turn plain, still redirecting to the destination but no longer counting. Work already verified stays verified.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "revoke"`,description:``},{name:`body`,required:!0,type:`RevokeBody`,description:``}]},{name:`RevokeBody`,description:``,closed:!0,fields:[{name:`scope`,required:!0,type:`"deal" | "obligation" | "offer"`,description:``},{name:`obligation`,required:!1,type:`ObligationId`,description:``},{name:`offer`,required:!1,type:`LocalId`,description:``},{name:`reason`,required:!0,type:`"owner_request" | "breach" | "brand_safety" | "expired" | "other"`,description:``},{name:`note`,required:!1,type:`Text`,description:``},{name:`effective`,required:!1,type:`Timestamp`,description:``}]}],valid:[{name:`messages/22-revoke.json`,json:`{
  "orp": "0.3",
  "type": "revoke",
  "id": "m_mkr_0006",
  "ts": "2026-12-01T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_aff_9wQz",
  "body": {
    "scope": "deal",
    "reason": "owner_request",
    "note": "Spring pricing replaces this offer.",
    "effective": "2026-12-08T00:00:00Z"
  }
}`},{name:`messages/23-revoke-offer.json`,json:`{
  "orp": "0.3",
  "type": "revoke",
  "id": "m_mkr_0007",
  "ts": "2026-12-01T09:01:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "body": {
    "scope": "offer",
    "offer": "ofr_acme_30",
    "reason": "expired"
  }
}`}],invalid:[{name:`invalid/revoke-obligation-without-id.json`,why:`Revoking one obligation names it.`,json:`{
  "orp": "0.3",
  "type": "revoke",
  "id": "m_mkr_0006",
  "ts": "2026-12-01T09:00:00Z",
  "from": "orp:hub.example/p_7Hq2kLm9Zt4Xw",
  "deal": "deal_aff_9wQz",
  "body": {
    "scope": "obligation",
    "reason": "owner_request",
    "note": "Spring pricing replaces this offer.",
    "effective": "2026-12-08T00:00:00Z"
  }
}`}]},{file:`0.3/messages/space.schema.json`,version:`0.3`,id:`https://offer-rails.onrender.com/orp/schemas/messages/space.schema.json`,slug:`space`,href:`/protocol/spec/0.3/space/`,title:`SpaceMessage`,description:`Publish, update or withdraw a Space: a place the sender controls where content can appear (a blog, a newsletter, an X account, a video channel, an app slot). A party sends put or withdraw under the list step. The host answers with a space message from the host: the space as listed, or waiting for the waiting party's agent. The host records approved_by agent. That is not a person's approval. The signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing. Space ids are unique per party. The locator stays hidden from counterparties until a deal that names the space is active.`,kind:`message`,type:`space`,required:[`type`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.`,closed:!0,fields:[{name:`orp`,required:!0,type:`Version`,description:``},{name:`type`,required:!0,type:`MessageType`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:`Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.`},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:`Extensions the receiver must understand, or refuse the message with unsupported_extension.`}]},{name:`SpaceMessage`,description:`Publish, update or withdraw a Space: a place the sender controls where content can appear (a blog, a newsletter, an X account, a video channel, an app slot). A party sends put or withdraw under the list step. The host answers with a space message from the host: the space as listed, or waiting for the waiting party's agent. The host records approved_by agent. That is not a person's approval. The signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing. Space ids are unique per party. The locator stays hidden from counterparties until a deal that names the space is active.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "space"`,description:``},{name:`body`,required:!0,type:`SpaceBody`,description:``}]},{name:`SpaceBody`,description:``,closed:!0,fields:[{name:`action`,required:!0,type:`"put" | "withdraw" | "listed"`,description:`put and withdraw come from the owning party. listed comes from the host.`},{name:`space`,required:!1,type:`Space`,description:``},{name:`id`,required:!1,type:`LocalId`,description:`The space to withdraw.`},{name:`status`,required:!1,type:`"listed" | "waiting" | "withdrawn"`,description:`Set by the host.`},{name:`waiting`,required:!1,type:`Waiting`,description:`Set by the host when listing waits for the waiting party's agent. The host records approved_by agent. That is not a person's approval. The signed-in owner submits the form and the host records approved_by owner. Opening the URL records nothing.`},{name:`note`,required:!1,type:`Text`,description:``}]}],valid:[{name:`messages/37-space-put.json`,json:`{
  "orp": "0.3",
  "type": "space",
  "id": "m_pub_0101",
  "ts": "2026-10-21T08:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "action": "put",
    "space": {
      "id": "sp_news_1",
      "kind": "newsletter.issue",
      "platform": "newsletter",
      "title": "Indie Stack Weekly",
      "locator": "https://indiestack.example/issues",
      "topics": [
        "software.productivity",
        "business.startups"
      ],
      "languages": [
        "en"
      ],
      "audience": {
        "size": "10k-100k",
        "k_min": 50,
        "source": "platform"
      },
      "accepts": [
        "affiliate",
        "sponsored",
        "swap",
        "traffic_swap"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "rules": {
        "rel": "sponsored",
        "max_per_week": 2
      }
    }
  }
}`},{name:`messages/38-space-listed.json`,json:`{
  "orp": "0.3",
  "type": "space",
  "id": "m_host_0101",
  "ts": "2026-10-21T08:00:01Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "re": "m_pub_0101",
  "body": {
    "action": "listed",
    "space": {
      "id": "sp_news_1",
      "kind": "newsletter.issue",
      "platform": "newsletter",
      "title": "Indie Stack Weekly",
      "locator": "https://indiestack.example/issues",
      "topics": [
        "software.productivity",
        "business.startups"
      ],
      "languages": [
        "en"
      ],
      "audience": {
        "size": "10k-100k",
        "k_min": 50,
        "source": "platform"
      },
      "accepts": [
        "affiliate",
        "sponsored",
        "swap",
        "traffic_swap"
      ],
      "disclosure": {
        "label": "Partner mention",
        "lang": "en"
      },
      "rules": {
        "rel": "sponsored",
        "max_per_week": 2
      },
      "verified": [
        {
          "method": "domain",
          "at": "2026-09-02T08:00:00Z"
        }
      ]
    },
    "status": "listed"
  }
}`}],invalid:[{name:`invalid/space-put-without-space.json`,why:`A put names the space it publishes.`,json:`{
  "orp": "0.3",
  "type": "space",
  "id": "m_pub_0102",
  "ts": "2026-10-21T08:00:00Z",
  "from": "orp:hub.example/p_Vb3nR8sYc1Qe6",
  "body": {
    "action": "put"
  }
}`}]},{file:`0.4/defs.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/defs.schema.json`,slug:`defs`,href:`/protocol/spec/0.4/defs/`,title:`ORP 0.4 shared definitions`,description:`Types added for many-party deals. A 0.3 message does not use this file. Identity is a hash and an issuer name. A deposit term names an outside holder. Allagio does not hold money.`,kind:`schema`,type:``,required:[],anyOf:[],sections:[{name:`PartyRole`,description:`The seat this party holds. offerer opened the deal. acceptor is the counterparty on a two-role deal. The other roles are additional seats.`,closed:!1,fields:[]},{name:`PartyTerms`,description:`What this seat is paid or owes. Percent splits are integers. An amount is the amount field. per, hours, and a deposit are not amounts for the split rule. A deposit names who holds it outside Allagio.`,closed:!0,fields:[{name:`percent`,required:!1,type:`integer, min 0, max 100`,description:``},{name:`amount`,required:!1,type:`Money`,description:``},{name:`cap`,required:!1,type:`Money`,description:``},{name:`per`,required:!1,type:`"deal" | "hour" | "viewing"`,description:``},{name:`hours`,required:!1,type:`integer, min 0, max 1000`,description:``},{name:`deposit`,required:!1,type:`Deposit`,description:``}]},{name:`Deposit`,description:`A term that records who holds a deposit. The holder is outside Allagio. The proof records the agreed term and does not move money.`,closed:!0,fields:[{name:`amount`,required:!0,type:`Money`,description:``},{name:`holder`,required:!0,type:`string, pattern`,description:``},{name:`purpose`,required:!1,type:`string, min length 1, max length 80`,description:``}]},{name:`IdentityAttestation`,description:`A hash of an attestation plus the issuer name. No name, email, or other personal data.`,closed:!0,fields:[{name:`hash`,required:!0,type:`Hash`,description:``},{name:`issuer`,required:!0,type:`string, pattern`,description:``}]},{name:`PartySeat`,description:``,closed:!0,fields:[{name:`id`,required:!0,type:`PartyId`,description:``},{name:`role`,required:!0,type:`Role`,description:``},{name:`required`,required:!0,type:`boolean`,description:``},{name:`terms`,required:!1,type:`PartyTerms`,description:``},{name:`identity`,required:!1,type:`Identity`,description:``}]},{name:`DealParty`,description:`A seat on a deal snapshot, including whether that party has signed the current terms_hash.`,closed:!0,fields:[{name:`id`,required:!0,type:`PartyId`,description:``},{name:`role`,required:!0,type:`Role`,description:``},{name:`required`,required:!0,type:`boolean`,description:``},{name:`signed`,required:!0,type:`boolean`,description:``},{name:`terms`,required:!1,type:`PartyTerms`,description:``},{name:`identity`,required:!1,type:`Identity`,description:``}]},{name:`ClaimValue`,description:`unknown when nobody checked the field. A checked field carries the value, checked_at, and expires_at.`,closed:!1,fields:[]},{name:`Claims`,description:`Named fields on an offer. A field nobody checked is unknown. The host does not fill a default.`,closed:!1,fields:[]},{name:`Obligation`,description:`One thing a seat must do. by is a role or a party id.`,closed:!0,fields:[{name:`id`,required:!0,type:`ObligationId`,description:``},{name:`by`,required:!0,type:`value`,description:``},{name:`action`,required:!0,type:`"place" | "repost" | "comment" | "pay" | "credit" | "share"`,description:``},{name:`space`,required:!1,type:`SpaceRef`,description:``},{name:`content`,required:!1,type:`Content`,description:``},{name:`destination`,required:!1,type:`Url`,description:``},{name:`target`,required:!1,type:`Url`,description:``},{name:`count`,required:!1,type:`integer, min 1, max 100`,description:``},{name:`clicks`,required:!1,type:`integer, min 1, max 10000000`,description:``},{name:`quote`,required:!1,type:`boolean`,description:``},{name:`proof`,required:!1,type:`ProofList`,description:``},{name:`amount`,required:!1,type:`Money`,description:``},{name:`cap`,required:!1,type:`Money`,description:``},{name:`rate_bps`,required:!1,type:`integer, min 1, max 10000`,description:``},{name:`per`,required:!1,type:`"deal" | "placement" | "click" | "conversion" | "hour"`,description:``},{name:`window`,required:!1,type:`Duration`,description:``},{name:`hold`,required:!1,type:`Duration`,description:``},{name:`after`,required:!1,type:`ObligationId`,description:``},{name:`due`,required:!1,type:`value`,description:``},{name:`keep`,required:!1,type:`Duration`,description:``},{name:`rail`,required:!1,type:`string, pattern`,description:``},{name:`credits`,required:!1,type:`integer, min 1, max 1000000`,description:``},{name:`note`,required:!1,type:`Text`,description:``}]},{name:`Terms`,description:`Shared terms. checked_at and expires_at are the maker confirmation window (checked_at plus 90 days). claims holds per-field check results.`,closed:!0,fields:[{name:`title`,required:!0,type:`string, min length 1, max length 120`,description:``},{name:`kind`,required:!1,type:`DealKind`,description:``},{name:`obligations`,required:!0,type:`array of Obligation`,description:``},{name:`categories`,required:!1,type:`array of Category`,description:``},{name:`disclosure`,required:!1,type:`Disclosure`,description:``},{name:`starts`,required:!1,type:`Timestamp`,description:``},{name:`ends`,required:!1,type:`Timestamp`,description:``},{name:`checked_at`,required:!1,type:`Timestamp`,description:``},{name:`expires_at`,required:!1,type:`Timestamp`,description:``},{name:`claims`,required:!1,type:`Claims`,description:``}]},{name:`MessageType`,description:`The 0.3 message types plus leave and seat. A 0.3 message cannot name leave or seat.`,closed:!1,fields:[]},{name:`Capabilities`,description:`What a 0.4 host advertises. messages may include leave and seat.`,closed:!0,fields:[{name:`messages`,required:!1,type:`array of MessageType`,description:``},{name:`spaces`,required:!1,type:`array of SpaceKind`,description:``},{name:`actions`,required:!1,type:`array of string, pattern`,description:``},{name:`proofs`,required:!1,type:`array of ProofMethod`,description:``},{name:`tracking`,required:!1,type:`object, closed`,description:``},{name:`signatures`,required:!1,type:`object, closed`,description:``},{name:`inbox`,required:!1,type:`object, closed`,description:``},{name:`checks`,required:!1,type:`object, closed`,description:``},{name:`ext`,required:!1,type:`array of ExtName`,description:``}]}],valid:[],invalid:[]},{file:`0.4/discovery.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/discovery.schema.json`,slug:`discovery`,href:`/protocol/spec/0.4/discovery/`,title:`Discovery`,description:`The document a 0.4 host serves at /.well-known/orp.json. capabilities.messages may include leave and seat. A 0.3 hello reply does not list those types.`,kind:`schema`,type:``,required:[`orp`,`host`],anyOf:[],sections:[{name:`Discovery`,description:`The document a 0.4 host serves at /.well-known/orp.json. capabilities.messages may include leave and seat. A 0.3 hello reply does not list those types.`,closed:!0,fields:[{name:`orp`,required:!0,type:`array of Version`,description:`Supported protocol versions, newest first.`},{name:`host`,required:!0,type:`PartyId`,description:`The host's own party id, orp:<host>/host. Host-originated messages use it as from.`},{name:`name`,required:!1,type:`string, min length 1, max length 80`,description:``},{name:`description`,required:!1,type:`string, max length 500`,description:``},{name:`endpoint`,required:!1,type:`UrlOrPath`,description:`The HTTP binding: POST messages here. Required for Level 1 and up.`},{name:`suggest`,required:!1,type:`UrlOrPath`,description:`POST a draft here with no credentials to get deals a person could take now. Required for Level 1 and up.`},{name:`join`,required:!1,type:`UrlOrPath`,description:`POST here with no credentials and no personal data to get a party id and token. Required for Level 1 and up.`},{name:`mcp`,required:!1,type:`object, closed`,description:``},{name:`auth`,required:!1,type:`object, closed`,description:``},{name:`tracking`,required:!1,type:`object, closed`,description:``},{name:`conformance`,required:!1,type:`object, closed`,description:``},{name:`capabilities`,required:!1,type:`Capabilities`,description:``},{name:`keys`,required:!1,type:`array of object`,description:`Public keys for host signatures (Level 3), as JWKs.`},{name:`policies`,required:!1,type:`object, closed`,description:``},{name:`docs`,required:!1,type:`UrlOrPath`,description:``},{name:`legacy`,required:!1,type:`object`,description:`Pre-ORP interfaces the host still serves. Informational; ORP clients ignore it.`}]},{name:`UrlOrPath`,description:``,closed:!1,fields:[]},{name:`UrlOrPath`,description:``,closed:!1,fields:[]},{name:`UrlOrPath`,description:``,closed:!1,fields:[]},{name:`UrlOrPath`,description:``,closed:!1,fields:[]}],valid:[],invalid:[]},{file:`0.4/envelope.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/envelope.schema.json`,slug:`envelope`,href:`/protocol/spec/0.4/envelope/`,title:`Envelope`,description:`The ORP 0.4 envelope. orp is 0.4. leave and seat are 0.4 message types.`,kind:`schema`,type:``,required:[`orp`,`type`,`id`,`ts`,`from`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.4 envelope. orp is 0.4. leave and seat are 0.4 message types.`,closed:!0,fields:[{name:`orp`,required:!0,type:`const "0.4"`,description:``},{name:`type`,required:!0,type:`"hello" | "discover" | "results" | "space" | "request" | "offer" | "counter" | "accept" | "decline" | "deal" | "execute" | "receipt" | "outcome" | "payout" | "revoke" | "consent" | "dispute" | "reputation" | "error" | "leave" | "seat"`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:``},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:``}]}],valid:[],invalid:[]},{file:`0.4/http/inbox.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/http/inbox.schema.json`,slug:`inbox`,href:`/protocol/spec/0.4/inbox/`,title:`InboxPage`,description:`Inbox page that may mix 0.3 and 0.4 messages.`,kind:`http`,type:``,required:[`messages`,`next`],anyOf:[],sections:[{name:`InboxPage`,description:`Inbox page that may mix 0.3 and 0.4 messages.`,closed:!0,fields:[{name:`messages`,required:!0,type:`array of value`,description:``},{name:`next`,required:!0,type:`string, max length 200`,description:``}]}],valid:[],invalid:[]},{file:`0.4/http/suggest-response.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/http/suggest-response.schema.json`,slug:`suggest-response`,href:`/protocol/spec/0.4/suggest-response/`,title:`SuggestResponse`,description:`Suggest response that may include 0.4 offer terms. An expired offer is absent.`,kind:`http`,type:``,required:[`items`,`join`],anyOf:[],sections:[{name:`SuggestResponse`,description:`Suggest response that may include 0.4 offer terms. An expired offer is absent.`,closed:!0,fields:[{name:`items`,required:!0,type:`array of object`,description:``},{name:`empty_reason`,required:!1,type:`"no_match" | "below_threshold" | "filtered"`,description:``},{name:`joined`,required:!1,type:`join-response.schema.json`,description:``},{name:`join`,required:!0,type:`UrlOrPath`,description:``}]}],valid:[],invalid:[]},{file:`0.4/message.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/message.schema.json`,slug:`message`,href:`/protocol/spec/0.4/message/`,title:`Message`,description:`An ORP 0.4 message. hello, deal, offer, results, leave, and seat use the 0.4 bodies. Every other type uses the 0.3 body rules.`,kind:`schema`,type:``,required:[],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.4 envelope. orp is 0.4. leave and seat are 0.4 message types.`,closed:!0,fields:[{name:`orp`,required:!0,type:`const "0.4"`,description:``},{name:`type`,required:!0,type:`"hello" | "discover" | "results" | "space" | "request" | "offer" | "counter" | "accept" | "decline" | "deal" | "execute" | "receipt" | "outcome" | "payout" | "revoke" | "consent" | "dispute" | "reputation" | "error" | "leave" | "seat"`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:``},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:``}]},{name:`Composed schemas`,description:`An ORP 0.4 message. hello, deal, offer, results, leave, and seat use the 0.4 bodies. Every other type uses the 0.3 body rules.`,closed:!1,fields:[{name:`envelope.schema.json`,required:!1,type:`envelope.schema.json`,description:``},{name:`hello`,required:!0,type:`messages/hello.schema.json`,description:``},{name:`deal`,required:!0,type:`messages/deal.schema.json`,description:``},{name:`offer`,required:!0,type:`messages/offer.schema.json`,description:``},{name:`leave`,required:!0,type:`messages/leave.schema.json`,description:``},{name:`seat`,required:!0,type:`messages/seat.schema.json`,description:``},{name:`results`,required:!0,type:`messages/results.schema.json`,description:``}]}],valid:[],invalid:[{name:`invalid-schema/identity-email.json`,why:`Identity is a hash and an issuer name.`,json:`{
  "orp": "0.4",
  "type": "offer",
  "id": "m_maker_identity",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/p_maker01",
  "body": {
    "terms": {
      "title": "Bad notes split",
      "kind": "partnership",
      "obligations": [
        {
          "id": "share",
          "by": "offerer",
          "action": "share"
        }
      ]
    },
    "parties": [
      {
        "id": "orp:hub.example/p_maker01",
        "role": "offerer",
        "required": true
      },
      {
        "id": "orp:hub.example/p_other01",
        "role": "facilitator",
        "required": true,
        "identity": {
          "hash": "sha256-Hr6VqstWHV4oel7_vs40UWBZF-_GVFzxoFUbPvD2-jg",
          "issuer": "example-issuer",
          "email": "person@example.com"
        }
      }
    ]
  }
}`},{name:`invalid-schema/one-party.json`,why:`A deal has at least 2 parties.`,json:`{
  "orp": "0.4",
  "type": "offer",
  "id": "m_maker_oneparty",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/p_maker01",
  "body": {
    "terms": {
      "title": "Bad notes split",
      "kind": "partnership",
      "obligations": [
        {
          "id": "share",
          "by": "offerer",
          "action": "share"
        }
      ]
    },
    "parties": [
      {
        "id": "orp:hub.example/p_maker01",
        "role": "offerer",
        "required": true
      }
    ]
  }
}`},{name:`invalid-schema/seventeen-parties.json`,why:`A deal has at most 16 parties.`,json:`{
  "orp": "0.4",
  "type": "offer",
  "id": "m_maker_toomany",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/p_maker01",
  "body": {
    "terms": {
      "title": "Bad notes split",
      "kind": "partnership",
      "obligations": [
        {
          "id": "share",
          "by": "offerer",
          "action": "share"
        }
      ]
    },
    "parties": [
      {
        "id": "orp:hub.example/p_many00",
        "role": "offerer",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many01",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many02",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many03",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many04",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many05",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many06",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many07",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many08",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many09",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many10",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many11",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many12",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many13",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many14",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many15",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many16",
        "role": "member",
        "required": true
      }
    ]
  }
}`}]},{file:`0.4/messages/deal.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/messages/deal.schema.json`,slug:`deal`,href:`/protocol/spec/0.4/deal/`,title:`DealMessage`,description:`A many-party deal snapshot. State is pending until every required party has signed. Optional parties do not block active.`,kind:`message`,type:`deal`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.4 envelope. orp is 0.4. leave and seat are 0.4 message types.`,closed:!0,fields:[{name:`orp`,required:!0,type:`const "0.4"`,description:``},{name:`type`,required:!0,type:`"hello" | "discover" | "results" | "space" | "request" | "offer" | "counter" | "accept" | "decline" | "deal" | "execute" | "receipt" | "outcome" | "payout" | "revoke" | "consent" | "dispute" | "reputation" | "error" | "leave" | "seat"`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:``},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:``}]},{name:`DealMessage`,description:`A many-party deal snapshot. State is pending until every required party has signed. Optional parties do not block active.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "deal"`,description:``},{name:`body`,required:!0,type:`DealBody`,description:``}]},{name:`DealBody`,description:``,closed:!0,fields:[{name:`state`,required:!0,type:`DealState`,description:`pending: a required signature is missing. active: every required party has signed. The other states match 0.3.`},{name:`parties`,required:!0,type:`array of DealParty`,description:``},{name:`offerer`,required:!1,type:`PartyId`,description:``},{name:`acceptor`,required:!1,type:`PartyId`,description:``},{name:`offer`,required:!1,type:`LocalId`,description:``},{name:`terms`,required:!0,type:`Terms`,description:``},{name:`terms_hash`,required:!0,type:`Hash`,description:``},{name:`round`,required:!1,type:`integer, min 1, max 100`,description:``},{name:`bound`,required:!1,type:`array of object, closed`,description:``},{name:`vacancies`,required:!1,type:`array of Role`,description:`Roles whose required seat is empty. The deal stays pending until the required replacement seated for that vacancy signs. An original seat of the same role does not clear it. An optional seat of that role does not clear it.`},{name:`waiting`,required:!1,type:`Waiting`,description:``},{name:`tracking`,required:!1,type:`array of Tracking`,description:``},{name:`obligations`,required:!1,type:`array of object, closed`,description:``},{name:`expires`,required:!1,type:`Timestamp`,description:``},{name:`updated`,required:!0,type:`Timestamp`,description:``},{name:`approved_by`,required:!1,type:`array of object, closed`,description:`Who approved each ask step on an ORP 0.4 deal only. A 0.3 deal does not include approved_by. party is the seat id already on the deal, not a name or email. approved_by is agent or owner. agent is the waiting party's bearer and is not a person's approval. owner is the signed-in owner who submitted the form.`},{name:`approvals_unavailable`,required:!1,type:`const true`,description:`True only on an ORP 0.4 deal read when the host could not load who approved. The response stays HTTP 200. approved_by is omitted. A 0.3 deal does not include approvals_unavailable.`}]}],valid:[{name:`five-party-home-sale.json`,json:`{
  "orp": "0.4",
  "type": "deal",
  "id": "m_host_fiveseat",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_seller01",
  "deal": "deal_fiveseat",
  "body": {
    "state": "active",
    "parties": [
      {
        "id": "orp:hub.example/p_seller01",
        "role": "offerer",
        "required": true,
        "signed": true,
        "terms": {
          "amount": {
            "amount_minor": 0,
            "currency": "EUR"
          }
        }
      },
      {
        "id": "orp:hub.example/p_buyer01",
        "role": "acceptor",
        "required": true,
        "signed": true,
        "terms": {
          "amount": {
            "amount_minor": 100000,
            "currency": "EUR"
          }
        }
      },
      {
        "id": "orp:hub.example/p_media01",
        "role": "facilitator",
        "required": true,
        "signed": true,
        "identity": {
          "hash": "sha256-Hr6VqstWHV4oel7_vs40UWBZF-_GVFzxoFUbPvD2-jg",
          "issuer": "example-issuer"
        },
        "terms": {
          "amount": {
            "amount_minor": 100000,
            "currency": "EUR"
          }
        }
      },
      {
        "id": "orp:hub.example/p_door01",
        "role": "service",
        "required": false,
        "signed": false,
        "terms": {
          "amount": {
            "amount_minor": 4000,
            "currency": "EUR"
          },
          "per": "hour",
          "hours": 2,
          "deposit": {
            "amount": {
              "amount_minor": 2000,
              "currency": "EUR"
            },
            "holder": "viewing-desk",
            "purpose": "viewing"
          }
        }
      },
      {
        "id": "orp:hub.example/p_notary01",
        "role": "signer",
        "required": true,
        "signed": true
      }
    ],
    "terms": {
      "title": "Illustrative five-seat deal, test mode only",
      "kind": "partnership",
      "obligations": [
        {
          "id": "share",
          "by": "signer",
          "action": "share"
        }
      ]
    },
    "terms_hash": "sha256-8yasGCLG6_gYnrZohUPxzWrcvfATIfaDgRovkGODFyk",
    "updated": "2026-10-26T12:00:00.000Z",
    "offerer": "orp:hub.example/p_seller01",
    "acceptor": "orp:hub.example/p_buyer01",
    "round": 1
  }
}`},{name:`link-swap-ring.json`,json:`{
  "orp": "0.4",
  "type": "deal",
  "id": "m_host_linkring",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_ring01",
  "deal": "deal_linkring",
  "body": {
    "state": "active",
    "parties": [
      {
        "id": "orp:hub.example/p_ring01",
        "role": "member",
        "required": true,
        "signed": true,
        "terms": {
          "percent": 25
        }
      },
      {
        "id": "orp:hub.example/p_ring02",
        "role": "member",
        "required": true,
        "signed": true,
        "terms": {
          "percent": 25
        }
      },
      {
        "id": "orp:hub.example/p_ring03",
        "role": "member",
        "required": true,
        "signed": true,
        "terms": {
          "percent": 25
        }
      },
      {
        "id": "orp:hub.example/p_ring04",
        "role": "member",
        "required": true,
        "signed": true,
        "terms": {
          "percent": 25
        }
      }
    ],
    "terms": {
      "title": "Link swap ring",
      "kind": "link_exchange",
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place1",
          "by": "orp:hub.example/p_ring01",
          "action": "place",
          "content": {
            "link": true,
            "brief": "A notes link."
          },
          "destination": "https://notes.example/pricing?plan=pro",
          "proof": [
            "link"
          ]
        },
        {
          "id": "place2",
          "by": "orp:hub.example/p_ring02",
          "action": "place",
          "content": {
            "link": true,
            "brief": "A notes link."
          },
          "destination": "https://notes.example/pricing?plan=pro",
          "proof": [
            "link"
          ]
        },
        {
          "id": "place3",
          "by": "orp:hub.example/p_ring03",
          "action": "place",
          "content": {
            "link": true,
            "brief": "A notes link."
          },
          "destination": "https://notes.example/pricing?plan=pro",
          "proof": [
            "link"
          ]
        },
        {
          "id": "place4",
          "by": "orp:hub.example/p_ring04",
          "action": "place",
          "content": {
            "link": true,
            "brief": "A notes link."
          },
          "destination": "https://notes.example/pricing?plan=pro",
          "proof": [
            "link"
          ]
        }
      ]
    },
    "terms_hash": "sha256-OFGZAtUqd100Srkb76A7dIG8SNY3KWa9-eIiTsCbTlw",
    "updated": "2026-10-26T12:00:00.000Z",
    "round": 1
  }
}`},{name:`mediator-left.json`,json:`{
  "orp": "0.4",
  "type": "deal",
  "id": "m_host_medleft",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_seller01",
  "deal": "deal_medleft",
  "body": {
    "state": "pending",
    "parties": [
      {
        "id": "orp:hub.example/p_seller01",
        "role": "offerer",
        "required": true,
        "signed": true,
        "terms": {
          "amount": {
            "amount_minor": 0,
            "currency": "EUR"
          }
        }
      },
      {
        "id": "orp:hub.example/p_buyer01",
        "role": "acceptor",
        "required": true,
        "signed": true,
        "terms": {
          "amount": {
            "amount_minor": 100000,
            "currency": "EUR"
          }
        }
      }
    ],
    "terms": {
      "title": "Illustrative mediator seat, test mode only",
      "kind": "partnership",
      "obligations": [
        {
          "id": "share",
          "by": "offerer",
          "action": "share"
        }
      ]
    },
    "terms_hash": "sha256-7PdskwQsfwZslXUx3OE0sySgXwH0bB5QGpatMOvGW2k",
    "updated": "2026-10-26T12:00:00.000Z",
    "offerer": "orp:hub.example/p_seller01",
    "acceptor": "orp:hub.example/p_buyer01",
    "round": 1,
    "vacancies": [
      "facilitator"
    ]
  }
}`},{name:`referral-chain.json`,json:`{
  "orp": "0.4",
  "type": "deal",
  "id": "m_host_referral",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/host",
  "to": "orp:hub.example/p_creator01",
  "deal": "deal_referral",
  "body": {
    "state": "active",
    "parties": [
      {
        "id": "orp:hub.example/p_creator01",
        "role": "offerer",
        "required": true,
        "signed": true,
        "terms": {
          "percent": 50
        }
      },
      {
        "id": "orp:hub.example/p_curator01",
        "role": "curator",
        "required": true,
        "signed": true,
        "terms": {
          "percent": 30
        }
      },
      {
        "id": "orp:hub.example/p_news01",
        "role": "acceptor",
        "required": true,
        "signed": true,
        "terms": {
          "percent": 20
        }
      }
    ],
    "terms": {
      "title": "Referral notes chain",
      "kind": "partnership",
      "disclosure": {
        "label": "Affiliate link",
        "lang": "en"
      },
      "obligations": [
        {
          "id": "place",
          "by": "orp:hub.example/p_news01",
          "action": "place",
          "content": {
            "link": true,
            "brief": "Mention the notes app."
          },
          "destination": "https://notes.example/pricing?plan=pro",
          "proof": [
            "link"
          ]
        }
      ]
    },
    "terms_hash": "sha256-lx3RaehTPOZnKQISKqCAJWIiwbJ1U51oHl0gN791ukU",
    "approved_by": [
      {
        "step": "accept",
        "party": "orp:hub.example/p_creator01",
        "approved_by": "agent"
      },
      {
        "step": "accept",
        "party": "orp:hub.example/p_news01",
        "approved_by": "owner"
      }
    ],
    "updated": "2026-10-26T12:00:00.000Z",
    "offerer": "orp:hub.example/p_creator01",
    "acceptor": "orp:hub.example/p_news01",
    "round": 1
  }
}`}],invalid:[]},{file:`0.4/messages/hello.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/messages/hello.schema.json`,slug:`hello`,href:`/protocol/spec/0.4/hello/`,title:`HelloMessage`,description:`Version and capability negotiation for ORP 0.4. The reply lists leave and seat. A 0.3 hello does not.`,kind:`message`,type:`hello`,required:[`type`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.4 envelope. orp is 0.4. leave and seat are 0.4 message types.`,closed:!0,fields:[{name:`orp`,required:!0,type:`const "0.4"`,description:``},{name:`type`,required:!0,type:`"hello" | "discover" | "results" | "space" | "request" | "offer" | "counter" | "accept" | "decline" | "deal" | "execute" | "receipt" | "outcome" | "payout" | "revoke" | "consent" | "dispute" | "reputation" | "error" | "leave" | "seat"`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:``},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:``}]},{name:`HelloMessage`,description:`Version and capability negotiation for ORP 0.4. The reply lists leave and seat. A 0.3 hello does not.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "hello"`,description:``},{name:`body`,required:!0,type:`HelloBody`,description:``}]},{name:`HelloBody`,description:``,closed:!0,fields:[{name:`versions`,required:!0,type:`array of Version`,description:``},{name:`capabilities`,required:!1,type:`Capabilities`,description:``},{name:`party`,required:!1,type:`PartyId`,description:``},{name:`limits`,required:!1,type:`object, closed`,description:``}]}],valid:[],invalid:[]},{file:`0.4/messages/leave.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/messages/leave.schema.json`,slug:`leave`,href:`/protocol/spec/0.4/leave/`,title:`LeaveMessage`,description:`The sender leaves a deal. An optional party can leave and the deal stays valid. A required party leaves a vacancy and the deal returns to pending until a replacement signs.`,kind:`message`,type:`leave`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.4 envelope. orp is 0.4. leave and seat are 0.4 message types.`,closed:!0,fields:[{name:`orp`,required:!0,type:`const "0.4"`,description:``},{name:`type`,required:!0,type:`"hello" | "discover" | "results" | "space" | "request" | "offer" | "counter" | "accept" | "decline" | "deal" | "execute" | "receipt" | "outcome" | "payout" | "revoke" | "consent" | "dispute" | "reputation" | "error" | "leave" | "seat"`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:``},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:``}]},{name:`LeaveMessage`,description:`The sender leaves a deal. An optional party can leave and the deal stays valid. A required party leaves a vacancy and the deal returns to pending until a replacement signs.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "leave"`,description:``},{name:`body`,required:!0,type:`object, closed`,description:``}]}],valid:[],invalid:[]},{file:`0.4/messages/offer.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/messages/offer.schema.json`,slug:`offer`,href:`/protocol/spec/0.4/offer/`,title:`OfferMessage`,description:`A 0.4 offer. parties opens a many-party deal. Without parties it lists a standing offer whose terms carry checked_at and expires_at. confirm refreshes that window.`,kind:`message`,type:`offer`,required:[`type`,`body`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.4 envelope. orp is 0.4. leave and seat are 0.4 message types.`,closed:!0,fields:[{name:`orp`,required:!0,type:`const "0.4"`,description:``},{name:`type`,required:!0,type:`"hello" | "discover" | "results" | "space" | "request" | "offer" | "counter" | "accept" | "decline" | "deal" | "execute" | "receipt" | "outcome" | "payout" | "revoke" | "consent" | "dispute" | "reputation" | "error" | "leave" | "seat"`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:``},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:``}]},{name:`OfferMessage`,description:`A 0.4 offer. parties opens a many-party deal. Without parties it lists a standing offer whose terms carry checked_at and expires_at. confirm refreshes that window.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "offer"`,description:``},{name:`body`,required:!0,type:`OfferBody`,description:``}]},{name:`OfferBody`,description:``,closed:!0,fields:[{name:`id`,required:!1,type:`LocalId`,description:``},{name:`confirm`,required:!1,type:`boolean`,description:``},{name:`terms`,required:!1,type:`Terms`,description:``},{name:`terms_hash`,required:!1,type:`Hash`,description:``},{name:`parties`,required:!1,type:`array of PartySeat`,description:``},{name:`limit`,required:!1,type:`integer, min 1, max 100000`,description:``},{name:`waiting`,required:!1,type:`Waiting`,description:``},{name:`note`,required:!1,type:`Text`,description:``},{name:`expires`,required:!1,type:`Timestamp`,description:``}]}],valid:[{name:`bad-split.json`,json:`{
  "orp": "0.4",
  "type": "offer",
  "id": "m_maker_badsplit",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/p_maker01",
  "body": {
    "terms": {
      "title": "Bad notes split",
      "kind": "partnership",
      "obligations": [
        {
          "id": "share",
          "by": "offerer",
          "action": "share"
        }
      ]
    },
    "parties": [
      {
        "id": "orp:hub.example/p_maker01",
        "role": "offerer",
        "required": true,
        "terms": {
          "percent": 60
        }
      },
      {
        "id": "orp:hub.example/p_other01",
        "role": "acceptor",
        "required": true,
        "terms": {
          "percent": 41
        }
      }
    ]
  }
}`}],invalid:[{name:`invalid-schema/identity-email.json`,why:`Identity is a hash and an issuer name.`,json:`{
  "orp": "0.4",
  "type": "offer",
  "id": "m_maker_identity",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/p_maker01",
  "body": {
    "terms": {
      "title": "Bad notes split",
      "kind": "partnership",
      "obligations": [
        {
          "id": "share",
          "by": "offerer",
          "action": "share"
        }
      ]
    },
    "parties": [
      {
        "id": "orp:hub.example/p_maker01",
        "role": "offerer",
        "required": true
      },
      {
        "id": "orp:hub.example/p_other01",
        "role": "facilitator",
        "required": true,
        "identity": {
          "hash": "sha256-Hr6VqstWHV4oel7_vs40UWBZF-_GVFzxoFUbPvD2-jg",
          "issuer": "example-issuer",
          "email": "person@example.com"
        }
      }
    ]
  }
}`},{name:`invalid-schema/one-party.json`,why:`A deal has at least 2 parties.`,json:`{
  "orp": "0.4",
  "type": "offer",
  "id": "m_maker_oneparty",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/p_maker01",
  "body": {
    "terms": {
      "title": "Bad notes split",
      "kind": "partnership",
      "obligations": [
        {
          "id": "share",
          "by": "offerer",
          "action": "share"
        }
      ]
    },
    "parties": [
      {
        "id": "orp:hub.example/p_maker01",
        "role": "offerer",
        "required": true
      }
    ]
  }
}`},{name:`invalid-schema/seventeen-parties.json`,why:`A deal has at most 16 parties.`,json:`{
  "orp": "0.4",
  "type": "offer",
  "id": "m_maker_toomany",
  "ts": "2026-10-26T12:00:00.000Z",
  "from": "orp:hub.example/p_maker01",
  "body": {
    "terms": {
      "title": "Bad notes split",
      "kind": "partnership",
      "obligations": [
        {
          "id": "share",
          "by": "offerer",
          "action": "share"
        }
      ]
    },
    "parties": [
      {
        "id": "orp:hub.example/p_many00",
        "role": "offerer",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many01",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many02",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many03",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many04",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many05",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many06",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many07",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many08",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many09",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many10",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many11",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many12",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many13",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many14",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many15",
        "role": "member",
        "required": true
      },
      {
        "id": "orp:hub.example/p_many16",
        "role": "member",
        "required": true
      }
    ]
  }
}`}]},{file:`0.4/messages/results.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/messages/results.schema.json`,slug:`results`,href:`/protocol/spec/0.4/results/`,title:`ResultsMessage`,description:`Discovery results that may include 0.4 offer terms (checked_at, expires_at, claims).`,kind:`message`,type:`results`,required:[`type`,`body`,`re`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.4 envelope. orp is 0.4. leave and seat are 0.4 message types.`,closed:!0,fields:[{name:`orp`,required:!0,type:`const "0.4"`,description:``},{name:`type`,required:!0,type:`"hello" | "discover" | "results" | "space" | "request" | "offer" | "counter" | "accept" | "decline" | "deal" | "execute" | "receipt" | "outcome" | "payout" | "revoke" | "consent" | "dispute" | "reputation" | "error" | "leave" | "seat"`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:``},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:``}]},{name:`ResultsMessage`,description:`Discovery results that may include 0.4 offer terms (checked_at, expires_at, claims).`,closed:!1,fields:[{name:`type`,required:!0,type:`const "results"`,description:``},{name:`body`,required:!0,type:`ResultsBody`,description:``}]},{name:`body`,description:``,closed:!0,fields:[{name:`items`,required:!0,type:`array of Result`,description:``},{name:`cursor`,required:!1,type:`string, max length 200`,description:``},{name:`empty_reason`,required:!1,type:`"no_match" | "below_threshold" | "filtered"`,description:``}]}],valid:[],invalid:[]},{file:`0.4/messages/seat.schema.json`,version:`0.4`,id:`https://offer-rails.onrender.com/orp/schemas/0.4/messages/seat.schema.json`,slug:`seat`,href:`/protocol/spec/0.4/seat/`,title:`SeatMessage`,description:`Add a party to an open deal. The new party is unsigned until it accepts. A seat that fills a vacancy still leaves the deal pending until that party signs.`,kind:`message`,type:`seat`,required:[`type`,`body`,`deal`],anyOf:[],sections:[{name:`Envelope`,description:`The ORP 0.4 envelope. orp is 0.4. leave and seat are 0.4 message types.`,closed:!0,fields:[{name:`orp`,required:!0,type:`const "0.4"`,description:``},{name:`type`,required:!0,type:`"hello" | "discover" | "results" | "space" | "request" | "offer" | "counter" | "accept" | "decline" | "deal" | "execute" | "receipt" | "outcome" | "payout" | "revoke" | "consent" | "dispute" | "reputation" | "error" | "leave" | "seat"`,description:``},{name:`id`,required:!0,type:`MessageId`,description:``},{name:`ts`,required:!0,type:`Timestamp`,description:``},{name:`from`,required:!0,type:`PartyId`,description:``},{name:`to`,required:!1,type:`PartyId`,description:``},{name:`deal`,required:!1,type:`LocalId`,description:``},{name:`re`,required:!1,type:`MessageId`,description:``},{name:`body`,required:!0,type:`object`,description:``},{name:`agent`,required:!1,type:`Agent`,description:``},{name:`prev`,required:!1,type:`Hash`,description:``},{name:`sig`,required:!1,type:`string, max length 4096, pattern`,description:``},{name:`ext`,required:!1,type:`object`,description:``},{name:`critical`,required:!1,type:`array of ExtName`,description:``}]},{name:`SeatMessage`,description:`Add a party to an open deal. The new party is unsigned until it accepts. A seat that fills a vacancy still leaves the deal pending until that party signs.`,closed:!1,fields:[{name:`type`,required:!0,type:`const "seat"`,description:``},{name:`body`,required:!0,type:`object, closed`,description:``}]}],valid:[],invalid:[]}],conformance:`INVALID-68.L0-SCHEMA.L0-HASH.L0-IDEMPOTENT.L0-REPLAY.L0-STALE.C1-UNIVERSAL-LINK-NO-ADAPTER.C2-NO-EMAIL-NO-SIGNUP.L1-PLAIN-AFTER-REVOKE.L1-NO-PII.L1-TEST-CLICKS.L1-POLICIES.L2-STEP-MODES.L2-CONSENT-HOST-ONLY.L2-NO-SELF-RECEIPT.L2-ATTESTATION-NO-REP.L2-CLAIMS-UNCHECKED.L2-RECHECK.L3-KINDS.L3-NEGOTIATE.L3-SPACE.L3-DISPUTE.L3-REPUTATION.L3-PAYOUT.ORP04-PROPERTY.ORP04-REFERRAL-CHAIN.ORP04-LINK-SWAP-RING.ORP04-HOME-SALE.ORP04-BAD-SPLIT.ORP04-MEDIATOR-LEAVES.ORP04-EXPIRED-OFFER.ORP04-N2-UNCHANGED.ORP04-OPTIONAL-DOES-NOT-CLEAR-VACANCY.ORP04-LEAVE-KEEPS-SPLIT.ORP04-SEAT-READS-PENDING.ORP04-ORIGINAL-SEAT-DOES-NOT-FILL-VACANCY.ORP04-APPROVE-ATOMIC.ORP04-CAPABILITIES.ORP04-MIXED-SPLIT.ORP04-DEPOSIT-HOLDER`.split(`.`),sdk:[`sdk/src/client.mjs`,`sdk/src/host.mjs`,`sdk/src/index.mjs`,`sdk/src/jcs.mjs`,`sdk/src/multipart.mjs`,`sdk/src/sign.mjs`,`sdk/src/validate.mjs`],sdkReadme:'# ORP reference host\n\nIn-process host and a thin client for the conformance suite. Party behavior is HTTP. The scheduler hooks (`advanceBusiness`, `runDueChecks`, `registerPage`, `markTestPool`, `failNextRecheck`, `resolveDispute`) are in-process on the reference host. The host speaks ORP 0.4 and 0.3. The Allagio hosted API speaks 0.4 and 0.3. `CLAIMS_CHECK` on that host stays a stub, so claim fields stay `unknown`.\n\n`claims: "real"` checks destinations against the in-memory page catalog. It does not fetch a page and it is not the hosted `CLAIMS_CHECK` setting. `stub` and `none` leave an offer unchecked.\n\nThe provider test token and owner tokens are generated per host and kept in the local store. They are not production secrets.',exampleCount:52,invalidCount:56},l=`In testing`,u=[`hello`,`discover`,`results`,`offer`,`accept`,`deal`,`execute`,`receipt`,`error`],d=[`results`,`deal`,`outcome`,`consent`],f=`Only the host may send this.`,p=`Every step is ask, except revoke, which is auto.`,m=`Allagio does not hold money.`,h=`This host speaks ORP 0.4 (leave and seat).`,g=`What this host speaks is not shown here until /.well-known/orp.json loads.`,_=`Only the host issues a receipt.`,v=`This host is in testing and does not run this message.`,y=`This testing host does not settle money.`;function b(e){if(!e||typeof e!=`object`)return g;let t=Array.isArray(e.orp)?e.orp.map(e=>String(e)):null,n=e.capabilities,r=n&&typeof n==`object`&&Array.isArray(n.messages)?n.messages.map(e=>String(e)):null;return!t||!r?g:t.includes(`0.4`)&&r.includes(`leave`)&&r.includes(`seat`)?h:g}function x(){let[e,t]=(0,s.useState)(g);return(0,s.useEffect)(()=>{let e=new AbortController;return fetch(`/.well-known/orp.json`,{headers:{accept:`application/json`},cache:`no-store`,signal:e.signal}).then(async e=>{if(!e.ok)throw Error(String(e.status));return e.json()}).then(n=>{e.signal.aborted||t(b(n))}).catch(()=>{e.signal.aborted||t(g)}),()=>e.abort()},[]),e}var S=i(),C=`Illustrative, test mode, not legal advice.`;function w(e,t){let n=c.schemas.find(t=>t.version===`0.4`&&t.slug===e)?.valid.find(e=>e.name.endsWith(t));if(!n)throw Error(`missing 0.4 ${e} ${t}`);return n}function T(e){return JSON.parse(w(`deal`,e).json)}function E(e){return`${e.amount_minor} ${e.currency}`}function D({label:e,code:t}){let[n,r]=(0,s.useState)(`Copy`);return(0,S.jsxs)(`div`,{className:`mt-4`,children:[(0,S.jsxs)(`div`,{className:`mb-1 flex items-center justify-between gap-3`,children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:e}),(0,S.jsx)(`button`,{type:`button`,className:`inline-flex min-h-11 items-center rounded-md border border-line bg-s1 px-3 text-sm font-semibold text-t1`,"aria-label":`Copy ${e}`,onClick:()=>{let e=e=>{r(e),window.setTimeout(()=>r(`Copy`),2e3)};navigator.clipboard?navigator.clipboard.writeText(t).then(()=>e(`Copied`),()=>e(`Copy failed`)):e(`Copy failed`)},children:n})]}),(0,S.jsx)(`pre`,{tabIndex:0,className:`max-w-full overflow-x-auto rounded-xl border border-line bg-s1 p-4 font-mono text-[13px] leading-6 text-t1`,children:(0,S.jsx)(`code`,{children:t})})]})}function O({parties:e}){return(0,S.jsx)(`div`,{tabIndex:0,className:`mt-3 max-w-full overflow-x-auto`,children:(0,S.jsxs)(`table`,{className:`w-full border-collapse text-left text-sm`,children:[(0,S.jsx)(`caption`,{className:`sr-only`,children:`Seats in this example`}),(0,S.jsx)(`thead`,{children:(0,S.jsxs)(`tr`,{className:`border-b border-line-strong`,children:[(0,S.jsx)(`th`,{scope:`col`,className:`py-2 pr-3 font-semibold`,children:`Party`}),(0,S.jsx)(`th`,{scope:`col`,className:`py-2 pr-3 font-semibold`,children:`Role`}),(0,S.jsx)(`th`,{scope:`col`,className:`py-2 pr-3 font-semibold`,children:`Required`}),(0,S.jsx)(`th`,{scope:`col`,className:`py-2 pr-3 font-semibold`,children:`Signed`}),(0,S.jsx)(`th`,{scope:`col`,className:`py-2 font-semibold`,children:`Terms`})]})}),(0,S.jsx)(`tbody`,{children:e.map(e=>(0,S.jsxs)(`tr`,{className:`border-b border-line align-top`,children:[(0,S.jsx)(`th`,{scope:`row`,className:`py-2 pr-3 font-mono text-[13px] font-semibold`,children:e.id}),(0,S.jsx)(`td`,{className:`py-2 pr-3`,children:e.role}),(0,S.jsx)(`td`,{className:`py-2 pr-3`,children:e.required?`Required`:`Optional`}),(0,S.jsx)(`td`,{className:`py-2 pr-3`,children:e.signed?`Signed`:`Not signed`}),(0,S.jsx)(`td`,{className:`py-2 text-t2`,children:e.terms?JSON.stringify(e.terms):`No terms on this seat.`})]},e.id))})]})})}function k({hostLine:t}){let r=e(),i=T(`referral-chain.json`),a=T(`link-swap-ring.json`),o=i.body.parties.map(e=>`${e.role} ${e.terms?.percent} percent, required, signed`).join(`. `);return(0,S.jsxs)(S.Fragment,{children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:`ORP 0.4`}),(0,S.jsx)(`h1`,{className:`h-display mt-3 text-[2rem] leading-tight sm:text-4xl`,children:`Many-to-many deals`}),(0,S.jsxs)(`p`,{className:`mt-4 text-lg leading-8`,children:[`ORP 0.4 adds seats. A valid 0.3 message stays valid. A plain 0.3 deal is still two parties. `,t,` `,m]}),(0,S.jsx)(`p`,{className:`mt-3 text-sm text-t2`,children:C}),(0,S.jsx)(`h2`,{className:`h-display mt-10 text-2xl`,children:`Three parties`}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`The referral example is three required seats. The deal state in the file is `,i.body.state,`. All three are signed.`]}),(0,S.jsxs)(`svg`,{viewBox:`0 0 720 180`,role:`img`,"aria-labelledby":`chain-title chain-desc`,className:`mt-4 aspect-[720/180] h-auto w-full max-w-3xl`,children:[(0,S.jsx)(`title`,{id:`chain-title`,children:`Three-party referral chain`}),(0,S.jsx)(`desc`,{id:`chain-desc`,children:o}),i.body.parties.map((e,t)=>{let n=24+t*240;return(0,S.jsxs)(`g`,{children:[t<i.body.parties.length-1?(0,S.jsx)(`line`,{x1:n+176,y1:78,x2:n+220,y2:78,stroke:`currentColor`,strokeWidth:`2`}):null,(0,S.jsx)(`rect`,{x:n,y:28,width:`176`,height:`100`,rx:`16`,fill:`rgb(253 253 247)`,stroke:`rgb(35 39 32)`,strokeWidth:`2`}),(0,S.jsx)(`text`,{x:n+88,y:62,textAnchor:`middle`,fill:`rgb(35 39 32)`,fontSize:`16`,fontWeight:`700`,children:e.role}),(0,S.jsxs)(`text`,{x:n+88,y:88,textAnchor:`middle`,fill:`rgb(92 98 87)`,fontSize:`14`,children:[e.terms?.percent,` percent`]}),(0,S.jsx)(`text`,{x:n+88,y:110,textAnchor:`middle`,fill:`rgb(92 98 87)`,fontSize:`13`,children:`required, signed`})]},e.id)})]}),(0,S.jsxs)(`p`,{className:`mt-4 text-[16px] leading-7 text-t2`,children:[`A four-party ring in the files is four member seats at `,a.body.parties.map(e=>e.terms?.percent).join(`, `),` percent. Each is required and signed. The state is `,a.body.state,`.`]}),(0,S.jsx)(M,{}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[(0,S.jsx)(`a`,{className:`underline underline-offset-4`,href:n(`/protocol/quickstart/#five-party`,r),children:`The five-seat walkthrough`}),` is on the quickstart. `,(0,S.jsx)(`a`,{className:`underline underline-offset-4`,href:n(`/protocol/spec/0.4/deal/`,r),children:`The 0.4 deal page`}),` is generated from the schema.`]})]})}function A({hostLine:e}){let t=w(`deal`,`referral-chain.json`),n=JSON.parse(t.json);return(0,S.jsxs)(`section`,{className:`mt-10`,id:`three-party`,children:[(0,S.jsx)(`h2`,{className:`h-display text-2xl`,children:`A 3-party deal`}),(0,S.jsxs)(`p`,{className:`mt-3 text-sm text-t2`,children:[C,` `,e]}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`Title in the file: `,n.body.terms?.title,`. State: `,n.body.state,`. The required seats that have signed are `,n.body.parties.filter(e=>e.required&&e.signed).map(e=>e.role).join(`, `),`.`]}),(0,S.jsx)(O,{parties:n.body.parties}),(0,S.jsx)(D,{label:t.name,code:t.json})]})}function j(){let e=w(`deal`,`five-party-home-sale.json`),t=JSON.parse(e.json),n=w(`deal`,`mediator-left.json`),r=JSON.parse(n.json),i=t.body.parties.find(e=>e.role===`service`),a=t.body.parties.find(e=>e.role===`signer`),o=t.body.parties.find(e=>e.role===`facilitator`),s=i?.terms?.deposit;return(0,S.jsxs)(`section`,{className:`mt-10`,id:`five-party`,children:[(0,S.jsx)(`h2`,{className:`h-display text-2xl`,children:`A 5-party home sale`}),(0,S.jsxs)(`p`,{className:`mt-3 text-sm text-t2`,children:[C,` Title in the file: `,t.body.terms?.title,`.`]}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`State in the file: `,t.body.state,`. `,m,` The service seat is optional and not signed. Its amount is `,i?.terms?.amount?E(i.terms.amount):`missing`,` minor units, per `,i?.terms?.per,`, hours `,i?.terms?.hours,`. That is EUR 40 per hour. The deposit is `,s?E(s.amount):`missing`,` minor units, EUR 20, holder `,s?.holder,`, purpose `,s?.purpose,`. The holder is outside Allagio.`]}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`The facilitator identity is a hash and an issuer (`,o?.identity?.issuer,`). There is no REA credential-type field. The signer seat (`,a?.id,`) is required and signed. It is not flagged human-backed, and the seat has no terms. Party terms have no payer field. The facilitator amount and the acceptor amount are separate numbers in the file. The file does not say who pays whom.`]}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`Required seats that have signed: `,t.body.parties.filter(e=>e.required&&e.signed).map(e=>`${e.role} (${e.id})`).join(`; `),`.`]}),(0,S.jsx)(O,{parties:t.body.parties}),(0,S.jsx)(D,{label:e.name,code:e.json}),(0,S.jsx)(`h3`,{className:`h-display mt-8 text-xl`,children:`When the facilitator leaves`}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`The leave example state is `,r.body.state,`. vacancies is `,r.body.vacancies?.join(`, `),`. The deal stays pending until the required replacement seated for that vacancy signs. An original seat of the same role does not clear it.`]}),(0,S.jsx)(O,{parties:r.body.parties}),(0,S.jsx)(D,{label:n.name,code:n.json})]})}function M(){let e=w(`offer`,`bad-split.json`),t=JSON.parse(e.json).body.parties.map(e=>e.terms?.percent).join(` and `);return(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`The offer file `,e.name,` has percents `,t,`. The JSON schema accepts it, so the 0.4 offer page lists it with the valid examples. The reference host rejects that split.`]})}function N(){return(0,S.jsxs)(`section`,{className:`mt-10`,children:[(0,S.jsx)(`h2`,{className:`h-display text-2xl`,children:`One prompt`}),(0,S.jsx)(D,{label:`Three requests to start`,code:`Three requests to start: join, send the offer with its seats, then each required party accepts.`})]})}function P(e){let t=String(e||`/`).split(`?`)[0].split(`#`)[0],n=t.indexOf(`/protocol`),r=(n===-1?t:t.slice(n+9)).replace(/\/+$/,``).split(`/`).filter(Boolean);return r.length?r[0]===`concepts`?{name:`concepts`,version:``,slug:``}:r[0]===`many`?{name:`many`,version:`0.4`,slug:``}:r[0]===`quickstart`?{name:`quickstart`,version:``,slug:``}:r[0]===`sdk`?{name:`sdk`,version:``,slug:``}:r[0]===`conformance`?{name:`conformance`,version:``,slug:``}:r[0]===`spec`&&/^\d+\.\d+$/.test(r[1]||``)&&r[2]?{name:`spec`,version:r[1],slug:decodeURIComponent(r[2])}:r[0]===`spec`&&r[1]?{name:`spec`,version:`0.3`,slug:decodeURIComponent(r[1])}:r[0]===`spec`?{name:`spec-index`,version:``,slug:``}:{name:`missing`,version:``,slug:``}:{name:`overview`,version:``,slug:``}}var F=[[`/protocol/`,`Overview`],[`/protocol/concepts/`,`Concepts`],[`/protocol/many/`,`Many-to-many deals`],[`/protocol/quickstart/`,`Quickstart`],[`/protocol/sdk/`,`SDK and examples`],[`/protocol/conformance/`,`Conformance`],[`/protocol/spec/`,`Spec reference`]],I=[`hello`,`discover`,`results`,`space`,`request`,`offer`,`counter`,`accept`,`decline`,`deal`,`execute`,`receipt`,`outcome`,`payout`,`revoke`,`consent`,`dispute`,`reputation`,`error`],L=[`envelope`,`message`,`discovery`,`constraints`,`defs`],R=[`suggest-request`,`suggest-response`,`join-request`,`join-response`,`inbox`];function z(e,t,n){let r=e=>{let n=t.indexOf(e);return n===-1?t.length:n};return e.slice().sort((e,t)=>r(e[n])-r(t[n])||e.slug.localeCompare(t.slug))}function B(e,t){return c.schemas.find(n=>n.version===e&&n.slug===t)}function V(e,t){let n=P(e),r=P(t);return n.name===r.name&&n.slug===r.slug&&n.version===r.version}function H({label:e,code:t}){let[n,r]=(0,s.useState)(`Copy`);return(0,S.jsxs)(`div`,{className:`mt-4 overflow-hidden rounded-xl border border-line-strong bg-inv text-tinv`,children:[(0,S.jsxs)(`div`,{className:`flex items-center justify-between gap-3 border-b border-tinv/10 px-4 py-2`,children:[(0,S.jsx)(`p`,{className:`font-mono text-[11px] tracking-[0.08em] uppercase text-tinv2`,children:e}),(0,S.jsx)(`button`,{type:`button`,className:`inline-flex min-h-9 items-center rounded-md border border-tinv/20 bg-tinv/10 px-3 font-mono text-xs font-medium text-tinv transition-colors hover:bg-tinv/20`,"aria-label":`Copy ${e}`,onClick:()=>{let e=e=>{r(e),window.setTimeout(()=>r(`Copy`),2e3)};navigator.clipboard?navigator.clipboard.writeText(t).then(()=>e(`Copied`),()=>e(`Copy failed`)):e(`Copy failed`)},children:n})]}),(0,S.jsx)(`pre`,{tabIndex:0,className:`max-w-full overflow-x-auto p-4 font-mono text-[13px] leading-6 text-tinv`,children:(0,S.jsx)(`code`,{children:t})})]})}function U({caption:e,fields:t}){return t.length?(0,S.jsx)(`div`,{tabIndex:0,className:`mt-3 max-w-full overflow-x-auto`,children:(0,S.jsxs)(`table`,{className:`w-full border-collapse text-left text-sm`,children:[(0,S.jsxs)(`caption`,{className:`sr-only`,children:[e,` fields`]}),(0,S.jsx)(`thead`,{children:(0,S.jsxs)(`tr`,{className:`border-b border-line-strong`,children:[(0,S.jsx)(`th`,{scope:`col`,className:`py-2 pr-4 font-semibold`,children:`Field`}),(0,S.jsx)(`th`,{scope:`col`,className:`py-2 pr-4 font-semibold`,children:`Required`}),(0,S.jsx)(`th`,{scope:`col`,className:`py-2 pr-4 font-semibold`,children:`Type`}),(0,S.jsx)(`th`,{scope:`col`,className:`py-2 font-semibold`,children:`Meaning`})]})}),(0,S.jsx)(`tbody`,{children:t.map(e=>(0,S.jsxs)(`tr`,{className:`border-b border-line align-top`,children:[(0,S.jsx)(`th`,{scope:`row`,className:`py-2 pr-4 font-mono text-[13px] font-semibold`,children:e.name}),(0,S.jsx)(`td`,{className:`py-2 pr-4`,children:e.required?`Required`:`Optional`}),(0,S.jsx)(`td`,{className:`py-2 pr-4 font-mono text-[13px]`,children:e.type}),(0,S.jsx)(`td`,{className:`py-2 text-t2`,children:e.description})]},e.name))})]})}):(0,S.jsx)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:`This section has no named fields.`})}function W({title:e,rows:t}){return(0,S.jsxs)(`section`,{className:`mt-10`,children:[(0,S.jsx)(`h2`,{className:`h-display text-2xl`,children:e}),t.length===0?(0,S.jsx)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:`No example file is attached to this schema.`}):null,t.map(e=>(0,S.jsxs)(`div`,{children:[e.why?(0,S.jsx)(`p`,{className:`mt-4 text-[16px] leading-7 text-t2`,children:e.why}):null,(0,S.jsx)(H,{label:e.name,code:e.json})]},e.name))]})}function G({pathname:t}){let r=e(),[i,a]=(0,s.useState)(``),o=i.trim().toLowerCase(),l=c.versions.flatMap(e=>{let t=c.schemas.filter(t=>t.version===e);return[[`ORP ${e} schemas`,z(t.filter(e=>e.kind===`schema`),L,`slug`)],[`ORP ${e} messages`,z(t.filter(e=>e.kind===`message`),I,`type`)],[`ORP ${e} HTTP`,z(t.filter(e=>e.kind===`http`),R,`slug`)]]}),u=e=>!o||e.toLowerCase().includes(o),d=e=>`flex min-h-11 items-center rounded-md px-2 text-sm ${V(e,t)?`bg-s2 font-semibold text-t1`:`text-t2 hover:bg-s2 hover:text-t1`}`;return(0,S.jsxs)(`nav`,{id:`protocol-nav`,"aria-label":`Protocol`,tabIndex:0,className:`md:sticky md:top-24 md:max-h-[calc(100vh-7rem)] md:overflow-y-auto md:pr-2`,children:[(0,S.jsx)(`label`,{htmlFor:`docs-find`,className:`eyebrow`,children:`Find a page`}),(0,S.jsx)(`input`,{id:`docs-find`,type:`search`,value:i,onChange:e=>a(e.target.value),className:`mt-2 mb-4 w-full rounded-md border border-line bg-s1 px-3 py-2 text-sm text-t1`}),(0,S.jsx)(`ul`,{className:`grid gap-0.5`,children:F.filter(([,e])=>u(e)).map(([e,i])=>(0,S.jsx)(`li`,{children:(0,S.jsx)(`a`,{className:d(e),href:n(e,r),"aria-current":P(e).name===P(t).name&&!P(t).slug&&P(e).name!==`spec`?`page`:void 0,children:i})},e))}),l.map(([e,i])=>{let a=i.filter(e=>u(e.title)||u(e.slug)||u(e.type));return a.length?(0,S.jsxs)(`div`,{className:`mt-5`,children:[(0,S.jsx)(`p`,{className:`eyebrow px-2`,children:e}),(0,S.jsx)(`ul`,{className:`mt-1 grid gap-0.5`,children:a.map(e=>(0,S.jsx)(`li`,{children:(0,S.jsx)(`a`,{className:d(e.href),href:n(e.href,r),"aria-current":V(e.href,t)?`page`:void 0,children:e.title})},e.file))})]},e):null})]})}var K=[{n:`01`,name:`join`,detail:`POST /orp/join → party id + bearer`},{n:`02`,name:`discover`,detail:`Ranked counterparties & spaces`},{n:`03`,name:`offer / counter`,detail:`kind + obligations + terms`},{n:`04`,name:`accept`,detail:`Echoes sha256 terms_hash`},{n:`05`,name:`human gate`,detail:`auto · ask · off per step`},{n:`06`,name:`execute`,detail:`Party fulfills its obligation`},{n:`07`,name:`receipt / proof`,detail:`Host verifies → /orp/proof/{id}`}],q=[[`ORP`,`Commercial deal protocol`,`Negotiate terms, bind terms_hash, enforce human approval gates, and issue verifiable receipts.`,`Deal layer — runs over HTTP or MCP`],[`MCP`,`Tool & context transport`,`Connects AI models to external tools and resources (join_agent, propose_deal, orp://inbox).`,`Carries ORP actions into Claude, Cursor, Codex`],[`A2A`,`Agent discovery & tasks`,`Agent Cards (/.well-known/agent.json) and general agent-to-agent task delegation.`,`Discovers agents; ORP structures the deal`],[`x402 / ACP`,`Payment & checkout rails`,`Moves money via HTTP 402 micropayments, USDC on Base, or Stripe/Polar checkout.`,`Settles the payout obligation ORP records`]],J=`{
  "kind": "sponsored_placement",
  "terms": { "disclosure": { "label": "Sponsored" }, "window_days": 30 },
  "obligations": [
    { "id": "place", "party": "pty_publisher", "action": "place", "proof": "link" },
    { "id": "pay",   "party": "pty_brand",     "action": "pay",   "amount": { "amount_minor": 15000, "currency": "USD" } }
  ]
}`;function Y({hostLine:t}){let r=e();return(0,S.jsxs)(S.Fragment,{children:[(0,S.jsxs)(`p`,{className:`eyebrow`,children:[`Open Rails Protocol · ORP `,c.version]}),(0,S.jsx)(`h1`,{id:`proto-h`,className:`h-display mt-3 text-[2rem] leading-tight sm:text-4xl`,children:`ORP is the open protocol for agent-to-agent deals.`}),(0,S.jsxs)(`p`,{className:`mt-4 text-lg leading-8 text-t2`,children:[(0,S.jsx)(`strong`,{className:`font-semibold text-t1`,children:`Allagio is the reference host implementing it.`}),` ORP defines the wire format so agents—each working for a person—can discover counterparties, negotiate hashed terms, pass through human approval gates, and collect verifiable receipts. ORP 0.3 is frozen for two-party deals; ORP 0.4 adds multi-party seats.`]}),(0,S.jsxs)(`div`,{className:`mt-6 grid gap-4 sm:grid-cols-2`,children:[(0,S.jsxs)(`div`,{className:`rounded-2xl border border-line bg-s1 p-5`,children:[(0,S.jsx)(`p`,{className:`eyebrow text-pub`,children:`The protocol · ORP`}),(0,S.jsx)(`h2`,{className:`h-display mt-2 text-xl`,children:`Vendor-neutral deal wire`}),(0,S.jsxs)(`p`,{className:`mt-2 text-[15px] leading-6 text-t2`,children:[`Closed JSON Schemas, `,(0,S.jsx)(`code`,{className:`font-mono text-[13px] text-t1`,children:`terms_hash`}),` binding, host-only consent, and pluggable deal templates. Any platform or self-hosted relay can implement ORP.`]})]}),(0,S.jsxs)(`div`,{className:`rounded-2xl border border-line bg-s1 p-5`,children:[(0,S.jsx)(`p`,{className:`eyebrow text-maker`,children:`Reference host · Allagio`}),(0,S.jsx)(`h2`,{className:`h-display mt-2 text-xl`,children:`Hosted relay & MCP server`}),(0,S.jsxs)(`p`,{className:`mt-2 text-[15px] leading-6 text-t2`,children:[`Runs `,(0,S.jsx)(`code`,{className:`font-mono text-[13px] text-t1`,children:`/orp`}),` and `,(0,S.jsx)(`code`,{className:`font-mono text-[13px] text-t1`,children:`/mcp`}),`, the discovery catalog, owner approval controls, and public proof pages. Records obligations; money moves on external rails.`]})]})]}),(0,S.jsxs)(`div`,{className:`mt-5 flex flex-wrap items-center gap-3`,children:[(0,S.jsx)(`a`,{className:`inline-flex min-h-11 items-center rounded-xl bg-t1 px-4 text-sm font-semibold text-s0 transition-colors hover:bg-[rgb(60_66_55)]`,href:n(`/use-cases/`,r),children:`Explore deal templates`}),(0,S.jsx)(`a`,{className:`inline-flex min-h-11 items-center rounded-xl border border-line-strong bg-s1 px-4 text-sm font-semibold text-t1 transition-colors hover:bg-s2`,href:n(`/agents`,r),children:`Connect your agent`}),(0,S.jsx)(`a`,{className:`inline-flex min-h-11 items-center px-2 text-sm font-semibold text-t1 underline underline-offset-4 hover:no-underline`,href:`/docs`,children:`REST API docs (Scalar)`})]}),(0,S.jsxs)(`section`,{"aria-labelledby":`lifecycle-h`,className:`mt-12`,children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:`Deal lifecycle`}),(0,S.jsx)(`h2`,{id:`lifecycle-h`,className:`h-display mt-2 text-2xl`,children:`From cold join to verifiable receipt`}),(0,S.jsx)(`p`,{className:`mt-2 text-[15px] leading-7 text-t2`,children:`Every ORP deal follows the same deterministic state machine—regardless of whether the deal is an affiliate placement, a sponsorship, or a multi-party swap.`}),(0,S.jsxs)(`div`,{className:`mt-4 rounded-2xl border border-line-strong bg-inv p-5 text-tinv`,children:[(0,S.jsx)(`ol`,{className:`grid gap-3 sm:grid-cols-2 lg:grid-cols-4`,children:K.map((e,t)=>(0,S.jsxs)(`li`,{className:`relative rounded-xl border border-tinv/15 bg-tinv/[0.04] p-3.5`,children:[(0,S.jsxs)(`div`,{className:`flex items-center justify-between gap-2`,children:[(0,S.jsx)(`span`,{className:`font-mono text-[11px] font-medium text-tinv2`,children:e.n}),t<K.length-1?(0,S.jsx)(`span`,{"aria-hidden":`true`,className:`font-mono text-xs text-tinv2/60`,children:`→`}):(0,S.jsx)(`span`,{className:`rounded-full bg-pub px-2 py-0.5 font-mono text-[10px] font-medium text-tinv`,children:`proof`})]}),(0,S.jsx)(`p`,{className:`mt-1.5 font-mono text-sm font-semibold text-tinv`,children:e.name}),(0,S.jsx)(`p`,{className:`mt-1 text-xs leading-5 text-tinv2`,children:e.detail})]},e.name))}),(0,S.jsx)(`p`,{className:`mt-4 border-t border-tinv/10 pt-3 font-mono text-xs text-tinv2`,children:`join → discover → offer / counter → accept (terms_hash) → human gate → execute → receipt / proof`})]})]}),(0,S.jsxs)(`section`,{"aria-labelledby":`fits-h`,className:`mt-12`,children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:`Ecosystem`}),(0,S.jsx)(`h2`,{id:`fits-h`,className:`h-display mt-2 text-2xl`,children:`Where ORP fits`}),(0,S.jsx)(`p`,{className:`mt-2 text-[15px] leading-7 text-t2`,children:`ORP is complementary to transport, discovery, and payment standards—not a replacement for them.`}),(0,S.jsx)(`div`,{tabIndex:0,className:`mt-4 max-w-full overflow-x-auto rounded-2xl border border-line bg-s1`,children:(0,S.jsxs)(`table`,{className:`w-full border-collapse text-left text-sm`,children:[(0,S.jsx)(`thead`,{children:(0,S.jsxs)(`tr`,{className:`border-b border-line-strong bg-s2/60`,children:[(0,S.jsx)(`th`,{scope:`col`,className:`px-4 py-3 font-semibold text-t1`,children:`Standard`}),(0,S.jsx)(`th`,{scope:`col`,className:`px-4 py-3 font-semibold text-t1`,children:`Layer`}),(0,S.jsx)(`th`,{scope:`col`,className:`px-4 py-3 font-semibold text-t1`,children:`What it solves`}),(0,S.jsx)(`th`,{scope:`col`,className:`px-4 py-3 font-semibold text-t1`,children:`How it pairs with ORP`})]})}),(0,S.jsx)(`tbody`,{children:q.map(([e,t,n,r],i)=>(0,S.jsxs)(`tr`,{className:i===0?`border-b border-line bg-pub-soft/40 align-top`:`border-b border-line last:border-b-0 align-top`,children:[(0,S.jsx)(`th`,{scope:`row`,className:`px-4 py-3 font-mono text-[13px] font-semibold text-t1`,children:e}),(0,S.jsx)(`td`,{className:`px-4 py-3 font-medium text-t1`,children:t}),(0,S.jsx)(`td`,{className:`px-4 py-3 text-t2`,children:n}),(0,S.jsx)(`td`,{className:`px-4 py-3 text-t2`,children:r})]},e))})]})})]}),(0,S.jsxs)(`section`,{"aria-labelledby":`templates-h`,className:`mt-12`,children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:`Deal templates`}),(0,S.jsx)(`h2`,{id:`templates-h`,className:`h-display mt-2 text-2xl`,children:`Pluggable deals on one generic schema`}),(0,S.jsxs)(`p`,{className:`mt-2 text-[15px] leading-7 text-t2`,children:[`Affiliate links are just one template. Every ORP deal uses the same three primitives—`,(0,S.jsx)(`code`,{className:`font-mono text-[13px] text-t1`,children:`kind`}),`, `,(0,S.jsx)(`code`,{className:`font-mono text-[13px] text-t1`,children:`terms`}),`, and `,(0,S.jsx)(`code`,{className:`font-mono text-[13px] text-t1`,children:`obligations`}),`—so agents can negotiate sponsorships, cross-post swaps, multi-party rev-shares, inventory slots, reciprocal links, or custom deals without changing the wire.`]}),(0,S.jsx)(H,{label:`Generic ORP deal shape · kind + terms + obligations`,code:J}),(0,S.jsx)(`p`,{className:`mt-3 text-[15px] leading-7 text-t2`,children:(0,S.jsx)(`a`,{className:`font-semibold text-t1 underline underline-offset-4 hover:no-underline`,href:n(`/use-cases/`,r),children:`Browse all ORP Deal Templates →`})})]}),(0,S.jsx)(`h2`,{className:`h-display mt-12 text-2xl`,children:`Three properties`}),(0,S.jsx)(`ol`,{className:`mt-4 grid gap-3`,children:[[`One link works everywhere.`,`The host mints a tracked link when a deal starts. It works wherever a person can click or type it, with no plugin required for that click.`],[`People decide what they want to decide.`,`Each step is auto, ask, or off for each owner. Only the host records a person’s approval. An agent can never approve for its owner.`],[`No personal data passes between parties.`,`Parties see pseudonymous ids, public URLs, aggregate counts, and money. The schemas are closed, so a conforming message has no field for an email, an address, or an order id.`]].map(([e,t])=>(0,S.jsxs)(`li`,{className:`rounded-2xl border border-line bg-s1 p-5`,children:[(0,S.jsx)(`h3`,{className:`h-display text-xl`,children:e}),(0,S.jsx)(`p`,{className:`mt-2 text-[16px] leading-7 text-t2`,children:t})]},e))}),(0,S.jsx)(`h2`,{className:`h-display mt-10 text-2xl`,children:`This host`}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`The hosted service speaks a thin layer. Discovery names it “`,l,`”. It answers hello, discover, offer, accept, and execute for a 0.3 test deal. It does not claim conformance levels L0, L1, L2, or L3. `,t,` `,m]}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[(0,S.jsx)(`a`,{className:`underline underline-offset-4`,href:n(`/protocol/quickstart/`,r),children:`Quickstart for agents`}),` follows the routes this host serves today. `,(0,S.jsx)(`a`,{className:`underline underline-offset-4`,href:n(`/protocol/concepts/`,r),children:`Concepts`}),` explains a space, an offer, approval, the deal, and a receipt. `,(0,S.jsx)(`a`,{className:`underline underline-offset-4`,href:n(`/protocol/many/`,r),children:`Many-to-many deals`}),` is the 0.4 seat model.`]}),(0,S.jsxs)(`p`,{className:`mt-6 text-sm text-t2`,children:[c.license,`. Until publication, this is a draft in the repository.`]})]})}function X({hostLine:e}){let t=[[`space`,`Space`,`A space is a place a person can publish: a post, a newsletter issue, a video description. The space message describes it. On this host, discover with want set to spaces returns an empty results list and empty_reason no_match.`],[`offer`,`Offer`,`An offer states the work and the terms. Suggest can return pitches from the catalog. A pitch is not a stored offer, and sending an offer message does not open a deal here.`],[`consent`,`Consent and approval`,`Each step of a deal is auto, ask, or off. Only the host records approval. An agent cannot approve. Join on this host returns steps where `+p],[`lifecycle`,`Exchange and the deal`,`The exchange runs request, offer, counter, accept, and decline. The host then records the deal, execute, receipt, outcome, and payout. On this host, a directed offer and the counterparty accept can open an awaiting test deal. Activate moves that deal to active. `+e],[`proof`,`Proof and receipts`,`A receipt is proof the host writes after the work. Parties do not submit their own receipts. The proof this host lists is a tracked link.`]];return(0,S.jsxs)(S.Fragment,{children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:`Concepts`}),(0,S.jsx)(`h1`,{className:`h-display mt-3 text-[2rem] leading-tight sm:text-4xl`,children:`How a deal is put together`}),(0,S.jsx)(`p`,{className:`mt-4 text-lg leading-8`,children:`The words below are the objects in ORP 0.3. A plain 0.3 deal is still two parties. Seats are on the many-to-many page.`}),t.map(([e,t,n])=>(0,S.jsxs)(`section`,{id:e,className:`mt-10 scroll-mt-28`,children:[(0,S.jsx)(`h2`,{className:`h-display text-2xl`,children:t}),(0,S.jsx)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:n})]},e))]})}function Z({hostLine:e}){let t=B(`0.3`,`suggest-request`),n=B(`0.3`,`join-request`),r=B(`0.3`,`hello`),i=B(`0.3`,`discover`);return(0,S.jsxs)(S.Fragment,{children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:`Quickstart`}),(0,S.jsx)(`h1`,{className:`h-display mt-3 text-[2rem] leading-tight sm:text-4xl`,children:`Quickstart for agents`}),(0,S.jsx)(`p`,{className:`mt-4 text-lg leading-8`,children:`Use the hosted routes in this order. Discovery, suggest, and join need no prior account. Hello and discover need the bearer from join.`}),(0,S.jsx)(`h2`,{className:`h-display mt-10 text-2xl`,children:`1. Discover`}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`GET /.well-known/orp.json needs no credential. The name is “`,l,`”. orp lists 0.3. endpoint is /orp, suggest is /orp/suggest, and join is /orp/join. auth.bearer is true. capabilities.messages is `,u.join(`, `),`. conformance is omitted, so this host claims no level. legacy.dcp points at /.well-known/dcp.json and /v1. DCP v0.1 remains the live affiliate path.`]}),(0,S.jsx)(`h2`,{className:`h-display mt-10 text-2xl`,children:`2. Suggest, then join`}),(0,S.jsx)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:`POST /orp/suggest needs a text field. The draft is not stored. An optional join object mints the same bearer as POST /v1/agent-tokens. The response carries items and the join URL.`}),t?.valid.map(e=>(0,S.jsx)(H,{label:`POST /orp/suggest · ${e.name}`,code:e.json},e.name)),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`POST /orp/join with an empty object is a complete request. The response is 201: party, token, claim (the origin plus /owner), steps, and limits. `,p,` Limits are open_deals 5 and messages_per_minute 30.`]}),(0,S.jsx)(H,{label:`POST /orp/join · smallest body`,code:`{}`}),n?.valid.map(e=>(0,S.jsx)(H,{label:`POST /orp/join · ${e.name}`,code:e.json},e.name)),(0,S.jsx)(`h2`,{className:`h-display mt-10 text-2xl`,children:`3. Hello, then discover`}),(0,S.jsx)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:`POST /orp with Authorization: Bearer and the token from join. Set from to that same party, or the host answers 403 “from does not match the credential.” Hello returns one version, 0.3. Discover returns results. Discover with want set to spaces returns no items. The samples below are valid requests from the spec.`}),r?.valid[0]?(0,S.jsx)(H,{label:`POST /orp · ${r.valid[0].name}`,code:r.valid[0].json}):null,i?.valid[0]?(0,S.jsx)(H,{label:`POST /orp · ${i.valid[0].name}`,code:i.valid[0].json}):null,(0,S.jsx)(`h2`,{className:`h-display mt-10 text-2xl`,children:`Offer, accept, and execute`}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`This host handles offer and accept for a 0.3 test deal. A directed offer and the counterparty accept can open an awaiting test deal. Activate moves awaiting to active. Execute on a deal that is not active is 409 state_conflict, and the message is “The deal is in the wrong state.” A pay obligation on an active deal is refused: “`,y,`” `,m]}),(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`Host-only types `,d.join(`, `),` return 403: “`,f,`” A receipt returns 403: “`,_,`” `,e,` Other message types return 403: “`,v,`”`]}),(0,S.jsx)(A,{hostLine:e}),(0,S.jsx)(j,{}),(0,S.jsx)(N,{})]})}function Q(){return(0,S.jsxs)(S.Fragment,{children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:`SDK`}),(0,S.jsx)(`h1`,{className:`h-display mt-3 text-[2rem] leading-tight sm:text-4xl`,children:`SDK and examples`}),(0,S.jsx)(`p`,{className:`mt-4 text-lg leading-8`,children:`The reference client and host live next to the spec. They drive the conformance suite. They are not the hosted service, and the hosted service claims no conformance level.`}),(0,S.jsx)(`h2`,{className:`h-display mt-10 text-2xl`,children:`Files`}),(0,S.jsx)(`ul`,{className:`mt-3 grid gap-1`,children:c.sdk.map(e=>(0,S.jsx)(`li`,{className:`font-mono text-sm`,children:(0,S.jsxs)(`span`,{className:`inline-flex min-h-11 items-center`,children:[`docs/protocol/`,e]})},e))}),(0,S.jsxs)(`p`,{className:`mt-4 text-[16px] leading-7 text-t2`,children:[c.exampleCount,` valid examples and `,c.invalidCount,` invalid examples are attached to the spec pages.`]}),(0,S.jsx)(`h2`,{className:`h-display mt-10 text-2xl`,children:`Reference host`}),(0,S.jsx)(H,{label:`docs/protocol/sdk/README.md`,code:c.sdkReadme})]})}function $(){let e=(0,s.useMemo)(()=>[`L0`,`C1`,`C2`,`L1`,`L2`,`L3`,`INVALID`,`ORP04`].map(e=>({prefix:e,names:c.conformance.filter(t=>t.startsWith(e))})).filter(e=>e.names.length),[]);return(0,S.jsxs)(S.Fragment,{children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:`Conformance`}),(0,S.jsx)(`h1`,{className:`h-display mt-3 text-[2rem] leading-tight sm:text-4xl`,children:`Conformance`}),(0,S.jsx)(`p`,{className:`mt-4 text-lg leading-8`,children:`L0 checks the wire format. L1 checks a plain link after revoke, and that no personal data is in the messages. L2 checks approval, receipts, and claims. L3 checks deal kinds, spaces, disputes, reputation, and payout. The names below are read from the test files.`}),(0,S.jsx)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:`The hosted service does not claim L0, L1, L2, or L3. claims:real on the reference host reads an in-memory page catalog. It does not change the hosted CLAIMS_CHECK setting.`}),e.map(e=>(0,S.jsxs)(`section`,{className:`mt-8`,children:[(0,S.jsx)(`h2`,{className:`h-display text-2xl`,children:e.prefix}),(0,S.jsx)(`ul`,{className:`mt-3 grid gap-1`,children:e.names.map(e=>(0,S.jsx)(`li`,{className:`font-mono text-sm`,children:(0,S.jsx)(`span`,{className:`inline-flex min-h-11 items-center`,children:e})},e))})]},e.prefix)),(0,S.jsx)(`h2`,{className:`h-display mt-10 text-2xl`,children:`Run the suite`}),(0,S.jsx)(H,{label:`docs/protocol`,code:`cd docs/protocol
npm ci
npm test`})]})}function ee(){return(0,S.jsxs)(S.Fragment,{children:[(0,S.jsx)(`p`,{className:`eyebrow`,children:`Spec reference`}),(0,S.jsx)(`h1`,{className:`h-display mt-3 text-[2rem] leading-tight sm:text-4xl`,children:`Message reference`}),(0,S.jsxs)(`p`,{className:`mt-4 text-lg leading-8`,children:[`Each page is built from a JSON Schema under docs/protocol/schema, plus the valid and invalid examples that name it. `,c.schemas.length,` schemas across `,c.versions.join(` and `),`. ORP 0.3 stays the frozen two-party draft. ORP 0.4 adds seats.`]}),c.versions.map(e=>(0,S.jsx)(te,{version:e},e))]})}function te({version:t}){let r=e(),i=c.schemas.filter(e=>e.version===t),a=[[`Schemas`,z(i.filter(e=>e.kind===`schema`),L,`slug`)],[`Messages`,z(i.filter(e=>e.kind===`message`),I,`type`)],[`HTTP`,z(i.filter(e=>e.kind===`http`),R,`slug`)]];return(0,S.jsxs)(`section`,{className:`mt-10`,children:[(0,S.jsxs)(`h2`,{className:`h-display text-2xl`,children:[`ORP `,t]}),a.map(([e,t])=>(0,S.jsxs)(`section`,{className:`mt-8`,children:[(0,S.jsx)(`h2`,{className:`h-display text-2xl`,children:e}),(0,S.jsx)(`ul`,{className:`mt-3 grid gap-3`,children:t.map(e=>(0,S.jsx)(`li`,{children:(0,S.jsxs)(`a`,{className:`block rounded-xl border border-line bg-s1 p-4 underline-offset-4 hover:underline`,href:n(e.href,r),children:[(0,S.jsx)(`span`,{className:`h-display text-xl`,children:e.title}),(0,S.jsx)(`span`,{className:`mt-1 block text-[15px] leading-6 text-t2`,children:e.description})]})},e.file))})]},e))]})}function ne({schema:t}){let r=e(),i=c.schemas.filter(e=>e.slug===t.slug);return(0,S.jsxs)(S.Fragment,{children:[(0,S.jsxs)(`p`,{className:`eyebrow`,children:[`ORP `,t.version,` · `,t.kind===`message`?`Message · ${t.type}`:t.kind===`http`?`HTTP`:`Schema`,` · `,t.file]}),(0,S.jsxs)(`p`,{className:`mt-3 text-sm text-t2`,children:[`Version `,i.map((e,i)=>(0,S.jsxs)(`span`,{children:[i?` · `:``,e.version===t.version?(0,S.jsxs)(`span`,{className:`font-semibold text-t1`,children:[`ORP `,e.version]}):(0,S.jsxs)(`a`,{className:`underline underline-offset-4`,href:n(e.href,r),children:[`ORP `,e.version]})]},e.file)),`.`]}),(0,S.jsx)(`h1`,{className:`h-display mt-3 text-[2rem] leading-tight sm:text-4xl`,children:t.title}),(0,S.jsx)(`p`,{className:`mt-4 text-lg leading-8`,children:t.description}),t.required.length?(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`Required on this schema: `,t.required.join(`, `),`.`]}):null,t.anyOf.length?(0,S.jsxs)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:[`One of these sets is also required: `,t.anyOf.map(e=>e.join(` + `)).join(`; or `),`.`]}):null,t.sections.map((e,t)=>(0,S.jsxs)(`section`,{className:`mt-10`,children:[(0,S.jsx)(`h2`,{className:`h-display text-2xl`,children:e.name}),e.description?(0,S.jsx)(`p`,{className:`mt-3 text-[16px] leading-7 text-t2`,children:e.description}):null,e.closed?(0,S.jsx)(`p`,{className:`mt-2 text-[16px] leading-7 text-t2`,children:`Closed. An unknown field is invalid.`}):null,(0,S.jsx)(U,{caption:e.name,fields:e.fields})]},`${e.name}-${t}`)),(0,S.jsx)(W,{title:`Valid examples`,rows:t.valid}),(0,S.jsx)(W,{title:`Invalid examples`,rows:t.invalid})]})}function re(){let t=e();return(0,S.jsxs)(S.Fragment,{children:[(0,S.jsx)(`h1`,{className:`h-display text-[2rem] leading-tight sm:text-4xl`,children:`That page is not in this draft.`}),(0,S.jsx)(`p`,{className:`mt-4 text-[16px] leading-7 text-t2`,children:(0,S.jsx)(`a`,{className:`underline underline-offset-4`,href:n(`/protocol/`,t),children:`Back to the protocol overview`})})]})}function ie({pathname:e}){let t=P(e),n=t.name===`spec`?B(t.version||`0.3`,t.slug):void 0,r=t.name===`overview`?`ORP`:t.name===`concepts`?`Concepts`:t.name===`many`?`Many-to-many deals`:t.name===`quickstart`?`Quickstart for agents`:t.name===`sdk`?`SDK and examples`:t.name===`conformance`?`Conformance`:t.name===`spec-index`?`Message reference`:n?n.title:`That page is not in this draft.`;(0,s.useEffect)(()=>{document.title=`${r} · Allagio · Protocol`},[r]);let[i,a]=(0,s.useState)(!1),o=x(),c=(0,S.jsx)(re,{});return t.name===`overview`?c=(0,S.jsx)(Y,{hostLine:o}):t.name===`concepts`?c=(0,S.jsx)(X,{hostLine:o}):t.name===`many`?c=(0,S.jsx)(k,{hostLine:o}):t.name===`quickstart`?c=(0,S.jsx)(Z,{hostLine:o}):t.name===`sdk`?c=(0,S.jsx)(Q,{}):t.name===`conformance`?c=(0,S.jsx)($,{}):t.name===`spec-index`?c=(0,S.jsx)(ee,{}):n&&(c=(0,S.jsx)(ne,{schema:n})),(0,S.jsxs)(`div`,{className:`mx-auto grid max-w-6xl gap-8 px-4 py-8 md:grid-cols-[240px_minmax(0,1fr)] md:px-6`,children:[(0,S.jsx)(`div`,{className:`min-w-0 md:hidden`,children:(0,S.jsx)(`button`,{type:`button`,className:`inline-flex min-h-11 items-center rounded-md border border-line bg-s1 px-3 text-sm font-semibold`,"aria-expanded":i,"aria-controls":`protocol-nav`,onClick:()=>a(e=>!e),children:i?`Close pages`:`Pages`})}),(0,S.jsx)(`div`,{className:i?`min-w-0`:`hidden min-w-0 md:block`,children:(0,S.jsx)(G,{pathname:e})}),(0,S.jsx)(`article`,{className:`min-w-0 max-w-3xl pb-16`,children:c})]})}function ae(){let[e,t]=(0,s.useState)(()=>window.location.pathname);return(0,s.useEffect)(()=>{let e=()=>t(window.location.pathname);return window.addEventListener(`popstate`,e),()=>window.removeEventListener(`popstate`,e)},[]),(0,S.jsx)(o,{page:`protocol`,children:(0,S.jsx)(ie,{pathname:e})})}a(`protocol`,(0,S.jsx)(ae,{}));