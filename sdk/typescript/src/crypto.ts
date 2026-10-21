import { createHash } from 'node:crypto'
import type { DealTerms, Sha256Hash } from './types.ts'

/**
 * Serializes any JSON value into RFC 8785 Canonical JSON (JCS) with sorted object keys.
 */
export function canonicalizeJson(value: unknown): string {
  if (value === null || typeof value !== 'object') {
    return JSON.stringify(value)
  }
  if (Array.isArray(value)) {
    return '[' + value.map((item) => canonicalizeJson(item)).join(',') + ']'
  }
  const obj = value as Record<string, unknown>
  const keys = Object.keys(obj)
    .filter((k) => obj[k] !== undefined)
    .sort()
  const pairs = keys.map((k) => `${JSON.stringify(k)}:${canonicalizeJson(obj[k])}`)
  return '{' + pairs.join(',') + '}'
}

/**
 * Computes a deterministic SHA-256 hash (`sha256:<hex>`) over RFC 8785 canonical JSON.
 */
export function computeSha256Hash(value: unknown): Sha256Hash {
  const canonical = typeof value === 'string' ? value : canonicalizeJson(value)
  const hex = createHash('sha256').update(canonical, 'utf8').digest('hex')
  return `sha256:${hex}`
}

/**
 * Computes the normative `terms_hash` for a `DealTerms` object.
 */
export function computeTermsHash(terms: DealTerms): Sha256Hash {
  return computeSha256Hash(terms)
}

/**
 * Mints the universal tracked link (`/v1/go/:token`) and deal tracking token (`orp_tok_...`).
 * The tracked link works on any platform (email, X, YouTube, Instagram, blog, podcast)
 * without requiring any platform integration or plugin.
 */
export function mintUniversalTracking(baseUrl: string, dealId: string): {
  tracking_url: string
  tracking_token: string
} {
  const cleanId = dealId.replace(/[^a-zA-Z0-9_-]/g, '')
  const shortHash = createHash('sha256').update(cleanId).digest('hex').slice(0, 10)
  const token = `trk_${shortHash}`
  const normalizedBase = baseUrl.replace(/\/+$/, '')
  return {
    tracking_url: `${normalizedBase}/v1/go/${token}`,
    tracking_token: `orp_tok_${shortHash}`,
  }
}

/**
 * Performs an unauthenticated `public_check` verification against observed page/post content.
 * Confirms that either the universal tracked link, tracking token, or target URI is present,
 * AND that a required disclosure marker is visible.
 */
export function verifyPublicContentReceipt(params: {
  observedContent: string
  expectedTargets: string[]
  requiredDisclosureMarkers: string[]
}): {
  targetMatched: string | null
  disclosureVerified: boolean
  observedDisclosure: string | null
  contentHash: Sha256Hash
} {
  const { observedContent, expectedTargets, requiredDisclosureMarkers } = params
  const lower = observedContent.toLowerCase()

  let targetMatched: string | null = null
  for (const target of expectedTargets) {
    if (target && observedContent.includes(target)) {
      targetMatched = target
      break
    }
  }

  let observedDisclosure: string | null = null
  for (const marker of requiredDisclosureMarkers) {
    if (marker && lower.includes(marker.toLowerCase())) {
      observedDisclosure = marker
      break
    }
  }

  return {
    targetMatched,
    disclosureVerified: observedDisclosure !== null,
    observedDisclosure,
    contentHash: computeSha256Hash(observedContent),
  }
}
