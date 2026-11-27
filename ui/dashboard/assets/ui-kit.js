/* Offer Rails UI kit (v7, DESIGN.md): overlays (modal, side panel / sheet, palette, help) with focus trap,
   inert background, Esc to close and focus returned to where it was; a ⌘K / Ctrl+K command palette fed by a
   per-page command registry; a ? help overlay; one guarded key dispatcher; the BLIP mascot (empty states only).
   No network calls here. Pages register commands with OR.ui.command({...}) and list keys with OR.ui.onKey(fn). */
(function () {
  'use strict';
  var OR = (window.OfferRails = window.OfferRails || {});
  function esc(s) { return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); }
  var MAC = /Mac|iPhone|iPad/.test(navigator.platform || '') || /Mac OS X/.test(navigator.userAgent);
  var MOD = MAC ? '⌘' : 'Ctrl';
  var FOCUSABLE = 'a[href],button:not([disabled]),input:not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';

  // Typing guard: never take a key while the user is in a field.
  function typing(el) {
    el = el || document.activeElement;
    if (!el) return false;
    if (el.isContentEditable) return true;
    var t = (el.tagName || '').toLowerCase();
    if (t === 'textarea' || t === 'select') return true;
    if (t === 'input') return !/^(checkbox|radio|button|submit|reset|range|color|file)$/i.test(el.type || 'text');
    return false;
  }

  // ---------------- overlays ----------------
  var stack = [];
  function background() { return Array.prototype.slice.call(document.body.children).filter(function (n) { return !n.hasAttribute('data-overlay') && n.tagName !== 'SCRIPT'; }); }
  function setInert(on) { background().forEach(function (n) { if (on) n.setAttribute('inert', ''); else n.removeAttribute('inert'); }); }
  function focusables(root) { return Array.prototype.slice.call(root.querySelectorAll(FOCUSABLE)).filter(function (n) { return n.offsetParent !== null || n === document.activeElement; }); }
  // kind: modal | panel | palette | help. body: html string. Returns {el, body, close}.
  function open(o) {
    while (stack.length) close(true); // one overlay at a time; focus goes back to the first opener
    var returnTo = o.returnTo || document.activeElement;
    var scrim = document.createElement('div');
    scrim.className = 'scrim' + (o.kind === 'panel' ? ' panel-scrim' : ''); scrim.setAttribute('data-overlay', ''); scrim.setAttribute('data-test', 'scrim');
    var el = document.createElement('div');
    var cls = { modal: 'modal', panel: 'side-panel', palette: 'palette', help: 'modal help' }[o.kind] || 'modal';
    el.className = 'overlay ' + cls; el.setAttribute('data-overlay', ''); el.setAttribute('role', 'dialog'); el.setAttribute('aria-modal', 'true');
    el.setAttribute('data-test', o.test || o.kind); el.id = 'overlay-' + o.kind;
    var hid = 'overlay-title-' + o.kind;
    el.setAttribute('aria-labelledby', hid);
    el.innerHTML = o.kind === 'palette' ? '<h2 id="' + hid + '" class="sr-only">' + esc(o.title) + '</h2>' + o.body
      : '<div class="overlay-head"><h2 id="' + hid + '">' + esc(o.title) + '</h2>' +
        '<button type="button" class="btn quiet overlay-close" data-close aria-label="Close ' + esc(o.title) + '">Close <kbd>Esc</kbd></button></div>' +
        '<div class="overlay-body" tabindex="-1">' + o.body + '</div>';
    document.body.appendChild(scrim); document.body.appendChild(el);
    setInert(true);
    var entry = { el: el, scrim: scrim, returnTo: returnTo, onClose: o.onClose, kind: o.kind };
    stack.push(entry);
    scrim.addEventListener('click', function () { close(); });
    el.addEventListener('click', function (e) { if (e.target.closest('[data-close]')) close(); });
    el.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(); return; }
      if (e.key !== 'Tab') return;
      var f = focusables(el); if (!f.length) { e.preventDefault(); return; }
      var first = f[0], last = f[f.length - 1];
      if (e.shiftKey && (document.activeElement === first || !el.contains(document.activeElement))) { e.preventDefault(); last.focus(); }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
    });
    var target = (o.focus && el.querySelector(o.focus)) || focusables(el)[0] || el.querySelector('.overlay-body');
    if (target) target.focus();
    document.dispatchEvent(new CustomEvent('offerrails:overlay', { detail: { kind: o.kind, open: true } }));
    return { el: el, body: el.querySelector('.overlay-body') || el, close: close };
  }
  function close(silent) {
    var e = stack.pop(); if (!e) return;
    e.el.remove(); e.scrim.remove();
    if (!stack.length) setInert(false);
    if (e.onClose) e.onClose();
    var r = e.returnTo;
    if (r && r.isConnected && typeof r.focus === 'function') r.focus();
    if (!silent) document.dispatchEvent(new CustomEvent('offerrails:overlay', { detail: { kind: e.kind, open: false } }));
  }
  function isOpen(kind) { return stack.some(function (e) { return !kind || e.kind === kind; }); }

  // ---------------- commands, palette, help ----------------
  // {id, label, keys:[...], group, hint, available: fn -> true | "reason", run: fn}
  var commands = [];
  function command(c) { commands = commands.filter(function (x) { return x.id !== c.id; }); commands.push(c); }
  function availability(c) { var a = c.available ? c.available() : true; return a === true ? null : (a || 'Not available here'); }
  function keysHTML(keys) { return (keys || []).length ? '<span class="kbd-group">' + keys.map(function (k) { return '<kbd>' + esc(k) + '</kbd>'; }).join('') + '</span>' : ''; }

  function palette() {
    if (isOpen('palette')) { close(); return; }
    var o = open({ kind: 'palette', title: 'Command palette', focus: 'input',
      body: '<input type="text" role="combobox" aria-expanded="true" aria-controls="cmd-list" aria-autocomplete="list" aria-label="Search commands" placeholder="Type a command…" autocomplete="off" spellcheck="false" data-test="palette-input">' +
        '<ul id="cmd-list" role="listbox" aria-label="Commands" data-test="palette-list"></ul>' +
        '<div class="palette-foot"><span><kbd>↑</kbd> <kbd>↓</kbd> move</span><span><kbd>↵</kbd> run</span><span><kbd>Esc</kbd> close</span><span><kbd>?</kbd> all shortcuts</span></div>' });
    var input = o.el.querySelector('input'), list = o.el.querySelector('ul'), sel = 0, shown = [];
    function paint() {
      var q = input.value.trim().toLowerCase();
      shown = commands.filter(function (c) { return !q || (c.label + ' ' + (c.group || '')).toLowerCase().indexOf(q) >= 0; });
      if (sel >= shown.length) sel = Math.max(0, shown.length - 1);
      list.innerHTML = shown.length ? shown.map(function (c, i) {
        var why = availability(c);
        return '<li class="cmd-row" role="option" id="cmd-' + esc(c.id) + '" data-cmd="' + esc(c.id) + '" aria-selected="' + (i === sel) + '"' + (why ? ' aria-disabled="true"' : '') + '>' +
          '<span class="cmd-label">' + esc(c.label) + (why ? '<small>' + esc(why) + '</small>' : c.hint ? '<small>' + esc(c.hint) + '</small>' : '') + '</span>' + keysHTML(c.keys) + '</li>';
      }).join('') : '<li class="palette-empty" role="presentation" data-test="palette-empty">No command matches “' + esc(input.value.trim()) + '”. Try “undo”, “pause” or “shortcuts”.</li>';
      input.setAttribute('aria-activedescendant', shown[sel] ? 'cmd-' + shown[sel].id : '');
    }
    function run(c) {
      if (!c || availability(c)) return;
      close();             // focus returns to where it was, then the command runs from there
      setTimeout(function () { c.run(); }, 0);
    }
    input.addEventListener('input', function () { sel = 0; paint(); });
    input.addEventListener('keydown', function (e) {
      if (e.key === 'ArrowDown') { e.preventDefault(); sel = Math.min(shown.length - 1, sel + 1); paint(); scrollSel(); }
      else if (e.key === 'ArrowUp') { e.preventDefault(); sel = Math.max(0, sel - 1); paint(); scrollSel(); }
      else if (e.key === 'Enter') { e.preventDefault(); run(shown[sel]); }
    });
    function scrollSel() { var r = list.querySelector('[aria-selected=true]'); if (r) r.scrollIntoView({ block: 'nearest' }); }
    list.addEventListener('click', function (e) { var r = e.target.closest('[data-cmd]'); if (r) run(commands.filter(function (c) { return c.id === r.dataset.cmd; })[0]); });
    paint();
  }
  function help() {
    if (isOpen('help')) { close(); return; }
    var rows = commands.filter(function (c) { return c.keys && c.keys.length; }).map(function (c) {
      var why = availability(c);
      return '<dt>' + keysHTML(c.keys) + '</dt><dd' + (why ? ' class="na"' : '') + '>' + esc(c.label) + (why ? ' · ' + esc(why) : '') + '</dd>';
    }).join('');
    open({ kind: 'help', title: 'Keyboard shortcuts', test: 'help',
      body: '<dl class="help-list" data-test="help-list">' + rows + '</dl>' +
        '<p class="caption" style="margin:16px 0 0">Shortcuts never fire while you are typing in a field. Actions follow the same rules as the buttons: what your session or key can’t do stays off.</p>' });
  }

  // ---------------- one key dispatcher ----------------
  var listHandlers = [];
  function onKey(fn) { listHandlers.push(fn); }
  document.addEventListener('keydown', function (e) {
    if (e.defaultPrevented || e.isComposing) return;
    if (typing(e.target)) return;                       // guard: the user is typing
    var k = e.key;
    if ((e.metaKey || e.ctrlKey) && !e.altKey && !e.shiftKey && (k === 'k' || k === 'K')) { e.preventDefault(); palette(); return; }
    if (e.metaKey || e.ctrlKey || e.altKey) return;     // leave browser and OS shortcuts alone
    if (stack.length) return;                           // overlays handle their own keys (Esc, Tab, arrows)
    if (k === '?') { e.preventDefault(); help(); return; }
    for (var i = 0; i < listHandlers.length; i++) if (listHandlers[i](e) === true) { e.preventDefault(); return; }
  });

  // ---------------- BLIP: the team mascot, a blinking text cursor. Empty states only. ----------------
  function blip(label) {
    return '<svg class="blip" viewBox="0 0 44 44" role="img" aria-label="' + esc(label || 'BLIP, the Allagio cursor, waiting') + '" data-test="blip">' +
      '<rect x="6" y="36" width="32" height="3" rx="1.5" style="fill:rgb(var(--rgb-line-strong))"/>' +
      '<g class="blip-bar"><rect x="16" y="5" width="12" height="29" rx="3" style="fill:rgb(var(--rgb-digest))"/>' +
      '<circle cx="20" cy="13" r="1.8" style="fill:rgb(var(--rgb-surface-1))"/><circle cx="24.5" cy="13" r="1.8" style="fill:rgb(var(--rgb-surface-1))"/></g>' +
      '<path d="M11 5h6M27 5h6M11 34h6M27 34h6" style="stroke:rgb(var(--rgb-text-1))" stroke-width="2" stroke-linecap="round"/></svg>';
  }

  OR.ui = { open: open, close: close, isOpen: isOpen, command: command, palette: palette, help: help, onKey: onKey,
    typing: typing, blip: blip, keysHTML: keysHTML, MOD: MOD, esc: esc, commands: function () { return commands.slice(); } };

  // Every page: palette and help are commands too, so the palette lists its own keys.
  command({ id: 'palette', label: 'Command palette', keys: [MOD, 'K'], group: 'General', run: palette });
  command({ id: 'help', label: 'Keyboard shortcuts', keys: ['?'], group: 'General', run: help });
})();
