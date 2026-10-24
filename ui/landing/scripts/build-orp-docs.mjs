import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const repo = join(here, '..', '..', '..')
const schemaRoot = join(repo, 'docs/protocol/schema/0.3')
const examplesRoot = join(repo, 'docs/protocol/schema/examples')
const conformanceRoot = join(repo, 'docs/protocol/conformance/test')
const sdkRoot = join(repo, 'docs/protocol/sdk/src')

function isDir(path) {
  try {
    readdirSync(path)
    return true
  } catch {
    return false
  }
}

function files(dir, suffix, out = []) {
  for (const name of readdirSync(dir).sort()) {
    const path = join(dir, name)
    if (isDir(path)) files(path, suffix, out)
    else if (name.endsWith(suffix)) out.push(path)
  }
  return out
}

const readJson = (path) => JSON.parse(readFileSync(path, 'utf8'))

function summarize(node) {
  if (!node || typeof node !== 'object') return 'value'
  if (node.$ref) return node.$ref.split('/').pop()
  if (node.const !== undefined) return `const ${JSON.stringify(node.const)}`
  if (Array.isArray(node.enum)) return node.enum.map((item) => JSON.stringify(item)).join(' | ')
  if (node.type === 'array') return `array of ${summarize(node.items || {})}`
  const bits = []
  if (node.type) bits.push(Array.isArray(node.type) ? node.type.join(' | ') : node.type)
  if (node.minLength !== undefined) bits.push(`min length ${node.minLength}`)
  if (node.maxLength !== undefined) bits.push(`max length ${node.maxLength}`)
  if (node.minimum !== undefined) bits.push(`min ${node.minimum}`)
  if (node.maximum !== undefined) bits.push(`max ${node.maximum}`)
  if (node.minItems !== undefined) bits.push(`min items ${node.minItems}`)
  if (node.maxItems !== undefined) bits.push(`max items ${node.maxItems}`)
  if (node.pattern) bits.push('pattern')
  if (node.additionalProperties === false) bits.push('closed')
  return bits.join(', ') || 'value'
}

function rows(schema) {
  const required = new Set(schema.required || [])
  return Object.entries(schema.properties || {}).map(([name, prop]) => ({
    name,
    required: required.has(name),
    type: summarize(prop),
    description: typeof prop.description === 'string' ? prop.description : '',
  }))
}

function localDef(schema, ref) {
  if (!ref.startsWith('#/$defs/')) return null
  return schema.$defs?.[ref.slice('#/$defs/'.length)] || null
}

function referencedSections(schema, filePath) {
  const out = []
  for (const part of schema.allOf || []) {
    if (typeof part.$ref !== 'string' || part.$ref.startsWith('#')) continue
    const target = join(dirname(filePath), part.$ref.split('#')[0])
    let other
    try {
      other = readJson(target)
    } catch {
      continue
    }
    const fields = rows(other)
    if (!fields.length) continue
    out.push({
      name: other.title || 'Referenced schema',
      description: other.description || '',
      closed: other.additionalProperties === false,
      fields,
    })
  }
  return out
}

function sectionsFor(schema) {
  const sections = []
  const own = rows(schema)
  if (own.length) {
    sections.push({
      name: schema.title || 'Fields',
      description: schema.description || '',
      closed: schema.additionalProperties === false,
      fields: own,
    })
  }
  for (const [name, prop] of Object.entries(schema.properties || {})) {
    if (!prop.$ref) continue
    const def = localDef(schema, prop.$ref)
    if (!def) continue
    sections.push({
      name: def.title || name,
      description: def.description || '',
      closed: def.additionalProperties === false,
      fields: rows(def),
    })
  }
  if (!sections.length && Array.isArray(schema.allOf)) {
    sections.push({
      name: 'Composed schemas',
      description: schema.description || '',
      closed: false,
      fields: schema.allOf.map((part, index) => ({
        name: part.$ref || part.if?.properties?.type?.const || `part ${index + 1}`,
        required: Boolean(part.if),
        type: part.then?.$ref || part.$ref || 'if/then',
        description: '',
      })),
    })
  }
  if (!own.length && schema.$defs) {
    for (const [name, def] of Object.entries(schema.$defs)) {
      sections.push({
        name: def.title || name,
        description: def.description || '',
        closed: def.additionalProperties === false,
        fields: rows(def),
      })
    }
  }
  return sections
}

function slugFor(rel) {
  const base = rel.replace(/\.schema\.json$/, '').split('/').pop()
  return base
}

function kindFor(rel) {
  if (rel.startsWith('messages/')) return 'message'
  if (rel.startsWith('http/')) return 'http'
  return 'schema'
}

export function buildCatalog() {
  const schemaFiles = files(schemaRoot, '.schema.json')
  const messageExamples = files(join(examplesRoot, 'messages'), '.json').map((path) => ({
    name: relative(examplesRoot, path),
    value: readJson(path),
  }))
  const otherExamples = [
    ['discovery.json', 'discovery'],
    ['constraints.json', 'constraints'],
    ['inbox.json', 'inbox'],
  ].map(([name, slug]) => ({ name, slug, value: readJson(join(examplesRoot, name)) }))
  const httpExamples = files(join(examplesRoot, 'http'), '.json').map((path) => ({
    name: relative(examplesRoot, path),
    slug: relative(join(examplesRoot, 'http'), path).replace(/\.json$/, ''),
    value: readJson(path),
  }))
  const invalid = files(join(examplesRoot, 'invalid'), '.json').map((path) => ({
    name: relative(examplesRoot, path),
    value: readJson(path),
  }))

  const schemas = schemaFiles.map((path) => {
    const rel = relative(schemaRoot, path)
    const schema = readJson(path)
    const slug = slugFor(rel)
    const kind = kindFor(rel)
    const type = schema.properties?.type?.const || ''
    const valid = []
    if (kind === 'message' && type) {
      for (const example of messageExamples) {
        if (example.value.type === type) valid.push({ name: example.name, json: JSON.stringify(example.value, null, 2) })
      }
    }
    if (slug === 'discovery' || slug === 'constraints' || slug === 'inbox') {
      for (const example of otherExamples) {
        if (example.slug === slug) valid.push({ name: example.name, json: JSON.stringify(example.value, null, 2) })
      }
    }
    if (kind === 'http') {
      for (const example of httpExamples) {
        if (example.slug === slug) valid.push({ name: example.name, json: JSON.stringify(example.value, null, 2) })
      }
    }
    const bad = []
    for (const example of invalid) {
      const instance = example.value.instance
      const named = example.value.schema
      const matchesType = kind === 'message' && type && instance && instance.type === type
      const matchesSchema = named === slug || (kind === 'message' && named === 'message' && matchesType) || (slug === 'message' && named === 'message')
      if (!matchesType && !matchesSchema) continue
      if (slug === 'message' && named === 'message' && !matchesType && kind !== 'schema') continue
      if (matchesType || (named === slug) || (slug === 'message' && named === 'message')) {
        bad.push({
          name: example.name,
          why: example.value.why,
          json: JSON.stringify(instance, null, 2),
        })
      }
    }
    const seenBad = new Set()
    const invalidExamples = bad.filter((item) => {
      if (seenBad.has(item.name)) return false
      seenBad.add(item.name)
      return true
    })
    return {
      file: rel,
      id: schema.$id,
      slug,
      href: `/protocol/spec/${slug}/`,
      title: schema.title || slug,
      description: schema.description || '',
      kind,
      type,
      required: schema.required || [],
      anyOf: (schema.anyOf || []).map((branch) => branch.required || []),
      sections: [...referencedSections(schema, path), ...sectionsFor(schema)],
      valid,
      invalid: invalidExamples,
    }
  })

  const tests = []
  for (const path of files(conformanceRoot, '.test.mjs')) {
    const text = readFileSync(path, 'utf8')
    for (const match of text.matchAll(/test\('([^']+)'/g)) tests.push(match[1])
  }

  return {
    version: '0.3',
    license: 'Apache-2.0 planned',
    schemas,
    conformance: tests,
    sdk: files(sdkRoot, '.mjs').map((path) => relative(join(repo, 'docs/protocol'), path)),
    sdkReadme: readFileSync(join(repo, 'docs/protocol/sdk/README.md'), 'utf8').trim(),
    exampleCount: messageExamples.length + otherExamples.length + httpExamples.length,
    invalidCount: invalid.length,
  }
}

function emit(catalog) {
  return `// Generated by ui/landing/scripts/build-orp-docs.mjs from docs/protocol. Do not edit.\nexport const catalog = ${JSON.stringify(catalog, null, 2)}\n`
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  const out = join(here, '..', 'src/protocol/generated/catalog.mjs')
  writeFileSync(out, emit(buildCatalog()))
}
