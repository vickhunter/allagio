import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import test from 'node:test'
import { mcpConnectionName } from '../lib/setup.mjs'

const root = resolve(import.meta.dirname, '../../..')

const files = [
  'packages/distributoor-setup/lib/setup.mjs',
  'packages/distributoor-setup/bin/setup.mjs',
  'distributoor-plugin/mcp.json',
  'distributoor-plugin/skills/distributoor-revenue/SKILL.md',
  'distributoor-plugin/skills/distributoor-onboard/SKILL.md',
  'cli/README.md',
]

test('MCP connection name is one value', () => {
  assert.equal(mcpConnectionName, 'allagio')
  for (const rel of files) {
    const text = readFileSync(resolve(root, rel), 'utf8')
    assert.equal(text.includes(mcpConnectionName), true, rel)
  }
  const bin = readFileSync(resolve(root, 'packages/distributoor-setup/bin/setup.mjs'), 'utf8')
  const lib = readFileSync(resolve(root, 'packages/distributoor-setup/lib/setup.mjs'), 'utf8')
  assert.equal(bin.includes("'login', 'distributoor'"), false)
  assert.equal(bin.includes('mcp_servers.distributoor'), false)
  assert.equal(lib.includes('mcp_servers.${mcpConnectionName}'), true)
  const plugin = JSON.parse(readFileSync(resolve(root, 'distributoor-plugin/mcp.json'), 'utf8'))
  assert.equal(plugin.mcpServers[mcpConnectionName].command, 'allagio-mcp')
  const revenue = readFileSync(resolve(root, 'distributoor-plugin/skills/distributoor-revenue/SKILL.md'), 'utf8')
  assert.match(revenue, /MCP connection \(`allagio`\)/)
})
