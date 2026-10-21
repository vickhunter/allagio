/**
 * Offer Rails Protocol (v0.3.0) — Optional Platform SpaceAdapter Interface
 *
 * CORE SPINE RULE:
 * The Universal Tracked Link (/v1/go/:token) and Tracking Token (orp_tok_...)
 * ARE the universal adapter. Every deal works on any surface (email, X,
 * Facebook, Instagram, YouTube, blog, podcast notes) with NO platform adapter.
 *
 * Platform adapters implementing `SpaceAdapter` are strictly OPTIONAL extras
 * that help agents auto-inspect inventory, format native disclosures,
 * auto-publish placements when platform credentials exist, or run automated
 * public checks. When no adapter is installed or no platform API key is present,
 * the protocol falls back to `UniversalLinkFallback` (tracked link + receipt).
 */

import type {
  DisclosureFormat,
  ReceiptPayload,
  ReceiptVerificationMode,
  Space,
  SpaceKind,
} from "../sdk/typescript/src/types.ts";

export interface SpaceOwnershipChallenge {
  space_id: string;
  surface_url: string;
  method: "dns_txt" | "well_known" | "profile_bio_token" | "oauth_scope" | "public_link_check";
  expected_token: string;
}

export interface SpaceOwnershipResult {
  verified: boolean;
  method: SpaceOwnershipChallenge["method"];
  checked_at: string;
  evidence_url?: string;
}

export interface PlacementDraft {
  deal_id: string;
  space_id: string;
  space_kind: SpaceKind;
  headline: string;
  body: string;
  call_to_action?: string;
  /**
   * The Universal Tracked Link (/v1/go/:token) works on every surface
   * regardless of whether a platform adapter is used.
   */
  tracking_url: string;
  tracking_token: string;
  disclosure: DisclosureFormat;
}

export interface PlacementExecutionResult {
  /**
   * Whether the adapter auto-posted via a platform API or prepared a manual/copy-paste
   * payload containing the universal tracked link.
   */
  mode: "auto_published" | "universal_link_ready";
  proof_url?: string;
  rendered_content: string;
  tracking_url: string;
  tracking_token: string;
  adapter_id?: string;
}

export interface ReceiptCheckResult {
  valid: boolean;
  verification_mode: ReceiptVerificationMode;
  disclosure_present: boolean;
  tracking_link_or_target_present: boolean;
  checked_at: string;
  reason?: string;
}

/**
 * Optional Platform Adapter interface (`SpaceAdapter`).
 * Implementations are optional helpers; they are never required for tracking.
 */
export interface SpaceAdapter {
  readonly adapter_id: string;
  readonly platform: string;
  readonly supported_kinds: readonly SpaceKind[];

  /**
   * Verify that the party controls the surface (optional helper).
   */
  verifyOwnership(challenge: SpaceOwnershipChallenge): Promise<SpaceOwnershipResult>;

  /**
   * Discover or inspect available Space inventory on the surface (optional helper).
   */
  inspectSpace(surfaceUrl: string): Promise<Partial<Space>>;

  /**
   * Render copy + FTC/platform disclosure + universal tracked link for the surface.
   */
  renderPlacement(draft: PlacementDraft): string;

  /**
   * Execute a placement. If platform API credentials are not configured, MUST
   * degrade gracefully to `universal_link_ready` so the human or browser agent
   * can post the universal tracked link directly.
   */
  executePlacement(draft: PlacementDraft): Promise<PlacementExecutionResult>;

  /**
   * Verify a placement receipt (`proof_url`, `public_check`, or `counterparty_attestation`).
   */
  verifyReceipt(
    receipt: ReceiptPayload,
    fetchedHtmlOrText?: string
  ): Promise<ReceiptCheckResult>;

  /**
   * Revoke a live placement (or mark the universal link revoked if no API is configured).
   */
  revokePlacement(dealId: string, proofUrl?: string): Promise<{ revoked: boolean; mode: "api_removed" | "link_revoked_at_relay" }>;
}
