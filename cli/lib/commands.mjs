import { apiBase, apiRequest } from './api.mjs';
import { authHeaders, loadConfig, offerFromScope, saveConfig, siteFromScope } from './config.mjs';
import { formatPayload, scanTarget } from './scan.mjs';
import { printSessionOnly } from './session.mjs';

export async function loginCommand(flags, io) {
  if (flags.agent) return agentLogin(flags, io);
  const code = await apiRequest(io, '/v1/device/code', { method: 'POST', body: {} });
  if (code.status !== 200 || !code.json.device_code) {
    io.stderr(`device code failed (${code.status})\n`);
    return { exitCode: 1 };
  }
  io.stdout(`user_code: ${code.json.user_code}\n`);
  io.stdout(`verification_uri: ${code.json.verification_uri}\n`);
  let wait = Number(code.json.interval);
  if (!Number.isFinite(wait) || wait < 0) wait = 5;
  const deadline = Date.now() + (Number(code.json.expires_in) || 300) * 1000;
  while (Date.now() < deadline + wait * 1000) {
    await io.sleep(wait * 1000);
    const poll = await apiRequest(io, '/v1/device/token', {
      method: 'POST',
      body: { device_code: code.json.device_code, grant_type: 'urn:ietf:params:oauth:grant-type:device_code' },
    });
    if (poll.status === 200 && poll.json.access_token) {
      const config = loadConfig(io);
      saveConfig(io, {
        ...config,
        api_url: apiBase(io.env),
        access_token: poll.json.access_token,
        key_header: poll.json.key_header,
        scope: poll.json.scope,
        human_login: true,
        account_kind: 'person',
      });
      io.stdout('A person approved the device code.\n');
      io.stdout(`scope: ${poll.json.scope}\n`);
      io.stdout(`key_header: ${poll.json.key_header}\n`);
      return { exitCode: 0 };
    }
    const error = poll.json && poll.json.error;
    if (error === 'authorization_pending') {
      wait = Number(poll.json.interval) || wait;
      continue;
    }
    if (error === 'slow_down') {
      wait = Number(poll.json.interval) || wait + 5;
      continue;
    }
    if (error === 'access_denied') {
      io.stdout('The device code was denied.\n');
      return { exitCode: 1 };
    }
    if (error === 'expired') {
      io.stdout('The device code expired.\n');
      return { exitCode: 1 };
    }
    io.stderr(`login failed (${poll.status})\n`);
    return { exitCode: 1 };
  }
  io.stdout('The device code expired.\n');
  return { exitCode: 1 };
}

async function agentLogin(flags, io) {
  const role = flags.role === 'publisher' ? 'publisher' : 'maker';
  if (!flags.name || !flags.email) {
    io.stderr('login --agent needs --name and --email\n');
    return { exitCode: 1 };
  }
  const body = { role, name: flags.name, email: flags.email, source: 'cli' };
  if (flags.domain) body.domain = flags.domain;
  const res = await apiRequest(io, '/v1/agent-accounts', { method: 'POST', body });
  if (res.status !== 201 || !res.json.api_key) {
    io.stderr(`agent account was not created (${res.status})\n`);
    return { exitCode: 1 };
  }
  const scope = res.json.maker_id ? `maker:${res.json.maker_id}` : `site:${res.json.site_id}`;
  const config = loadConfig(io);
  saveConfig(io, {
    ...config,
    api_url: apiBase(io.env),
    access_token: res.json.api_key,
    key_header: res.json.key_header,
    scope,
    human_login: false,
    account_kind: 'agent',
  });
  io.stdout('An agent alone created the account.\n');
  io.stdout(`scope: ${scope}\n`);
  io.stdout(`key_header: ${res.json.key_header}\n`);
  return { exitCode: 0 };
}

export async function scanCommand(target, io, options = {}) {
  const payload = await scanTarget(target || '.', io);
  io.stdout('--- payload ---\n');
  io.stdout(`${formatPayload(payload)}\n`);
  io.stdout('--- end ---\n');
  if (options.confirm === false) {
    io.stdout('Not sent. Confirm to upload this listing card.\n');
    return { exitCode: 0, payload, sent: false };
  }
  const config = loadConfig(io);
  let send = options.confirm === true || io.env.OFFER_RAILS_YES === '1' || config.upload_confirmed === true;
  if (!send) {
    io.stdout('Send this listing card? [y/N]\n');
    const line = String(await io.readLine() || '').trim().toLowerCase();
    send = line === 'y' || line === 'yes';
  }
  if (!send) {
    io.stdout('Not sent.\n');
    return { exitCode: 0, payload, sent: false };
  }
  const res = await apiRequest(io, '/v1/listing-cards', {
    method: 'POST',
    body: payload,
    headers: authHeaders(config),
  });
  if (res.status !== 201 || !res.json.id) {
    io.stderr(`listing card was not stored (${res.status})\n`);
    io.stdout(`${JSON.stringify(res.json)}\n`);
    return { exitCode: 1, payload, sent: false };
  }
  saveConfig(io, { ...loadConfig(io), upload_confirmed: true, listing_card_id: res.json.id, last_scan: { score: payload.score } });
  io.stdout(`listing_card: ${res.json.id}\n`);
  return { exitCode: 0, payload, sent: true, id: res.json.id };
}

function textField(value) {
  return typeof value === 'string' && value.trim() ? value.trim() : '';
}

// reviewFailure reads the check outcome off a submit (or readiness refusal)
// response. On a listed offer it is an advisory flag, not a hold: the offer
// went live anyway and the maker should still see what to fix.
function reviewFailure(body) {
  if (!body || typeof body !== 'object') return null;
  const check = textField(body.review_reason) || textField(body.reason);
  if (!check) return null;
  const message = textField(body.message) || textField(body.error);
  const sentences = message.split(/(?<=\.)\s+/).map((part) => part.trim()).filter(Boolean);
  const next = sentences.length > 1 ? sentences.slice(1).join(' ') : 'Submit again after this check passes.';
  return { check, message, next };
}

function printServerMode(io, payload) {
  const mode = textField(payload && payload.effective_mode) || textField(payload && payload.mode);
  const preselected = textField(payload && payload.mode_preselected);
  if (mode) io.stdout(`mode: ${mode}\n`);
  if (preselected) io.stdout(`mode_preselected: ${preselected}\n`);
  return mode;
}

function printClaimsState(io, payload) {
  if (payload && payload.claims_check === 'not_checked') io.stdout('claims: not checked yet\n');
}

function printDigestLimit(io, payload, items) {
  const body = payload && typeof payload === 'object' ? payload : {};
  if (body.truncated === true) {
    const total = typeof body.total === 'number' ? body.total : null;
    io.stdout('truncated: true\n');
    if (total !== null) {
      io.stdout(`total: ${total}\n`);
      io.stdout(`Showing ${items.length} of ${total}. The list was cut.\n`);
    } else {
      io.stdout('The list was cut.\n');
    }
  }
  if (body.claims_check === 'not_checked' || items.some((item) => item && item.claims_check === 'not_checked')) {
    io.stdout('claims: not checked yet\n');
  }
}

async function serverMode(io, headers) {
  const setup = await apiRequest(io, '/v1/setup/settings', { headers });
  if (setup.status !== 200 || !setup.json) {
    io.stderr(`mode was not loaded (${setup.status})\n`);
    return '';
  }
  return printServerMode(io, setup.json);
}

export async function listCommand(io) {
  const config = loadConfig(io);
  const headers = authHeaders(config);
  const offerID = offerFromScope(config.scope);
  if (!offerID) {
    io.stderr('list needs an offer-scoped project key\n');
    return { exitCode: 1 };
  }
  const current = await apiRequest(io, `/v1/maker/offers/${offerID}`, { headers });
  if (current.status !== 200) {
    io.stderr(`offer was not loaded (${current.status})\n`);
    io.stdout(`${current.text || ''}\n`);
    return { exitCode: 1 };
  }
  let status = current.json.status;
  let review = null;
  if (status === 'draft') {
    const submitted = await apiRequest(io, `/v1/maker/offers/${offerID}/submit`, { method: 'POST', headers });
    review = reviewFailure(submitted.json);
    printClaimsState(io, submitted.json);
    if (!review && submitted.status !== 200) {
      io.stderr(`offer was not submitted (${submitted.status})\n`);
      io.stdout(`${JSON.stringify(submitted.json)}\n`);
      return { exitCode: 1 };
    }
    if (submitted.json && submitted.json.status) status = submitted.json.status;
  }
  io.stdout(`listing_card: ${config.listing_card_id || 'none'}\n`);
  io.stdout(`offer: ${offerID}\n`);
  if (review && status !== 'listed') {
    io.stdout('status: in review\n');
    io.stdout('live: false\n');
    io.stdout(`check: ${review.check}\n`);
    if (review.message) io.stdout(`message: ${review.message}\n`);
    io.stdout(`next: ${review.next}\n`);
  } else {
    io.stdout(`status: ${status}\n`);
    io.stdout(`live: ${status === 'listed'}\n`);
    if (review) {
      io.stdout(`check: ${review.check}\n`);
      if (review.message) io.stdout(`message: ${review.message}\n`);
      io.stdout(`next: ${review.next}\n`);
    }
  }
  const mode = await serverMode(io, headers);
  if (!mode) return { exitCode: 1 };
  return { exitCode: status === 'listed' || review ? 0 : 1 };
}

export async function statusCommand(io) {
  const config = loadConfig(io);
  const headers = authHeaders(config);
  io.stdout(`scope: ${config.scope || ''}\n`);
  io.stdout(`key_header: ${config.key_header || ''}\n`);
  io.stdout(`account_kind: ${config.account_kind || ''}\n`);
  io.stdout(`human_login: ${config.human_login === true}\n`);
  io.stdout(`listing_card: ${config.listing_card_id || 'none'}\n`);
  const offerID = offerFromScope(config.scope);
  if (offerID) {
    const current = await apiRequest(io, `/v1/maker/offers/${offerID}`, { headers });
    if (current.status === 200) {
      io.stdout(`status: ${current.json.status}\n`);
      io.stdout(`live: ${current.json.status === 'listed'}\n`);
      printClaimsState(io, current.json);
    }
  }
  if (config.listing_card_id) {
    const card = await apiRequest(io, `/v1/listing-cards/${config.listing_card_id}`, { headers });
    if (card.status === 200 && card.json.score !== undefined) io.stdout(`score: ${card.json.score}\n`);
  }
  return { exitCode: 0 };
}

export async function digestCommand(io) {
  const config = loadConfig(io);
  const path = config.key_header === 'X-Maker-Key' ? '/v1/maker/digest' : '/v1/digest';
  const res = await apiRequest(io, path, { headers: authHeaders(config) });
  if (res.status !== 200) {
    io.stderr(`digest failed (${res.status})\n`);
    io.stdout(`${res.text || ''}\n`);
    return { exitCode: 1 };
  }
  const items = Array.isArray(res.json.items) ? res.json.items : [];
  const setup = await apiRequest(io, '/v1/setup/settings', { headers: authHeaders(config) });
  const modePayload = {
    ...(setup.status === 200 && setup.json ? setup.json : {}),
    effective_mode: textField(res.json.effective_mode) || textField(setup.json && setup.json.mode),
  };
  if (!printServerMode(io, modePayload)) {
    io.stderr('mode was not loaded\n');
    return { exitCode: 1 };
  }
  if (items[0] && items[0].undo_token) {
    saveConfig(io, { ...loadConfig(io), undo_token: items[0].undo_token });
  }
  printDigestLimit(io, res.json, items);
  const conversions = Array.isArray(res.json.conversions) ? res.json.conversions : [];
  for (const row of conversions) {
    const refundStatus = row.refund_status == null || row.refund_status === '' ? 'none' : row.refund_status;
    io.stdout(`conversion: ${row.kind} ${row.object_id} trust ${row.trust_level} refund ${refundStatus}\n`);
  }
  if (items.length === 0) {
    io.stdout('items: 0\n');
    return { exitCode: 0 };
  }
  for (const item of items) {
    io.stdout(`summary: ${item.summary}\n`);
    io.stdout(`live_now: ${item.live_now}\n`);
    io.stdout(`undo_token: ${item.undo_token}\n`);
  }
  return { exitCode: 0 };
}

export async function undoCommand(io) {
  const config = loadConfig(io);
  if (!config.undo_token) {
    io.stderr('No undo token. Run allagio digest first.\n');
    return { exitCode: 1 };
  }
  const res = await apiRequest(io, '/v1/digest/undo', {
    method: 'POST',
    body: { undo_token: config.undo_token },
    headers: authHeaders(config),
  });
  if (res.status !== 200) {
    io.stderr(`undo failed (${res.status})\n`);
    io.stdout(`${JSON.stringify(res.json)}\n`);
    return { exitCode: 1 };
  }
  io.stdout(`status: ${res.json.status}\n`);
  io.stdout(`kind: ${res.json.kind}\n`);
  const next = loadConfig(io);
  next.undo_token = '';
  saveConfig(io, next);
  return { exitCode: 0 };
}

export async function killCommand(io) {
  const config = loadConfig(io);
  const headers = authHeaders(config);
  const offerID = offerFromScope(config.scope);
  const siteID = siteFromScope(config.scope);
  if (offerID) {
    const res = await apiRequest(io, '/v1/account/pause', {
      method: 'POST',
      body: { scope: 'offer', id: offerID },
      headers,
    });
    const applied = res.json && Array.isArray(res.json.applied) ? res.json.applied[0] : null;
    if (res.status === 200 && applied && applied.kill_switch === true) {
      io.stdout('kill_switch: true\n');
      return { exitCode: 0 };
    }
    io.stderr(`kill switch was not turned on (${res.status})\n`);
    io.stdout(`${JSON.stringify(res.json)}\n`);
    return { exitCode: 1 };
  }
  if (siteID) {
    const res = await apiRequest(io, `/v1/sites/${siteID}/kill-switch`, {
      method: 'POST',
      body: { enabled: true },
      headers,
    });
    if (res.status === 200 && res.json.kill_switch === true) {
      io.stdout('kill_switch: true\n');
      return { exitCode: 0 };
    }
    io.stderr(`kill switch was not turned on (${res.status})\n`);
    io.stdout(`${JSON.stringify(res.json)}\n`);
    return { exitCode: 1 };
  }
  io.stderr('kill needs a site or offer project key\n');
  return { exitCode: 1 };
}

export function sessionCommand(label, io) {
  return printSessionOnly(label, io);
}
