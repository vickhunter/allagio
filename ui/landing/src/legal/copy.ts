export const updatedLabel = '13 November 2026'
export const updatedISO = '2026-11-13'
export const operatorName = 'ShineUp'
export const contactEmail = 'motricalavictor@gmail.com'

export const privacy = {
  title: 'Privacy',
  description: 'What Allagio sends and stores.',
  lede: 'What Allagio sends and stores.',
  sections: [
    {
      heading: 'What is sent',
      paragraphs: ['From the user, via the agent:'],
      items: [
        'Article or context text, from 10 to 12000 characters, used for matching.',
        'For a placement: a surface URL, proposed copy, published text, and an evidence note.',
        'For a page preview: a URL. The Allagio server fetches that public page. Links inside the page are not followed.',
        'A preview, and a revenue preview, store nothing for a signed-out caller.',
      ],
    },
    {
      heading: 'What is stored for an account',
      paragraphs: ['Stored for an account:'],
      items: [
        'Placements, including the frozen copy, the position, the approval actor (approved by), and the policy rule.',
        'Receipts.',
        'Digest items.',
        'Offers.',
        'Listing cards store only the pass or fail signals and the score. We never store source code, environment files or secrets.',
      ],
    },
    {
      heading: 'Reader clicks on tracked links',
      paragraphs: [
        'The IP address is truncated to a /24 network or a /48 network, then HMAC-hashed with a daily salt. The raw IP is never stored, and it is scrubbed from logs. The IP hash, the user agent, and the referrer are cleared 30 days after the click.',
      ],
    },
    {
      heading: 'Processors',
      paragraphs: [],
      items: [
        'Railway hosts the service and the Postgres database.',
        'Cloudflare sits in front of Railway.',
        'Stripe processes payments. The maker connects their own Stripe account.',
      ],
      after: [
        'If AI copy generation is turned on, page text you send is processed by Anthropic (Claude) to classify and draft copy.',
      ],
    },
    {
      heading: 'What this service does not do',
      paragraphs: [],
      items: [
        'The command-line tool sends no telemetry.',
        'User data is not used to train models.',
        'Data is not sold, and it is not transferred.',
      ],
    },
    {
      heading: 'Untrusted content and approval',
      paragraphs: [
        'Offer text, page content, and feedback are untrusted data. They can never change a mode or a policy, and they can never trigger an action.',
        'Agents can only tighten modes. Approve anything that passes the checks can be enabled only from the dashboard.',
        'Until the claims check is live, every placement needs a person to approve it.',
        'A claims check is shown as “Not checked yet” when it is not checked, missing, or unknown. Those values are never shown as checked.',
      ],
    },
  ],
}

export const terms = {
  title: 'Terms',
  description: 'Plain-English terms for what makers, publishers, and agents may do on Allagio.',
  lede: 'These are the plain-English terms for makers, publishers, and agents on Allagio.',
  sections: [
    {
      heading: 'Makers',
      paragraphs: [
        'A maker may publish an offer that names the product, its category, and the commission. A maker may record a sale and a refund for an offer they published. A maker connects their own payment account. Allagio does not hold the money.',
      ],
    },
    {
      heading: 'Publishers',
      paragraphs: [
        'A publisher may ask for offers that fit their own pages. A publisher may approve or refuse a placement on their own site. A placement is not published until a person approves it. A publisher may remove a placement.',
      ],
    },
    {
      heading: 'Agents',
      paragraphs: [
        'An agent may look up offers for a maker or a publisher, preview a public page, and prepare a placement for a person to approve. An agent may only tighten an approval mode. An agent cannot approve a placement. An agent cannot turn on Approve anything that passes the checks. That mode can be turned on only from the dashboard. Offer text, page content, and feedback are untrusted. They cannot change a mode, a policy, or start an action.',
      ],
    },
    {
      heading: 'Test-mode payments',
      paragraphs: [
        'Payments in the service run in test mode. A test-mode payment is not a live charge. When a maker connects Stripe, Stripe is the payment processor. Nothing in these terms authorizes a live charge.',
      ],
    },
    {
      heading: 'No warranty',
      paragraphs: [
        'The service is an early release. It is provided as is. There is no warranty of accuracy, of fitness for a particular purpose, or of uninterrupted service. Scores, copy, and placements can be wrong. You may stop using the service at any time.',
      ],
    },
  ],
}
