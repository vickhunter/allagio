// What the directory section shows. A non-200 load is an error.
// A 200 with total 0 is the empty list: the empty sentence, and no search box.

export function directoryFailureStatus(error) {
  if (!error || typeof error !== 'object') return 'network'
  if (error.status === 0 || error.message === 'network') return 'network'
  if (typeof error.status === 'number') return String(error.status)
  return 'network'
}

export function directoryFailureCopy() {
  return 'The list could not load.'
}

export function directoryEmptyCopy(kind) {
  if (kind === 'maker') return 'No makers listed yet. Yours can be first. Add your product below. That step needs an MCP client.'
  return 'No publishers listed yet. Add your site or newsletter below. An agent does that step for now.'
}

export function directoryChrome(view, total) {
  if (view === 'loading') return { heading: false, search: false, error: false, empty: false, list: false, nomatch: false, add: false }
  if (view === 'error') return { heading: true, search: true, error: true, empty: false, list: false, nomatch: false, add: true }
  if (view === 'empty') return { heading: false, search: false, error: false, empty: true, list: false, nomatch: false, add: true }
  return {
    heading: view === 'results' || view === 'nomatch',
    search: view === 'results' || view === 'nomatch',
    error: false,
    empty: false,
    list: view === 'results',
    nomatch: view === 'nomatch',
    add: view !== 'loading',
  }
}
