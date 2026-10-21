import { readFileSync, readdirSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const SCHEMA_DIR = dirname(fileURLToPath(import.meta.url))
export const DRAFT_2020_12 = 'https://json-schema.org/draft/2020-12/schema'

const RFC3339_REGEX = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/
const URI_REGEX = /^[a-zA-Z][a-zA-Z0-9+.-]*:[^\s]+$/

/**
 * Loads all JSON Schema (2020-12) files from the schema/ directory.
 */
export function loadSchemas(dir = SCHEMA_DIR) {
  const registry = new Map()
  for (const file of readdirSync(dir)) {
    if (!file.endsWith('.schema.json')) continue
    const raw = JSON.parse(readFileSync(join(dir, file), 'utf8'))
    registry.set(file, raw)
    if (raw.$id) registry.set(raw.$id, raw)
  }
  return registry
}

function resolveRef(ref, currentSchema, registry) {
  const [filePart, pointerPart] = ref.split('#')
  const targetSchema = filePart ? registry.get(filePart) : currentSchema
  if (!targetSchema) {
    throw new Error(`Unresolved schema reference: ${ref}`)
  }
  if (!pointerPart || pointerPart === '' || pointerPart === '/') {
    return { schema: targetSchema, root: targetSchema }
  }
  const segments = pointerPart.replace(/^\//, '').split('/')
  let node = targetSchema
  for (const seg of segments) {
    if (node == null || typeof node !== 'object' || !(seg in node)) {
      throw new Error(`Unresolved JSON pointer #${pointerPart} in ${filePart || currentSchema.$id}`)
    }
    node = node[seg]
  }
  return { schema: node, root: targetSchema }
}

/**
 * Validates a value against a JSON Schema (Draft 2020-12).
 * Returns { valid: boolean, errors: string[] }.
 */
export function validateWithSchema(value, schemaOrName, registry = loadSchemas()) {
  const rootSchema = typeof schemaOrName === 'string' ? registry.get(schemaOrName) : schemaOrName
  if (!rootSchema) {
    throw new Error(`Unknown schema: ${schemaOrName}`)
  }
  const errors = []
  validateNode(value, rootSchema, rootSchema, registry, '$', errors)
  return { valid: errors.length === 0, errors }
}

function validateNode(value, schema, rootSchema, registry, path, errors) {
  if (schema.$ref) {
    const resolved = resolveRef(schema.$ref, rootSchema, registry)
    validateNode(value, resolved.schema, resolved.root, registry, path, errors)
    return
  }

  if (Array.isArray(schema.oneOf)) {
    let matches = 0
    const branchErrors = []
    for (const sub of schema.oneOf) {
      const subErrs = []
      validateNode(value, sub, rootSchema, registry, path, subErrs)
      if (subErrs.length === 0) matches++
      else branchErrors.push(subErrs.join('; '))
    }
    if (matches !== 1) {
      errors.push(`${path}: value must match exactly one schema in oneOf (matched ${matches})`)
    }
    return
  }

  if ('const' in schema) {
    if (value !== schema.const) {
      errors.push(`${path}: expected const ${JSON.stringify(schema.const)}, got ${JSON.stringify(value)}`)
      return
    }
  }

  if (Array.isArray(schema.enum)) {
    if (!schema.enum.includes(value)) {
      errors.push(`${path}: value ${JSON.stringify(value)} not in enum [${schema.enum.join(', ')}]`)
      return
    }
  }

  if (schema.type) {
    if (!checkType(value, schema.type)) {
      errors.push(`${path}: expected type ${schema.type}, got ${describeType(value)}`)
      return
    }
  }

  if (typeof value === 'string') {
    if (typeof schema.minLength === 'number' && value.length < schema.minLength) {
      errors.push(`${path}: string length ${value.length} < minLength ${schema.minLength}`)
    }
    if (typeof schema.maxLength === 'number' && value.length > schema.maxLength) {
      errors.push(`${path}: string length ${value.length} > maxLength ${schema.maxLength}`)
    }
    if (typeof schema.pattern === 'string') {
      const re = new RegExp(schema.pattern)
      if (!re.test(value)) {
        errors.push(`${path}: string ${JSON.stringify(value)} does not match pattern ${schema.pattern}`)
      }
    }
    if (schema.format === 'date-time' && !RFC3339_REGEX.test(value)) {
      errors.push(`${path}: invalid RFC3339 date-time ${JSON.stringify(value)}`)
    }
    if (schema.format === 'uri' && !URI_REGEX.test(value)) {
      errors.push(`${path}: invalid URI ${JSON.stringify(value)}`)
    }
  }

  if (typeof value === 'number') {
    if (typeof schema.minimum === 'number' && value < schema.minimum) {
      errors.push(`${path}: number ${value} < minimum ${schema.minimum}`)
    }
    if (typeof schema.maximum === 'number' && value > schema.maximum) {
      errors.push(`${path}: number ${value} > maximum ${schema.maximum}`)
    }
  }

  if (Array.isArray(value)) {
    if (typeof schema.minItems === 'number' && value.length < schema.minItems) {
      errors.push(`${path}: array length ${value.length} < minItems ${schema.minItems}`)
    }
    if (typeof schema.maxItems === 'number' && value.length > schema.maxItems) {
      errors.push(`${path}: array length ${value.length} > maxItems ${schema.maxItems}`)
    }
    if (schema.items) {
      value.forEach((item, idx) => {
        validateNode(item, schema.items, rootSchema, registry, `${path}[${idx}]`, errors)
      })
    }
  }

  if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
    if (Array.isArray(schema.required)) {
      for (const reqKey of schema.required) {
        if (!(reqKey in value) || value[reqKey] === undefined) {
          errors.push(`${path}: missing required property '${reqKey}'`)
        }
      }
    }
    const props = schema.properties || {}
    for (const [k, v] of Object.entries(value)) {
      if (k in props) {
        validateNode(v, props[k], rootSchema, registry, `${path}.${k}`, errors)
      } else if (schema.additionalProperties === false) {
        errors.push(`${path}: unexpected additional property '${k}'`)
      }
    }
  }
}

function checkType(value, expected) {
  switch (expected) {
    case 'string':
      return typeof value === 'string'
    case 'integer':
      return typeof value === 'number' && Number.isInteger(value)
    case 'number':
      return typeof value === 'number' && Number.isFinite(value)
    case 'boolean':
      return typeof value === 'boolean'
    case 'array':
      return Array.isArray(value)
    case 'object':
      return value !== null && typeof value === 'object' && !Array.isArray(value)
    default:
      return true
  }
}

function describeType(value) {
  if (value === null) return 'null'
  if (Array.isArray(value)) return 'array'
  if (typeof value === 'number' && Number.isInteger(value)) return 'integer'
  return typeof value
}
