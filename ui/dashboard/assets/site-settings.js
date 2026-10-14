/* v8: site settings panel (plugin key) on the digest. Session-only server routes from assets/api.js (PR #21 @ 6042dcb):
   GET /v1/sites/:id/plugin-key (prefixes only), POST …/plugin-key (201 or 409), POST …/rotate (new key once), POST …/revoke.
   401/403 -> "Sign in again to continue" (nothing changed). Key contexts (no session): actions hidden, one line says they need the dashboard.
   Revoke is a danger confirm modal that says what stops working. No optimistic state: the panel changes only after a 2xx. */
(function () {
  'use strict';
  var OR = (window.OfferRails = window.OfferRails || {});
  function $(sel, el) { return (el || document).querySelector(sel); }
  function esc(s) { return OR.keyUI.esc(s); }
  function day(iso) { try { return new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Europe/Rome' }); } catch (e) { return ''; } }
  function emit(n, d) { document.dispatchEvent(new CustomEvent('offerrails:' + n, { detail: d, bubbles: true })); }
  var state = {}; // per site ref: {keys, msg, reauth}

  function sec(ref) { return OR.digest.sections().filter(function (s) { return OR.digest.refOf(s) === ref; })[0]; }
  function signedIn() { var d = OR.digest.data; return !!(d.session && d.session.signed_in); }
  function changeUrl() { return OR.digest.data.change_url || '/owner'; }

  function statusHTML(st) {
    if (!st.keys) return '<p class="key-status caption" data-test="key-status" aria-busy="true">Checking the plugin key…</p>';
    var active = st.keys.filter(function (k) { return !k.revoked; })[0];
    if (active) return '<p class="key-status" data-test="key-status" data-active="true">Active key starting <code class="mono" data-test="key-prefix">' + esc(active.key_prefix) + '</code> · created ' + esc(day(active.created_at)) + '. The full key is never shown again.</p>';
    var last = st.keys.filter(function (k) { return k.revoked; })[0];
    return '<p class="key-status" data-test="key-status" data-active="false"><strong>No active plugin key.</strong> ' +
      (last ? 'The key starting <code class="mono">' + esc(last.key_prefix) + '</code> was revoked' + (last.revoked_at ? ' on ' + esc(day(last.revoked_at)) : '') + '. ' : '') +
      'The plugin can\u2019t confirm placements, so new boxes won\u2019t count as live until you generate a key and paste it in WordPress.</p>';
  }
  function bodyHTML(s, st) {
    var head = '<div class="key-settings" data-test="key-settings"><h3 id="pk-h">Plugin key</h3>' +
      '<p class="caption">The WordPress plugin on ' + esc(s.name) + ' uses it to confirm boxes. Pasted in ' + esc(OR.keyUI.WHERE) + '.</p>';
    if (!signedIn()) return head + '<p class="approval-note" data-test="key-context-line">Generating, rotating and revoking the plugin key need the dashboard. Sign in there to change it.</p></div>';
    if (st.reauth) return head + OR.keyUI.signInHTML(changeUrl(), 'Plugin keys', st.reauth) + '</div>';
    if (st.once) return head + OR.keyUI.onceHTML({ key: st.once.plugin_key, site: s.name, title: st.onceTitle, continueLabel: 'Continue',
      intro: st.onceIntro }) + '</div>';
    var active = st.keys && st.keys.some(function (k) { return !k.revoked; });
    var tag = '<span class="session-only">Signed-in only · may ask you to sign in again</span>';
    return head + statusHTML(st) +
      (st.msg ? '<p class="key-msg' + (st.msg.error ? ' error' : '') + '" role="' + (st.msg.error ? 'alert' : 'status') + '" data-test="key-msg">' + esc(st.msg.text) + '</p>' : '') +
      (st.keys ? '<div class="key-actions">' +
        (!active ? '<button type="button" class="btn secondary" data-key-act="generate">Generate key</button>' : '') +
        '<button type="button" class="btn secondary" data-key-act="rotate">Rotate key</button>' +
        (active ? '<button type="button" class="btn danger-outline" data-key-act="revoke">Revoke key…</button>' : '') + '</div>' + tag : '') +
      (st.confirmRotate ? '<div class="confirm-inline" data-test="rotate-confirm"><p><strong>Rotate the key?</strong> The current key stops working at once. You\u2019ll see the new one once, then paste it in WordPress.</p>' +
        '<div class="actions"><button type="button" class="btn secondary" data-key-act="rotate-yes">Yes, rotate</button><button type="button" class="btn quiet" data-key-act="rotate-no">Cancel</button></div></div>' : '') +
      '</div>';
  }

  function open(ref, opener, focusSel) {
    var s = sec(ref); if (!s) return;
    var st = state[ref] = state[ref] || {};
    var o = OR.ui.open({ kind: 'panel', title: 'Settings · ' + s.name, body: bodyHTML(s, st), returnTo: opener, test: 'site-settings',
      focus: focusSel || null, onClose: function () { if (st.once) { st.once = null; st.keys = null; } } });
    wire(ref, o.body);
    if (signedIn() && !st.keys && !st.reauth) load(ref);
    return o;
  }
  function repaint(ref, focusSel) {
    var el = $('[data-test=site-settings] .overlay-body'); if (!el) return;
    el.innerHTML = bodyHTML(sec(ref), state[ref]); wire(ref, el);
    var f = focusSel && el.querySelector(focusSel); if (f) f.focus();
  }
  function load(ref) {
    var st = state[ref];
    return OR.pluginKey.list(sec(ref).server_id).then(function (r) {
      if (OR.pluginKey.needsSignIn(r)) st.reauth = r;
      else if (r.status === 200 && r.body && Array.isArray(r.body.keys)) st.keys = r.body.keys;
      else st.msg = { error: true, text: 'We couldn\u2019t read the plugin key right now. Nothing changed. Close and try again.' }, st.keys = st.keys || null;
      repaint(ref);
    });
  }
  function busy(btn, label) { if (!btn) return; btn.disabled = true; btn.setAttribute('aria-busy', 'true'); btn.textContent = label; }
  function result(ref, action, r) { emit('plugin-key-result', { site_id: sec(ref).server_id, action: action, http_status: r.status }); }

  function wire(ref, root) {
    var st = state[ref];
    if (st.once) { OR.keyUI.wireOnce(root.querySelector('[data-test=key-once]') || root, function () { st.once = null; st.keys = null; st.msg = { text: st.doneText }; repaint(ref); load(ref); }); return; }
    if (root.dataset.keyWired) return; // repaints reuse the panel body: one listener only
    root.dataset.keyWired = '1';
    root.addEventListener('click', function (e) {
      var b = e.target.closest('[data-key-act]'); if (!b) return;
      var act = b.dataset.keyAct, id = sec(ref).server_id;
      if (act === 'rotate') { st.confirmRotate = true; st.msg = null; return repaint(ref, '[data-key-act=rotate-yes]'); }
      if (act === 'rotate-no') { st.confirmRotate = false; return repaint(ref, '[data-key-act=rotate]'); }
      if (act === 'revoke') return revokeModal(ref, b);
      if (act === 'generate' || act === 'rotate-yes') {
        busy(b, act === 'generate' ? 'Generating…' : 'Rotating…');
        (act === 'generate' ? OR.pluginKey.issue(id) : OR.pluginKey.rotate(id)).then(function (r) {
          result(ref, act === 'generate' ? 'generate' : 'rotate', r); st.confirmRotate = false;
          if (OR.pluginKey.needsSignIn(r) && OR.pluginKey.reason(r) !== 'key_exists') { st.reauth = r; return repaint(ref, '[data-test=reauth-link]'); }
          if ((r.status === 201 || r.status === 200) && r.body && typeof r.body.plugin_key === 'string') {
            st.once = r.body; st.onceTitle = act === 'generate' ? 'Copy your new plugin key' : 'Copy your rotated plugin key';
            st.onceIntro = act === 'generate' ? 'This site had no plugin key. Until the plugin has this one, new boxes won\u2019t count as live.'
              : 'The old key stopped working just now. Until you paste this one, the plugin can\u2019t confirm boxes.';
            st.doneText = (act === 'generate' ? 'Key generated' : 'Key rotated') + ' (starts ' + r.body.key_prefix + '). Paste it in WordPress if you haven\u2019t yet.';
            repaint(ref); var h = $('#key-once-h'); if (h) { h.setAttribute('tabindex', '-1'); h.focus(); } return;
          }
          if (OR.pluginKey.reason(r) === 'key_exists') { var km = OR.pluginKey.message(r);
            st.msg = { error: true, text: (km ? km + ' ' : 'This site already has a plugin key. ') + 'To get a new one, rotate it: the old key stops working and you\u2019ll see the new one once.' }; st.keys = null; repaint(ref); return load(ref).then(function () { repaint(ref, '[data-key-act=rotate]'); }); }
          st.msg = { error: true, text: (act === 'generate' ? 'We couldn\u2019t generate a key' : 'We couldn\u2019t rotate the key') + ' right now. Nothing changed. Try again.' }; repaint(ref, '[data-key-act]');
        });
      }
    });
  }

  function revokeModal(ref, opener) {
    var s = sec(ref), st = state[ref], active = (st.keys || []).filter(function (k) { return !k.revoked; })[0];
    var body = '<div data-test="revoke-modal-body"><p>You\u2019re about to revoke the plugin key' + (active ? ' starting <code class="mono">' + esc(active.key_prefix) + '</code>' : '') + ' for <strong>' + esc(s.name) + '</strong>.</p>' +
      '<p><strong>What stops working right away:</strong></p><ul class="danger-list">' +
        '<li>The plugin can\u2019t confirm placements any more.</li>' +
        '<li>New boxes won\u2019t count as live, or earn, until you paste a new key in ' + esc(OR.keyUI.WHERE) + '.</li>' +
        '<li>Undo still stops clicks at once, but the plugin can\u2019t confirm the box left the post.</li></ul>' +
      '<p><strong>Not changed:</strong> boxes already live stay live. Nothing is removed from your posts.</p>' +
      '<p class="key-msg error" role="alert" data-test="revoke-error" hidden></p>' +
      '<div class="actions"><button type="button" class="btn danger" id="revoke-yes" data-test="revoke-yes">Revoke key</button>' +
      '<button type="button" class="btn quiet" id="revoke-no">Cancel</button></div></div>';
    var reopen = function (focusSel) { setTimeout(function () { open(ref, document.querySelector('[data-site-settings="' + ref + '"]'), focusSel); }, 0); };
    var closing = 'cancel';
    var m = OR.ui.open({ kind: 'modal', title: 'Revoke the plugin key?', body: body, test: 'revoke-modal', focus: '#revoke-no',
      returnTo: document.querySelector('[data-site-settings="' + ref + '"]'),
      onClose: function () { if (closing === 'cancel') reopen('[data-key-act=revoke]'); } });
    m.el.classList.add('danger-modal');
    $('#revoke-no', m.el).addEventListener('click', function () { m.close(); });
    $('#revoke-yes', m.el).addEventListener('click', function () {
      var yes = this; busy(yes, 'Revoking…');
      OR.pluginKey.revoke(s.server_id).then(function (r) {
        result(ref, 'revoke', r);
        if (OR.pluginKey.needsSignIn(r)) { st.reauth = r; closing = 'done'; m.close(); return reopen('[data-test=reauth-link]'); }
        if (r.status === 200) { st.keys = null; st.msg = { text: 'Key revoked. The plugin can\u2019t confirm placements until you generate a new key and paste it in WordPress.' }; closing = 'done'; m.close(); return reopen('[data-key-act=generate]'); }
        yes.disabled = false; yes.setAttribute('aria-busy', 'false'); yes.textContent = 'Revoke key';
        var e = $('[data-test=revoke-error]', m.el); e.hidden = false; e.textContent = 'We couldn\u2019t revoke the key right now. It still works. Try again.';
      });
    });
  }

  document.addEventListener('click', function (e) {
    var b = e.target.closest('[data-site-settings]'); if (b) open(b.dataset.siteSettings, b);
  });
  OR.siteSettings = { open: open, state: state };
})();
