import test from "node:test";
import assert from "node:assert/strict";
import { XSpaceAdapter } from "../x/index.ts";
import { NewsletterSpaceAdapter } from "../newsletter/index.ts";
import { mintUniversalTracking } from "../../sdk/typescript/src/index.ts";
import type { PlacementDraft } from "../interface.ts";

test("XSpaceAdapter operates in zero-API universal_link_ready mode when no API token is set", async () => {
  const xAdapter = new XSpaceAdapter();
  const link = mintUniversalTracking(
    "https://offer-rails.com",
    "deal_x_01"
  );

  const draft: PlacementDraft = {
    deal_id: "deal_x_01",
    space_id: "space_x_founder",
    space_kind: "social_post",
    headline: "Try DevTool",
    body: "Cut CI runtimes in half.",
    tracking_url: link.tracking_url,
    tracking_token: link.tracking_token,
    disclosure: {
      type: "hashtag_ad",
      text: "#ad",
      placement: "inline",
    },
  };

  const result = await xAdapter.executePlacement(draft);
  assert.equal(result.mode, "universal_link_ready");
  assert.equal(result.adapter_id, undefined);
  assert.ok(result.rendered_content.includes(link.tracking_url));
  assert.ok(result.rendered_content.includes("#ad"));

  // Verify public_check receipt for a zero-click repost
  const check = await xAdapter.verifyReceipt(
    {
      deal_id: "deal_x_01",
      space_id: "space_x_founder",
      verification_mode: "public_check",
      proof_url: "https://x.com/founder/status/123456",
      public_check: {
        target_url: "https://x.com/founder/status/123456",
        method: "oembed",
        expected_target_url: "https://x.com/maker/status/999999",
      },
      disclosure_verified: true,
      published_at: "2026-10-21T10:00:00Z",
    },
    "RT @maker: Check out https://x.com/maker/status/999999 #ad"
  );
  assert.equal(check.valid, true);
  assert.equal(check.disclosure_present, true);
  assert.equal(check.tracking_link_or_target_present, true);
});

test("NewsletterSpaceAdapter renders rel=sponsored HTML with universal tracked link and revokes via relay", async () => {
  const nlAdapter = new NewsletterSpaceAdapter({ provider: "beehiiv" });
  const link = mintUniversalTracking(
    "https://offer-rails.com",
    "deal_nl_01"
  );

  const draft: PlacementDraft = {
    deal_id: "deal_nl_01",
    space_id: "space_nl_weekly",
    space_kind: "newsletter_slot",
    headline: "Sponsor: CloudDB",
    body: "Serverless Postgres with branching.",
    call_to_action: "Start free",
    tracking_url: link.tracking_url,
    tracking_token: link.tracking_token,
    disclosure: {
      type: "html_rel_sponsored",
      text: "Sponsored",
      placement: "before",
      rel_attribute: "sponsored nofollow",
    },
  };

  const exec = await nlAdapter.executePlacement(draft);
  assert.equal(exec.mode, "universal_link_ready");
  assert.ok(exec.rendered_content.includes(`href="${link.tracking_url}"`));
  assert.ok(exec.rendered_content.includes(`rel="sponsored nofollow"`));

  const receiptCheck = await nlAdapter.verifyReceipt(
    {
      deal_id: "deal_nl_01",
      space_id: "space_nl_weekly",
      verification_mode: "proof_url",
      proof_url: "https://weekly.example.com/archive/issue-42",
      tracking_url: link.tracking_url,
      tracking_token: link.tracking_token,
      disclosure_verified: true,
      published_at: "2026-10-21T10:00:00Z",
    },
    exec.rendered_content
  );
  assert.equal(receiptCheck.valid, true);

  const revokeRes = await nlAdapter.revokePlacement("deal_nl_01");
  assert.equal(revokeRes.revoked, true);
  assert.equal(revokeRes.mode, "link_revoked_at_relay");
});
