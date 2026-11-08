import { samplePreview, slotLabel } from '../lib/preview-map.mjs'

// Sanitized preview fixture (preview-map.mjs samplePreview). Not a live offer,
// not an earnings claim, and not a relevance score.
const offer = samplePreview.offer

export const deskLabel = 'Illustrative example'
export const deskWorks = 'Works today. Affiliate placement. Not verified end to end.'
export const deskComing = 'Coming. Suggesting an offer while you write is not on the hosted server today.'
export const deskOffer = `${offer.name}. ${offer.summary} ${offer.commission_value}% commission.`
export const deskContent = samplePreview.title
export const deskPlacement = `${slotLabel(samplePreview.slot)}. Disclosure label. rel="sponsored".`
export const deskAsk = 'A new site stores the publish step as ask. This picture is not a live result.'
export const deskMcp = 'Works over MCP. A2A support is not implemented.'
export const howLead = 'An offer is matched to a post, then prepared as a labelled placement. The in-writing suggestion is Coming.'
export const makerBenefit = 'Pay only for sales.'
export const publisherBenefit = 'Earn from more of your posts.'

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
  return deskParagraphs.map((line) => `<p>${line}</p>`).join('\n      ')
}
