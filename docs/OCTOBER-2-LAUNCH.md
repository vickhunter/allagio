# October 2 launch plan

Superseded by [MVP-DELIVERY.md](MVP-DELIVERY.md). The release objective is a usable MVP; a presentation or rehearsal is not a release requirement.

This file preserves the earlier proposed plan, not a deployment record.

## Outcome

Present Distributoor as a private beta for developer products and developer
content. Demonstrate one real builder repository, one consenting publisher,
an approved placement, an attributed Stripe test checkout and its refund.
Invite the first five interested people from the approximately 50 meetup
developers after rehearsal. A test checkout proves the integration; the first
commercial result requires an unrelated live buyer and an actual commission
payment received by the publisher.

Suggested positioning: **You built it. Give it a distribution channel.**
Builders use their existing coding agent to prepare their product for referrals.
Publishers earn from relevant recommendations they approve. Distributoor
provides matching, permissions, attribution and commission records.

## Verified starting point

- The backend, hosted MCP/OAuth implementation, installer and WordPress setup
  have passed their documented checks on `feat/agent-revenue-poc`.
- The local `/pilot` proof uses synthetic events and fixture approvals.
- Hosted HTTPS, real provider login, Cursor desktop and a real builder checkout
  remain unverified. The hosted pilot is not deployed.
- Installer and WordPress archives are built, but public installation is not
  released. No participant outreach has been sent.
- WordPress Connect and resumable scanning work. Its recommendation screens use
  legacy affiliate APIs, separate from hosted publisher identity and native
  placement approval. Installing that ZIP alone does not complete the new flow.
- Builder setup hands off to an existing agent. A visible, durable integration
  task runner is not implemented.

## Work order for today

| Order | Deliverable | Evidence before moving on |
| --- | --- | --- |
| 1 | Select a product and publisher; freeze pilot terms | Working product checkout, consenting article owner, agreed commission and refund hold. Use a supported one-time Stripe price. |
| 2 | Deploy an isolated pilot and configure Google OIDC | Public HTTPS, successful owner login, separate database, migrations applied, healthy OAuth discovery, maker and publisher connections. |
| 3 | Run the installer in the real builder repository | Agent detects the checkout, preserves existing metadata, adds attribution, runs that project's checks and verifies the deployed integration. |
| 4 | Complete one publisher journey | Relevant proposal, exact copy/disclosure/terms, owner approval, committed link, actual published article. |
| 5 | Exercise Stripe delivery | Browser follows that link; the actual checkout sends a paid test event; both ledgers show the same pending commission; a refund reverses it once. Replaying delivery creates no duplicate. |
| 6 | Polish the release surfaces and distribute artifacts | Clear landing page, usable owner review, truthful status, working install/download links and a fresh-machine rehearsal. |

Start the UI polish while hosting configuration is being supplied, but keep
checkout attribution and publisher publication ahead of extra visual features.
Keep the existing Go/PostgreSQL stack for this release.

The WordPress path needs an explicit bridge: a WordPress administrator connects
the existing installation to a signed-in publisher, and native placement review
uses that owner authority. Prove site control through the installation and
administrator session, never by matching an email or domain string. Then expose
the native proposal, approval and committed link in the plugin and verify the
result on a real article.

If that bridge does not pass its checks today, the first publisher can use hosted
owner approval and their existing publishing tool, with assisted onboarding.
Describe that path accurately; keep the plugin's automatic scan available without
advertising a complete native one-click publishing journey.

## Installation without marketplace delays

Host the reviewed installer tarball and WordPress ZIP on the pilot's HTTPS
origin. npm accepts remote tarball package specifications, so the landing page
can provide one copyable command without waiting for ownership of an npm scope:

```sh
npm exec --yes --package=https://PILOT_ORIGIN/downloads/distributoor-setup-0.1.0.tgz -- distributoor-setup --api-url https://PILOT_ORIGIN
```

This is a proposed release command. Replace `PILOT_ORIGIN`, implement the
download route, inspect the public package contents and run it from a clean
project before displaying it as available. The archive must include its skills;
source repository access must not be required. Provide a direct WordPress ZIP
download and upload/activate instructions. Verify the plugin's configured backend
is the pilot, since its existing default points at the older service.

References: [npm exec](https://docs.npmjs.com/cli/v11/commands/npm-exec/),
[remote package specifications](https://docs.npmjs.com/cli/v11/using-npm/package-spec/).

## UI and UX brief

Use the supplied Amp screenshots for the quiet workspace: warm ivory, dark
ink text, narrow sidebar, subtle borders and room to read. Give Distributoor
its own small coral accent and typography. Borrow Convex's clear agent setup
entry point and PostHog's command-first setup and visible task progress.
References: [Amp](https://ampcode.com/home), [Convex](https://www.convex.dev/),
[PostHog](https://posthog.com/).

Build three small surfaces:

1. **Landing and setup.** Two clear paths: “Get customers” and “Earn from your
   content.” Builders copy the install command or agent prompt; publishers
   download the plugin. Explain supported clients and any assisted steps.
2. **Workspace and review.** Compact navigation for products, placements,
   earnings and settings. Show actual work and the next required owner action.
   The placement review leads with the article, proposed copy, relevance reason,
   disclosure, commission, payment mode and approval action.
3. **Revenue evidence.** Show clicks, attributed purchases, refunds and pending
   commission from the backend. Distinguish test/live and pending/received. Start
   with a useful empty state instead of invented earnings or growth charts.

Setup progress should separate detected project, connected agent, checkout code
changes, deployed readiness, verified Stripe configuration and delivered test
purchase/refund. Display “waiting for owner” when owner action is necessary.
Only mark completion from recorded evidence. For this release, the existing
agent's setup report can provide the integration evidence; a new autonomous
orchestrator is not a release dependency.

## Presentation rehearsal

Prepare one verified flow and record it as a backup after it works.

1. Show the actual builder install and the resulting integration report.
2. Show a relevant offer on the publisher's article and approve its exact terms.
3. Open the published article and follow its committed tracked link.
4. Complete the real Stripe checkout in clearly labelled test mode.
5. Show the attributed event and pending commission on both sides.
6. Refund through Stripe and show the matching reversal.
7. End with the pilot URL and the first cohort's onboarding path.

Do not show cleared or received commissions from this short test. Commercial
commission clearance and receipt need the real refund window and payment.

## Release decision

Call it a working private beta only after public login, a supported client,
real article publication and Stripe-delivered checkout/refund pass. Rehearse
from a clean browser and a project without existing Distributoor configuration.
Keep a dated verification record with the deployed commit and test-mode evidence,
without credentials or buyer personal data.

If these checks fail before the presentation, announce an early-access preview
with a clearly labelled technical demo and assisted pilot recruitment. Do not
turn synthetic events into claimed customer revenue.

The remaining inputs from Victor are the product/repository, consenting
publisher/article, pilot origin and availability of Google OAuth configuration.
Enter credentials through hosting or provider secret settings, not chat.

## First iteration after the presentation

Measure setup time, steps needing assistance, relevant versus rejected proposals,
published placements, attributed clicks and purchases. Complete one live buyer
and separately confirmed publisher payout before expanding the cohort. Fix the
largest observed onboarding failure first. Extend to recurring subscriptions,
additional channels and ranking models when evidence supports their priority.
