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

// hostSpeech reads a discovery document. A missing or partial document stays
// neutral. It does not claim the host is silent unless that document loaded
// and its version list and message list are both present and both omit 0.4.
export function hostSpeech(doc) {
  if (!doc || typeof doc !== 'object') return hostSpeechUnknown
  const versions = Array.isArray(doc.orp) ? doc.orp.map((item) => String(item)) : null
  const caps = doc.capabilities
  const messages = caps && typeof caps === 'object' && Array.isArray(caps.messages) ? caps.messages.map((item) => String(item)) : null
  if (!versions || !messages) return hostSpeechUnknown
  if (versions.includes('0.4') && messages.includes('leave') && messages.includes('seat')) return hostSpeechSpeaks
  return hostSpeechUnknown
}
