// What the hosted service actually does. Kept beside the generated schema catalog.
export const discoveryName = 'In testing'
export const handledMessages = ['hello', 'discover', 'results', 'offer', 'accept', 'deal', 'execute', 'receipt', 'error']
export const hostOnlyTypes = ['results', 'deal', 'outcome', 'consent']
export const hostOnlyRefusal = 'Only the host may send this.'
export const joinSteps = 'Every step is ask, except revoke, which is auto.'
export const noMoney = 'Offer Rails does not hold money.'
export const hostSpeechSpeaks = 'This host speaks ORP 0.4 (leave and seat).'
export const hostSpeechUnknown = 'What this host speaks is not shown here until /.well-known/orp.json loads.'
export const receiptRefusal = 'Only the host issues a receipt.'
export const otherRefusal = 'This host is in testing and does not run this message.'
export const payRefusal = 'This testing host does not settle money.'

function stringList(value) {
  if (!Array.isArray(value) || value.length === 0) return null
  const items = []
  for (const item of value) {
    if (typeof item !== 'string' && typeof item !== 'number') return null
    items.push(String(item))
  }
  return items
}

// hostDiscovery reads orp.json. A missing list stays null. Callers then show
// the unknown line only, and do not fill in a version.
export function hostDiscovery(doc) {
  if (!doc || typeof doc !== 'object') return null
  const versions = stringList(doc.orp)
  const caps = doc.capabilities
  const messages = caps && typeof caps === 'object' ? stringList(caps.messages) : null
  if (!versions || !messages) return null
  const legacy = doc.legacy && typeof doc.legacy === 'object' ? doc.legacy : null
  const dcp = legacy && legacy.dcp && typeof legacy.dcp === 'object' ? legacy.dcp : null
  return {
    versions,
    messages,
    name: typeof doc.name === 'string' ? doc.name : '',
    bearer: Boolean(doc.auth && typeof doc.auth === 'object' && doc.auth.bearer === true),
    conformance: Object.prototype.hasOwnProperty.call(doc, 'conformance'),
    speaks04: versions.includes('0.4') && messages.includes('leave') && messages.includes('seat'),
    endpoint: typeof doc.endpoint === 'string' ? doc.endpoint : '',
    suggest: typeof doc.suggest === 'string' ? doc.suggest : '',
    join: typeof doc.join === 'string' ? doc.join : '',
    dcp,
  }
}

// hostSpeech reads a discovery document. A missing or partial document stays
// neutral. The 0.4 label is returned only when that document lists 0.4 and
// the messages leave and seat. Any other document stays on the unknown line.
export function hostSpeech(doc) {
  const found = hostDiscovery(doc)
  if (!found || !found.speaks04) return hostSpeechUnknown
  return hostSpeechSpeaks
}

function named(found) {
  return found.name ? `Discovery names it “${found.name}”. ` : ''
}

export function hostOverview(doc) {
  const found = hostDiscovery(doc)
  if (!found) return hostSpeechUnknown
  const label = found.speaks04 ? `${hostSpeechSpeaks} ` : ''
  return `${label}The hosted service speaks a thin layer. ${named(found)}orp lists ${found.versions.join(', ')}. It answers ${found.messages.join(', ')}. It does not claim conformance levels L0, L1, L2, or L3. ${noMoney}`
}

export function hostQuickstartDiscover(doc) {
  const found = hostDiscovery(doc)
  if (!found) return hostSpeechUnknown
  const name = found.name ? `The name is “${found.name}”. ` : ''
  const routes = [
    found.endpoint ? `endpoint is ${found.endpoint}` : '',
    found.suggest ? `suggest is ${found.suggest}` : '',
    found.join ? `join is ${found.join}` : '',
  ].filter(Boolean)
  const routeSentence = routes.length ? `${routes.join(', ')}. ` : ''
  const auth = `auth.bearer is ${found.bearer ? 'true' : 'false'}. `
  const messages = `capabilities.messages is ${found.messages.join(', ')}. `
  const level = found.conformance ? '' : 'conformance is omitted, so this host claims no level. '
  let legacy = ''
  if (found.dcp) {
    const discovery = typeof found.dcp.discovery === 'string' ? found.dcp.discovery : ''
    const http = typeof found.dcp.http_base === 'string' ? found.dcp.http_base : ''
    const version = typeof found.dcp.version === 'string' ? found.dcp.version : ''
    const points = [discovery, http].filter(Boolean)
    if (points.length) legacy += `legacy.dcp points at ${points.join(' and ')}. `
    if (version) legacy += `DCP v${version} remains the live affiliate path.`
  }
  return `GET /.well-known/orp.json needs no credential. ${name}orp lists ${found.versions.join(', ')}. ${routeSentence}${auth}${messages}${level}${legacy}`.replace(/[ \t]+/g, ' ').trim()
}

export function hostQuickstartHello(doc) {
  const found = hostDiscovery(doc)
  if (!found) return hostSpeechUnknown
  const version = found.versions.length === 1
    ? `Hello returns one version, ${found.versions[0]}.`
    : `Hello returns one version from the orp list: ${found.versions.join(', ')}.`
  return `POST /orp with Authorization: Bearer and the token from join. Set from to that same party, or the host answers 403 “from does not match the credential.” ${version} Discover returns results. Discover with want set to spaces returns no items. The samples below are valid requests from the spec.`
}

export function hostQuickstartOffer(doc) {
  const found = hostDiscovery(doc)
  if (!found) return hostSpeechUnknown
  const label = found.speaks04 ? `${hostSpeechSpeaks} ` : ''
  return `${label}This host handles ${found.messages.join(', ')}.`
}
