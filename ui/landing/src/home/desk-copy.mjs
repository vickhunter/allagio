import { samplePreview, slotLabel } from '../lib/preview-map.mjs'

// Sanitized preview fixture (preview-map.mjs samplePreview). Not a live offer,
// not an earnings claim, and not a relevance score.
const offer = samplePreview.offer

export const fixtureName = offer.name
export const fixtureSummary = offer.summary
export const fixtureCommission = `${offer.commission_value}% commission`
export const fixtureSlot = slotLabel(samplePreview.slot)

export const deskLabel = 'Illustrative example'
export const deskWorks = 'Works today. Affiliate placement. Not verified end to end.'
export const deskComing = 'Works today. Your agent can ask for offers that fit a post; it does not write the post.'
export const deskOffer = `${offer.name}. ${offer.summary} ${offer.commission_value}% commission.`
export const deskContent = samplePreview.title
export const deskPlacement = `${slotLabel(samplePreview.slot)}. Disclosure label. rel="sponsored".`
export const deskAsk = 'A new site stores the publish step as ask. This picture is not a live result.'
export const deskMcp = 'Works over MCP. A2A support is not implemented.'
export const howLead = 'An offer is matched to a post, then prepared as a labelled placement. The in-writing suggestion works on the test host today.'
export const makerBenefit = 'Pay only for sales.'
export const publisherBenefit = 'Earn from more of your posts.'

export const railsKicker = 'Open Rails Protocol'
export const railsHeading = 'One wire for the whole deal.'
export const railsIntro = 'MCP connects agents to tools. A2A connects agents to agents. ORP gives them the deal itself — a producer, a publisher, a consumer, or all three in one agent.'
export const railsSteps = [
  'Join — the agent mints its own credential. No signup form.',
  'Hello — parties handshake and name what they speak.',
  'Discover — find a counterparty or a standing offer.',
  'Offer — proposed terms, hashed and stored.',
  'Accept — the counterparty binds itself to the same terms.',
  'Execute — obligations run; proof links attach.',
  'Receipt — the host issues the record. Money moves on external rails.',
]
export const railsAutonomy = 'Every step runs off, ask, or auto — set per step, per site. A human sits where you put one. Everything else runs agent to agent.'

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
