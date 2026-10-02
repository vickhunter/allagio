#!/usr/bin/env node
import { parseArgs } from 'node:util';
import { resolve } from 'node:path';
import { validateAPI, detectProject, checkServer, mergeMCP, installSkills, chooseClient, available, run, codexConfig } from '../lib/setup.mjs';

try {
  const { values } = parseArgs({ options: {
    'api-url': { type: 'string' }, client: { type: 'string', default: 'auto' },
    project: { type: 'string', default: process.cwd() }, website: { type: 'string' },
    'install-only': { type: 'boolean' }, offline: { type: 'boolean' }, help: { type: 'boolean' }
  } });
  if (values.help) {
    console.log('distributoor-setup --api-url https://pilot.example.com [--client auto|codex|cursor|claude|skill] [--website https://your-product.com] [--project PATH] [--install-only] [--offline]');
  } else {
    if (!values['api-url']) throw new Error('--api-url is required until a public hosted pilot is available.');
    const api = validateAPI(values['api-url']);
    if (values.website) {
      const website = new URL(values.website);
      if (website.protocol !== 'https:' || website.username || website.password) throw new Error('--website must be a public HTTPS product URL without credentials.');
    }
    if (!['auto', 'codex', 'cursor', 'claude', 'skill'].includes(values.client)) throw new Error('Unsupported client. Use codex, cursor, claude or skill.');
    const root = resolve(values.project);
    const client = await chooseClient(root, values.client);
    const project = await detectProject(root);
    console.log(`Detected ${project.frameworks.join(', ') || 'an unclassified project'}; payment libraries: ${project.providers.join(', ') || 'none detected'}.`);
    const resource = values.offline ? `${api}/mcp` : await checkServer(api);
    if (client === 'codex' && !values['install-only'] && !values.offline && !await available('codex')) throw new Error('Install Codex CLI first, or choose --client skill.');
    if (client === 'claude' || client === 'cursor') console.log(`Connection installed: ${await mergeMCP(root, client, resource)}`);
    const paths = await installSkills(root, client, { api, project, website: values.website });
    console.log(`Installed ${paths.length} skills for ${client}. Source files and secrets have not been uploaded by this installer.`);
    if (values.offline || values['install-only'] || !process.stdin.isTTY) {
      console.log('Agent setup installed. Product integration and paid conversion verification are still pending.');
      if (client === 'codex') console.log(`Launch: codex -c 'mcp_servers.distributoor={url=${JSON.stringify(resource)}}' 'Use distributoor-onboard to set up this product.'`);
      else console.log(`In your agent, connect ${resource} with maker access and ask: Use distributoor-onboard to set up this product.`);
      if (values.offline) console.log('Offline preparation only: the hosted connection has not been verified.');
    } else if (client === 'codex') {
      const configuration = codexConfig(resource);
      console.log('Sign in once and allow maker access. Your existing Codex account powers the integration.');
      await run('codex', [...configuration, 'mcp', 'login', 'distributoor', '--scopes', 'maker', '--oauth-client-registration', 'dcr', '--oauth-resource', resource], root);
      await run('codex', [...configuration, 'Use distributoor-onboard to set up this product. Continue through detection, registration, attribution wiring, tests and assessment; report any remaining owner action.'], root);
    } else if (client === 'claude' && await available('claude')) {
      await run('claude', ['Use distributoor-onboard to set up this product. Connect the Distributoor MCP server with maker access, then perform integration and verification.'], root);
    } else {
      console.log(`Open ${client === 'cursor' ? 'Cursor' : 'your coding agent'} and ask: Use distributoor-onboard to set up this product. Your agent handles the remaining integration using ${resource}.`);
    }
  }
} catch (error) {
  console.error(`Setup stopped: ${error.message}`);
  process.exitCode = 1;
}
