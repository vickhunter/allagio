import { defaultPublicBaseURL } from '../public-base.mjs'

// The page origin is the public host the browser is on. Node, and a page
// with no location, use the one default shared with the Go API.
export function publicOrigin() {
  if (typeof window !== 'undefined' && window.location && window.location.origin && window.location.origin !== 'null') {
    return window.location.origin.replace(/\/$/, '')
  }
  return defaultPublicBaseURL
}
