// Typed message set for the landing pages. `en` is the source of truth; every
// other locale must provide the same keys. Strings that are literal payloads —
// the join line, endpoint names, llms.txt — stay in English on purpose and are
// not part of this type.

export type CardItem = { title: string; body: string }
export type Pair = { t: string; b: string }
export type DealItem = { name: string; status: string; note: string }
export type QA = { q: string; a: string }
export type ErrorCopy = { message: string; next: string }
export type NamedNote = { name: string; note: string }

export type ShowcaseId = 'x' | 'pinterest' | 'instagram' | 'tiktok' | 'youtube' | 'newsletter' | 'blog' | 'podcast'
export type ShowcaseMock = { author: string; handle: string; title: string; body: string; offer: string; meta: string }
export type ShowcasePlatform = {
  id: ShowcaseId
  name: string
  partnership: string
  terms: string
  parties: string[]
  offerStep: string
  counterStep: string
  mock: ShowcaseMock
}

export interface Messages {
  chrome: {
    skipToContent: string
    menu: string
    menuClose: string
    language: string
    signIn: string
    setupCta: string
    navYourLimits: string
    navMakers: string
    navPublishers: string
    navUseCases: string
    navProtocol: string
    navAgents: string
    navReference: string
    useCaseLinks: { id: string; title: string }[]
    commands: Record<'home' | 'maker' | 'publisher' | 'protocol' | 'you' | 'reference' | 'agents' | 'faq' | 'join', string>
    paletteJump: string
    paletteSearchLabel: string
    palettePlaceholder: string
    paletteListLabel: string
    paletteEmpty: (q: string) => string
    paletteHint: string
  }
  hero: {
    eyebrow: string
    h1: string
    support: string
    ctaMaker: string
    ctaMakerHref: string
    ctaPublisher: string
    ctaPublisherHref: string
  }
  clients: {
    heading: string
    link: string
  }
  why: {
    heading: string
    cards: CardItem[]
    trust: string
  }
  how: {
    eyebrow: string
    heading: string
    lead: string
    steps: { title: string; description: string }[]
    more: string
  }
  connect: {
    eyebrow: string
    heading: string
    lead: string
    summary: string
    joinLead: string
    steps: string[]
    noSignIn: string
    pathCaption: string
    pathAgent: string
    pathAgentNote: string
    pathBearer: string
    pathBearerNote: string
    pathSend: string
    pathMcp: string
    pathWait: string
    copyLabel: string
    copiedLabel: string
    copiedStatus: string
  }
  rails: {
    kicker: string
    heading: string
    intro: string
    autonomy: string
    protocolLink: string
    figcaption: string
    steps: { name: string; body: string }[]
    cta: string
  }
  counter: { kicker: string; caption: string }
  makers: {
    kicker: string
    heading: string
    lead: string
    rows: string[]
    link: string
    workedCaption: string
    workedTerms: string
    workedBody: string
    workedTicker: string
    exampleProduct: string
  }
  publishers: {
    kicker: string
    heading: string
    lead: string
    rows: string[]
    link: string
    placementCaption: string
    placementBody: string
    placementNote: string
  }
  compare: {
    heading: string
    beforeTitle: string
    beforeBody: string
    afterTitle: string
    afterBody: string
  }
  vision: {
    doesHeading: string
    doesLead: string
    deals: DealItem[]
    keepHeading: string
    keep: Pair[]
    keepLink: string
    replacesHeading: string
    replaces: Pair[]
  }
  previewSection: {
    kicker: string
    heading: string
    lead: string
    tryLabel: string
  }
  showcase: {
    exampleTag: string
    footnote: string
    partnershipLabel: string
    termsLabel: string
    partiesLabel: string
    timelineLabel: string
    agreedStep: string
    approvedStep: string
    liveStep: string
    tabsLabel: string
    platforms: ShowcasePlatform[]
  }
  preview: {
    ariaLabel: string
    label: string
    submit: string
    loading: string
    publicNote: string
    tryAnother: string
    verifiedHeading: string
    suggestedHeading: string
    closestHeading: string
    approve: string
    sponsored: string
    unverified: string
    postFallback: string
    notify: string
    saved: string
    saveFailed: string
    suggestedNote: string
    closestLead: string
    slotLabels: Record<'after_intro' | 'mid_article' | 'before_conclusion' | 'end_of_article' | 'fallback', string>
    errors: Record<'400' | '403' | '422' | '429' | 'empty', ErrorCopy>
  }
  tradeoffs: {
    eyebrow: string
    heading: string
    lead: string
    withTitle: string
    chip: string
    withItems: string[]
    withoutTitle: string
    withoutItems: string[]
  }
  faq: {
    eyebrow: string
    heading: string
    items: QA[]
  }
  cta: {
    eyebrow: string
    heading: string
    lead: string
    primary: string
    signIn: string
    setup: string
    whoMaker: string
    whoAgent: string
    whoPublisher: string
    feed: { text: string; meta: string }[]
  }
  footer: {
    heading: string
    skylineAlt: string
    tagline: string
    blip: string
    shortcuts: string
    product: string
    people: string
    paper: string
    home: string
    mcpSetup: string
    howItWorks: string
    forMakers: string
    forPublishers: string
    yourLimits: string
    protocol: string
    reference: string
    privacy: string
    terms: string
  }
  plugin: {
    download: string
  }
}
