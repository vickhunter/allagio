/* Digest, keyboard-first (v7, DESIGN.md). J/K move between items, Enter opens the item panel, R undoes the
   focused item, A approves. Every key does exactly what the visible button would do, under the same rules:
   R only where an Undo / "Leave it removed" button is showing and enabled; there is nothing to approve on this
   screen, so A only says so. Redo stays in the toast (session only). No new server calls: actions click the
   item's own buttons. Needs ui-kit.js, digest-item.js, digest.js. */
(function () {
  'use strict';
  var OR = (window.OfferRails = window.OfferRails || {});
  var ui = OR.ui; if (!ui) return;
  var esc = ui.esc;
  function $(s, el) { return (el || document).querySelector(s); }
  function $all(s, el) { return Array.prototype.slice.call((el || document).querySelectorAll(s)); }
  function say(msg) { var s = $('#digest-status'); if (s) { s.textContent = ''; setTimeout(function () { s.textContent = msg; }, 30); } }
  var MODES = [
    { v: 'manual', label: 'Ask me every time', text: 'Nothing new goes live until you approve it.' },
    { v: 'auto', label: 'Approve within my rules', text: 'Your agent approves what fits your rules. Everything else waits for you.' },
    { v: 'yolo', label: 'Approve anything that passes the checks', text: 'Anything that passes every hard check goes live right away, with one-tap undo.' }
  ];

  // ---------- list navigation ----------
  function visibleItems() { return $all('digest-item').filter(function (el) { var li = el.closest('li'); return !(li && li.hidden) && el.querySelector('.digest-item'); }); }
  var lastItem = null;
  document.addEventListener('focusin', function (e) {
    if (e.target.closest && e.target.closest('[data-overlay]')) return;
    lastItem = e.target.closest ? e.target.closest('digest-item') : null;
  });
  function focusItem(el) { var a = el && el.querySelector('.digest-item'); if (a) { a.focus(); a.scrollIntoView({ block: 'nearest' }); lastItem = el; } }
  function move(dir) {
    var list = visibleItems(); if (!list.length) return;
    var cur = document.activeElement && document.activeElement.closest ? document.activeElement.closest('digest-item') : null;
    var i = list.indexOf(cur);
    focusItem(list[i < 0 ? (dir > 0 ? 0 : 0) : Math.max(0, Math.min(list.length - 1, i + dir))]);
  }
  function undoButton(el) {
    var b = el && el.querySelector('[data-action=toggle]');
    return b && !b.hidden && !b.disabled && /^(Undo|Leave it removed)$/.test(b.textContent) ? b : null;
  }
  function undoReason(el) {
    if (!el) return 'Move to an item first (J or K)';
    if (el.isListing && el.isListing()) return 'Listings can’t be undone here';
    return undoButton(el) ? true : 'Nothing to undo on this item right now';
  }
  var APPROVE_REASON = 'Nothing waits for approval on this screen. Approving happens in the dashboard.';

  ui.onKey(function (e) {
    var k = e.key;
    if (k === 'j' || k === 'J') { move(1); return true; }
    if (k === 'k' || k === 'K') { move(-1); return true; }
    if (k === 'Enter') {
      var a = document.activeElement;
      if (a && a.classList && a.classList.contains('digest-item')) { openPanel(a.closest('digest-item'), a); return true; }
      return false; // Enter on a button or link keeps its normal meaning
    }
    if (k === 'r' || k === 'R') {
      var el = document.activeElement && document.activeElement.closest ? document.activeElement.closest('digest-item') : null;
      var why = undoReason(el);
      if (why === true) undoButton(el).click(); else say(why + '.');
      return true;
    }
    if (k === 'a' || k === 'A') { say(APPROVE_REASON); return true; }
    return false;
  });

  // ---------- item panel (side panel; full-screen sheet at phone width) ----------
  function fact(label, value) { return value == null || value === '' ? '' : '<dt>' + esc(label) + '</dt><dd>' + esc(value) + '</dd>'; }
  function day(iso) { try { return iso ? new Date(iso).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' }) : ''; } catch (e) { return ''; } }
  function openPanel(el, opener) {
    if (!el) return;
    var it = el.item || {}, listing = el.isListing && el.isListing();
    var a = OR.approvedText ? OR.approvedText(it) : null;
    var who = OR.whoText && el.status && el.status() !== 'live' ? OR.whoText(it.undone_by) : null;
    var note = (el.querySelector('[data-test=note]') || {}).textContent || '';
    var undo = undoButton(el);
    var body =
      '<div class="di-box" data-test="panel-box"><span class="copy">' + esc(it.copy) + '</span> ' +
        '<span class="panel-link">' + esc(it.link_text || 'Vedi il prodotto') + '</span>' +
        (it.disclosure_label === null ? '' : ' <span class="label">' + esc(it.disclosure_label || '(link sponsorizzato)') + '</span>') + '</div>' +
      '<p class="caption" style="margin:6px 0 0">Preview only. The link in the box is never opened from here.</p>' +
      '<dl class="panel-facts" data-test="panel-facts">' +
        fact('Status', el.stateText ? el.stateText() : '') +
        fact(listing ? 'Listed on' : 'Where', (it.site || {}).name || (it.site || {}).domain) +
        fact('Post', (it.article || {}).title) +
        fact('Position', OR.positionLabel ? OR.positionLabel(it.position) : it.position) +
        fact('Approved', a ? (a.lead + (a.rest ? ' ' + a.rest : '')) : '') +
        fact('Rules', it.policy_version != null && it.approved_by !== 'owner' ? 'version ' + it.policy_version : '') +
        fact('Clicks', it.clicks == null ? '' : String(it.clicks)) +
        (it.status === 'in_review' ? '' : fact(listing ? 'Listed' : 'Went live', day(it.went_live_at))) +
        fact('Undone by', who) +
      '</dl>' +
      (note ? '<p class="outcome" data-test="panel-note">' + esc(note) + '</p>' : '') +
      '<div class="actions" style="margin-top:18px">' +
        (undo ? '<button type="button" class="btn secondary" data-panel-undo>' + esc(undo.textContent) + '</button>' : '') +
        '<button type="button" class="btn quiet" data-close>Back to the list</button></div>';
    var title = listing ? 'Listing on ' + ((it.site || {}).name || '') : 'Box on ' + ((it.site || {}).name || (it.site || {}).domain || 'your site');
    var o = ui.open({ kind: 'panel', title: title, body: body, returnTo: opener || document.activeElement, test: 'panel' });
    var pu = o.el.querySelector('[data-panel-undo]');
    if (pu) pu.addEventListener('click', function () { ui.close(); var b = undoButton(el); if (b) b.click(); });
  }
  OR.openItemPanel = openPanel;
  document.addEventListener('click', function (e) {
    var b = e.target.closest && e.target.closest('[data-action=details]');
    if (b) openPanel(b.closest('digest-item'), b);
  });

  // ---------- approval modes modal (the "Change" link) ----------
  function openModes(secEl, opener) {
    var dg = OR.digest; if (!dg || !secEl) return;
    var ref = secEl.getAttribute('data-section');
    var sec = dg.sections().filter(function (s) { return dg.refOf(s) === ref; })[0]; if (!sec) return;
    var data = dg.data, signed = !!(data.session && data.session.signed_in);
    var href = (opener && opener.getAttribute('href')) || '#preview';
    var body =
      '<p data-test="modes-now"><b>Now:</b> ' + esc(dg.MODE_LABEL[sec.mode] || 'Unknown') + (data.paused ? ' (everything is paused)' : '') + '</p>' +
      (data.approval && data.approval.message ? '<p class="approval-note" data-test="modes-why">' + esc(data.approval.message) + '</p>' : '') +
      '<ul class="choices" style="list-style:none;padding:0;margin:0 0 16px">' + MODES.map(function (m) {
        var cur = m.v === sec.mode;
        return '<li class="choice" style="cursor:default;grid-template-columns:1fr"' + (cur ? ' aria-current="true"' : '') + '><span><b>' + esc(m.label) +
          (cur ? ' <span class="pill live">Current</span>' : '') + '</b><small>' + esc(m.text) + '</small></span></li>';
      }).join('') + '</ul>' +
      '<p class="caption">Making it stricter takes one tap in the dashboard. Making it looser needs you signed in there' + (signed ? '.' : ', and this page was opened from your agent.') + ' The hard checks apply in every mode.</p>' +
      '<div class="actions" style="margin-top:12px"><a class="btn secondary" data-test="modes-dashboard" href="' + esc(href) + '">Change it in the dashboard</a>' +
      '<button type="button" class="btn quiet" data-close>Close</button></div>';
    ui.open({ kind: 'modal', title: 'Approval for ' + sec.name, body: body, returnTo: opener, test: 'modes-modal' });
  }
  OR.openModes = openModes;
  document.addEventListener('click', function (e) {
    var a = e.target.closest && e.target.closest('[data-section] .mode a[data-test=change]');
    if (!a) return;
    e.preventDefault(); openModes(a.closest('[data-section]'), a);
  });

  // ---------- palette commands for this screen ----------
  var paused = function () { return !!(OR.digest && OR.digest.data && OR.digest.data.paused); };
  ui.command({ id: 'next', label: 'Next item', keys: ['J'], group: 'List', run: function () { move(1); } });
  ui.command({ id: 'prev', label: 'Previous item', keys: ['K'], group: 'List', run: function () { move(-1); } });
  ui.command({ id: 'open', label: 'Open item details', keys: ['↵'], group: 'List',
    available: function () { return lastItem && lastItem.isConnected ? true : 'Move to an item first (J or K)'; },
    run: function () { openPanel(lastItem, lastItem.querySelector('.digest-item')); } });
  ui.command({ id: 'undo', label: 'Undo this item', keys: ['R'], group: 'List',
    available: function () { return undoReason(lastItem && lastItem.isConnected ? lastItem : null); },
    run: function () { var b = undoButton(lastItem); if (b) b.click(); } });
  ui.command({ id: 'approve', label: 'Approve', keys: ['A'], group: 'List', available: function () { return APPROVE_REASON; }, run: function () {} });
  ui.command({ id: 'pause', label: 'Pause everything…', group: 'Account', hint: 'Asks you to confirm first',
    available: function () { var b = $('#pause-all'); return b && !paused() && !$('#kill').hidden ? true : 'Already paused'; },
    run: function () { $('#pause-all').click(); } });
  ui.command({ id: 'resume', label: 'Resume…', group: 'Account', hint: 'Asks you to confirm first',
    available: function () { if (!paused()) return 'Nothing is paused'; var b = $('#resume-all'); return b && !b.disabled ? true : 'Needs you signed in to the dashboard'; },
    run: function () { $('#resume-all').click(); } });
  ui.command({ id: 'modes', label: 'Approval modes…', group: 'Settings',
    available: function () { return $('[data-section] .mode a[data-test=change]') ? true : 'No sites or projects yet'; },
    run: function () { var a = $('[data-section] .mode a[data-test=change]'); openModes(a.closest('[data-section]'), a); } });
  ui.command({ id: 'continue-setup', label: 'Continue setup', group: 'Setup',
    available: function () { return $('[data-test=continue-setup]') ? true : 'Setup is finished'; },
    run: function () { location.href = $('[data-test=continue-setup]').getAttribute('href'); } });
  ui.command({ id: 'show-all', label: 'Show all items', group: 'List',
    available: function () { return $('[data-more]') ? true : 'Every item is already shown'; },
    run: function () { $all('[data-more]').forEach(function (b) { b.click(); }); } });
})();
