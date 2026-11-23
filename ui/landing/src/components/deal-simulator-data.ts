// Interactive ORP Deal Simulator — templates, wire payloads, and state machine.
// Pure TypeScript with no TS-only runtime syntax so node --test imports it directly.

export type DealTemplateId = 'affiliate' | 'sponsored' | 'swap' | 'ad_space' | 'rev_share'

export type SimulatorStage =
  | 'discover'
  | 'offer'
  | 'counter'
  | 'ask'
  | 'active'
  | 'execute'
  | 'receipt'
  | 'refused'

export type HumanDecision = 'approve' | 'counter' | 'refuse'

export interface AgentParty {
  id: string
  label: string
  role: 'maker' | 'publisher' | 'sponsor' | 'co-publisher'
  handle: string
  host: string
  keyFingerprint: string
}

export interface TermsVariant {
  headline: string
  rateLabel: string
  windowLabel: string
  disclosure: string
  termsHash: string
  summaryLines: string[]
  orpTerms: Record<string, unknown>
}

export interface PreviewArtifact {
  kind: 'placement_card' | 'sponsored_slot' | 'swap_receipts' | 'inventory_slot' | 'rev_share_ledger'
  badge: string
  title: string
  subtitle: string
  body: string
  disclosureLabel: string
  trackedUrl: string
  metrics: Array<{ label: string; value: string; accent?: 'emerald' | 'amber' | 'neutral' }>
  ledgerRows?: Array<{ party: string; role: string; share: string; payout: string }>
  swapSides?: Array<{ publisher: string; issue: string; target: string; verified: string }>
}

export interface DealTemplate {
  id: DealTemplateId
  label: string
  shortKind: string
  orpKind: string
  dealId: string
  proofId: string
  offerer: AgentParty
  acceptor: AgentParty
  extraParty?: AgentParty
  discoverMatch: string
  discoverScore: string
  initialTerms: TermsVariant
  counteredTerms: TermsVariant
  counterNote: string
  hardChecks: string[]
  artifact: PreviewArtifact
  counteredArtifact: PreviewArtifact
}

export interface SimulatorState {
  templateId: DealTemplateId
  stage: SimulatorStage
  countered: boolean
  isPlaying: boolean
  showJson: boolean
  history: SimulatorStage[]
}

export const DEAL_TEMPLATES: DealTemplate[] = [
  {
    id: 'affiliate',
    label: 'Affiliate',
    shortKind: 'affiliate',
    orpKind: 'affiliate',
    dealId: 'deal_01jv8a2affil',
    proofId: 'prf_01jv8a2f9c3e',
    offerer: {
      id: 'ag_acme_deploy',
      label: 'Maker Agent',
      role: 'maker',
      handle: 'acme-deploy.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:7a4f…c91b',
    },
    acceptor: {
      id: 'ag_priya_ships',
      label: 'Publisher Agent',
      role: 'publisher',
      handle: 'priya-ships.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:3e18…8d02',
    },
    discoverMatch: 'Matched article "Caching the dependency graph" (relevance 94/100)',
    discoverScore: '94 / 100 topic match',
    initialTerms: {
      headline: '15% recurring commission on paid plans',
      rateLabel: '15% recurring',
      windowLabel: '30-day attribution · 14-day refund window',
      disclosure: 'Sponsored · rel="sponsored"',
      termsHash: 'sha256:9f4c2e8a71b0d35e6c8a41f0b2e7a9d4c3185f6e2a1b9c0d4e5f6a7b8c9d0e1f',
      summaryLines: [
        '15% recurring share on paid plans',
        'Single labelled box placed after intro',
        'Zero cookies · daily-salted click hash',
      ],
      orpTerms: {
        kind: 'affiliate',
        commission: { type: 'recurring', rate_bps: 1500, currency: 'EUR' },
        attribution_days: 30,
        refund_window_days: 14,
        slot: 'after_intro',
        disclosure: { label: 'Sponsored, via Allagio', rel: 'sponsored' },
      },
    },
    counteredTerms: {
      headline: '20% recurring commission on paid plans',
      rateLabel: '20% recurring',
      windowLabel: '45-day attribution · 14-day refund window',
      disclosure: 'Sponsored · rel="sponsored"',
      termsHash: 'sha256:4b8e1d93a6c2f50e8b3a7d1c4e9f02a6b5d8c1e3f7a904b2d6e8c0a1f3b5d7e9',
      summaryLines: [
        '20% recurring share (countered +500 bps)',
        '45-day attribution window agreed',
        'Single labelled box placed after intro',
      ],
      orpTerms: {
        kind: 'affiliate',
        commission: { type: 'recurring', rate_bps: 2000, currency: 'EUR' },
        attribution_days: 45,
        refund_window_days: 14,
        slot: 'after_intro',
        disclosure: { label: 'Sponsored, via Allagio', rel: 'sponsored' },
      },
    },
    counterNote: 'Publisher Agent countered 15% → 20% recurring with 45-day attribution; Maker Agent accepted within policy ceiling (25%).',
    hardChecks: [
      'Disclosure label present ("Sponsored, via Allagio")',
      'rel="sponsored" enforced on outbound link',
      'Copy length within 160-char cap · single box',
    ],
    artifact: {
      kind: 'placement_card',
      badge: 'Labelled Placement · After Intro',
      title: 'Acme Deploy — One-click preview environments',
      subtitle: 'Inserted in priya.dev/caching-dependency-graph',
      body: 'Cache build graphs automatically and spin up branch previews in 12 seconds. Starts at €0/mo for indie repos.',
      disclosureLabel: 'Sponsored, via Allagio · rel="sponsored"',
      trackedUrl: 'https://allagio.dev/v1/go/ac94k2',
      metrics: [
        { label: 'Commission', value: '15% recurring', accent: 'emerald' },
        { label: 'Attribution', value: '30 days', accent: 'neutral' },
        { label: 'Hard checks', value: '9 / 9 passed', accent: 'emerald' },
      ],
    },
    counteredArtifact: {
      kind: 'placement_card',
      badge: 'Labelled Placement · After Intro',
      title: 'Acme Deploy — One-click preview environments',
      subtitle: 'Inserted in priya.dev/caching-dependency-graph',
      body: 'Cache build graphs automatically and spin up branch previews in 12 seconds. Starts at €0/mo for indie repos.',
      disclosureLabel: 'Sponsored, via Allagio · rel="sponsored"',
      trackedUrl: 'https://allagio.dev/v1/go/ac94k2',
      metrics: [
        { label: 'Commission', value: '20% recurring', accent: 'emerald' },
        { label: 'Attribution', value: '45 days', accent: 'amber' },
        { label: 'Hard checks', value: '9 / 9 passed', accent: 'emerald' },
      ],
    },
  },
  {
    id: 'sponsored',
    label: 'Sponsored Placement',
    shortKind: 'sponsored',
    orpKind: 'sponsored',
    dealId: 'deal_01jv8b7spons',
    proofId: 'prf_01jv8b7a4d12',
    offerer: {
      id: 'ag_parcel_ops',
      label: 'Maker Agent',
      role: 'maker',
      handle: 'parcel-ops.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:5c21…9a44',
    },
    acceptor: {
      id: 'ag_founders_notes',
      label: 'Publisher Agent',
      role: 'publisher',
      handle: 'founders-notes.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:8b09…4f71',
    },
    discoverMatch: 'Matched Issue #89 of Founders Notes (18k verified subscribers)',
    discoverScore: '91 / 100 audience fit',
    initialTerms: {
      headline: '€350 flat fee · 1 featured newsletter block',
      rateLabel: '€350 flat',
      windowLabel: '1 issue · 30-day archive keep',
      disclosure: 'Partner mention · rel="sponsored"',
      termsHash: 'sha256:2d7a9c1e5b80f43d6a2e9c1b7f405a8d3c6e1b9a2f5d804c7e1a3b5d9f2c4e6a',
      summaryLines: [
        '€350 flat fee settled on verified publish',
        'Featured mid-issue block in Issue #89',
        '30-day archive link verification',
      ],
      orpTerms: {
        kind: 'sponsored',
        payout: { amount_minor: 35000, currency: 'EUR', per: 'placement' },
        keep_duration: 'P30D',
        slot: 'mid_issue',
        disclosure: { label: 'Partner mention', rel: 'sponsored' },
      },
    },
    counteredTerms: {
      headline: '€450 flat fee · featured block + promo code',
      rateLabel: '€450 flat',
      windowLabel: '1 issue + archive · 30-day keep',
      disclosure: 'Partner mention · rel="sponsored"',
      termsHash: 'sha256:7e3b5a9d1c40f62e8a1d3b5c9f074e2a6d8c1b3e5f7a902c4d6e8b0a1c3e5f7b',
      summaryLines: [
        '€450 flat fee (countered +€100 for top slot)',
        'Includes reader promo code FOUNDERS20',
        'Proof-of-publish checked before settlement',
      ],
      orpTerms: {
        kind: 'sponsored',
        payout: { amount_minor: 45000, currency: 'EUR', per: 'placement' },
        promo_code: 'FOUNDERS20',
        keep_duration: 'P30D',
        slot: 'top_issue',
        disclosure: { label: 'Partner mention', rel: 'sponsored' },
      },
    },
    counterNote: 'Publisher Agent countered €350 → €450 for top-of-issue placement with code FOUNDERS20; Maker Agent accepted within budget.',
    hardChecks: [
      'Archive URL verified live on foundersnotes.example',
      'Disclosure banner rendered above fold',
      'Hash-chained acceptance signed by both agents',
    ],
    artifact: {
      kind: 'sponsored_slot',
      badge: 'Sponsored Issue Block · #89',
      title: 'Parcel — Ship returns without the spreadsheet',
      subtitle: 'Founders Notes · Issue #89 · Mid-issue feature',
      body: 'Automate return labels, customs forms, and warehouse restock alerts in one API call.',
      disclosureLabel: 'Partner mention · Verified on archive page',
      trackedUrl: 'https://allagio.dev/v1/go/pr89m1',
      metrics: [
        { label: 'Flat fee', value: '€350', accent: 'emerald' },
        { label: 'Keep check', value: '30 days', accent: 'neutral' },
        { label: 'Settlement', value: 'On proof', accent: 'amber' },
      ],
    },
    counteredArtifact: {
      kind: 'sponsored_slot',
      badge: 'Top-of-Issue Sponsor · #89',
      title: 'Parcel — Ship returns without the spreadsheet',
      subtitle: 'Founders Notes · Issue #89 · Promo FOUNDERS20',
      body: 'Automate return labels, customs forms, and warehouse restock alerts in one API call. Use FOUNDERS20 for 20% off.',
      disclosureLabel: 'Partner mention · Verified on archive page',
      trackedUrl: 'https://allagio.dev/v1/go/pr89t1',
      metrics: [
        { label: 'Flat fee', value: '€450', accent: 'emerald' },
        { label: 'Promo code', value: 'FOUNDERS20', accent: 'amber' },
        { label: 'Keep check', value: '30 days', accent: 'emerald' },
      ],
    },
  },
  {
    id: 'swap',
    label: 'Cross-Post Swap',
    shortKind: 'swap',
    orpKind: 'swap',
    dealId: 'deal_01jv8c4xswap',
    proofId: 'prf_01jv8c4e8a90',
    offerer: {
      id: 'ag_design_weekly',
      label: 'Publisher A Agent',
      role: 'publisher',
      handle: 'design-weekly.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:1a92…6c3d',
    },
    acceptor: {
      id: 'ag_frontend_notes',
      label: 'Publisher B Agent',
      role: 'publisher',
      handle: 'frontend-notes.agent',
      host: 'agents.frontend.example',
      keyFingerprint: 'ed25519:9d40…2b1e',
    },
    discoverMatch: 'Reciprocal audience overlap: 12k–15k readers, zero cash moved',
    discoverScore: '96 / 100 audience parity',
    initialTerms: {
      headline: '1-for-1 newsletter recommendation swap · €0 cash',
      rateLabel: '1:1 swap · €0',
      windowLabel: '14-day keep window · both issues',
      disclosure: 'Recommended partner · reciprocal proof',
      termsHash: 'sha256:6c1e4a8b2d5f903e7a1c4b6d8e0f2a4c6e8b0d2f4a6c8e0b2d4f6a8c0e2b4d6f',
      summaryLines: [
        'Design Weekly #215 mentions Frontend Notes',
        'Frontend Notes #104 mentions Design Weekly',
        'Both proofs checked before deal marks complete',
      ],
      orpTerms: {
        kind: 'swap',
        obligations: [
          { party: 'offerer', action: 'place', space: 'issue_215', keep: 'P14D' },
          { party: 'acceptor', action: 'place', space: 'issue_104', keep: 'P14D' },
        ],
        cash_moved: 0,
      },
    },
    counteredTerms: {
      headline: '1-for-1 swap + synced Friday send window · €0 cash',
      rateLabel: '1:1 synced · €0',
      windowLabel: '30-day archive keep · synced Friday',
      disclosure: 'Recommended partner · reciprocal proof',
      termsHash: 'sha256:3a5c7e9b1d3f507a9c1e3b5d7f901a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b3d5f',
      summaryLines: [
        'Synced Friday send for both issues',
        'Archive keep extended 14d → 30d',
        'Automated recheck flags early removal',
      ],
      orpTerms: {
        kind: 'swap',
        obligations: [
          { party: 'offerer', action: 'place', space: 'issue_215', keep: 'P30D', send_day: 'friday' },
          { party: 'acceptor', action: 'place', space: 'issue_104', keep: 'P30D', send_day: 'friday' },
        ],
        cash_moved: 0,
      },
    },
    counterNote: 'Frontend Notes Agent countered to sync Friday send dates and extend archive keep from 14d to 30d.',
    hardChecks: [
      'Reciprocal links verified on both archive URLs',
      'Cross-host signature verified (agents.frontend.example)',
      'Zero subscriber data exchanged between agents',
    ],
    artifact: {
      kind: 'swap_receipts',
      badge: 'Reciprocal Swap Receipts · 2 / 2 Verified',
      title: 'Mutual recommendation live on both newsletters',
      subtitle: 'Design Weekly #215 ↔ Frontend Notes #104',
      body: 'Both placements are checked on their public archive pages. If either link drops early, the protocol flags the obligation automatically.',
      disclosureLabel: 'Reciprocal swap · €0 moved',
      trackedUrl: 'https://allagio.dev/v1/go/sw215',
      metrics: [
        { label: 'Cash moved', value: '€0', accent: 'neutral' },
        { label: 'Keep window', value: '14 days', accent: 'emerald' },
        { label: 'Receipts', value: '2 / 2 signed', accent: 'emerald' },
      ],
      swapSides: [
        { publisher: 'Design Weekly', issue: 'Issue #215', target: '→ Frontend Notes', verified: 'Checked live' },
        { publisher: 'Frontend Notes', issue: 'Issue #104', target: '→ Design Weekly', verified: 'Checked live' },
      ],
    },
    counteredArtifact: {
      kind: 'swap_receipts',
      badge: 'Reciprocal Swap Receipts · Synced Friday',
      title: 'Mutual recommendation live on both newsletters',
      subtitle: 'Design Weekly #215 ↔ Frontend Notes #104',
      body: 'Both placements shipped on the same Friday window with a 30-day verified archive keep.',
      disclosureLabel: 'Reciprocal swap · €0 moved',
      trackedUrl: 'https://allagio.dev/v1/go/sw215f',
      metrics: [
        { label: 'Cash moved', value: '€0', accent: 'neutral' },
        { label: 'Keep window', value: '30 days', accent: 'amber' },
        { label: 'Receipts', value: '2 / 2 signed', accent: 'emerald' },
      ],
      swapSides: [
        { publisher: 'Design Weekly', issue: 'Issue #215 · Fri', target: '→ Frontend Notes', verified: 'Checked live' },
        { publisher: 'Frontend Notes', issue: 'Issue #104 · Fri', target: '→ Design Weekly', verified: 'Checked live' },
      ],
    },
  },
  {
    id: 'ad_space',
    label: 'Inventory Slot',
    shortKind: 'space_booking',
    orpKind: 'sponsored',
    dealId: 'deal_01jv8d9space',
    proofId: 'prf_01jv8d9b2c41',
    offerer: {
      id: 'ag_goji_analytics',
      label: 'Sponsor Agent',
      role: 'sponsor',
      handle: 'goji-analytics.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:4f72…1e8c',
    },
    acceptor: {
      id: 'ag_dev_digest',
      label: 'Publisher Agent',
      role: 'publisher',
      handle: 'dev-digest.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:6b31…7a95',
    },
    discoverMatch: 'Matched open sidebar & feed slot sp_devdigest_w48 (privacy tech)',
    discoverScore: '92 / 100 category match',
    initialTerms: {
      headline: '€220 / week · 4-week reserved inventory slot',
      rateLabel: '€220 / wk',
      windowLabel: '4 weeks · 1 slot cap per page',
      disclosure: 'Sponsored · rel="sponsored"',
      termsHash: 'sha256:8d0f2b4c6e8a0c2e4f6a8b0d2e4f6a8c0b2d4e6f8a0c2e4b6d8f0a2c4e6b8d0f',
      summaryLines: [
        'Reserved slot sp_devdigest_w48 for 4 weeks',
        '€220/week settled per verified weekly check',
        'No third-party scripts · static labelled card',
      ],
      orpTerms: {
        kind: 'sponsored',
        space_id: 'sp_devdigest_w48',
        payout: { amount_minor: 22000, currency: 'EUR', per: 'placement', count: 4 },
        keep_duration: 'P28D',
        disclosure: { label: 'Sponsored', rel: 'sponsored' },
      },
    },
    counteredTerms: {
      headline: '€260 / week · 4 weeks + changelog header slot',
      rateLabel: '€260 / wk',
      windowLabel: '4 weeks · header + sidebar slot',
      disclosure: 'Sponsored · rel="sponsored"',
      termsHash: 'sha256:5e7a9c1e3b5d7f902a4c6e8b0d2f4a6c8e0b2d4f6a8c0e2b4d6f8a0c2e4b6d8e',
      summaryLines: [
        'Upgraded to changelog header + sidebar slot',
        '€260/week (€1,040 total) across 4 weekly checks',
        'Zero tracking pixels · privacy-safe redirect',
      ],
      orpTerms: {
        kind: 'sponsored',
        space_id: 'sp_devdigest_header',
        payout: { amount_minor: 26000, currency: 'EUR', per: 'placement', count: 4 },
        keep_duration: 'P28D',
        disclosure: { label: 'Sponsored', rel: 'sponsored' },
      },
    },
    counterNote: 'Publisher Agent offered the changelog header slot at €260/wk; Sponsor Agent accepted within monthly budget cap.',
    hardChecks: [
      'Static HTML slot · zero third-party JS beacons',
      'Weekly live recheck scheduled on devdigest.example',
      'Disclosure & rel="sponsored" validated',
    ],
    artifact: {
      kind: 'inventory_slot',
      badge: 'Booked Inventory Slot · 4 Weeks',
      title: 'Goji Analytics — Privacy-first metrics for indie teams',
      subtitle: 'Slot sp_devdigest_w48 · Static card, zero tracking scripts',
      body: 'Lightweight web analytics in 1.4 KB. EU-hosted, no cookie banners required.',
      disclosureLabel: 'Sponsored · Verified weekly by ORP host',
      trackedUrl: 'https://allagio.dev/v1/go/gj48s',
      metrics: [
        { label: 'Rate', value: '€220 / wk', accent: 'emerald' },
        { label: 'Duration', value: '4 weeks', accent: 'neutral' },
        { label: 'Scripts', value: '0 KB JS', accent: 'emerald' },
      ],
    },
    counteredArtifact: {
      kind: 'inventory_slot',
      badge: 'Booked Header Slot · 4 Weeks',
      title: 'Goji Analytics — Privacy-first metrics for indie teams',
      subtitle: 'Slot sp_devdigest_header · Header + sidebar placement',
      body: 'Lightweight web analytics in 1.4 KB. EU-hosted, no cookie banners required.',
      disclosureLabel: 'Sponsored · Verified weekly by ORP host',
      trackedUrl: 'https://allagio.dev/v1/go/gj48h',
      metrics: [
        { label: 'Rate', value: '€260 / wk', accent: 'emerald' },
        { label: 'Total (4w)', value: '€1,040', accent: 'amber' },
        { label: 'Scripts', value: '0 KB JS', accent: 'emerald' },
      ],
    },
  },
  {
    id: 'rev_share',
    label: 'Rev-Share',
    shortKind: 'rev_share',
    orpKind: 'partnership',
    dealId: 'deal_01jv8e1rshar',
    proofId: 'prf_01jv8e1c7f05',
    offerer: {
      id: 'ag_ledger_app',
      label: 'Maker Agent',
      role: 'maker',
      handle: 'ledger-app.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:2c84…5d1a',
    },
    acceptor: {
      id: 'ag_frugal_blog',
      label: 'Publisher Agent',
      role: 'publisher',
      handle: 'frugal-eng.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:7e55…0c92',
    },
    extraParty: {
      id: 'ag_frugal_mail',
      label: 'Newsletter Agent',
      role: 'co-publisher',
      handle: 'frugal-notes.agent',
      host: 'allagio.dev',
      keyFingerprint: 'ed25519:3b19…8a40',
    },
    discoverMatch: '3-party ORP 0.4 co-promotion bundle: Maker + Blog + Newsletter',
    discoverScore: '3-party split verified',
    initialTerms: {
      headline: '55 / 25 / 20 revenue split on €80 bundle',
      rateLabel: '55 / 25 / 20',
      windowLabel: '90-day partnership · 14-day refund hold',
      disclosure: 'Partner bundle · all 3 seats signed',
      termsHash: 'sha256:1b3d5f7a9c1e305b7d9f1a3c5e7b9d1f305a7c9e1b3d5f7a9c1e3b5d7f9a1c3e',
      summaryLines: [
        'Maker 55% (€44) · Blog 25% (€20) · Newsletter 20% (€16)',
        'ORP 0.4 multi-party envelope — all 3 seats sign',
        'Automated ledger split per verified Stripe webhook',
      ],
      orpTerms: {
        kind: 'partnership',
        parties: 3,
        split_bps: { maker: 5500, blog: 2500, newsletter: 2000 },
        bundle_price_minor: 8000,
        currency: 'EUR',
        duration: 'P90D',
      },
    },
    counteredTerms: {
      headline: '50 / 30 / 20 revenue split on €80 bundle',
      rateLabel: '50 / 30 / 20',
      windowLabel: '90-day partnership · 14-day refund hold',
      disclosure: 'Partner bundle · all 3 seats signed',
      termsHash: 'sha256:9a7c5e3b1d0f8a6c4e2b0d9f7a5c3e1b8d6f4a2c0e9b7d5f3a1c8e6b4d2f0a9c',
      summaryLines: [
        'Maker 50% (€40) · Blog 30% (€24) · Newsletter 20% (€16)',
        'Countered +500 bps to blog for evergreen guide',
        'Each party gets an identical signed receipt',
      ],
      orpTerms: {
        kind: 'partnership',
        parties: 3,
        split_bps: { maker: 5000, blog: 3000, newsletter: 2000 },
        bundle_price_minor: 8000,
        currency: 'EUR',
        duration: 'P90D',
      },
    },
    counterNote: 'Blog Agent countered 55/25/20 → 50/30/20 for pinning the evergreen budgeting guide; Maker & Newsletter Agents co-signed.',
    hardChecks: [
      'All 3 required seats signed identical terms_hash',
      'Sum of split_bps equals 10,000 (100%)',
      'External payout rails linked · zero custody',
    ],
    artifact: {
      kind: 'rev_share_ledger',
      badge: 'Multi-Party Rev-Share Ledger · €80 Sale',
      title: 'Ledger Budgeting Bundle — Automated 3-way split',
      subtitle: 'ORP 0.4 multi-party receipt · 3 signed seats',
      body: 'Every verified checkout splits cleanly across Maker, Blog, and Newsletter with one shared proof ID.',
      disclosureLabel: 'Partner bundle · Signed by 3 agents',
      trackedUrl: 'https://allagio.dev/v1/go/lg80b',
      metrics: [
        { label: 'Bundle', value: '€80.00', accent: 'neutral' },
        { label: 'Split', value: '55 / 25 / 20', accent: 'emerald' },
        { label: 'Seats', value: '3 / 3 signed', accent: 'emerald' },
      ],
      ledgerRows: [
        { party: 'Ledger App', role: 'Maker', share: '55%', payout: '€44.00' },
        { party: 'The Frugal Engineer', role: 'Blog', share: '25%', payout: '€20.00' },
        { party: 'Frugal Notes', role: 'Newsletter', share: '20%', payout: '€16.00' },
      ],
    },
    counteredArtifact: {
      kind: 'rev_share_ledger',
      badge: 'Multi-Party Rev-Share Ledger · €80 Sale',
      title: 'Ledger Budgeting Bundle — Automated 3-way split',
      subtitle: 'ORP 0.4 multi-party receipt · Countered 50 / 30 / 20',
      body: 'Every verified checkout splits cleanly across Maker, Blog, and Newsletter with one shared proof ID.',
      disclosureLabel: 'Partner bundle · Signed by 3 agents',
      trackedUrl: 'https://allagio.dev/v1/go/lg80c',
      metrics: [
        { label: 'Bundle', value: '€80.00', accent: 'neutral' },
        { label: 'Split', value: '50 / 30 / 20', accent: 'amber' },
        { label: 'Seats', value: '3 / 3 signed', accent: 'emerald' },
      ],
      ledgerRows: [
        { party: 'Ledger App', role: 'Maker', share: '50%', payout: '€40.00' },
        { party: 'The Frugal Engineer', role: 'Blog', share: '30%', payout: '€24.00' },
        { party: 'Frugal Notes', role: 'Newsletter', share: '20%', payout: '€16.00' },
      ],
    },
  },
]

export function getTemplateById(id: DealTemplateId): DealTemplate {
  return DEAL_TEMPLATES.find((t) => t.id === id) ?? DEAL_TEMPLATES[0]
}

export function getActiveTerms(template: DealTemplate, countered: boolean): TermsVariant {
  return countered ? template.counteredTerms : template.initialTerms
}

export function getActiveArtifact(template: DealTemplate, countered: boolean): PreviewArtifact {
  return countered ? template.counteredArtifact : template.artifact
}

export function shortHash(hash: string): string {
  const clean = hash.replace(/^sha256:/, '')
  return `sha256:${clean.slice(0, 12)}…${clean.slice(-6)}`
}

export function createInitialState(templateId: DealTemplateId = 'affiliate'): SimulatorState {
  return {
    templateId,
    stage: 'ask',
    countered: false,
    isPlaying: false,
    showJson: false,
    history: ['discover', 'offer', 'ask'],
  }
}

export function startPlaybackState(templateId: DealTemplateId): SimulatorState {
  return {
    templateId,
    stage: 'discover',
    countered: false,
    isPlaying: true,
    showJson: false,
    history: ['discover'],
  }
}

export function selectTemplate(state: SimulatorState, templateId: DealTemplateId): SimulatorState {
  return {
    ...createInitialState(templateId),
    showJson: state.showJson,
  }
}

export function resetSimulation(state: SimulatorState, autoPlay = false): SimulatorState {
  if (autoPlay) {
    return {
      ...startPlaybackState(state.templateId),
      showJson: state.showJson,
    }
  }
  return {
    ...createInitialState(state.templateId),
    showJson: state.showJson,
  }
}

export function toggleJsonDrawer(state: SimulatorState): SimulatorState {
  return { ...state, showJson: !state.showJson }
}

export function advanceStep(state: SimulatorState): SimulatorState {
  switch (state.stage) {
    case 'discover':
      return {
        ...state,
        stage: 'offer',
        history: appendUnique(state.history, 'offer'),
      }
    case 'offer':
      return {
        ...state,
        stage: 'ask',
        isPlaying: false,
        history: appendUnique(state.history, 'ask'),
      }
    case 'counter':
      return {
        ...state,
        stage: 'ask',
        isPlaying: false,
        history: appendUnique(state.history, 'ask'),
      }
    case 'active':
      return {
        ...state,
        stage: 'execute',
        history: appendUnique(state.history, 'execute'),
      }
    case 'execute':
      return {
        ...state,
        stage: 'receipt',
        isPlaying: false,
        history: appendUnique(state.history, 'receipt'),
      }
    case 'ask':
    case 'receipt':
    case 'refused':
      return { ...state, isPlaying: false }
  }
}

export function applyHumanDecision(
  state: SimulatorState,
  decision: HumanDecision,
  options?: { immediate?: boolean }
): SimulatorState {
  if (state.stage !== 'ask') return state
  if (decision === 'refuse') {
    return {
      ...state,
      stage: 'refused',
      isPlaying: false,
      history: appendUnique(state.history, 'refused'),
    }
  }
  if (decision === 'counter') {
    if (options?.immediate) {
      return {
        ...state,
        countered: true,
        stage: 'ask',
        isPlaying: false,
        history: appendUnique(appendUnique(state.history, 'counter'), 'ask'),
      }
    }
    return {
      ...state,
      countered: true,
      stage: 'counter',
      isPlaying: true,
      history: appendUnique(state.history, 'counter'),
    }
  }
  // approve
  if (options?.immediate) {
    return {
      ...state,
      stage: 'receipt',
      isPlaying: false,
      history: appendUnique(appendUnique(appendUnique(state.history, 'active'), 'execute'), 'receipt'),
    }
  }
  return {
    ...state,
    stage: 'active',
    isPlaying: true,
    history: appendUnique(state.history, 'active'),
  }
}

function appendUnique(list: SimulatorStage[], item: SimulatorStage): SimulatorStage[] {
  return list.includes(item) ? list : [...list, item]
}

export function buildOrpEnvelope(template: DealTemplate, state: SimulatorState): Record<string, unknown> {
  const terms = getActiveTerms(template, state.countered)
  const seqMap: Record<SimulatorStage, number> = {
    discover: 1,
    offer: 2,
    counter: 3,
    ask: state.countered ? 4 : 3,
    active: state.countered ? 5 : 4,
    execute: state.countered ? 6 : 5,
    receipt: state.countered ? 7 : 6,
    refused: state.countered ? 5 : 4,
  }
  const typeMap: Record<SimulatorStage, string> = {
    discover: 'discover',
    offer: 'offer',
    counter: 'counter',
    ask: 'offer',
    active: 'accept',
    execute: 'execute',
    receipt: 'receipt',
    refused: 'decline',
  }

  const fromAgent =
    state.stage === 'counter' || state.stage === 'refused' || state.stage === 'active' || state.stage === 'execute'
      ? template.acceptor
      : template.offerer
  const toAgent = fromAgent === template.offerer ? template.acceptor : template.offerer
  const fromParty = `orp:${fromAgent.host}/${fromAgent.id}`
  const toParty = `orp:${toAgent.host}/${toAgent.id}`
  const msgType = typeMap[state.stage]
  const hexDigest = terms.termsHash.replace(/^sha256:/, '')
  const b64urlChars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_'
  let wireHashSuffix = ''
  for (let i = 0; i < 43; i++) {
    const byteVal = parseInt(hexDigest.slice((i * 2) % hexDigest.length, ((i * 2) % hexDigest.length) + 2), 16) || 0
    wireHashSuffix += b64urlChars[(byteVal + i) % 64]
  }
  const wireTermsHash = `sha256-${wireHashSuffix}`

  let body: Record<string, unknown>
  if (state.stage === 'discover') {
    body = { want: 'offers', kind: template.orpKind }
  } else if (state.stage === 'active') {
    body = { terms_hash: wireTermsHash }
  } else if (state.stage === 'execute') {
    body = { obligation: 'place', proof_url: `/orp/proof/${template.proofId}` }
  } else if (state.stage === 'receipt') {
    body = {
      proof_id: template.proofId,
      proof_url: `/orp/proof/${template.proofId}`,
      terms_hash: wireTermsHash,
      hard_checks: '9/9 passed',
      status: 'verified',
    }
  } else if (state.stage === 'refused') {
    body = {
      reason: 'halted_by_human',
      placements_inserted: 0,
      cash_moved: 0,
    }
  } else {
    body = {
      id: template.dealId,
      terms: terms.orpTerms,
      terms_hash: wireTermsHash,
    }
  }

  return {
    orp: '0.4',
    type: msgType,
    id: `msg_${template.dealId}_${seqMap[state.stage]}`,
    ts: '2026-11-23T12:00:00Z',
    from: fromParty,
    to: toParty,
    deal: template.dealId,
    body,
    sig: `ed25519..${template.offerer.keyFingerprint.replace(/[^A-Za-z0-9]/g, '').slice(0, 16)}`,
  }
}
