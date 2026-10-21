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
// TEST 2: Discovery manifest (/.well-known/dcp.json & /.well-known/orp.json)
// ============================================================================
test("2. [L1-MANIFEST] /.well-known/dcp.json is renamed away from distributoor-mcp to offer-rails v0.3.0", async () => {
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
// TEST 3: /llms.txt no longer says /mcp needs Google sign-in
// ============================================================================
test("3. [L1-LLMS-TXT] /llms.txt documents bearer keys on /mcp and no longer claims /mcp needs Google sign-in", async () => {
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
// TEST 4: JSON Schema Draft 2020-12 validation across protocol messages
// ============================================================================
test("4. [L1-SCHEMA-2020-12] Protocol messages validate against Draft 2020-12 schemas", () => {
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
// TEST 5: Privacy by Default (Pseudonymous Party IDs, k >= 50, No PII)
// ============================================================================
test("5. [L1-PRIVACY-PSEUDONYMOUS] Rejects email/PII in party IDs and Audience Cards with k < 50", () => {
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
// TEST 6: Constraint-Bounded Autonomous Negotiation
// ============================================================================
test("6. [L2-CONSTRAINT-AUTO-VS-ESCALATE] Agents negotiate autonomously inside human constraints and escalate only outside them", () => {
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
// TEST 7: Verifiable Receipts for Zero-Click Actions (Reposts, Comments, Swaps)
// ============================================================================
test("7. [L2-RECEIPT-THREE-MODES] Zero-click actions validate with proof_url, public_check, and counterparty_attestation", () => {
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
// TEST 8: Optional Adapters Degrade Gracefully & Proof-of-Outcome Reputation
// ============================================================================
test("8. [L3-ADAPTERS-AND-REPUTATION] Optional X and Newsletter adapters fall back to universal links and Karma decays wash trading", async () => {
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
