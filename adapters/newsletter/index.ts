/**
 * Offer Rails Protocol (v0.3.0) — Optional Newsletter SpaceAdapter Stub
 *
 * Supports:
 * - `newsletter_slot` (Sponsored blurb in Substack, Beehiiv, ConvertKit, Ghost, or custom HTML email)
 * - `cross_promo_slot` (Newsletter-to-newsletter recommendation / traffic swap)
 *
 * CORE SPINE RULE:
 * Even in private email clients where open pixels are blocked or stripped,
 * the Universal Tracked Link (`/v1/go/:token`) works with zero ESP integration.
 * This adapter is an OPTIONAL extra for HTML rendering and web-archive verification.
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

export interface NewsletterAdapterConfig {
  provider?: "substack" | "beehiiv" | "kit" | "ghost" | "generic_html";
  apiKey?: string;
}

export class NewsletterSpaceAdapter implements SpaceAdapter {
  readonly adapter_id = "orp-adapter-newsletter/0.3.0";
  readonly platform: string;
  readonly supported_kinds: readonly SpaceKind[] = [
    "newsletter_slot",
    "cross_promo_slot",
  ];

  private readonly config: NewsletterAdapterConfig;

  constructor(config: NewsletterAdapterConfig = {}) {
    this.config = config;
    this.platform = config.provider ?? "newsletter";
  }

  async verifyOwnership(challenge: SpaceOwnershipChallenge): Promise<SpaceOwnershipResult> {
    const isUrl = /^https:\/\/[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}/.test(challenge.surface_url);
    return {
      verified: isUrl && challenge.expected_token.startsWith("orp_verify_"),
      method: challenge.method,
      checked_at: new Date().toISOString(),
      evidence_url: challenge.surface_url,
    };
  }

  async inspectSpace(surfaceUrl: string): Promise<Partial<Space>> {
    return {
      id: "space_newsletter_primary",
      kind: "newsletter_slot",
      platform: this.platform,
      surface_url: surfaceUrl,
      accepted_settlement_modes: ["cpc", "cpa", "flat_fee", "cpm", "space_swap", "traffic_swap"],
      disclosure_capabilities: ["plain_text_sponsored", "html_rel_sponsored"],
    };
  }

  renderPlacement(draft: PlacementDraft): string {
    const cta = draft.call_to_action ?? "Learn more";
    const rel = draft.disclosure.rel_attribute ?? "sponsored nofollow";
    return [
      `<aside class="orp-newsletter-slot" data-orp-token="${draft.tracking_token}">`,
      `  <p><small><strong>${draft.disclosure.text}</strong></small></p>`,
      `  <h4>${draft.headline}</h4>`,
      `  <p>${draft.body} <a href="${draft.tracking_url}" rel="${rel}">${cta}</a></p>`,
      `</aside>`,
    ].join("\n");
  }

  async executePlacement(draft: PlacementDraft): Promise<PlacementExecutionResult> {
    const rendered = this.renderPlacement(draft);

    if (!this.config.apiKey) {
      return {
        mode: "universal_link_ready",
        rendered_content: rendered,
        tracking_url: draft.tracking_url,
        tracking_token: draft.tracking_token,
      };
    }

    return {
      mode: "auto_published",
      proof_url: `https://newsletter.example.com/p/${draft.deal_id}`,
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
      const ok = Boolean(
        receipt.counterparty_attestation?.signature && receipt.disclosure_verified
      );
      return {
        valid: ok,
        verification_mode: "counterparty_attestation",
        disclosure_present: receipt.disclosure_verified,
        tracking_link_or_target_present: ok,
        checked_at: now,
      };
    }

    const html = fetchedHtmlOrText ?? "";
    const disclosurePresent =
      receipt.disclosure_verified &&
      (html === "" ||
        /sponsored/i.test(html) ||
        /partner/i.test(html) ||
        /rel=["'][^"']*sponsored/i.test(html));

    const linkPresent =
      html === ""
        ? Boolean(receipt.tracking_url || receipt.proof_url)
        : Boolean(
            (receipt.tracking_url && html.includes(receipt.tracking_url)) ||
              (receipt.tracking_token && html.includes(receipt.tracking_token)) ||
              (receipt.public_check?.expected_target_url &&
                html.includes(receipt.public_check.expected_target_url))
          );

    return {
      valid: disclosurePresent && linkPresent,
      verification_mode: receipt.verification_mode,
      disclosure_present: disclosurePresent,
      tracking_link_or_target_present: linkPresent,
      checked_at: now,
    };
  }

  async revokePlacement(
    _dealId: string,
    _proofUrl?: string
  ): Promise<{ revoked: boolean; mode: "api_removed" | "link_revoked_at_relay" }> {
    // Email already sent to inboxes cannot be un-sent, so the Universal Tracked Link
    // relay immediately stops redirecting/counting on revoke.
    if (!this.config.apiKey) {
      return { revoked: true, mode: "link_revoked_at_relay" };
    }
    return { revoked: true, mode: "api_removed" };
  }
}
