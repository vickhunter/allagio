export function routeFor(pathname) {
  const path = String(pathname || '/').split('?')[0].split('#')[0]
  const marker = '/protocol'
  const at = path.indexOf(marker)
  const rest = (at === -1 ? path : path.slice(at + marker.length)).replace(/\/+$/, '')
  const parts = rest.split('/').filter(Boolean)
  if (!parts.length) return { name: 'overview', version: '', slug: '' }
  if (parts[0] === 'concepts') return { name: 'concepts', version: '', slug: '' }
  if (parts[0] === 'many') return { name: 'many', version: '0.4', slug: '' }
  if (parts[0] === 'quickstart') return { name: 'quickstart', version: '', slug: '' }
  if (parts[0] === 'sdk') return { name: 'sdk', version: '', slug: '' }
  if (parts[0] === 'conformance') return { name: 'conformance', version: '', slug: '' }
  if (parts[0] === 'spec' && /^\d+\.\d+$/.test(parts[1] || '') && parts[2]) {
    return { name: 'spec', version: parts[1], slug: decodeURIComponent(parts[2]) }
  }
  if (parts[0] === 'spec' && parts[1]) return { name: 'spec', version: '0.3', slug: decodeURIComponent(parts[1]) }
  if (parts[0] === 'spec') return { name: 'spec-index', version: '', slug: '' }
  return { name: 'missing', version: '', slug: '' }
}
