import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { loadSchemas, validateWithSchema } from '../../../schema/validator.mjs'
import { assertNever, type OrpMessage } from './types.ts'

const SCHEMA_DIR = resolve(dirname(fileURLToPath(import.meta.url)), '../../../schema')
const registry = loadSchemas(SCHEMA_DIR)

/**
 * Validates any ORP v0.3 message against its JSON Schema (Draft 2020-12) definition.
 */
export function validateOrpMessage(message: OrpMessage): {
  valid: boolean
  errors: string[]
} {
  let schemaName: string
  switch (message.message_type) {
    case 'discover':
      schemaName = 'discover.schema.json'
      break
    case 'offer':
      schemaName = 'offer.schema.json'
      break
    case 'ask':
    case 'counter':
      schemaName = 'counter.schema.json'
      break
    case 'accept':
      schemaName = 'accept.schema.json'
      break
    case 'consent':
      schemaName = 'consent.schema.json'
      break
    case 'execute':
      schemaName = 'execute.schema.json'
      break
    case 'receipt':
      schemaName = 'receipt.schema.json'
      break
    case 'revoke':
      schemaName = 'revoke.schema.json'
      break
    case 'dispute':
      schemaName = 'dispute.schema.json'
      break
    case 'reputation':
      schemaName = 'reputation.schema.json'
      break
    default:
      return assertNever(message)
  }

  return validateWithSchema(message, schemaName, registry)
}
