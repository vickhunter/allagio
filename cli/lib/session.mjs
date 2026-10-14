import { dashboardURL } from './api.mjs';

export const SESSION_ONLY = [
  { command: 'loosen', tool: 'loosen_mode', label: 'loosen the approval mode' },
  { command: 'raise-cap', tool: 'raise_cap', label: 'raise a cap or lower minimum relevance' },
  { command: 'resume', tool: 'resume', label: 'turn the kill switch off' },
  { command: 'account-key', tool: 'account_key', label: 'create an account-wide key' },
  { command: 'plugin-key', tool: 'plugin_key', label: 'generate, rotate, or revoke a plugin key' },
  { command: 'restore', tool: 'restore_placement', label: 'restore a placement' },
];

export function sessionOnlyResult(label, env) {
  const link = dashboardURL(env);
  return {
    message: `Sign in again to ${label} from the dashboard: ${link}`,
    sign_in_url: link,
  };
}

export function printSessionOnly(label, io) {
  const result = sessionOnlyResult(label, io.env);
  io.stdout(`${result.message}\n`);
  io.stdout(`sign_in_url: ${result.sign_in_url}\n`);
  return { exitCode: 0, ...result };
}
