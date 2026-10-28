// What the directory section shows. A failed load is not an empty list.
// total 0 hides the marketplace chrome and leaves the add-yours path.

export function directoryFailureStatus(error) {
  if (!error || typeof error !== 'object') return 'network'
  if (error.status === 0 || error.message === 'network') return 'network'
  if (typeof error.status === 'number') return String(error.status)
  return 'network'
}

export function directoryFailureCopy(error) {
  return `Couldn't load the directory. ${directoryFailureStatus(error)}`
}

export function directoryChrome(view, total) {
  if (view === 'loading') return { heading: false, search: false, error: false, list: false, nomatch: false, add: false }
  if (view === 'error') return { heading: false, search: false, error: true, list: false, nomatch: false, add: true }
  if (total === 0) return { heading: false, search: false, error: false, list: false, nomatch: false, add: true }
  return {
    heading: view === 'results' || view === 'nomatch',
    search: view === 'results' || view === 'nomatch',
    error: false,
    list: view === 'results',
    nomatch: view === 'nomatch',
    add: view !== 'loading',
  }
}
