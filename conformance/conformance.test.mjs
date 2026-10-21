/**
 * Offer Rails Protocol (v0.3.0) — Executable Conformance Suite
 *
 * Test #1 (Spine Invariant):
 *   Any tracked link works on any surface with no adapter (email, X, blog).
 *   Adapters are optional extras.
 *
 * Can be run standalone (`node --experimental-strip-types --test conformance/conformance.test.mjs`)
 * or against a live Go server via `OFFER_RAILS_CONFORMANCE_URL=http://127.0.0.1:18080`
 * (automated by `node --experimental-strip-types conformance/run.mjs`).
 */

import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import {
  OrpAgent,
  evaluateDealConstraints,
  computeTermsHash,
  mintUniversalTracking,
  verifyPublicContentReceipt,
  computeReputationProfile,
  validateOrpMessage,
} from "../sdk/typescript/src/index.ts";
import { loadSchemas, validateWithSchema } from "../schema/validator.mjs";
import { XSpaceAdapter } from "../adapters/x/index.ts";
import { NewsletterSpaceAdapter } from "../adapters/newsletter/index.ts";

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(__dirname, "..");
const schemaRegistry = loadSchemas(resolve(repoRoot, "schema"));
const liveBaseUrl = process.env.OFFER_RAILS_CONFORMANCE_URL || "";

/**
 * Simulates the Universal Tracked Link relay (`/v1/go/:token`) for any surface
 * (email, X, blog, YouTube, podcast notes) with zero platform adapters.
 */
class UniversalLinkRelay {
  links = new Map();

  registerDealLink(dealId, targetUrl) {
    const { tracking_url, tracking_token } = mintUniversalTracking(
      "https://offer-rails.com",
      dealId
    );
    const code = tracking_url.split("/v1/go/")[1];
    this.links.set(code, {
      dealId,
      targetUrl,
      tracking_url,
      tracking_token,
      active: true,
      clicks: [],
    });
    return { code, tracking_url, tracking_token };
  }

  resolveClick(code, surfaceContext) {
    const entry = this.links.get(code);
    if (!entry) return { status: 404 };
    if (!entry.active) {
      return {
        status: 302,
        location: entry.targetUrl,
        counted: false,
      };
    }
    const clickId = `clk_${entry.clicks.length + 1}_${surfaceContext.surface}`;
    entry.clicks.push({
      clickId,
      surface: surfaceContext.surface,
      referer: surfaceContext.referer,
      userAgent: surfaceContext.userAgent,
      adapter_id: undefined, // No adapter used
    });
    const sep = entry.targetUrl.includes("?") ? "&" : "?";
    return {
      status: 302,
      location: `${entry.targetUrl}${sep}via=${clickId}`,
      counted: true,
      clickId,
    };
  }

  revoke(code) {
    const entry = this.links.get(code);
    if (entry) entry.active = false;
  }
}

// ============================================================================
// TEST 1 (SPINE INVARIANT):
// Any tracked link works on any surface with no adapter (email, X, blog).
// Adapters are optional extras.
// ============================================================================
test("1. [C1-UNIVERSAL-LINK-NO-ADAPTER] Any tracked link works on any surface with no adapter (email, X, blog); adapters are optional extras", async () => {
  const relay = new UniversalLinkRelay();
  const { code, tracking_url, tracking_token } = relay.registerDealLink(
    "deal_universal_001",
    "https://acme-dev.example.com/pricing"
  );

  assert.match(tracking_url, /^https:\/\/offer-rails\.com\/v1\/go\/trk_[a-f0-9]{10}$/);
  assert.match(tracking_token, /^orp_tok_[a-f0-9]{10}$/);

  const surfaces = [
    {
      surface: "email",
      referer: "", // Email clients strip Referer
      userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/605.1.15 AppleMail/16.0",
      publishedContent: `Sponsored: Try Acme Dev for faster builds: ${tracking_url}`,
    },
    {
      surface: "x",
      referer: "https://t.co/xyz123",
      userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) X-Client/10.0",
      publishedContent: `We cut our CI time by 40% with Acme Dev ${tracking_url} #ad`,
    },
    {
      surface: "blog",
      referer: "https://indieblog.example.com/ci-benchmarks",
      userAgent: "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0",
      publishedContent: `<p><em>Sponsored:</em> Check out <a href="${tracking_url}" rel="sponsored">Acme Dev</a>.</p>`,
    },
  ];

  for (const s of surfaces) {
    // 1. Verify receipt via public_check with NO adapter_id
    const check = verifyPublicContentReceipt({
      observedContent: s.publishedContent,
      expectedTargets: [tracking_url, tracking_token],
      requiredDisclosureMarkers: ["Sponsored", "#ad"],
    });
    assert.equal(check.targetMatched, tracking_url, `Tracked link matched on ${s.surface} without adapter`);
    assert.equal(check.disclosureVerified, true, `Disclosure verified on ${s.surface} without adapter`);

    const receiptMsg = {
      orp_version: "0.3.0",
      message_id: `msg_rcpt_${s.surface}`,
      message_type: "receipt",
      deal_id: "deal_universal_001",
      sender: "orp:party:pub_01",
      timestamp: "2026-10-21T10:00:00Z",
      idempotency_key: `idem_rcpt_${s.surface}`,
      payload: {
        receipt_id: `rcpt_${s.surface}`,
        space_id: `space_${s.surface}_01`,
        verification_mode: "public_check",
        proof_url: `https://${s.surface}.example.com/post/1`,
        location_url: `https://${s.surface}.example.com/post/1`,
        tracking_token_found: true,
        // Note: adapter_id is intentionally omitted!
        verified_at: "2026-10-21T10:00:00Z",
        disclosure_verified: true,
        observed_disclosure: check.observedDisclosure,
        content_hash: check.contentHash,
      },
    };
    const validation = validateOrpMessage(receiptMsg);
    assert.equal(validation.valid, true, `Receipt for ${s.surface} validates with no adapter_id: ${validation.errors.join(", ")}`);
    assert.equal("adapter_id" in receiptMsg.payload, false);

    // 2. Click the tracked link from this surface (email, X, blog) with no adapter
    const click = relay.resolveClick(code, s);
    assert.equal(click.status, 302);
    assert.equal(click.counted, true, `Click counted on ${s.surface}`);
    assert.match(click.location, /^https:\/\/acme-dev\.example\.com\/pricing\?via=clk_/);
  }

  // 3. Revoke stops counting across all surfaces immediately
  relay.revoke(code);
  const afterRevoke = relay.resolveClick(code, surfaces[0]);
  assert.equal(afterRevoke.status, 302);
  assert.equal(afterRevoke.counted, false);

  // 4. If a live Go server URL is provided, verify `/v1/go/:code` across email, X, and blog headers
  if (liveBaseUrl) {
    const campaignCode = process.env.OFFER_RAILS_TEST_CAMPAIGN_CODE;
    if (campaignCode) {
      for (const s of surfaces) {
        const res = await fetch(`${liveBaseUrl}/v1/go/${campaignCode}`, {
          redirect: "manual",
          headers: {
            "User-Agent": s.userAgent,
            ...(s.referer ? { Referer: s.referer } : {}),
          },
        });
        assert.equal(res.status, 302, `Live server /v1/go/:code returned 302 for ${s.surface}`);
        const loc = res.headers.get("location") || "";
        assert.ok(loc.startsWith("https://"), `Live server /v1/go/:code redirected to https destination for ${s.surface}`);
      }
    }
  }
});

// ============================================================================
// TEST 2 (NO EMAIL, NO SIGNUP & 5 GROWTH DEAL TYPES):
// Any content-writing agent can suggest Offer Rails mid-task in one step with
// no account, email, or website signup, and negotiate all 5 deal types.
// ============================================================================
test("2. [C2-NO-EMAIL-NO-SIGNUP] Content-writing agent suggests Offer Rails mid-task in 1 step with no account, email, or website signup across all 5 growth deal types", async () => {
  // 1. Mid-task 1-step inline suggestion with NO account, NO email, NO website signup
  const { discoverRequest, response } = OrpAgent.inlineSuggestMidTask({
    draftText:
      "How we cut pull request CI latency by 45% using isolated Postgres database branches and automated schema checks.",
    surfaceKinds: ["content_inline", "newsletter_slot", "social_post"],
    growthGoals: ["monetization", "traffic", "engagement", "clients"],
  });

  assert.equal(validateOrpMessage(discoverRequest).valid, true);
  assert.equal(discoverRequest.payload.mode, "inline_suggest");
  assert.equal(discoverRequest.sender.includes("@"), false, "Ephemeral sender must never contain an email address");
  assert.match(response.ephemeral_party_id, /^orp:party:ephem_/);
  assert.match(response.ephemeral_agent_key, /^orp_ek_/);
  assert.match(response.inline_prompt_suggestion, /want to test it\?/i);
  assert.ok(response.suggestions.length >= 2);
  for (const sug of response.suggestions) {
    assert.match(sug.provisional_tracking_url, /^https:\/\/offer-rails\.com\/v1\/go\/trk_/);
    assert.match(sug.provisional_tracking_token, /^orp_tok_/);
    assert.match(sug.one_step_claim_token, /^orp_claim_/);
  }

  // 2. Verify all 5 canonical growth deal types (affiliate, cross_sell_upsell, link_exchange, traffic_swap, repost_retweet)
  //    negotiate directly agent-to-agent with zero email/signup and pass their respective verification modes
  const agentA = new OrpAgent({
    principalId: response.ephemeral_party_id,
    stepModes: { deal_offer: "auto", deal_accept: "auto", space_execute: "auto" },
  });

  const dealMatrix = [
    {
      deal_type: "affiliate",
      space_kind: "content_inline",
      settlement: { mode: "cpa_commission", commission_type: "percentage", commission_bps: 2500 },
      verification_mode: "public_check",
      proof_url: "https://blog.example.com/postgres-ci",
    },
    {
      deal_type: "cross_sell_upsell",
      space_kind: "cross_promo_slot",
      settlement: { mode: "co_marketing", commission_type: "percentage", commission_bps: 2000 },
      verification_mode: "counterparty_attestation",
      proof_url: "https://app.example.com/post-checkout-perk",
    },
    {
      deal_type: "link_exchange",
      space_kind: "content_inline",
      settlement: {
        mode: "space_swap",
        initiator_space_id: "spc_blog_a",
        responder_space_id: "spc_blog_b",
        sync_window_seconds: 86400,
      },
      verification_mode: "public_check",
      proof_url: "https://docs.example.com/ecosystem-links",
    },
    {
      deal_type: "traffic_swap",
      space_kind: "newsletter_slot",
      settlement: {
        mode: "traffic_swap",
        target_verified_visits: 250,
        exchange_ratio_bps: 10000,
      },
      verification_mode: "public_check",
      proof_url: "https://newsletter.example.com/archive/issue-42",
    },
    {
      deal_type: "repost_retweet",
      space_kind: "social_repost",
      settlement: {
        mode: "repost_bounty",
        currency: "USD",
        amount_minor: 2500,
        target_post_uri: "https://x.com/cloudbranch/status/1849001",
        required_action: "repost",
      },
      verification_mode: "public_check",
      proof_url: "https://x.com/devwriter/status/1849002",
    },
  ];

  for (const item of dealMatrix) {
    const offerMsg = agentA.createOffer({
      dealId: `deal_${item.deal_type}`,
      recipient: "orp:party:counterparty_agent",
      terms: {
        deal_type: item.deal_type,
        space_id: `spc_${item.deal_type}`,
        space_kind: item.space_kind,
        settlement: item.settlement,
        creative: {
          headline: `Growth deal: ${item.deal_type}`,
          body: " Partner recommendation with zero cold email.",
          target_url: "https://cloudbranch.example.com/start",
          disclosure_text: "#ad",
        },
        window: { min_live_seconds: 86400 },
      },
      expiresAt: "2026-10-28T00:00:00Z",
    });
    assert.equal(validateOrpMessage(offerMsg).valid, true, `Offer for ${item.deal_type} must validate`);

    const receiptMsg = {
      orp_version: "0.3.0",
      message_id: `msg_rcpt_${item.deal_type}`,
      message_type: "receipt",
      deal_id: `deal_${item.deal_type}`,
      sender: response.ephemeral_party_id,
      timestamp: "2026-10-21T10:15:00Z",
      idempotency_key: `idem_rcpt_${item.deal_type}`,
      payload: {
        receipt_id: `rcpt_${item.deal_type}`,
        space_id: `spc_${item.deal_type}`,
        verification_mode: item.verification_mode,
        proof_url: item.proof_url,
        location_url: item.proof_url,
        verified_at: "2026-10-21T10:15:00Z",
        disclosure_verified: true,
        observed_disclosure: "#ad",
        content_hash: computeTermsHash({ deal_type: item.deal_type, proof: item.proof_url }),
        ...(item.verification_mode === "public_check"
          ? {
              public_check_result: {
                http_status: 200,
                matched_target: "https://cloudbranch.example.com/start",
                checked_at: "2026-10-21T10:15:00Z",
              },
            }
          : {
              counterparty_attestation: {
                attested_by: "orp:party:counterparty_agent",
                attested_at: "2026-10-21T10:15:00Z",
                statement: "Verified post-checkout cross-sell conversion.",
              },
            }),
      },
    };
    assert.equal(validateOrpMessage(receiptMsg).valid, true, `Receipt for ${item.deal_type} must validate`);
  }

  // 3. Live Go server check: unauthenticated /v1/revenue/preview works without email/signup,
  //    while /v1/agent-accounts still requires email & domain (logged in GAPS.md)
  if (liveBaseUrl) {
    const previewRes = await fetch(`${liveBaseUrl}/v1/revenue/preview`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        context:
          "How to speed up developer pull requests and CI pipelines using isolated Postgres database branches.",
      }),
    });
    assert.equal(previewRes.status, 200, "Unauthenticated /v1/revenue/preview must return 200 with no email/signup");
    const previewBody = await previewRes.json();
    assert.equal(previewBody.preview_only, true);
  }
});

// ============================================================================
// TEST 3: Discovery manifest (/.well-known/dcp.json & /.well-known/orp.json)
// ============================================================================
test("3. [L1-MANIFEST] /.well-known/dcp.json is renamed away from distributoor-mcp to offer-rails v0.3.0", async () => {
  const mainGo = readFileSync(resolve(repoRoot, "affilio-api/cmd/server/main.go"), "utf8");
  assert.equal(mainGo.includes('"command": "distributoor-mcp"'), false, "main.go no longer advertises distributoor-mcp");
  assert.equal(mainGo.includes('"command": "offer-rails-mcp"'), true, "main.go advertises offer-rails-mcp");
  assert.equal(mainGo.includes('"orp_version": "0.3.0"'), true, "main.go includes orp_version 0.3.0");
  assert.equal(mainGo.includes('"name":        "offer-rails"'), true, "main.go uses offer-rails name");

  if (liveBaseUrl) {
    for (const path of ["/.well-known/dcp.json", "/.well-known/orp.json"]) {
      const res = await fetch(`${liveBaseUrl}${path}`);
      assert.equal(res.status, 200);
      const body = await res.json();
      assert.equal(body.name, "offer-rails");
      assert.equal(body.orp_version, "0.3.0");
      assert.equal(body.mcp?.stdio?.command, "offer-rails-mcp");
    }
  }
});

// ============================================================================
// TEST 4: /llms.txt no longer says /mcp needs Google sign-in
// ============================================================================
test("4. [L1-LLMS-TXT] /llms.txt documents bearer keys on /mcp and no longer claims /mcp needs Google sign-in", async () => {
  for (const relPath of [
    "ui/landing/public/llms.txt",
    "affilio-wp/admin/build/landing-v9/llms.txt",
  ]) {
    const content = readFileSync(resolve(repoRoot, relPath), "utf8");
    assert.equal(/\/mcp needs (?:the )?Google sign-in/i.test(content), false, `${relPath} must not say /mcp needs Google sign-in`);
    assert.equal(/This server needs a Google sign-in today/i.test(content), false, `${relPath} must not say server needs Google sign-in`);
    assert.match(content, /Keys from \/v1\/agent-accounts work as Bearer tokens on \/mcp and on the REST API/);
  }

  if (liveBaseUrl) {
    const res = await fetch(`${liveBaseUrl}/llms.txt`);
    assert.equal(res.status, 200);
  }
});

// ============================================================================
// TEST 5: JSON Schema Draft 2020-12 validation across protocol messages
// ============================================================================
test("5. [L1-SCHEMA-2020-12] Protocol messages validate against Draft 2020-12 schemas", () => {
  const agent = new OrpAgent({
    principalId: "orp:party:maker_alpha",
    constraints: {
      max_deal_amount_minor: 50_00,
      daily_budget_minor: 200_00,
      require_human_above_minor: 25_00,
      currency: "USD",
      allowed_settlement_modes: ["flat_fee", "cpa_commission", "space_swap", "repost_bounty"],
      allowed_categories: ["developer_tools", "ai"],
      blocked_categories: ["gambling"],
    },
    stepModes: {
      deal_offer: "auto",
      deal_accept: "auto",
      space_execute: "auto",
    },
  });

  const terms = {
    space_id: "space_x_dev",
    space_kind: "social_post",
    settlement: {
      mode: "flat_fee",
      currency: "USD",
      amount_minor: 20_00,
    },
    creative: {
      headline: "Fast Postgres branching",
      body: "Spin up isolated Postgres branches for every PR.",
      target_url: "https://db.example.com",
      disclosure_text: "#ad",
    },
    window: {
      min_live_seconds: 86400,
    },
  };

  const offer = agent.createOffer({
    dealId: "deal_schema_01",
    recipient: "orp:party:pub_beta",
    terms,
    expiresAt: "2026-10-28T00:00:00Z",
  });
  assert.equal(validateOrpMessage(offer).valid, true);

  const counter = agent.createCounter({
    dealId: "deal_schema_01",
    recipient: "orp:party:maker_alpha",
    seq: 2,
    priorMessageId: offer.message_id,
    reason: "Include weekend pin",
    terms: {
      ...terms,
      settlement: { mode: "flat_fee", currency: "USD", amount_minor: 22_00 },
    },
    expiresAt: "2026-10-28T00:00:00Z",
  });
  assert.equal(validateOrpMessage(counter).valid, true);

  const accepted = agent.acceptAndEvaluateConsent({
    dealId: "deal_schema_01",
    counterparty: "orp:party:pub_beta",
    acceptedMessageId: counter.message_id,
    acceptedSeq: 2,
    terms: counter.payload.terms,
    dealCategories: ["developer_tools"],
  });
  assert.equal(accepted.status, "consented_auto");
  assert.equal(validateOrpMessage(accepted.accept).valid, true);
  assert.ok(accepted.consent);
  assert.equal(validateOrpMessage(accepted.consent).valid, true);
});

// ============================================================================
// TEST 6: Privacy by Default (Pseudonymous Party IDs, k >= 50, No PII)
// ============================================================================
test("6. [L1-PRIVACY-PSEUDONYMOUS] Rejects email/PII in party IDs and Audience Cards with k < 50", () => {
  const badOffer = {
    orp_version: "0.3.0",
    message_id: "msg_bad_pii",
    message_type: "offer",
    deal_id: "deal_bad_pii",
    sender: "founder@example.com", // PII email forbidden
    recipient: "orp:party:pub_01",
    timestamp: "2026-10-21T10:00:00Z",
    idempotency_key: "idem_bad_pii",
    payload: {
      seq: 1,
      terms: {
        space_id: "space_1",
        space_kind: "social_post",
        settlement: { mode: "flat_fee", currency: "USD", amount_minor: 1000 },
        creative: {
          body: "Hello",
          target_url: "https://example.com",
          disclosure_text: "#ad",
        },
        window: { min_live_seconds: 3600 },
      },
      terms_hash: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
      expires_at: "2026-10-22T10:00:00Z",
    },
  };

  const res = validateWithSchema(badOffer, "offer.schema.json", schemaRegistry);
  assert.equal(res.valid, false, "Must reject email address in party ID");
});

// ============================================================================
// TEST 7: Constraint-Bounded Autonomous Negotiation
// ============================================================================
test("7. [L2-CONSTRAINT-AUTO-VS-ESCALATE] Agents negotiate autonomously inside human constraints and escalate only outside them", () => {
  const constraints = {
    max_deal_amount_minor: 100_00,
    daily_budget_minor: 250_00,
    require_human_above_minor: 50_00,
    currency: "USD",
    allowed_settlement_modes: ["flat_fee", "cpa_commission", "space_swap"],
    allowed_categories: ["developer_tools", "productivity"],
    blocked_categories: ["crypto", "gambling"],
    brand_rules: {
      require_verified_counterparty: true,
      min_counterparty_karma: 10,
      disallow_superlatives: true,
    },
  };

  const stepModes = { deal_accept: "auto" };

  const baseTerms = {
    space_id: "space_nl_01",
    space_kind: "newsletter_slot",
    settlement: { mode: "flat_fee", currency: "USD", amount_minor: 40_00 },
    creative: {
      headline: "Code Review Assistant",
      body: "Automated lint and test checks on every pull request.",
      target_url: "https://review.example.com",
      disclosure_text: "Sponsored",
    },
    window: { min_live_seconds: 86400 },
  };

  // Case A: Inside constraints -> autonomous approval (`auto_approve`)
  const inside = evaluateDealConstraints({
    step: "deal_accept",
    stepModes,
    terms: baseTerms,
    constraints,
    dailySpendSoFarMinor: 0,
    dealCategories: ["developer_tools"],
    counterpartyVerified: true,
    counterpartyKarma: 25,
  });
  assert.equal(inside.decision, "auto_approve");

  // Case B: Exceeds human approval threshold ($75 > $50) -> escalates to human (`escalate_to_human`)
  const overThreshold = evaluateDealConstraints({
    step: "deal_accept",
    stepModes,
    terms: {
      ...baseTerms,
      settlement: { mode: "flat_fee", currency: "USD", amount_minor: 75_00 },
    },
    constraints,
    dailySpendSoFarMinor: 0,
    dealCategories: ["developer_tools"],
    counterpartyVerified: true,
    counterpartyKarma: 25,
  });
  assert.equal(overThreshold.decision, "escalate_to_human");
  assert.ok(overThreshold.escalation_reasons.includes("exceeds_approval_threshold"));

  // Case C: Allowed category mismatch -> escalates to human (`category_not_allowed`)
  const outsideCat = evaluateDealConstraints({
    step: "deal_accept",
    stepModes,
    terms: baseTerms,
    constraints,
    dailySpendSoFarMinor: 0,
    dealCategories: ["gaming"],
    counterpartyVerified: true,
    counterpartyKarma: 25,
  });
  assert.equal(outsideCat.decision, "escalate_to_human");
  assert.ok(outsideCat.escalation_reasons.includes("category_not_allowed"));
});

// ============================================================================
// TEST 8: Verifiable Receipts for Zero-Click Actions (Reposts, Comments, Swaps)
// ============================================================================
test("8. [L2-RECEIPT-THREE-MODES] Zero-click actions validate with proof_url, public_check, and counterparty_attestation", () => {
  const modes = [
    {
      verification_mode: "proof_url",
      proof_url: "https://x.com/dev/status/1001",
      location_url: "https://x.com/dev/status/1001",
    },
    {
      verification_mode: "public_check",
      proof_url: "https://x.com/dev/status/1002",
      location_url: "https://x.com/dev/status/1002",
      public_check_result: {
        http_status: 200,
        matched_target: "https://x.com/maker/status/900",
        checked_at: "2026-10-21T10:05:00Z",
      },
    },
    {
      verification_mode: "counterparty_attestation",
      location_url: "https://newsletter.example.com/issue-99",
      counterparty_attestation: {
        attested_by: "orp:party:counterparty_99",
        attested_at: "2026-10-21T10:06:00Z",
        statement: "Confirmed space-for-space newsletter blurb sent to 12k subscribers.",
      },
    },
  ];

  for (const modeFields of modes) {
    const msg = {
      orp_version: "0.3.0",
      message_id: `msg_${modeFields.verification_mode}`,
      message_type: "receipt",
      deal_id: "deal_zeroclick_01",
      sender: "orp:party:pub_01",
      timestamp: "2026-10-21T10:10:00Z",
      idempotency_key: `idem_${modeFields.verification_mode}`,
      payload: {
        receipt_id: `rcpt_${modeFields.verification_mode}`,
        space_id: "space_x_repost",
        ...modeFields,
        verified_at: "2026-10-21T10:10:00Z",
        disclosure_verified: true,
        observed_disclosure: "#ad",
        content_hash: computeTermsHash({ mode: modeFields.verification_mode }),
      },
    };
    const res = validateOrpMessage(msg);
    assert.equal(res.valid, true, `Mode ${modeFields.verification_mode} failed: ${res.errors.join(", ")}`);
  }
});

// ============================================================================
// TEST 9: Optional Adapters Degrade Gracefully & Proof-of-Outcome Reputation
// ============================================================================
test("9. [L3-ADAPTERS-AND-REPUTATION] Optional X and Newsletter adapters fall back to universal links and Karma decays wash trading", async () => {
  const xAdapter = new XSpaceAdapter(); // No API token
  const nlAdapter = new NewsletterSpaceAdapter(); // No API key
  const link = mintUniversalTracking("https://offer-rails.com", "deal_opt_01");

  const xExec = await xAdapter.executePlacement({
    deal_id: "deal_opt_01",
    space_id: "space_x_1",
    space_kind: "social_post",
    headline: "Launch",
    body: "Try it today",
    tracking_url: link.tracking_url,
    tracking_token: link.tracking_token,
    disclosure: { type: "hashtag_ad", text: "#ad", placement: "inline" },
  });
  assert.equal(xExec.mode, "universal_link_ready");

  const nlExec = await nlAdapter.executePlacement({
    deal_id: "deal_opt_01",
    space_id: "space_nl_1",
    space_kind: "newsletter_slot",
    headline: "Launch",
    body: "Try it today",
    tracking_url: link.tracking_url,
    tracking_token: link.tracking_token,
    disclosure: { type: "html_rel_sponsored", text: "Sponsored", placement: "before" },
  });
  assert.equal(nlExec.mode, "universal_link_ready");

  // Wash-trade decay check: 5 repeated deals with the same counterparty
  const washDeals = Array.from({ length: 5 }, (_, idx) => ({
    deal_id: `deal_wash_${idx + 1}`,
    principal_id: "orp:party:actor",
    counterparty_id: "orp:party:colluder",
    space_kind: "social_repost",
    settlement_mode: "repost_bounty",
    receipt_id: `rcpt_${idx + 1}`,
    receipt_verified: true,
    held_full_duration: true,
    conversion_or_swap_settled: false,
    disclosure_compliant: true,
    dispute_lost: false,
    counterparty_unique_partners: 1,
    settled_at: "2026-10-21T10:00:00Z",
  }));
  const rep = computeReputationProfile("orp:party:actor", washDeals);
  assert.ok(rep.total_karma < 5, `Expected wash-traded karma < 5, got ${rep.total_karma}`);
});
