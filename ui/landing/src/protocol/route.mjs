export function routeFor(pathname) {
  const path = String(pathname || '/').split('?')[0].split('#')[0]
  const marker = '/protocol'
  const at = path.indexOf(marker)
  const rest = (at === -1 ? path : path.slice(at + marker.length)).replace(/\/+$/, '')
  const parts = rest.split('/').filter(Boolean)
  if (!parts.length) return { name: 'overview', slug: '' }
  if (parts[0] === 'concepts') return { name: 'concepts', slug: '' }
  if (parts[0] === 'quickstart') return { name: 'quickstart', slug: '' }
  if (parts[0] === 'sdk') return { name: 'sdk', slug: '' }
  if (parts[0] === 'conformance') return { name: 'conformance', slug: '' }
  if (parts[0] === 'spec' && parts[1]) return { name: 'spec', slug: decodeURIComponent(parts[1]) }
  if (parts[0] === 'spec') return { name: 'spec-index', slug: '' }
  return { name: 'missing', slug: '' }
}
