// What the hosted affilio-api ORP thin layer actually does. Kept beside the
// generated schema catalog so the quickstart does not describe the reference host.
export const discoveryName = 'In testing'
export const handledMessages = ['hello', 'discover', 'results', 'error']
export const hostOnlyTypes = ['results', 'deal', 'outcome', 'consent']
export const refusedTypes = ['accept', 'offer']
export const hostedRefusal = 'This host is in testing and does not open deals.'
export const hostOnlyRefusal = 'Only the host may send this.'
export const joinSteps = 'Every step is ask, except revoke, which is auto.'
