/* Plugin key UI shared by setup (shown-once step) and the digest's site settings panel.
   Server: OR.pluginKey (assets/api.js), PR #21 @ 6042dcb handlers/plugin_keys.go. Session-only routes.
   The raw key lives only in the DOM of the shown-once block: never in localStorage, events, URLs or console. */
(function () {
  'use strict';
  var OR = (window.OfferRails = window.OfferRails || {});
  function esc(s) { return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); }
  var WHERE = 'Allagio › Settings › Plugin key';

  // The shown-once block. opts: {key, prefix, site, continueLabel, intro}
  function onceHTML(o) {
    return '<section class="key-once" data-test="key-once" aria-labelledby="key-once-h">' +
      '<p class="eyebrow">Plugin key · shown once</p>' +
      '<h' + (o.level || 2) + ' id="key-once-h">' + esc(o.title || 'Copy your plugin key now') + '</h' + (o.level || 2) + '>' +
      '<p>' + esc(o.intro || 'Your WordPress plugin uses this key to confirm each box it puts in or takes out of a post.') + ' ' +
        'Paste it in WordPress under <strong>' + esc(WHERE) + '</strong>, then press Save there.</p>' +
      '<div class="field"><label for="plugin-key-value">Plugin key' + (o.site ? ' for ' + esc(o.site) : '') + '</label>' +
        '<div class="key-row"><input type="text" id="plugin-key-value" class="mono" readonly spellcheck="false" autocomplete="off" value="' + esc(o.key) + '" aria-describedby="key-once-warn">' +
        '<button type="button" class="btn secondary" id="copy-key" data-test="copy-key" data-copied="false">Copy</button></div></div>' +
      '<p class="key-warn" id="key-once-warn" data-test="key-once-warn"><strong>We won\'t show this key again.</strong> If you lose it, rotate it from Settings and paste the new one.</p>' +
      '<label class="check"><input type="checkbox" id="key-saved" data-test="key-saved"> I saved it somewhere safe</label>' +
      '<div class="actions"><button type="button" class="keycap" id="key-continue" data-test="key-continue" disabled aria-describedby="key-gate">' +
        '<span class="keycap-face">' + esc(o.continueLabel || 'Continue') + '</span></button>' +
        '<span class="caption reassure" id="key-gate" data-test="key-gate">Continue unlocks once you copy the key or tick “I saved it”.</span></div>' +
      '<p class="sr-only" role="status" aria-live="polite" id="key-status"></p>' +
    '</section>';
  }
  function wireOnce(root, onContinue) {
    var input = root.querySelector('#plugin-key-value'), copy = root.querySelector('#copy-key'), box = root.querySelector('#key-saved'),
        go = root.querySelector('#key-continue'), gate = root.querySelector('#key-gate'), status = root.querySelector('#key-status');
    var copied = false;
    function sync() {
      var ok = copied || box.checked; go.disabled = !ok;
      gate.textContent = ok ? (copied ? 'Copied. Paste it in WordPress, then continue.' : 'Saved. Paste it in WordPress, then continue.')
        : 'Continue unlocks once you copy the key or tick “I saved it”.';
    }
    function done() {
      copied = true; copy.textContent = 'Copied ✓'; copy.dataset.copied = 'true'; copy.classList.add('copied');
      status.textContent = 'Plugin key copied to the clipboard.'; sync();
    }
    function manual() { input.focus(); input.select(); status.textContent = 'Couldn\u2019t copy automatically. The key is selected; press Ctrl+C or ⌘C.'; gate.textContent = status.textContent; }
    copy.addEventListener('click', function () {
      if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(input.value).then(done, function () {
        try { input.select(); if (document.execCommand('copy')) return done(); } catch (e) {} manual();
      });
      else { try { input.select(); if (document.execCommand('copy')) return done(); } catch (e) {} manual(); }
    });
    input.addEventListener('focus', function () { input.select(); });
    box.addEventListener('change', sync);
    go.addEventListener('click', function () { if (!go.disabled) { input.value = ''; onContinue(); } });
    sync();
  }
  // Fresh sign-in needed (401 reauth_required, 403 forbidden_key_type / forbidden). r = the server response, optional.
  // 2d3aed7: the heading and next step follow `reason`; the server's `message` is shown word for word. No reason = v8 text.
  function signInHTML(url, what, r) {
    var rs = r && OR.pluginKey ? OR.pluginKey.reason(r) : '', msg = r && OR.pluginKey ? OR.pluginKey.message(r) : '';
    var lead = 'Sign in again to continue.', why = (what || 'Plugin keys') + ' only change from a fresh dashboard sign-in.';
    if (rs === 'forbidden_key_type') { lead = 'This needs the dashboard, not a key.'; why = 'Open the dashboard and sign in there to change ' + (what ? 'your plugin key' : 'plugin keys') + '.'; }
    return '<div class="reauth" data-test="reauth" data-reason="' + esc(rs) + '" role="alert"><p><strong>' + esc(lead) + '</strong> ' +
      (msg ? '<span data-test="reauth-server">' + esc(msg) + '</span> ' : '') + esc(why) + ' Nothing was changed.</p>' +
      '<a class="btn secondary" data-test="reauth-link" href="' + esc(url || '/owner') + '">' + (rs === 'forbidden_key_type' ? 'Open the dashboard' : 'Sign in again') + '</a></div>';
  }
  OR.keyUI = { onceHTML: onceHTML, wireOnce: wireOnce, signInHTML: signInHTML, WHERE: WHERE, esc: esc };
})();
