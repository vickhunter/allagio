import { approvalModes } from './lib/modes.mjs'

// D-098. The vision may name any agent, any platform, and any post.
// A status says Coming unless that deal works today. Nothing is Verified end to end
// until adapterTests and e2e are both true. They are not.

export const publishStepParagraph = 'A new site stores the publish step as ask, so prepare waits for a person. A stored auto still waits for a person until the domain is verified and claim checks are live, and only then does prepare commit. A site key can set that publish step to auto. Ask me every time, Approve within my rules, and Approve anything that passes the checks are a separate control for the decide step. An agent tool can only tighten a site or offer mode. While claims are not checked, Approve within my rules and Approve anything that passes the checks still wait for a person.'

export const visionHonest = 'That is the direction. The deals on this page work on the test host. ' + publishStepParagraph + ' Nothing here is Verified end to end.'

export const evolutionLine = 'This is agent-to-agent commerce on shared rails. Sponsored placements are one live example; the same protocol carries discovery, terms, obligations, and receipts for broader deals.'

export const agentDoesLead = 'ORP lets your agent discover counterparties, negotiate terms, execute obligations, and keep receipts. In the live content example, your agent can ask for offers that fit the post: "this fits your post, want to monetize it?" The agent does not write the post, and it does not choose the offer.'

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
    t: 'Your limits come first',
    b: 'A new site waits for a person because the publish step starts as ask, and that stays when claim checks are on. ' + publishStepParagraph,
  },
  {
    t: 'You approve',
    b: publishStepParagraph,
  },
  {
    t: 'No personal data shared with the other agent',
    b: 'The other agent does not get your subscriber list. A click on this server stores a truncated hash of the IP address. That hash is not sent to the other agent.',
  },
  {
    t: 'No cold emails',
    b: 'Allagio does not send cold emails. Deals with other agents are the aim. Link swaps, cross-sells, upsells, and reposts work on the test host today.',
  },
  {
    t: 'No signup on each program site',
    b: 'The aim is that your agent joins an offer without you filling in a form on the program site. Your agent joins here with POST /v1/agent-tokens and a bearer. No sign-in link. Google sign-in stays on the owner dashboard.',
  },
]

export const replaces = [
  {
    t: 'Manual affiliate signups',
    b: 'Filling in a form on each program site is what this is built to replace. ' + publishStepParagraph + ' Your agent joins with POST /v1/agent-tokens and a bearer.',
  },
  {
    t: 'Outreach email bots',
    b: 'Cold email is what this is built to replace. Agent-to-agent link swaps, cross-sells, upsells, and reposts work on the test host today.',
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
