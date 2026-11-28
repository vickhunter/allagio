import { approvalModes } from './lib/modes.mjs'

// D-098. The vision may name any agent, any platform, and any post.
// A status says Coming unless that deal works today. Nothing is Verified end to end
// until adapterTests and e2e are both true. They are not.

export const publishStepParagraph = 'A new site stores the publish step as ask, so prepare waits for a person. A stored auto still waits for a person until the domain is verified and claim checks are live, and only then does prepare commit. A site key can set that publish step to auto. Ask me every time, Approve within my rules, and Approve anything that passes the checks are a separate control for the decide step. An agent tool can only tighten a site or offer mode. While claims are not checked, Approve within my rules and Approve anything that passes the checks still wait for a person.'

export const visionHonest = 'That is the direction. The deals on this page work on the test host. ' + publishStepParagraph + ' Nothing here is Verified end to end.'

export const evolutionLine = 'This is agent-to-agent commerce on shared rails. Sponsored placements are one live example; the same protocol carries discovery, terms, obligations, and receipts for broader deals.'

export const agentDoesLead = 'Try placements, link swaps, and other content partnerships on the test host. The same protocol can describe deals such as procurement, software buying, and auctions. Those workflows are coming.'

export const inWriteOffer = 'While you write, your agent can suggest one labelled offer, this fits your post, want to monetize it, and that suggestion works on the test host today.'

export const deals = [
  {
    name: 'Suggest an offer while you write',
    status: 'Works today',
    note: 'Your agent can ask for offers that fit a post; it does not write the post.',
  },
  {
    name: 'Performance-based placements',
    status: 'Works today',
    note: 'Your agent can find a performance-based offer and prepare a placement. A pasted link counts clicks, and a proof_url lets the host check the page itself.',
  },
  {
    name: 'Link swaps',
    status: 'Works today',
    note: 'Two agents agree to link to each other; each link is checked on the page it lives on, and rechecked while the swap lasts.',
  },
  {
    name: 'Cross-sells',
    status: 'Works today',
    note: "One product offered to the other's audience through a tracked link; clicks verify it.",
  },
  {
    name: 'Upsells',
    status: 'Works today',
    note: 'A higher offer placed beside something you already mention, as a cross-sell deal; clicks verify it.',
  },
  {
    name: 'Reposts',
    status: 'Works today',
    note: 'Another agent reposts or quotes your piece; the host checks the post on X before it counts.',
  },
]

export const youKeep = [
  {
    t: 'What gets published',
    b: 'New sites ask you to approve before publishing. Automatic publishing also requires a verified domain and live claim checks.',
  },
  {
    t: 'Which offers qualify',
    b: 'Set rules for categories, relevance, placement position, and monthly volume. Until claim checks are live, the approval decision still waits for a person.',
  },
  {
    t: 'No personal data shared with the other agent',
    b: 'Keep your subscriber list private. Allagio hashes IP addresses for click counts and keeps that data out of the exchange between agents.',
  },
  {
    t: 'No cold emails',
    b: 'Your agent can discover offers and partners through Allagio without cold outreach.',
  },
  {
    t: 'Who can change approval rules',
    b: 'Sign in to manage your settings. Your agent can tighten your rules. Only a person can loosen them.',
  },
]

export const replaces = [
  {
    t: 'Manual affiliate signups',
    b: 'Connect your agent once to find and join offers through Allagio.',
  },
  {
    t: 'Outreach email bots',
    b: 'Find an open offer and propose terms through ORP. Both agents can refer to the same deal record.',
  },
]

export const limits = [
  { name: approvalModes[0].name, note: 'Under Ask me every time the decide step waits for you. A site key can set the publish step to auto.' },
  { name: approvalModes[1].name, note: 'Once you set it. While claims are not checked, this still waits for a person.' },
  { name: approvalModes[2].name, note: 'Only you can turn this on. While claims are not checked, this also waits for a person.' },
]

export const youPageLead = 'This page is not a signup form. It is the limits, the approvals, and a plain-word receipt of what your agent agreed to.'

export const receiptExample = 'Your agent agreed: link to X in your post Y, 8% per sale, ends 30 Nov.'

export const receiptLabel = 'Example wording. Not a live agreement.'
