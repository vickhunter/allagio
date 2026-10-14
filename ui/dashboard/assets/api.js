/* Tiny API client shared by the D-032 pages. Never throws on HTTP errors:
   resolves {status, body}; status 0 = network error. body is parsed JSON or null.
   Modes (OR.apiMode):
     "fixture" (default) : page data from fixtures/*.json, server calls go to the same origin (tests route them).
     "mock"   (?mock=1)  : preview; assets/mock-api.js answers server calls from fixtures/api/*.json.
     "real"   (?api=real): page data AND server calls go to the D-032a API (PR #21 contract) at OR.apiBase.
   Base URL (OR.apiBase), first match wins:
     1. window.OfferRails.apiBase set by the host page before this script,
     2. <meta name="or-api-base" content="https://api.example.com">,
     3. ?api_base=... only when it is a same-origin path ("/backend") or http(s)://localhost|127.0.0.1[:port]
        (so a shared link can never point the page, and its session cookie, at another host),
     4. "" = same origin.
   Cross-origin bases are called with credentials:"include" (session cookie); same origin with "same-origin". */
(function () {
  'use strict';
  var OR = (window.OfferRails = window.OfferRails || {});
  var q = new URLSearchParams(location.search);
  function safeQueryBase(b) {
    if (!b) return null;
    if (/^\/[A-Za-z0-9._~\/-]*$/.test(b) && b.indexOf('//') < 0) return b.replace(/\/+$/, '');
    if (/^https?:\/\/(localhost|127\.0\.0\.1)(:\d{1,5})?(\/[A-Za-z0-9._~\/-]*)?$/.test(b)) return b.replace(/\/+$/, '');
    return null;
  }
  var meta = document.querySelector('meta[name="or-api-base"]');
  if (OR.apiBase == null) OR.apiBase = (meta && meta.content ? meta.content.replace(/\/+$/, '') : null) || safeQueryBase(q.get('api_base')) || '';
  if (!OR.apiMode) OR.apiMode = q.get('api') === 'real' ? 'real' : q.get('mock') ? 'mock' : 'fixture';
  function crossOrigin() { try { return new URL(OR.apiBase || '/', location.href).origin !== location.origin; } catch (e) { return false; } }

  function csrf() {
    var m = document.querySelector('meta[name="csrf-token"]'); return m && m.content ? m.content : '';
  }
  function call(method, path, body, opts) {
    if (/^\/v1\/go\//.test(path)) return Promise.resolve({ status: 0, body: null }); // never touch a go link
    var init = { method: method, credentials: crossOrigin() ? 'include' : 'same-origin', headers: { Accept: 'application/json' } };
    if (body !== undefined) { init.headers['Content-Type'] = 'application/json'; init.body = JSON.stringify(body); }
    // Owner-session writes need the dashboard's CSRF token. 2d3aed7: owner pages render <meta name="csrf-token"> and set no
    // window global, so the meta tag is the only source. It is sent only on writes, never put in a URL.
    if (method !== 'GET') { var tok = csrf(); if (tok) init.headers['X-CSRF-Token'] = tok; }
    // keepalive: an Undo still reaches the server if the tab closes right after the tap.
    if (opts && opts.keepalive) init.keepalive = true;
    return fetch((OR.apiBase || '') + path, init).then(function (r) {
      return r.text().then(function (t) { var j = null; try { j = t ? JSON.parse(t) : null; } catch (e) { j = null; } return { status: r.status, body: j }; });
    }, function () { return { status: 0, body: null }; });
  }
  OR.api = {
    get: function (p) { return call('GET', p); },
    post: function (p, b, o) { return call('POST', p, b, o); },
    put: function (p, b) { return call('PUT', p, b); }
  };
  // Plugin key (PR #21 @ 6042dcb, handlers/plugin_keys.go). All four are session-only: any key or bearer is 403
  // {error}, no session is 401 {error}. Issue: 201 {plugin_key, key_prefix} or 409 {error:"a plugin key already exists"}.
  // List: {keys:[{id, key_prefix, created_at, revoked, revoked_at?}]}, never the secret. Rotate: 200 {plugin_key, key_prefix}.
  // Revoke: 200 {revoked}. Site ids are UUIDs; anything else is refused here before it reaches the server.
  function siteKeyPath(siteId, tail) {
    if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(String(siteId))) return null;
    return '/v1/sites/' + siteId + '/plugin-key' + (tail || '');
  }
  function guarded(method, siteId, tail) {
    var p = siteKeyPath(siteId, tail);
    return p ? call(method, p, method === 'POST' ? {} : undefined) : Promise.resolve({ status: 400, body: { error: 'invalid site id' } });
  }
  OR.pluginKey = {
    list: function (id) { return guarded('GET', id, ''); },        // endpoint: GET /v1/sites/:id/plugin-key
    issue: function (id) { return guarded('POST', id, ''); },      // endpoint: POST /v1/sites/:id/plugin-key
    rotate: function (id) { return guarded('POST', id, '/rotate'); }, // endpoint: POST /v1/sites/:id/plugin-key/rotate
    revoke: function (id) { return guarded('POST', id, '/revoke'); }, // endpoint: POST /v1/sites/:id/plugin-key/revoke
    // 401 (no session) and 403 (a key, a bearer, or another site) both mean: sign in to the dashboard again.
    needsSignIn: function (r) { return r && (r.status === 401 || r.status === 403); },
    // 2d3aed7: every plugin-key error is {error, reason, message}. reason picks the UI; message is shown as the server's words.
    // Older bodies with only `error` still work: reason falls back from the status code.
    reason: function (r) {
      var b = (r && r.body) || {}, rs = typeof b.reason === 'string' ? b.reason : '';
      if (/^(reauth_required|key_exists|forbidden_key_type|forbidden)$/.test(rs)) return rs;
      return r && r.status === 401 ? 'reauth_required' : r && r.status === 409 ? 'key_exists' : r && r.status === 403 ? 'forbidden' : rs || '';
    },
    message: function (r) { var m = r && r.body && r.body.message; return typeof m === 'string' && m.length < 200 && !/[{}\[\]<>]/.test(m) ? m.trim() : ''; },
    // The server's own short sentence when it is plain text, else ours.
    serverText: function (r) { var e = r && r.body && r.body.error; return typeof e === 'string' && e.length < 160 && !/[{}\[\]<>]/.test(e) ? e : ''; }
  };
})();
