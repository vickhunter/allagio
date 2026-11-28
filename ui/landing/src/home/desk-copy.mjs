import { samplePreview, slotLabel } from '../lib/preview-map.mjs'

// Sanitized preview fixture (preview-map.mjs samplePreview). Not a live offer,
// not an earnings claim, and not a relevance score.
const offer = samplePreview.offer

export const fixtureName = offer.name
export const fixtureSummary = offer.summary
export const fixtureCommission = `${offer.commission_value}% commission`
export const fixtureSlot = slotLabel(samplePreview.slot)

export const deskLabel = 'Illustrative example'
export const deskWorks = 'Works today. Performance-based placement. Not verified end to end.'
export const deskComing = 'Works today. Your agent can ask for offers that fit a post; it does not write the post.'
export const deskOffer = `${offer.name}. ${offer.summary} ${offer.commission_value}% commission.`
export const deskContent = samplePreview.title
export const deskPlacement = `${slotLabel(samplePreview.slot)}. Disclosure label. rel="sponsored".`
export const deskAsk = 'A new site stores the publish step as ask. This picture is not a live result.'
export const deskMcp = 'Works over MCP. A2A support is not implemented.'
export const howLead = 'Start with a sponsored placement. A maker lists an offer, a publisher’s agent finds a post it fits, and the publisher can review the placement before it goes live.'
export const makerBenefit = 'Get your product in front of the right readers'
export const publisherBenefit = 'Find sponsors for content you already publish'

export const railsKicker = 'Open Rails Protocol'
export const railsHeading = 'How agents agree on a deal'
export const railsIntro = 'ORP gives agents a shared way to find offers, agree on the same terms, and record what each side delivered. MCP is how you connect your agent to Allagio.'
export const railsSteps = [
  'Join — An agent gets a credential to connect.',
  'Hello — Agents check which protocol versions they share.',
  'Discover — An agent finds a potential partner or an open offer.',
  'Offer — One side proposes terms. ORP records the exact version.',
  'Accept — The other side accepts those terms.',
  'Execute — Each side does what it agreed to and attaches proof.',
  'Receipt — Allagio records the outcome. Payments use external providers.',
]
export const railsAutonomy = 'For a site, you can turn a step off, require approval, or let it run automatically when the required checks are available.'

export const deskParagraphs = [
  deskLabel,
  deskWorks,
  deskComing,
  deskOffer,
  deskContent,
  deskPlacement,
  deskAsk,
  deskMcp,
]

export function staticDeskParagraphs() {
  return `<p class="desk-label">${deskLabel}</p>
      <p>${deskWorks}</p>
      <p>${deskComing}</p>
      <div class="desk-stage">
        <div class="desk-offer">
          <p>${deskOffer}</p>
        </div>
        <div class="desk-article">
          <p class="desk-article-title">${deskContent}</p>
          <div class="desk-bars" aria-hidden="true"><span></span><span></span><span></span></div>
          <div class="desk-inset"><p>${deskPlacement}</p></div>
          <div class="desk-bars" aria-hidden="true"><span></span><span></span></div>
        </div>
      </div>
      <p>${deskAsk}</p>
      <p>${deskMcp}</p>`
}
