/* Generated from schema/0.3 by scripts/gen-types.ts. Do not edit; run npm run gen. */

/**
 * Protocol version, MAJOR.MINOR.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Version".
 */
export type Version = string;
/**
 * Pseudonymous party id: orp:<host>/<opaque>. The opaque part is random and never encodes a name, email, handle or domain of the owner.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "PartyId".
 */
export type PartyId = string;
/**
 * Sender-chosen id, unique per sender. Doubles as the idempotency key.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "MessageId".
 */
export type MessageId = string;
/**
 * Host-scoped id for deals, spaces, offers, escalations, consents and disputes.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "LocalId".
 */
export type LocalId = string;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ObligationId".
 */
export type ObligationId = string;
/**
 * sha256- followed by the unpadded base64url SHA-256 digest of the RFC 8785 (JCS) canonical JSON.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Hash".
 */
export type Hash = string;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Timestamp".
 */
export type Timestamp = string;
/**
 * ISO 8601 duration, for example P7D or PT12H.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Duration".
 */
export type Duration = string;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Url".
 */
export type Url = string;
/**
 * Reverse-DNS extension name, for example com.example.boost.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ExtName".
 */
export type ExtName = string;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Text".
 */
export type Text = string;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "MessageType".
 */
export type MessageType =
  | (
      | 'hello'
      | 'discover'
      | 'results'
      | 'ask'
      | 'offer'
      | 'counter'
      | 'accept'
      | 'decline'
      | 'deal'
      | 'execute'
      | 'receipt'
      | 'outcome'
      | 'revoke'
      | 'consent'
      | 'dispute'
      | 'reputation'
      | 'error'
    )
  | ExtName;
/**
 * Core kinds are family.kind. Extension kinds use reverse-DNS with at least three labels.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "SpaceKind".
 */
export type SpaceKind =
  | (
      | 'web.article'
      | 'web.page'
      | 'newsletter.issue'
      | 'newsletter.dedicated'
      | 'social.post'
      | 'social.reply'
      | 'social.repost'
      | 'social.profile'
      | 'video.description'
      | 'video.segment'
      | 'video.comment'
      | 'audio.segment'
      | 'audio.notes'
      | 'community.post'
      | 'community.comment'
      | 'community.message'
      | 'app.slot'
      | 'data.feed'
    )
  | string;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Platform".
 */
export type Platform =
  | (
      | 'web'
      | 'newsletter'
      | 'x'
      | 'youtube'
      | 'instagram'
      | 'tiktok'
      | 'facebook'
      | 'linkedin'
      | 'reddit'
      | 'discord'
      | 'telegram'
      | 'podcast'
      | 'app'
      | 'other'
    )
  | ExtName;
/**
 * A hint for discovery. The obligations are what bind.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DealKind".
 */
export type DealKind = 'affiliate' | 'sponsored' | 'swap' | 'repost' | 'comment' | 'traffic' | 'data' | 'partnership';
/**
 * link: human clicks on the deal's tracking link. proof_url: a public URL the verifier fetched. platform_check: the platform's public interface (oEmbed, public API, feed, archive). attestation: the counterparty confirms. witness: a host-controlled observer, such as a seed inbox. provider: a payment provider event.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ProofMethod".
 */
export type ProofMethod = ('link' | 'proof_url' | 'platform_check' | 'attestation' | 'witness' | 'provider') | ExtName;
/**
 * A step an owner can set to auto, ask or off.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Step".
 */
export type Step = 'list' | 'negotiate' | 'accept' | 'execute' | 'amend' | 'settle' | 'share' | 'revoke';
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "StepMode".
 */
export type StepMode = 'auto' | 'ask' | 'off';
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "EscalationReason".
 */
export type EscalationReason =
  | 'step_ask'
  | 'over_budget'
  | 'over_threshold'
  | 'new_counterparty'
  | 'low_reputation'
  | 'category_outside'
  | 'space_outside'
  | 'kind_outside'
  | 'duration_over'
  | 'mandate_expired';
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "SizeBucket".
 */
export type SizeBucket = '<1k' | '1k-10k' | '10k-100k' | '100k-1m' | '>1m';
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ShareBucket".
 */
export type ShareBucket = '<10%' | '10-25%' | '25-50%' | '>50%';
/**
 * Lowercase dotted category, for example software.devtools.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Category".
 */
export type Category = string;
/**
 * offerer is the party that opened the deal; acceptor is the other party. Roles never swap during a deal.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Role".
 */
export type Role = 'offerer' | 'acceptor';
/**
 * Proofs the counterparty accepts for this obligation; any one is enough.
 *
 * @minItems 1
 * @maxItems 6
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ProofList".
 */
export type ProofList = ProofMethod[];
/**
 * A timestamp, or a duration counted from the moment the deal becomes active.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Deadline".
 */
export type Deadline = Timestamp | Duration;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Obligation".
 */
export type Obligation =
  | PlaceObligation
  | RepostObligation
  | CommentObligation
  | PayObligation
  | CreditObligation
  | ShareObligation
  | ExtensionObligation;
/**
 * Version and capability negotiation. Optional for agents, required for hosts to answer. The reply carries exactly one version: the one both sides will use.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "HelloMessage".
 */
export type HelloMessage = Envelope & HelloMessage1;
/**
 * Search the host's index for offers (publisher side) or spaces (maker side). Read-only: it never commits anyone.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DiscoverMessage".
 */
export type DiscoverMessage = Envelope & DiscoverMessage1;
/**
 * The host's answer to discover. Every item says why it fits, with evidence links, and shows aggregate trust.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ResultsMessage".
 */
export type ResultsMessage = Envelope & ResultsMessage1;
/**
 * A directed request for terms, like a request for quote: 'do you have a slot?' or 'would you do this offer at a different rate?'. It commits nobody. The answer is an offer (with re set to this message), a decline, or nothing.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "AskMessage".
 */
export type AskMessage = Envelope & AskMessage1;
/**
 * Proposed terms. With to, it is directed: the host opens a deal and answers with a deal snapshot. Without to, it is a standing offer: the host lists it in discovery and answers with the offer as listed, id and terms_hash filled in. Anyone may accept a standing offer without negotiating.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "OfferMessage".
 */
export type OfferMessage = Envelope & OfferMessage1;
/**
 * Replacement terms for an open deal. Either party may counter while the deal is proposed. Each counter replaces the terms and clears every earlier acceptance, so nobody is bound to terms they did not see.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "CounterMessage".
 */
export type CounterMessage = Envelope & CounterMessage1;
/**
 * Agree to exact terms, named by terms_hash. Accept an open deal (deal set) or a standing offer (body.offer set, which opens a new deal). The host binds the sender only if the owner's constraints allow it; otherwise the deal waits for a person and the reply says so.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "AcceptMessage".
 */
export type AcceptMessage = Envelope & AcceptMessage1;
/**
 * Say no to an open deal before it is active. The reason is coarse on purpose: it must not reveal the decliner's budget or private rules.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DeclineMessage".
 */
export type DeclineMessage = Envelope & DeclineMessage1;
/**
 * proposed: terms on the table. awaiting: a party's person must decide. active: both bound, tracking live. completed: every obligation verified or waived. declined, expired, revoked: ended before or after activation. disputed: a dispute is open.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DealState".
 */
export type DealState =
  'proposed' | 'awaiting' | 'active' | 'completed' | 'declined' | 'expired' | 'revoked' | 'disputed';
/**
 * The host's snapshot of a deal. It is the reply to every message that changes a deal, and it lands in both parties' inboxes. Each party gets its own copy: escalation ids, reasons and approval URLs appear only in the copy for the party that is waiting.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DealMessage".
 */
export type DealMessage = Envelope & DealMessage1;
/**
 * The obligor says an obligation is done. A claim is not proof: it moves the obligation to claimed, and only a receipt moves it to verified. For click-tracked placements the claim is optional, since human clicks on the tracking link are the proof.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ExecuteMessage".
 */
export type ExecuteMessage = Envelope & ExecuteMessage1;
/**
 * A verifier's finding about one obligation. The host sends receipts for link, proof_url, platform_check, witness and provider checks. The counterparty sends a receipt with method attestation to confirm (or reject) work it saw itself. Only receipts count toward reputation; execute claims never do.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ReceiptMessage".
 */
export type ReceiptMessage = Envelope & ReceiptMessage1;
/**
 * Aggregate results for a period, sent by the host to both parties. Counts and money only. Rows are closed objects: there is no field for an address, user agent, referrer, email or order id.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "OutcomeMessage".
 */
export type OutcomeMessage = Envelope & OutcomeMessage1;
/**
 * Stop a deal, one obligation, or a standing offer. Future obligations stop. Tracking links never break: they turn plain, still redirecting to the destination but no longer counting. Work already verified stays verified.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "RevokeMessage".
 */
export type RevokeMessage = Envelope & RevokeMessage1;
/**
 * The host's record that a person decided an escalation. Only a host sends it, never an agent. It names the exact terms_hash the person saw, how the host knows it was the owner (assurance), and when. The counterparty's copy omits reasons. A signed consent (envelope sig) is the verifiable human-consent record.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ConsentMessage".
 */
export type ConsentMessage = Envelope & ConsentMessage1;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Remedy".
 */
export type Remedy = 'redo' | 'pay' | 'refund' | 'void' | 'none';
/**
 * Open, answer, resolve or withdraw a dispute about one obligation. The host (or a resolver it names) resolves. Evidence is public URLs and hashes, never personal data.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DisputeMessage".
 */
export type DisputeMessage = Envelope & DisputeMessage1;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ReputationBody".
 */
export type ReputationBody = ReputationQuery | ReputationAttestation;
/**
 * Ask the host for a party's reputation (body has only subject), or the host's signed answer (an attestation). Scores come only from verified receipts and resolved disputes. Components are counts; they never list counterparties.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ReputationMessage".
 */
export type ReputationMessage = Envelope & ReputationMessage1;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ErrorCode".
 */
export type ErrorCode =
  | (
      | 'invalid_message'
      | 'unsupported_version'
      | 'unsupported_type'
      | 'unsupported_extension'
      | 'unauthorized'
      | 'forbidden'
      | 'not_found'
      | 'state_conflict'
      | 'terms_mismatch'
      | 'step_off'
      | 'brand_rule'
      | 'replay'
      | 'stale'
      | 'bad_signature'
      | 'chain_broken'
      | 'rate_limited'
      | 'internal'
    )
  | ExtName;
/**
 * A refusal. An error never changes state. Escalation is not an error: a step that needs a person returns a deal snapshot in state awaiting.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ErrorMessage".
 */
export type ErrorMessage = Envelope & ErrorMessage1;
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "UrlOrPath".
 */
export type UrlOrPath = Url;

/**
 * Integer amount in the currency's minor unit (cents for EUR and USD).
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Money".
 */
export interface Money {
  amount_minor: number;
  currency: string;
}
/**
 * The label readers see next to the placement, in the reader's language.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Disclosure".
 */
export interface Disclosure {
  label: string;
  lang?: string;
}
/**
 * Aggregate audience description. Buckets only. Any segment smaller than k_min people is omitted.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Audience".
 */
export interface Audience {
  size?: SizeBucket;
  /**
   * @maxItems 5
   */
  regions?: {
    code: string;
    share: ShareBucket;
  }[];
  k_min?: number;
  source?: 'self_report' | 'platform' | 'host_measured';
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Verification".
 */
export interface Verification {
  method: 'domain' | 'dns' | 'platform_oauth' | 'bio_token' | 'challenge_post';
  at: Timestamp;
  by?: PartyId;
}
/**
 * A place an owner controls where content can appear. The locator may be withheld until a deal is active.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Space".
 */
export interface Space {
  id: LocalId;
  kind: SpaceKind;
  platform: Platform;
  title?: string;
  locator?: Url;
  /**
   * @maxItems 10
   */
  topics?: Category[];
  /**
   * @maxItems 5
   */
  languages?: string[];
  audience?: Audience;
  accepts?: DealKind[];
  disclosure?: Disclosure;
  /**
   * @maxItems 5
   */
  verified?: Verification[];
  rules?: {
    rel?: 'sponsored' | 'nofollow' | 'none';
    max_per_week?: number;
    /**
     * @maxItems 20
     */
    excluded_categories?: Category[];
  };
  adapter?: string;
}
/**
 * Either one named space, or any space of these kinds owned by the obligor.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "SpaceRef".
 */
export interface SpaceRef {
  id?: LocalId;
  /**
   * @minItems 1
   * @maxItems 10
   */
  kinds?: SpaceKind[];
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Content".
 */
export interface Content {
  /**
   * Must carry the deal's tracking link.
   */
  link?: boolean;
  /**
   * Must carry the deal's tracking code.
   */
  code?: boolean;
  brief?: string;
  /**
   * Exact text, when the counterparty fixes it.
   */
  copy?: string;
  max_chars?: number;
  /**
   * @maxItems 10
   */
  must_include?: string[];
  /**
   * @maxItems 20
   */
  must_not_include?: string[];
}
/**
 * Put content, usually the tracking link or code, somewhere. Without space, any surface the obligor controls counts: the tracking link works everywhere.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "PlaceObligation".
 */
export interface PlaceObligation {
  id: ObligationId;
  by: Role;
  action: 'place';
  space?: SpaceRef;
  content?: Content;
  destination?: Url;
  count?: number;
  due?: Deadline;
  keep?: Duration;
  proof: ProofList;
}
/**
 * Repost or share a public item. No click is needed; proof is a receipt.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "RepostObligation".
 */
export interface RepostObligation {
  id: ObligationId;
  by: Role;
  action: 'repost';
  target: Url;
  space?: SpaceRef;
  quote?: boolean;
  due?: Deadline;
  keep?: Duration;
  proof: ProofList;
}
/**
 * Comment on a public item, or let the counterparty use a comment space.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "CommentObligation".
 */
export interface CommentObligation {
  id: ObligationId;
  by: Role;
  action: 'comment';
  target: Url;
  content?: Content;
  due?: Deadline;
  keep?: Duration;
  proof: ProofList;
}
/**
 * Pay money: a fixed amount, per click, per conversion (fixed or rate), or per placement. Settlement happens on a payment rail, never inside ORP.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "PayObligation".
 */
export interface PayObligation {
  id: ObligationId;
  by: Role;
  action: 'pay';
  per: 'deal' | 'placement' | 'click' | 'conversion';
  amount?: Money;
  /**
   * Share of the sale in basis points, for per conversion.
   */
  rate_bps?: number;
  cap?: Money;
  window?: Duration;
  after?: ObligationId;
  due?: Deadline;
  /**
   * stripe, lemonsqueezy, polar, manual, or an extension name.
   */
  rail?: string;
  proof?: ProofList;
}
/**
 * Give non-cash credits, for traffic exchange or reposts. Credits are host-scoped and never convert to money inside ORP.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "CreditObligation".
 */
export interface CreditObligation {
  id: ObligationId;
  by: Role;
  action: 'credit';
  credits: number;
  after?: ObligationId;
}
/**
 * Share an aggregate, non-personal dataset. Personal data never crosses between agents.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ShareObligation".
 */
export interface ShareObligation {
  id: ObligationId;
  by: Role;
  action: 'share';
  data: {
    class: 'aggregate';
    /**
     * @minItems 1
     * @maxItems 20
     */
    fields: string[];
    k_min: number;
    purpose: string;
    retention: Duration;
    location?: Url;
  };
  due?: Deadline;
  proof: ProofList;
}
/**
 * An obligation defined by an extension. Hosts that do not know the action must refuse the terms.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ExtensionObligation".
 */
export interface ExtensionObligation {
  id: ObligationId;
  by: Role;
  action: string;
  proof?: ProofList;
}
/**
 * What each side will do. terms_hash is the JCS SHA-256 of this object.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Terms".
 */
export interface Terms {
  title: string;
  kind?: DealKind;
  /**
   * @minItems 1
   * @maxItems 10
   */
  obligations: Obligation[];
  /**
   * @maxItems 5
   */
  categories?: Category[];
  disclosure?: Disclosure;
  starts?: Timestamp;
  ends?: Timestamp;
}
/**
 * The link and optional code the host mints for one obligation when a deal becomes active. They work wherever a person can click or type them: email, social posts, video descriptions, podcast notes, a blog. No adapter or platform integration is needed. After revoke or end, status is plain: the link still redirects but no longer counts.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Tracking".
 */
export interface Tracking {
  obligation: ObligationId;
  link: Url;
  code?: string;
  destination: Url;
  status: 'live' | 'plain';
  expires?: Timestamp;
}
/**
 * Aggregate click counts by class. Only human clicks count toward pay or reputation. Never per-click rows, addresses, user agents or referrers.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ClickCounts".
 */
export interface ClickCounts {
  human: number;
  bot?: number;
  preview?: number;
  test?: number;
}
/**
 * Why a party is bound. mandate: the owner's standing constraints allowed it. consent: a person approved this exact terms_hash.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Authority".
 */
export interface Authority {
  type: 'mandate' | 'consent';
  ref?: LocalId;
}
/**
 * Who a deal or a listing is waiting for. escalation, reasons and url appear only in the waiting party's own copy.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Waiting".
 */
export interface Waiting {
  party: PartyId;
  step?: Step;
  since: Timestamp;
  escalation?: LocalId;
  /**
   * @maxItems 10
   */
  reasons?: EscalationReason[];
  url?: Url;
}
/**
 * Why a result fits, with evidence links a human can open.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Fit".
 */
export interface Fit {
  score: number;
  /**
   * @minItems 1
   * @maxItems 5
   */
  reasons: string[];
  /**
   * @maxItems 5
   */
  evidence?: Url[];
}
/**
 * Aggregate trust signals. Never a list of the party's counterparties.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Trust".
 */
export interface Trust {
  reputation?: number | null;
  completed_deals?: number;
  verified?: ('domain' | 'dns' | 'platform_oauth' | 'bio_token' | 'challenge_post')[];
  /**
   * @maxItems 3
   */
  path?: string[];
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Capabilities".
 */
export interface Capabilities {
  messages?: MessageType[];
  spaces?: SpaceKind[];
  actions?: string[];
  proofs?: ProofMethod[];
  tracking?: {
    links?: boolean;
    codes?: boolean;
  };
  signatures?: {
    algs?: ('EdDSA' | 'ES256')[];
    required?: boolean;
  };
  inbox?: {
    poll?: boolean;
    webhook?: Url;
  };
  ext?: ExtName[];
}
/**
 * The ORP 0.3 envelope. Six fields are required. Trust fields (prev, sig) and extensions are optional.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Envelope".
 */
export interface Envelope {
  orp: Version;
  type: MessageType;
  id: MessageId;
  ts: Timestamp;
  from: PartyId;
  to?: PartyId;
  deal?: LocalId;
  re?: MessageId;
  body: object;
  /**
   * The software acting for the party. Never a person.
   */
  agent?: {
    name: string;
    version?: string;
  };
  prev?: Hash;
  /**
   * Detached compact JWS (RFC 7515 Appendix F) over the JCS form of the envelope without sig.
   */
  sig?: string;
  ext?: {
    [k: string]: unknown | undefined;
  };
  /**
   * Extensions the receiver must understand, or refuse the message with unsupported_extension.
   */
  critical?: ExtName[];
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "HelloBody".
 */
export interface HelloBody {
  /**
   * @minItems 1
   * @maxItems 8
   */
  versions: Version[];
  capabilities?: Capabilities;
  party?: PartyId;
  limits?: {
    messages_per_minute?: number;
    max_body_bytes?: number;
  };
}
export interface HelloMessage1 {
  type: 'hello';
  body: HelloBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DiscoverBody".
 */
export interface DiscoverBody {
  want: 'offers' | 'spaces';
  query?: string;
  /**
   * The caller's own public content to match against. Never someone else's data.
   */
  context?: {
    text?: string;
    url?: Url;
    space?: LocalId;
  };
  filters?: {
    kinds?: DealKind[];
    space_kinds?: SpaceKind[];
    platforms?: Platform[];
    /**
     * @maxItems 10
     */
    categories?: Category[];
    /**
     * @maxItems 5
     */
    languages?: string[];
    audience_min?: SizeBucket;
    min_reputation?: number;
    pool?: string;
  };
  limit?: number;
  cursor?: string;
}
export interface DiscoverMessage1 {
  type: 'discover';
  body: DiscoverBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ResultsBody".
 */
export interface ResultsBody {
  /**
   * @maxItems 50
   */
  items: Result[];
  cursor?: string;
  empty_reason?: 'no_match' | 'below_threshold' | 'filtered';
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Result".
 */
export interface Result {
  offer?: OfferSummary;
  space?: Space;
  party: PartyId;
  fit: Fit;
  trust?: Trust;
  pool?: string;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "OfferSummary".
 */
export interface OfferSummary {
  id: LocalId;
  terms: Terms;
  terms_hash: Hash;
  expires?: Timestamp;
}
export interface ResultsMessage1 {
  type: 'results';
  body: ResultsBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "AskBody".
 */
export interface AskBody {
  offer?: LocalId;
  space?: LocalId;
  /**
   * @maxItems 8
   */
  kinds?: DealKind[];
  /**
   * Obligation actions the asker wants from the other side.
   *
   * @maxItems 6
   */
  actions?: ('place' | 'repost' | 'comment' | 'pay' | 'credit' | 'share')[];
  /**
   * Obligation actions the asker is willing to do in return.
   *
   * @maxItems 6
   */
  give?: ('place' | 'repost' | 'comment' | 'pay' | 'credit' | 'share')[];
  max_price?: Money;
  window?: {
    from?: Timestamp;
    to?: Timestamp;
  };
  note?: Text;
  expires?: Timestamp;
}
export interface AskMessage1 {
  type: 'ask';
  body: AskBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "OfferBody".
 */
export interface OfferBody {
  id?: LocalId;
  terms: Terms;
  terms_hash?: Hash;
  /**
   * Standing offers only: the most deals it may open.
   */
  limit?: number;
  waiting?: Waiting;
  note?: Text;
  expires?: Timestamp;
}
export interface OfferMessage1 {
  type: 'offer';
  body: OfferBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "CounterBody".
 */
export interface CounterBody {
  terms: Terms;
  terms_hash?: Hash;
  note?: Text;
  expires?: Timestamp;
}
export interface CounterMessage1 {
  type: 'counter';
  body: CounterBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "AcceptBody".
 */
export interface AcceptBody {
  terms_hash: Hash;
  offer?: LocalId;
  /**
   * Optional: which of the sender's spaces each obligation will use. Leave out to keep the choice open; the tracking link works on any of them.
   *
   * @maxItems 10
   */
  spaces?: {
    obligation: ObligationId;
    space: LocalId;
  }[];
  note?: Text;
}
export interface AcceptMessage1 {
  type: 'accept';
  body: AcceptBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DeclineBody".
 */
export interface DeclineBody {
  reason: 'not_relevant' | 'price' | 'timing' | 'brand' | 'capacity' | 'policy' | 'spam' | 'other';
  note?: Text;
}
export interface DeclineMessage1 {
  type: 'decline';
  body: DeclineBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DealBody".
 */
export interface DealBody {
  state: DealState;
  offerer: PartyId;
  acceptor: PartyId;
  offer?: LocalId;
  terms: Terms;
  terms_hash: Hash;
  round?: number;
  /**
   * Who is bound to terms_hash, and by what authority.
   *
   * @maxItems 2
   */
  bound?: {
    party: PartyId;
    authority: Authority;
    at: Timestamp;
  }[];
  waiting?: Waiting;
  /**
   * @maxItems 20
   */
  tracking?: Tracking[];
  /**
   * @maxItems 10
   */
  obligations?: {
    id: ObligationId;
    state: 'pending' | 'claimed' | 'verified' | 'failed' | 'revoked' | 'waived';
    due?: Timestamp;
    receipt?: MessageId;
  }[];
  /**
   * Spaces named in the deal, with locators revealed once the deal is active.
   *
   * @maxItems 10
   */
  spaces?: Space[];
  expires?: Timestamp;
  updated: Timestamp;
}
export interface DealMessage1 {
  type: 'deal';
  body: DealBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ExecuteBody".
 */
export interface ExecuteBody {
  obligation: ObligationId;
  proof_url?: Url;
  space?: LocalId;
  at?: Timestamp;
  note?: Text;
}
export interface ExecuteMessage1 {
  type: 'execute';
  body: ExecuteBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ReceiptBody".
 */
export interface ReceiptBody {
  obligation: ObligationId;
  result: 'verified' | 'failed' | 'partial';
  method: ProofMethod;
  at: Timestamp;
  url?: Url;
  /**
   * What the check saw at url.
   */
  found?: {
    link?: boolean;
    code?: boolean;
    disclosure?: boolean;
    target?: boolean;
    content?: boolean;
  };
  reason?:
    | 'not_found'
    | 'link_missing'
    | 'disclosure_missing'
    | 'content_mismatch'
    | 'removed'
    | 'unreachable'
    | 'too_early'
    | 'too_late'
    | 'not_public'
    | 'other';
  /**
   * For pay obligations: where the money moved. ORP never holds funds.
   */
  settlement?: {
    rail: string;
    ref?: string;
    amount: Money;
  };
  next_check?: Timestamp;
}
export interface ReceiptMessage1 {
  type: 'receipt';
  body: ReceiptBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "OutcomeBody".
 */
export interface OutcomeBody {
  period: {
    from: Timestamp;
    to: Timestamp;
  };
  /**
   * @maxItems 100
   */
  rows: OutcomeRow[];
  /**
   * No later outcome will change these numbers.
   */
  final?: boolean;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "OutcomeRow".
 */
export interface OutcomeRow {
  obligation: ObligationId;
  clicks: ClickCounts;
  conversions?: number;
  refunds?: number;
  revenue?: Money;
  earned?: Money;
  credits?: number;
}
export interface OutcomeMessage1 {
  type: 'outcome';
  body: OutcomeBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "RevokeBody".
 */
export interface RevokeBody {
  scope: 'deal' | 'obligation' | 'offer';
  obligation?: ObligationId;
  offer?: LocalId;
  reason: 'owner_request' | 'breach' | 'brand_safety' | 'expired' | 'other';
  note?: Text;
  effective?: Timestamp;
}
export interface RevokeMessage1 {
  type: 'revoke';
  body: RevokeBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ConsentBody".
 */
export interface ConsentBody {
  consent: LocalId;
  escalation?: LocalId;
  party: PartyId;
  decision: 'approved' | 'refused';
  step: Step;
  terms_hash: Hash;
  /**
   * @maxItems 10
   */
  reasons?: EscalationReason[];
  /**
   * session: the owner approved in a signed-in host session. passkey: the owner's passkey signed a challenge derived from terms_hash. email_link: a one-time link sent to the owner. client_prompt: an MCP client asked the person (the host cannot tell a person from an auto-approving client, so it is the weakest).
   */
  assurance: 'session' | 'passkey' | 'email_link' | 'client_prompt';
  at: Timestamp;
  expires?: Timestamp;
}
export interface ConsentMessage1 {
  type: 'consent';
  body: ConsentBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "DisputeBody".
 */
export interface DisputeBody {
  action: 'open' | 'respond' | 'resolve' | 'withdraw';
  dispute?: LocalId;
  obligation?: ObligationId;
  claim?:
    | 'not_done'
    | 'removed_early'
    | 'disclosure_missing'
    | 'content_mismatch'
    | 'not_paid'
    | 'invalid_traffic'
    | 'bad_receipt'
    | 'other';
  statement?: string;
  /**
   * @maxItems 10
   */
  evidence?: {
    url?: Url;
    hash?: Hash;
    note?: string;
  }[];
  remedy?: Remedy;
  resolution?: {
    outcome: 'upheld' | 'rejected' | 'split';
    remedy: Remedy;
    at_fault?: PartyId;
    note?: Text;
  };
}
export interface DisputeMessage1 {
  type: 'dispute';
  body: DisputeBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ReputationQuery".
 */
export interface ReputationQuery {
  subject: PartyId;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ReputationAttestation".
 */
export interface ReputationAttestation {
  subject: PartyId;
  algorithm: 'orp-rep-1' | ExtName;
  /**
   * null when there is not enough verified history yet.
   */
  score: number | null;
  components?: {
    verified?: number;
    failed?: number;
    disputes_lost?: number;
    counterparties?: number;
    evidence_weight?: number;
  };
  computed_at: Timestamp;
  valid_until?: Timestamp;
}
export interface ReputationMessage1 {
  type: 'reputation';
  body: ReputationBody;
}
/**
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "ErrorBody".
 */
export interface ErrorBody {
  code: ErrorCode;
  message: string;
  retryable?: boolean;
  /**
   * Seconds.
   */
  retry_after?: number;
  /**
   * @maxItems 8
   */
  supported?: Version[];
  step?: Step;
  /**
   * JSON Pointer to the offending field.
   */
  field?: string;
}
export interface ErrorMessage1 {
  type: 'error';
  body: ErrorBody;
}
/**
 * The document a host serves at /.well-known/orp.json. Serving it correctly is all Level 0 asks. URLs may be absolute or a path on the document's origin.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Discovery".
 */
export interface Discovery {
  /**
   * Supported protocol versions, newest first.
   *
   * @minItems 1
   * @maxItems 8
   */
  orp: Version[];
  host: PartyId;
  name?: string;
  description?: string;
  endpoint?: UrlOrPath;
  mcp?: {
    url: UrlOrPath;
    transport: 'streamable-http' | 'legacy-http-jsonrpc';
    auth?: ('bearer' | 'oauth')[];
    /**
     * @maxItems 64
     */
    tools?: string[];
    /**
     * True when the MCP server exposes the ORP binding tools (orp_send, orp_inbox, orp_deal).
     */
    binding?: boolean;
  };
  auth?: {
    /**
     * Agents authenticate with Authorization: Bearer <key>.
     */
    bearer?: boolean;
    signup?: UrlOrPath;
    oauth_metadata?: UrlOrPath;
    docs?: UrlOrPath;
  };
  tracking?: {
    links?: boolean;
    codes?: boolean;
    base?: UrlOrPath;
  };
  conformance?: {
    levels?: ('L0' | 'L1' | 'L2' | 'L3')[];
    profiles?: string[];
    report?: UrlOrPath;
  };
  capabilities?: Capabilities;
  /**
   * Public keys for host signatures (Level 3), as JWKs.
   *
   * @maxItems 8
   */
  keys?: {
    kty: 'OKP' | 'EC';
    crv?: string;
    x?: string;
    y?: string;
    kid: string;
    alg?: 'EdDSA' | 'ES256';
    use?: 'sig';
  }[];
  policies?: {
    terms?: UrlOrPath;
    privacy?: UrlOrPath;
    abuse?: UrlOrPath;
  };
  docs?: UrlOrPath;
  /**
   * Pre-ORP interfaces the host still serves. Informational; ORP clients ignore it.
   */
  legacy?: {
    [k: string]: {} | undefined;
  };
}
/**
 * An owner's private limits for its agent, kept by the host and never sent to a counterparty. Inside them the agent acts alone. Outside an allowance the host escalates to the owner. A deny rule or a step set to off refuses outright. Anything missing defaults to the safe side: every step that can bind the owner (list, negotiate, accept, execute, amend, settle, share) asks; only revoke is auto. With no budget, any spend asks.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "Constraints".
 */
export interface Constraints {
  steps: {
    list?: StepMode;
    negotiate?: StepMode;
    accept?: StepMode;
    execute?: StepMode;
    amend?: StepMode;
    settle?: StepMode;
    share?: StepMode;
    revoke?: StepMode;
  };
  /**
   * Caps on money the owner pays. A deal that would cross one escalates with over_budget. Pay per click or per conversion counts at its cap; with no cap it always escalates.
   */
  budget?: {
    per_deal?: Money;
    per_day?: Money;
    per_month?: Money;
    /**
     * Highest share of a sale the agent may promise alone. A share is paid out of the sale, so it does not count toward the cash caps. Unset: any share escalates with over_budget.
     */
    max_rate_bps?: number;
  };
  approve_over?: Money;
  /**
   * Deal kinds the agent may do alone. Others escalate (kind_outside).
   */
  kinds?: DealKind[];
  categories?: {
    /**
     * Outside these, escalate (category_outside).
     *
     * @maxItems 50
     */
    allow?: Category[];
    /**
     * Never: refuse with brand_rule.
     *
     * @maxItems 50
     */
    deny?: Category[];
  };
  spaces?: {
    /**
     * @maxItems 50
     */
    allow?: LocalId[];
    kinds?: SpaceKind[];
  };
  counterparties?: {
    /**
     * Below this, or with no score yet, escalate (low_reputation).
     */
    min_reputation?: number;
    new?: StepMode;
    /**
     * @maxItems 500
     */
    deny?: PartyId[];
  };
  /**
   * Hard content rules. Terms that break them are refused with brand_rule, never escalated.
   */
  brand?: {
    /**
     * @maxItems 10
     */
    must_include?: string[];
    /**
     * @maxItems 50
     */
    must_not_include?: string[];
    disclosure?: Disclosure;
  };
  max_duration?: Duration;
  expires?: Timestamp;
}
/**
 * The response to GET {endpoint}/inbox?after=<cursor>: messages for the caller's party, oldest first. Pass next back as after to continue.
 *
 * This interface was referenced by `OrpSchemas`'s JSON-Schema
 * via the `definition` "InboxPage".
 */
export interface InboxPage {
  /**
   * @maxItems 100
   */
  messages: Message[];
  next: string;
}

export type Message =
  | HelloMessage
  | DiscoverMessage
  | ResultsMessage
  | AskMessage
  | OfferMessage
  | CounterMessage
  | AcceptMessage
  | DeclineMessage
  | DealMessage
  | ExecuteMessage
  | ReceiptMessage
  | OutcomeMessage
  | RevokeMessage
  | ConsentMessage
  | DisputeMessage
  | ReputationMessage
  | ErrorMessage;
