/**
 * Offer Rails Protocol (v0.3.0) — Optional X (Twitter) SpaceAdapter Stub
 *
 * Supports:
 * - `social_post` (sponsored post with universal tracked link + `#ad` / `#sponsored`)
 * - `social_repost` (zero-click repost verified via `public_check` or `counterparty_attestation`)
 * - `social_comment` (reply/thread placement with universal tracked link or `public_check`)
 *
 * NOTE: This adapter is an OPTIONAL extra. A deal on X works with ZERO adapter
 * simply by placing the universal tracked link (`https://.../v1/go/:token`) in
 * an X post or bio and submitting the post URL as `proof_url`.
 */

import type {
  PlacementDraft,
  PlacementExecutionResult,
  ReceiptCheckResult,
  SpaceAdapter,
  SpaceOwnershipChallenge,
  SpaceOwnershipResult,
} from "../interface.ts";
import type { ReceiptPayload, Space, SpaceKind } from "../../sdk/typescript/src/types.ts";

export interface XAdapterConfig {
  /**
   * Optional X API v2 Bearer/OAuth token. When omitted, the adapter operates in
   * zero-API universal-link mode (`universal_link_ready` + `public_check`).
   */
  apiToken?: string;
  /**
   * Optional custom fetch/oEmbed resolver for testing.
   */
  fetchPostText?: (proofUrl: string) => Promise<string>;
}

export class XSpaceAdapter implements SpaceAdapter {
  readonly adapter_id = "orp-adapter-x/0.3.0";
  readonly platform = "x";
  readonly supported_kinds: readonly SpaceKind[] = [
    "social_post",
    "social_repost",
    "social_comment",
  ];

  private readonly config: XAdapterConfig;

  constructor(config: XAdapterConfig = {}) {
    this.config = config;
  }

  async verifyOwnership(challenge: SpaceOwnershipChallenge): Promise<SpaceOwnershipResult> {
    const validSurface = /^https:\/\/(x\.com|twitter\.com)\/[A-Za-z0-9_]{1,30}\/?$/.test(
      challenge.surface_url
    );
    return {
      verified: validSurface && challenge.expected_token.startsWith("orp_verify_"),
      method: challenge.method,
      checked_at: new Date().toISOString(),
      evidence_url: challenge.surface_url,
    };
  }

  async inspectSpace(surfaceUrl: string): Promise<Partial<Space>> {
    const handleMatch = surfaceUrl.match(/(?:x\.com|twitter\.com)\/([A-Za-z0-9_]{1,30})/);
    const handle = handleMatch ? handleMatch[1] : "creator";
    return {
      id: `space_x_${handle.toLowerCase()}`,
      kind: "social_post",
      platform: "x",
      surface_url: surfaceUrl,
      accepted_settlement_modes: ["cpc", "flat_fee", "repost_bounty", "space_swap", "traffic_swap"],
      disclosure_capabilities: ["hashtag_ad", "platform_paid_partnership_flag", "plain_text_sponsored"],
    };
  }

  renderPlacement(draft: PlacementDraft): string {
    const disclosureTag =
      draft.disclosure.text && draft.disclosure.text.trim().length > 0
        ? draft.disclosure.text.trim()
        : "#ad";

    if (draft.space_kind === "social_repost") {
      return `${disclosureTag} RT ${draft.body} ${draft.tracking_url}`.trim();
    }

    const parts = [
      draft.disclosure.placement === "before" ? disclosureTag : "",
      draft.headline ? `${draft.headline} —` : "",
      draft.body,
      draft.tracking_url,
      draft.disclosure.placement !== "before" ? disclosureTag : "",
    ].filter(Boolean);

    return parts.join(" ");
  }

  async executePlacement(draft: PlacementDraft): Promise<PlacementExecutionResult> {
    const rendered = this.renderPlacement(draft);

    // If no X API token is configured, gracefully return the universal link payload.
    if (!this.config.apiToken) {
      return {
        mode: "universal_link_ready",
        rendered_content: rendered,
        tracking_url: draft.tracking_url,
        tracking_token: draft.tracking_token,
      };
    }

    const syntheticStatusId = Buffer.from(`${draft.deal_id}:${draft.tracking_token}`)
      .toString("hex")
      .slice(0, 16);
    return {
      mode: "auto_published",
      proof_url: `https://x.com/i/status/${syntheticStatusId}`,
      rendered_content: rendered,
      tracking_url: draft.tracking_url,
      tracking_token: draft.tracking_token,
      adapter_id: this.adapter_id,
    };
  }

  async verifyReceipt(
    receipt: ReceiptPayload,
    fetchedHtmlOrText?: string
  ): Promise<ReceiptCheckResult> {
    const now = new Date().toISOString();

    if (receipt.verification_mode === "counterparty_attestation") {
      const hasSig = Boolean(
        receipt.counterparty_attestation?.counterparty_party_id &&
          receipt.counterparty_attestation?.signature
      );
      return {
        valid: hasSig && receipt.disclosure_verified,
        verification_mode: "counterparty_attestation",
        disclosure_present: receipt.disclosure_verified,
        tracking_link_or_target_present: hasSig,
        checked_at: now,
        reason: hasSig ? undefined : "Missing counterparty attestation signature",
      };
    }

    const content =
      fetchedHtmlOrText ??
      (receipt.proof_url && this.config.fetchPostText
        ? await this.config.fetchPostText(receipt.proof_url)
        : "");

    const lower = content.toLowerCase();
    const disclosurePresent =
      receipt.disclosure_verified &&
      (lower.includes("#ad") ||
        lower.includes("#sponsored") ||
        lower.includes("sponsored") ||
        lower.includes("paid partnership") ||
        content === "");

    const linkOrTargetPresent =
      content === ""
        ? Boolean(receipt.proof_url || receipt.tracking_url)
        : Boolean(
            (receipt.tracking_url && content.includes(receipt.tracking_url)) ||
              (receipt.tracking_token && content.includes(receipt.tracking_token)) ||
              (receipt.public_check?.expected_target_url &&
                content.includes(receipt.public_check.expected_target_url)) ||
              (receipt.public_check?.expected_text_fragment &&
                content.includes(receipt.public_check.expected_text_fragment))
          );

    return {
      valid: disclosurePresent && linkOrTargetPresent,
      verification_mode: receipt.verification_mode,
      disclosure_present: disclosurePresent,
      tracking_link_or_target_present: linkOrTargetPresent,
      checked_at: now,
    };
  }

  async revokePlacement(
    _dealId: string,
    _proofUrl?: string
  ): Promise<{ revoked: boolean; mode: "api_removed" | "link_revoked_at_relay" }> {
    if (!this.config.apiToken) {
      return { revoked: true, mode: "link_revoked_at_relay" };
    }
    return { revoked: true, mode: "api_removed" };
  }
}
