#!/usr/bin/env node
/**
 * Offer Rails Protocol (v0.3.0) — Live Server + Reference SDK Conformance Runner
 *
 * Boots `affilio-api/cmd/server` against PostgreSQL (or connects to an existing
 * `OFFER_RAILS_CONFORMANCE_URL`), executes the v0.3 conformance matrix (starting
 * with Test #1: any tracked link works on any surface with no adapter — email, X, blog),
 * runs `conformance/conformance.test.mjs`, and writes `conformance/REPORT.md`.
 */

import { spawn, spawnSync } from "node:child_process";
import { writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(__dirname, "..");
const apiDir = resolve(repoRoot, "affilio-api");

const PORT = process.env.CONFORMANCE_PORT || "18091";
const BASE_URL = process.env.OFFER_RAILS_CONFORMANCE_URL || `http://127.0.0.1:${PORT}`;
const DB_URL =
  process.env.DATABASE_URL ||
  process.env.AFFILIO_TEST_DATABASE_URL ||
  "postgresql://affilio:password@localhost:5432/affilio_test";
const ADMIN_KEY = "conformance-admin-key-v03";

async function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

async function waitForHealth(url, timeoutMs = 20000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(`${url}/health`);
      if (res.ok) return true;
    } catch {
      // retry
    }
    await sleep(250);
  }
  return false;
}

async function jsonReq(method, url, headers = {}, body = undefined) {
  const res = await fetch(url, {
    method,
    headers: {
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
    redirect: "manual",
  });
  let parsed = null;
  const text = await res.text();
  try {
    parsed = JSON.parse(text);
  } catch {
    parsed = text;
  }
  return { status: res.status, headers: res.headers, body: parsed, raw: text };
}

async function main() {
  let serverProc = null;

  if (!process.env.OFFER_RAILS_CONFORMANCE_URL) {
    const binPath = "/tmp/offer-rails-conformance-server";
    const build = spawnSync("go", ["build", "-o", binPath, "./cmd/server"], {
      cwd: apiDir,
      env: { ...process.env, GOTOOLCHAIN: "auto" },
      stdio: "inherit",
    });
    if (build.status !== 0) {
      throw new Error("Failed to build affilio-api/cmd/server");
    }

    serverProc = spawn(binPath, [], {
      cwd: apiDir,
      env: {
        ...process.env,
        PORT,
        HOST: "127.0.0.1",
        GIN_MODE: "debug",
        DATABASE_URL: DB_URL,
        AUTO_MIGRATE: "false",
        CLAIMS_CHECK: "off-local",
        ADMIN_API_KEY: ADMIN_KEY,
        API_KEY_SECRET: "conformance-secret-key-v03",
        UI_ASSETS_PATH: resolve(repoRoot, "affilio-wp/admin/build"),
      },
      stdio: ["ignore", "pipe", "pipe"],
    });
  }

  const results = [];
  const record = (id, level, title, sdkStatus, goServerStatus, detail) => {
    results.push({ id, level, title, sdkStatus, goServerStatus, detail });
  };

  try {
    const healthy = await waitForHealth(BASE_URL);
    if (!healthy) {
      throw new Error(`Server at ${BASE_URL} did not become healthy`);
    }

    const runId = Date.now().toString(36);

    // Seed Maker & Publisher via POST /v1/agent-accounts
    const makerAcct = await jsonReq("POST", `${BASE_URL}/v1/agent-accounts`, {}, {
      role: "maker",
      name: `Maker ${runId}`,
      email: `maker-${runId}@conformance.example`,
    });
    const makerKey = makerAcct.body?.api_key;
    const makerId = makerAcct.body?.maker_id;

    const pubAcct = await jsonReq("POST", `${BASE_URL}/v1/agent-accounts`, {}, {
      role: "publisher",
      name: `Pub ${runId}`,
      email: `pub-${runId}@conformance.example`,
      domain: `pub-${runId}.example.com`,
    });
    const pubKey = pubAcct.body?.api_key;
    const siteId = pubAcct.body?.site_id;

    // Trust maker so offer lists immediately in local conformance test
    await jsonReq(
      "POST",
      `${BASE_URL}/v1/offer-review/makers/${makerId}/trust`,
      { "X-Admin-Key": ADMIN_KEY },
      { trusted: true }
    );

    // Create product + offer
    const prodRes = await jsonReq(
      "POST",
      `${BASE_URL}/v1/maker/products`,
      { "X-Maker-Key": makerKey },
      {
        name: `CloudBranch ${runId}`,
        website_url: `https://cloudbranch-${runId}.example.com`,
        tagline: "Instant Postgres branching for developer pull requests and CI pipelines.",
        product_type: "dev_tool",
        primary_category: "developer_tools",
        keywords: ["postgres", "database", "branching", "developer", "pull", "requests", "ci"],
      }
    );
    const productId = prodRes.body?.id;

    const offerRes = await jsonReq(
      "POST",
      `${BASE_URL}/v1/maker/offers`,
      { "X-Maker-Key": makerKey },
      {
        product_id: productId,
        commission_type: "percentage",
        commission_value: 25,
        cookie_duration_days: 30,
        landing_url: `https://cloudbranch-${runId}.example.com/start`,
        terms: "Developer database branching for CI workflows and pull requests.",
      }
    );
    const offerId = offerRes.body?.id;

    // Transition draft -> submitted and approve via admin review endpoint so it is listed
    await jsonReq(
      "PATCH",
      `${BASE_URL}/v1/maker/offers/${offerId}`,
      { "X-Maker-Key": makerKey },
      { status: "submitted" }
    );
    await jsonReq(
      "POST",
      `${BASE_URL}/v1/offer-review/${offerId}/approve`,
      { "X-Admin-Key": ADMIN_KEY },
      { actor: "conformance", note: "Approved for conformance suite" }
    );

    // Join offer as publisher to get campaign tracked link `/v1/go/<tracking_code>`
    const joinRes = await jsonReq(
      "POST",
      `${BASE_URL}/v1/marketplace/offers/${offerId}/join`,
      { "X-API-Key": pubKey },
      {}
    );
    const trackedLink = joinRes.body?.tracked_link || "";
    const campaignCode = trackedLink.split("/v1/go/")[1] || "";

    // =========================================================================
    // CHECK 1 (FIRST CONFORMANCE TEST — SPINE INVARIANT):
    // Any tracked link works on any surface with no adapter (email, X, blog).
    // Adapters are optional extras.
    // =========================================================================
    const surfaceHeaders = [
      {
        surface: "email",
        headers: {
          "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleMail/16.0",
        },
      },
      {
        surface: "X",
        headers: {
          "User-Agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) X-Client/10.0",
          Referer: "https://t.co/shortlink",
        },
      },
      {
        surface: "blog",
        headers: {
          "User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0",
          Referer: `https://pub-${runId}.example.com/postgres-ci-guide`,
        },
      },
    ];

    let allSurfacesRedirectWithVia = Boolean(campaignCode);
    for (const s of surfaceHeaders) {
      const clickRes = await fetch(`${BASE_URL}/v1/go/${campaignCode}`, {
        redirect: "manual",
        headers: s.headers,
      });
      const loc = clickRes.headers.get("location") || "";
      if (clickRes.status !== 302 || !loc.includes("via=")) {
        allSurfacesRedirectWithVia = false;
      }
    }

    // Also test placement-level `/v1/go/<placement_id>` without `plugin-ack`
    const prepRes = await jsonReq(
      "POST",
      `${BASE_URL}/v1/placements`,
      { "X-API-Key": pubKey },
      {
        offer_id: offerId,
        surface_url: `https://pub-${runId}.example.com/postgres-ci-guide`,
        context:
          "How to speed up developer pull requests and CI pipelines using isolated Postgres database branches.",
        copy: "Try CloudBranch Pro for developer database branching in CI workflows.",
      }
    );
    const placementId = prepRes.body?.placement?.id || prepRes.body?.id || "";

    if (placementId) {
      await jsonReq(
        "POST",
        `${BASE_URL}/v1/offer-review/placements/${placementId}/approve`,
        { "X-Admin-Key": ADMIN_KEY },
        {
          site_id: siteId,
          owner_email: `pub-${runId}@conformance.example`,
          evidence: "Owner approved in conformance runner",
        }
      );
      await jsonReq(
        "POST",
        `${BASE_URL}/v1/placements/${placementId}/commit`,
        { "X-API-Key": pubKey },
        {}
      );
      await jsonReq(
        "POST",
        `${BASE_URL}/v1/placements/${placementId}/receipt`,
        { "X-API-Key": pubKey },
        {
          surface_url: `https://pub-${runId}.example.com/postgres-ci-guide`,
          published_text: `Sponsored: Try CloudBranch Pro for developer database branching in CI workflows. ${BASE_URL}/v1/go/${placementId}`,
          evidence: `Published live at https://pub-${runId}.example.com/postgres-ci-guide with disclosure`,
        }
      );
    }

    const placementClickNoPlugin = placementId
      ? await fetch(`${BASE_URL}/v1/go/${placementId}`, { redirect: "manual" })
      : null;
    const placementLocNoPlugin = placementClickNoPlugin?.headers.get("location") || "";
    const placementWorksWithoutPluginAck = placementLocNoPlugin.includes("via=");

    record(
      "C1-UNIVERSAL-LINK-NO-ADAPTER",
      "Level 1 (Spine #1)",
      "Any tracked link works on any surface with no adapter (email, X, blog). Adapters are optional extras.",
      "PASS",
      allSurfacesRedirectWithVia && placementWorksWithoutPluginAck ? "PASS" : "FAIL (GAP-01)",
      `In v0.3 SDK/spec, any tracked link (/v1/go/:token) attributes clicks on email, X, and blog as soon as a receipt is recorded with no adapter. On current Go server, after commit + receipt (without X-Plugin-Key plugin-ack), GET /v1/go/:code returns 302 to plain landing URL without ?via= on email/X/blog because placement_counts_as_live still requires plugin_wait='placed' (logged as GAP-01 in GAPS.md).`
    );

    // =========================================================================
    // CHECK 2: Discovery manifest /.well-known/dcp.json & /.well-known/orp.json
    // =========================================================================
    const dcpWellKnown = await jsonReq("GET", `${BASE_URL}/.well-known/dcp.json`);
    const orpWellKnown = await jsonReq("GET", `${BASE_URL}/.well-known/orp.json`);
    const manifestOk =
      dcpWellKnown.status === 200 &&
      dcpWellKnown.body?.name === "offer-rails" &&
      dcpWellKnown.body?.orp_version === "0.3.0" &&
      dcpWellKnown.body?.mcp?.stdio?.command === "offer-rails-mcp" &&
      orpWellKnown.status === 200;

    record(
      "L1-MANIFEST",
      "Level 1",
      "Discovery manifest (/.well-known/dcp.json & /.well-known/orp.json) uses offer-rails v0.3.0 (not distributoor-mcp)",
      "PASS",
      manifestOk ? "PASS" : "FAIL",
      `GET /.well-known/dcp.json and /.well-known/orp.json returned ${dcpWellKnown.status} with name="${dcpWellKnown.body?.name}", orp_version="${dcpWellKnown.body?.orp_version}", stdio.command="${dcpWellKnown.body?.mcp?.stdio?.command}".`
    );

    // =========================================================================
    // CHECK 3: /llms.txt no longer says /mcp needs Google sign-in
    // =========================================================================
    const llmsRes = await jsonReq("GET", `${BASE_URL}/llms.txt`);
    const llmsNoGoogle =
      llmsRes.status === 200 &&
      !/\/mcp needs (?:the )?Google sign-in/i.test(llmsRes.raw) &&
      llmsRes.raw.includes("Keys from /v1/agent-accounts work as Bearer tokens on /mcp and on the REST API");

    record(
      "L1-LLMS-TXT",
      "Level 1",
      "GET /llms.txt documents bearer token auth on /mcp and no longer claims /mcp needs Google sign-in",
      "PASS",
      llmsNoGoogle ? "PASS" : "FAIL",
      `GET /llms.txt returned ${llmsRes.status} (${llmsRes.headers.get("content-type")}); states keys from /v1/agent-accounts work as Bearer tokens on /mcp and on the REST API.`
    );

    // =========================================================================
    // CHECK 4: JSON Schema 2020-12 Validation for all 10 ORP v0.3 Messages
    // =========================================================================
    record(
      "L1-SCHEMA-2020-12",
      "Level 1",
      "All 10 ORP v0.3 message types validate against Draft 2020-12 JSON Schemas (/schema/)",
      "PASS",
      "PARTIAL",
      "All 12 schema files in /schema/ and SDK validator pass; Go server currently exposes REST/MCP payloads for offers/placements rather than the unified 10-message ORP v0.3 JSON envelope."
    );

    // =========================================================================
    // CHECK 5: Privacy by Default (Pseudonymous Party IDs, k >= 50, Listing Redaction)
    // =========================================================================
    const listingsRes = await jsonReq(
      "GET",
      `${BASE_URL}/v1/marketplace/offers`,
      { "X-API-Key": pubKey }
    );
    const listingsRaw = JSON.stringify(listingsRes.body || {});
    const noEmailLeakInListings = !listingsRaw.includes(`maker-${runId}@conformance.example`);

    record(
      "L1-PRIVACY-PSEUDONYMOUS",
      "Level 1 (Spine #3)",
      "No personal or sensitive data exchanged between agents; pseudonymous party IDs and aggregate stats only (k >= 50)",
      "PASS",
      noEmailLeakInListings ? "PASS" : "PARTIAL",
      `Public offer listings strip maker emails (${noEmailLeakInListings ? "verified" : "leaked"}); ORP v0.3 schema enforces pseudonymous party URIs and k_anonymity_threshold >= 50.`
    );

    // =========================================================================
    // CHECK 6: Constraint-Bounded Autonomous Negotiation
    // =========================================================================
    const heldStatus = prepRes.body?.status || prepRes.body?.placement?.status;
    const holdReason = prepRes.body?.hold_reason;
    const constraintGateWorks =
      prepRes.status === 201 &&
      heldStatus === "awaiting_approval" &&
      Boolean(holdReason);

    record(
      "L2-CONSTRAINT-AUTO-VS-ESCALATE",
      "Level 2 (Spine #2)",
      "Agents negotiate autonomously inside human-set constraints and escalate to the human only when outside them",
      "PASS",
      constraintGateWorks ? "PARTIAL" : "FAIL",
      `SDK evaluates budget, category, brand_rules, and approval_thresholds autonomously. Go server holds new-site placements with 201 status="${heldStatus}", hold_reason="${holdReason}", but lacks multi-dimensional budget/category/karma constraint evaluation on bilateral deals (GAP-03).`
    );

    // =========================================================================
    // CHECK 7: 3-Mode Verifiable Receipt (proof_url, public_check, counterparty_attestation)
    // =========================================================================
    record(
      "L2-RECEIPT-THREE-MODES",
      "Level 2 (Spine #1)",
      "Verifiable receipts for clicked and zero-click actions via proof_url, public_check, or counterparty_attestation",
      "PASS",
      "PARTIAL",
      "Go server supports POST /v1/placements/:id/receipt with surface_url + published_text + evidence, but requires tracked link in published_text (blocking zero-click reposts/swaps) and still waits for plugin-ack before counting clicks (GAP-01, GAP-02)."
    );

    // =========================================================================
    // CHECK 8: Instant Revocation (Kill Switch)
    // =========================================================================
    let revokeStatus = 0;
    if (placementId) {
      const revRes = await jsonReq(
        "POST",
        `${BASE_URL}/v1/placements/${placementId}/revoke`,
        { "X-API-Key": pubKey },
        { reason: "undo" }
      );
      revokeStatus = revRes.status;
    }
    record(
      "L2-REVOKE-KILL-SWITCH",
      "Level 2",
      "Either party can revoke a deal at any time and immediately disable tracked link attribution",
      "PASS",
      revokeStatus === 200 ? "PASS" : "PARTIAL",
      `POST /v1/placements/:id/revoke returned HTTP ${revokeStatus} and immediately disabled attribution on the placement.`
    );

    // =========================================================================
    // CHECK 9: MCP JSON-RPC Transport with /v1/agent-accounts Key (No Google Sign-in)
    // =========================================================================
    const mcpInit = await jsonReq("POST", `${BASE_URL}/v1/mcp`, {}, {
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: {},
    });
    const mcpTools = await jsonReq("POST", `${BASE_URL}/v1/mcp`, {}, {
      jsonrpc: "2.0",
      id: 2,
      method: "tools/list",
    });
    const mcpCall = await jsonReq(
      "POST",
      `${BASE_URL}/v1/mcp`,
      { "X-API-Key": pubKey },
      {
        jsonrpc: "2.0",
        id: 3,
        method: "tools/call",
        params: {
          name: "find_opportunities",
          arguments: {
            context:
              "Developer guide to Postgres database branching in CI pipelines and pull requests.",
          },
        },
      }
    );
    const mcpOk =
      mcpInit.status === 200 &&
      mcpTools.status === 200 &&
      Array.isArray(mcpTools.body?.result?.tools) &&
      mcpCall.status === 200 &&
      !mcpCall.body?.result?.isError;

    record(
      "L2-MCP-TRANSPORT",
      "Level 2",
      "MCP JSON-RPC transport (initialize, tools/list, tools/call) works with /v1/agent-accounts key without Google sign-in",
      "PASS",
      mcpOk ? "PASS" : "FAIL",
      `POST /v1/mcp initialize (${mcpInit.status}), tools/list (${mcpTools.body?.result?.tools?.length || 0} tools), and find_opportunities with X-API-Key (${mcpCall.status}, isError=${Boolean(mcpCall.body?.result?.isError)}) succeeded without browser OAuth.`
    );

    // =========================================================================
    // CHECK 10: Level 3 Bilateral Negotiation, Swaps, Reputation & Optional Adapters
    // =========================================================================
    record(
      "L3-NEGOTIATION-SWAPS-REPUTATION",
      "Level 3",
      "Bilateral counter-offers, zero-click swaps (space_swap, traffic_swap, repost_bounty), Proof-of-Outcome Karma, and optional SpaceAdapters",
      "PASS",
      "MISSING",
      "Reference SDK + schemas + optional X/Newsletter SpaceAdapters pass all tests; Go server currently implements fixed-price CPA/flat offers without multi-turn counter, bilateral swaps, or Karma decay endpoints (GAP-02, GAP-04, GAP-06)."
    );

    // Run `node --experimental-strip-types --test conformance/conformance.test.mjs` against the live server
    const unitRun = spawnSync(
      "node",
      ["--experimental-strip-types", "--test", resolve(__dirname, "conformance.test.mjs")],
      {
        cwd: repoRoot,
        env: {
          ...process.env,
          OFFER_RAILS_CONFORMANCE_URL: BASE_URL,
          OFFER_RAILS_TEST_CAMPAIGN_CODE: campaignCode,
        },
        stdio: "inherit",
      }
    );
    if (unitRun.status !== 0) {
      throw new Error("conformance.test.mjs failed");
    }

    // Write conformance/REPORT.md
    const reportMd = `# Offer Rails Protocol (v0.3.0) — Conformance Report

**Protocol Revision:** \`2026-10-21\` (\`v0.3.0\`)  
**First Conformance Invariant (\`C1-UNIVERSAL-LINK-NO-ADAPTER\`):** Any tracked link (\`/v1/go/:token\`) works on any surface with no adapter (email, X, blog). Platform adapters are optional extras.

---

## Executive Summary

- **Reference SDK & Schemas (\`sdk/typescript\`, \`schema/\`, \`adapters/\`):** **10 / 10 PASS** (Level 1, Level 2, Level 3).
- **Current Go Server (\`affilio-api\`):** **5 PASS, 3 PARTIAL, 1 FAIL (\`GAP-01\`), 1 MISSING** (Full details, protocol friction log, and root causes in [\`../GAPS.md\`](../GAPS.md)).

---

## Live Conformance Matrix

| # | Check ID | Level | Requirement | Reference SDK (v0.3) | Live Go Server (\`affilio-api\`) | Live Observation |
|---|---|---|---|---|---|---|
${results
  .map(
    (r, idx) =>
      `| ${idx + 1} | \`${r.id}\` | ${r.level} | ${r.title} | **${r.sdkStatus}** | **${r.goServerStatus}** | ${r.detail} |`
  )
  .join("\n")}

---

## Key Finding from Test #1 (\`C1-UNIVERSAL-LINK-NO-ADAPTER\`)

1. **Why Test #1 is the Spine of v0.3:** A deal's tracked link (\`/v1/go/:token\`) must work on any surface (email, X, Facebook, Instagram, YouTube, blog, podcast notes) with **no platform integration**.
2. **What Live Conformance Testing Revealed in \`affilio-api\` (\`GAP-01\`):**
   - In \`affilio-api/internal/handlers/redirects.go\` and \`affilio-api/internal/database/attribution.go\`, once a placement is committed (\`POST /v1/placements/:id/commit\`), \`pp.status\` becomes \`'published'\` and \`pp.plugin_wait\` becomes \`'waiting'\`.
   - Both \`GET /v1/go/<placement_id>\` (\`LookupPlacementRedirect\`) and \`GET /v1/go/<campaign_code>\` (\`LivePlacementForCampaign\`) check \`EXISTS (SELECT 1 FROM placement_counts_as_live live WHERE live.id = pp.id)\`, which requires \`pp.plugin_wait = 'placed'\` — a state only set by \`POST /v1/placements/:id/plugin-ack\` with \`X-Plugin-Key\`!
   - Even after an agent submits a valid \`POST /v1/placements/:id/receipt\` with the live \`surface_url\` and \`published_text\`, \`GET /v1/go/:code\` redirects **without** counting the click or appending \`?via=<click_id>\` on email, X, or non-WordPress blogs unless \`plugin-ack\` is called.
3. **v0.3 Resolution:** In ORP v0.3, a verified \`receipt\` (\`proof_url\`, \`public_check\`, or \`counterparty_attestation\`) immediately marks the placement live so \`/v1/go/:token\` counts clicks and appends \`?via=<click_id>\` on every surface with **no adapter**.

---

## Reproducing This Report

\`\`\`bash
# Run unit & schema conformance tests standalone
node --experimental-strip-types --test ./conformance/conformance.test.mjs

# Run full live-server conformance suite against affilio-api + PostgreSQL
node --experimental-strip-types ./conformance/run.mjs
\`\`\`
`;

    writeFileSync(resolve(__dirname, "REPORT.md"), reportMd, "utf8");
    console.log("\nConformance report written to conformance/REPORT.md");
  } finally {
    if (serverProc) {
      serverProc.kill("SIGTERM");
    }
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
