/* Offer Rails D-032 digest: every project (maker) and site (publisher) on one page.
   Data in : JSON in #or-digest-data (Go template) or fixtures/digest-<fixture>.json,
             or in real mode (?api=real) GET /v1/account/digest + GET /v1/digest (PR #21 fb493f0; paused comes with the digest).
   Server  : POST /v1/account/pause | /v1/account/resume {scope:"all"}; placements via <digest-item>.
   Data out: offerrails:digest-opened, offerrails:pause-all, offerrails:resume-all, offerrails:account-result
             (+ item events from <digest-item>), CustomEvents on document. See INTERFACE.md. */
(function () {
  'use strict';
  var params = new URLSearchParams(location.search);
  var TOAST_MS = Number(params.get('toast_ms')) || 6000;
  var SHOW_FIRST = 6;
  var PAGE = 50; // fb493f0 returns at most 50 items per project (LIMIT 50), with no paging
  var MODE_LABEL = { manual: 'Ask me every time', auto: 'Approve within my rules', yolo: 'Approve anything that passes the checks' };
  function esc(s) { return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); }
  function $(s, el) { return (el || document).querySelector(s); }
  function $all(s, el) { return Array.prototype.slice.call((el || document).querySelectorAll(s)); }
  function num(n) { return n == null ? '–' : Number(n).toLocaleString('en-GB'); }
  function day(iso) { try { return new Date(iso).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' }); } catch (e) { return ''; } }
  // Missing earnings are said plainly. Never a fake 0. One line per currency; currencies are never added.
  function fmtMoney(e) {
    return new Intl.NumberFormat('en-GB', { style: 'currency', currency: e.currency }).format(e.amount_minor / 100);
  }
  function moneyLines(e) {
    if (!e) return null;
    if (Array.isArray(e)) return e.length ? e.map(fmtMoney) : null;
    return e.amount_minor == null ? null : [fmtMoney(e)];
  }
  function money(e) { var l = moneyLines(e); return l ? l.join(' · ') : null; }
  function moneyHTML(e) {
    var l = moneyLines(e);
    if (l == null) return '<b class="none">No earnings data yet</b>';
    return '<span class="money-lines">' + l.map(function (x) { return '<b data-money>' + esc(x) + '</b>'; }).join('') + '</span>';
  }
  function emit(name, detail) { document.dispatchEvent(new CustomEvent(name, { detail: detail, bubbles: true })); }
  function plural(n, one, many) { return n + ' ' + (n === 1 ? one : many); }

  var d, toastTimer, toastItem;
  var OR = window.OfferRails;
  var REAL = OR.apiMode === 'real';

  // Real mode. Every read is a parity-table row of PR #21.
  function loadReal() {
    return Promise.all([OR.api.get('/v1/account/digest'), OR.api.get('/v1/digest')]).then(function (rs) { // endpoint: GET /v1/account/digest
      var acct = rs[0], dig = rs[1];                                                                         // endpoint: GET /v1/digest
      if (acct.status !== 200 || !acct.body) throw new Error('account digest ' + acct.status);
      return OR.contract.digestFromServer(acct.body, dig.status === 200 ? dig.body : null);
    });
  }
  // After anything the server confirmed, re-read the numbers: Live now is the server's went_live, not our arithmetic.
  var refreshTimer;
  function refreshNumbers() {
    if (!REAL) return;
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(function () {
      OR.api.get('/v1/account/digest').then(function (r) {
        if (r.status !== 200 || !r.body) return;
        (r.body.projects || []).forEach(function (p) {
          var sec = sections().filter(function (s) { return s.server_id === p.id; })[0]; if (!sec) return;
          sec.live_count = p.went_live;
          var b = $('[data-section="' + refOf(sec) + '"] [data-stat=live_count]'); if (b) b.textContent = num(p.went_live);
        });
        var t = r.body.totals || {}; d.totals.live_items = t.went_live;
        var tb = $('[data-total=live_items] b'); if (tb) tb.textContent = num(t.went_live);
        emit('offerrails:numbers-refreshed', { live_items: t.went_live });
      });
    }, 400);
  }
  // Modes from the server's pause/resume answer ({applied:[{kind,id,mode}]}); our own guess only for ids it does not list.
  function appliedModes(body) {
    var out = {};
    // 6042dcb: each row carries effective_mode (what applies now), approval_reason and approval_message.
    ((body && body.applied) || []).forEach(function (a) {
      var m = a && MODE_LABEL[a.effective_mode] ? a.effective_mode : a && a.mode;
      if (a && a.id && MODE_LABEL[m]) out[a.id] = m;
    });
    return out;
  }

  function appliedWhy(body) {
    var rows = (body && Array.isArray(body.applied)) ? body.applied : [];
    for (var i = 0; i < rows.length; i++) if (rows[i] && typeof rows[i].approval_reason === 'string' && rows[i].approval_reason)
      return { reason: rows[i].approval_reason, message: typeof rows[i].approval_message === 'string' ? rows[i].approval_message : '' };
    return null;
  }

  function load() {
    if (REAL) return loadReal();
    var inline = $('#or-digest-data');
    if (inline && inline.textContent.trim()) return Promise.resolve(JSON.parse(inline.textContent));
    var f = (params.get('fixture') || 'normal').replace(/[^a-z-]/g, '');
    return fetch('fixtures/digest-' + f + '.json').then(function (r) { if (!r.ok) throw new Error(f); return r.json(); });
  }

  function sections() { return (d.projects || []).concat(d.sites || []); }
  function refOf(s) { return s.project_id || s.site_id; }

  function render() {
    var t = d.totals || {}, signedIn = !!(d.session && d.session.signed_in);
    var nothing = !t.live_items && !t.went_live;
    $('#period-label').textContent = d.period ? 'Since ' + day(d.period.since) : '';
    $('#digest-root').innerHTML =
      '<div class="digest-head">' +
        '<div><p class="eyebrow">Digest · since ' + esc(day((d.period || {}).since)) + (d.test_mode ? ' · <span class="pill test">Test mode</span>' : '') + '</p>' +
        '<h1>All your projects and sites</h1>' +
        '<p>' + plural(t.projects || 0, 'project', 'projects') + ' and ' + plural(t.sites || 0, 'site', 'sites') + ' on one page. Undo any item with one tap.</p></div>' +
        '<div class="kill" id="kill">' +
          '<button type="button" class="btn secondary" id="pause-all" aria-controls="confirm-pause" aria-expanded="false">Pause everything</button>' +
          '<span class="caption">Back to “Ask me every time” everywhere</span>' +
        '</div>' +
      '</div>' +
      '<div class="confirm" id="confirm-pause" hidden>' +
        '<p><strong>Pause everything?</strong> All ' + plural((d.sites || []).length, 'site', 'sites') + ' and ' + plural((d.projects || []).length, 'project', 'projects') +
        ' go back to “Ask me every time”. Nothing new goes live until you approve it. Boxes already live stay until you undo them.</p>' +
        '<div class="actions"><button type="button" class="btn danger" id="confirm-pause-yes">Yes, pause everything</button>' +
        '<button type="button" class="btn quiet" id="confirm-pause-no">Cancel</button></div>' +
      '</div>' +
      '<div class="paused-banner" id="paused-banner" hidden>' +
        '<p><strong>Everything is paused.</strong> Every site and project asks you before anything new goes live.</p>' +
        '<div class="kill"><button type="button" class="btn secondary" id="resume-all" aria-controls="confirm-resume" aria-expanded="false"' + (signedIn ? '' : ' disabled aria-describedby="resume-note"') + '>Resume</button>' +
        '<span class="session-only" id="resume-note">' + (signedIn ? 'Signed-in only' : 'Sign in to the dashboard to resume') + '</span></div>' +
      '</div>' +
      '<div class="confirm resume" id="confirm-resume" hidden>' +
        '<p><strong>Resume?</strong> Each site and project goes back to the approval mode it had before the pause.</p>' +
        '<div class="actions"><button type="button" class="btn primary" id="confirm-resume-yes">Yes, resume</button>' +
        '<button type="button" class="btn quiet" id="confirm-resume-no">Cancel</button></div>' +
      '</div>' +
      '<div class="alert" id="kill-error" role="alert" tabindex="-1" hidden><p></p></div>' +
      '<p class="outcome card" id="kill-outcome" data-test="kill-outcome" role="status" hidden></p>' +
      continuePill() +
      (signedIn ? '' : '<p class="approval-note" data-test="key-note"><b>Opened from your agent.</b> You can undo, make approval stricter, pause and stop everything here. ' +
        'Putting a box back, Redo, approving and plugin keys need the dashboard: sign in there.</p>') +
      (d.approval && d.approval.message ? '<p class="approval-note" data-test="approval-note"><b>Every item asks you first for now.</b> ' + esc(d.approval.message) + '</p>' : '') +
      '<section class="totals" aria-label="Account totals" data-test="totals">' +
        tile('Live items', num(t.live_items), t.went_live != null ? plural(t.went_live, 'item', 'items') + ' went live since last digest' : '', 'live_items') +
        tile('Clicks', num(t.clicks), 'since last digest', 'clicks') +
        tile('Earnings', moneyHTML(t.earnings), d.test_mode ? 'Test mode, not real money' : 'since last digest', 'earnings', true) +
        tile('Projects', num(t.projects), 'you promote', 'projects') +
        tile('Sites', num(t.sites), 'you publish on', 'sites') +
      '</section>' +
      (nothing ? '<section class="empty-state card" data-test="empty"><div class="empty-inner">' + (OR.ui ? OR.ui.blip() : '') + '<div>' +
        '<p class="eyebrow">Digest</p><h2>All clear.</h2>' +
        '<p data-test="next-digest">Your next digest arrives ' + nextDigest() + '. Boxes and listings show up here once one goes live, each with one-tap undo.</p>' +
        '<div class="empty-actions"><button type="button" class="btn secondary" id="copy-scan" data-test="empty-action">Copy the scan command</button>' +
        '<span class="done-note" id="copy-note" role="status"></span></div></div></div></section>' : '') +
      group('Your projects', 'You promote these', d.projects || [], 'Project') +
      group('Your sites', 'Boxes appear on these', d.sites || [], 'Site');

    OR.signedIn = signedIn;  // (c) before items paint: Redo / putting back are session-only
    OR.paused = !!d.paused; // set before items paint, so the first count already follows the go-link rule
    sections().forEach(function (s) {
      var ul = $('[data-items="' + refOf(s) + '"]'); if (!ul) return;
      (s.items || []).forEach(function (it, i) {
        var li = document.createElement('li'); if (i >= SHOW_FIRST) li.hidden = true;
        var el = document.createElement('digest-item'); el.item = it; li.appendChild(el); ul.appendChild(li);
      });
    });
    $all('[data-more]').forEach(function (b) {
      b.addEventListener('click', function () { $all('li[hidden]', $('[data-items="' + b.dataset.more + '"]')).forEach(function (li) { li.hidden = false; }); b.remove(); });
    });
    wireKill();
    wireEmpty();
    setPaused(!!d.paused, true);
    var root = $('#digest-root'); root.removeAttribute('aria-busy'); root.classList.add('first-load');
    OR.digest = { data: d, sections: sections, refOf: refOf, MODE_LABEL: MODE_LABEL, setupMarker: setupMarker };
    emit('offerrails:digest-rendered', {});
  }

  // Anticipatory detail: the real date and the timezone, when the server gives one. Otherwise say when, plainly.
  function nextDigest() {
    var at = d.next_digest_at, tz = '';
    try { tz = Intl.DateTimeFormat().resolvedOptions().timeZone || ''; } catch (e) {}
    if (!at || isNaN(new Date(at))) return 'as soon as something goes live';
    var dt = new Date(at);
    return 'on ' + dt.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' }) + ' at ' +
      dt.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' }) + (tz ? ' (' + tz + ')' : '') + ', or sooner if something goes live';
  }
  // Lesson 2: setup can be skipped; one pill resumes it at the first unanswered question.
  function setupMarker() { try { var m = JSON.parse(localStorage.getItem('offerrails.setup') || 'null'); return m && m.state === 'skipped' ? m : null; } catch (e) { return null; } }
  function continuePill() {
    var m = setupMarker(); if (!m) return '';
    var q = Math.min(4, Math.max(1, Number(m.resume) || 1)), keep = new URLSearchParams();
    ['api', 'api_base', 'mock'].forEach(function (k) { if (params.get(k)) keep.set(k, params.get(k)); });
    keep.set('resume', q);
    return '<div class="continue-row" data-test="continue-row"><a class="continue-setup" data-test="continue-setup" href="setup.html?' + esc(keep.toString()) + '">Continue setup <span aria-hidden="true">→</span></a>' +
      '<span class="caption">You stopped at question ' + q + ' of 4. Your agent keeps working with what it has.</span></div>';
  }
  function wireEmpty() {
    var b = $('#copy-scan'); if (!b) return;
    b.addEventListener('click', function () {
      var cmd = 'npx offer-rails', note = $('#copy-note');
      function ok() { note.textContent = 'Copied: ' + cmd + '. Paste it into your terminal or agent to run a scan.'; }
      function fallback() {
        var ta = document.createElement('textarea'); ta.value = cmd; ta.setAttribute('readonly', ''); ta.style.position = 'fixed'; ta.style.opacity = '0';
        document.body.appendChild(ta); ta.select(); var done = false; try { done = document.execCommand('copy'); } catch (e) {} ta.remove();
        if (done) ok(); else note.textContent = 'Couldn\'t copy. The command is: ' + cmd;
        b.focus();
      }
      if (navigator.clipboard && navigator.clipboard.writeText) navigator.clipboard.writeText(cmd).then(ok, fallback); else fallback();
    });
  }
  function tile(label, value, sub, key, html) {
    return '<div class="total" data-total="' + key + '"><span class="flag">' + esc(label) + '</span>' +
      (html ? value : value == null ? '<b class="none">No earnings data yet</b>' : '<b>' + esc(value) + '</b>') +
      (sub ? '<small>' + esc(sub) + '</small>' : '') + '</div>';
  }

  function group(title, sub, list, kind) {
    if (!list.length) return '';
    return '<div class="group-title"><h2>' + esc(title) + '</h2><span class="caption">' + esc(sub) + '</span></div>' + list.map(function (s) { return section(s, kind); }).join('');
  }

  function section(s, kind) {
    var ref = refOf(s), items = s.items || [];
    var commission = s.earnings_kind ? s.earnings_kind === 'commission' : kind === 'Project'; // a maker's amount is commission paid, not income
    var src = (s.top_sources || []).slice(0, 3);
    return '<section class="project" data-section="' + esc(ref) + '" aria-labelledby="h-' + esc(ref) + '">' +
      '<div class="project-head"><div><span class="flag">' + kind + '</span><h3 id="h-' + esc(ref) + '">' + esc(s.name) +
        (s.paused ? ' <span class="pill paused" data-test="section-paused">Paused</span>' : '') + '</h3>' +
        (s.url ? '<span class="caption" data-test="site-url">' + esc(s.url.replace(/^https?:\/\//, '')) + '</span>' : '') +
        (REAL && kind === 'Site' && s.server_id ? '<button type="button" class="btn quiet site-settings" data-site-settings="' + esc(ref) + '" aria-label="Settings for ' + esc(s.name) + '">Settings</button>' : '') + '</div>' +
        '<div class="mode"><span class="caption">Approval</span><b data-mode="' + esc(ref) + '">' + esc(MODE_LABEL[s.mode] || '') + '</b>' +
        '<a href="' + esc(s.mode_change_url || '#preview') + '" data-test="change" aria-label="Change approval for ' + esc(s.name) + '">Change</a>' +
        (d.approval && d.approval.message && s.mode === 'manual' ? '<span class="caption why" data-test="mode-why">' + esc(d.approval.message) + '</span>' : '') +
        '<span class="session-only" title="Stricter is one tap. Looser needs you signed in to the dashboard.">Looser: signed-in only</span></div></div>' +
      '<div class="stats">' +
        (s.went_live_count == null ? '' : stat('Went live', num(s.went_live_count))) + stat('Live now', num(s.live_count), 'live_count') + stat('Clicks', num(s.clicks)) +
        '<div class="stat" data-stat-earnings="' + (commission ? 'commission' : 'income') + '"><span class="flag">' + (commission ? 'Commission to pay' : 'Earnings') + (d.test_mode ? ' · test' : '') + '</span>' + moneyHTML(s.earnings) +
          (commission ? '<small class="caption">Not income</small>' : '') + '</div>' +
        '<div class="stat"><span class="flag">Top click sources</span>' + (src.length ? '<ul class="sources">' + src.map(function (x) {
          return '<li><span>' + esc(x.source) + '</span><span>' + num(x.clicks) + '</span></li>'; }).join('') + '</ul>' : '<span class="caption">No clicks yet</span>') + '</div>' +
      '</div>' +
      (items.length ? '<ul class="items" data-items="' + esc(ref) + '" aria-label="Live items for ' + esc(s.name) + '"></ul>' +
        (REAL && pageNote(s, items) ? '<p class="caption page-note" data-test="page-note">' + pageNote(s, items) +
          ' <a href="' + esc(s.mode_change_url || '#preview') + '">See them all in the dashboard</a></p>' : '') +
        (items.length > SHOW_FIRST ? '<p style="padding:0 24px 16px;margin:0"><button type="button" class="btn secondary" data-more="' + esc(ref) + '">Show all ' + items.length + ' items</button></p>' : '')
        : '<p class="items-empty">Nothing live here yet.</p>') +
      '</section>';
  }
  // 6042dcb: total_items is every item for the project; the digest sends at most 50 (no paging).
  // truncated is the server flag that this page was cut, even when total_items is not larger than the page.
  function pageNote(s, items) {
    var count = '';
    if (typeof s.total_items === 'number') {
      count = s.total_items > items.length ? 'Showing the latest <b>' + num(items.length) + '</b> of <b>' + num(s.total_items) + '</b>.' : '';
    } else if (items.length >= PAGE && s.live_count > itemCount(items)) {
      count = 'Showing the latest ' + plural(itemCount(items), 'item', 'items') + '. ' + num(s.live_count) + ' are live.';
    }
    if (s.truncated === true) {
      var cut = 'The list was cut.';
      return count ? count + ' ' + cut : cut;
    }
    return count;
  }
  function itemCount(items) { return items.filter(function (i) { return i.kind !== 'listing'; }).length; }
  function stat(l, v, key) { return '<div class="stat"><span class="flag">' + esc(l) + '</span><b' + (key ? ' data-stat="' + key + '"' : '') + '>' + esc(v) + '</b></div>'; }

  function wireKill() {
    var pause = $('#pause-all'), cp = $('#confirm-pause'), resume = $('#resume-all'), cr = $('#confirm-resume');
    pause.addEventListener('click', function () { cp.hidden = false; pause.setAttribute('aria-expanded', 'true'); $('#confirm-pause-yes').focus(); });
    $('#confirm-pause-no').addEventListener('click', function () { cp.hidden = true; pause.setAttribute('aria-expanded', 'false'); pause.focus(); });
    $('#confirm-pause-yes').addEventListener('click', function () {
      var before = {}, yes = this;
      sections().forEach(function (s) { before[refOf(s)] = s.mode; });
      var body = { scope: 'all' };
      emit('offerrails:pause-all', {
        digest_id: d.digest_id, user_id: d.user_id, scope: 'all',
        site_ids: (d.sites || []).map(refOf), project_ids: (d.projects || []).map(refOf),
        modes_before: before, mode_after: 'manual', pause_unpublished_auto_approvals: true,
        request: { method: 'POST', path: '/v1/account/pause', body: body },
        actor: 'owner_ui', session_only: false, at: new Date().toISOString() });
      yes.disabled = true; yes.textContent = 'Pausing…'; yes.setAttribute('aria-busy', 'true'); killError('');
      OR.api.post('/v1/account/pause', body).then(function (r) {
        yes.disabled = false; yes.textContent = 'Yes, pause everything'; yes.setAttribute('aria-busy', 'false');
        var ok = r.status >= 200 && r.status < 300;
        emit('offerrails:account-result', { action: 'pause', scope: 'all', http_status: r.status, outcome: ok ? 'paused' : 'error' });
        if (!ok) { killError("Couldn't pause right now. Nothing changed. Try again."); return; }
        var am = appliedModes(r.body);
        sections().forEach(function (s) { s.mode_before_pause = s.mode; s.mode = am[s.server_id] || 'manual'; });
        cp.hidden = true; setPaused(true);
        var ap = (r.body && Array.isArray(r.body.applied)) ? r.body.applied : null;
        var ns = ap ? ap.filter(function (a) { return a.kind === 'site'; }).length : (d.sites || []).length;
        var np = ap ? ap.filter(function (a) { return a.kind !== 'site'; }).length : (d.projects || []).length;
        outcome('<b>Paused ' + plural(ns, 'site', 'sites') + ' and ' + plural(np, 'project', 'projects') + '.</b> They ask you before anything new goes live, and tracked links stop counting clicks. ' +
          '<b>Not changed:</b> boxes already in posts stay there until you undo them.');
        $('#digest-status').textContent = 'Everything is paused.';
      });
    });
    resume.addEventListener('click', function () { cr.hidden = false; resume.setAttribute('aria-expanded', 'true'); $('#confirm-resume-yes').focus(); });
    $('#confirm-resume-no').addEventListener('click', function () { cr.hidden = true; resume.setAttribute('aria-expanded', 'false'); resume.focus(); });
    $('#confirm-resume-yes').addEventListener('click', function () {
      if (!(d.session && d.session.signed_in)) return;
      var restore = {}, yes = this, body = { scope: 'all' };
      sections().forEach(function (s) { restore[refOf(s)] = s.mode_before_pause || s.mode; });
      emit('offerrails:resume-all', {
        digest_id: d.digest_id, user_id: d.user_id, scope: 'all', restore_modes: restore,
        request: { method: 'POST', path: '/v1/account/resume', body: body },
        actor: 'owner_ui', session_only: true, at: new Date().toISOString() });
      yes.disabled = true; yes.textContent = 'Resuming…'; yes.setAttribute('aria-busy', 'true'); killError('');
      OR.api.post('/v1/account/resume', body).then(function (r) {
        yes.disabled = false; yes.textContent = 'Yes, resume'; yes.setAttribute('aria-busy', 'false');
        var ok = r.status >= 200 && r.status < 300;
        emit('offerrails:account-result', { action: 'resume', scope: 'all', http_status: r.status, outcome: ok ? 'resumed' : r.status === 403 ? 'forbidden' : 'error' });
        if (!ok) {
          killError(r.status === 403 ? 'Resume needs you signed in to the dashboard in this browser. Sign in again, then press Resume. Everything stays paused until then.'
            : "Couldn't resume right now. Everything stays paused. Try again.");
          return;
        }
        var am = appliedModes(r.body);
        // applied[].mode is the saved mode. While the claim check is a stub the effective mode stays manual (approval_reason).
        var why = appliedWhy(r.body);
        d.approval = why ? { reason: why.reason, message: why.message, effective_mode: 'manual' } : (Array.isArray(r.body && r.body.applied) ? null : d.approval);
        sections().forEach(function (s) { s.mode = am[s.server_id] || (d.approval ? 'manual' : restore[refOf(s)]); });
        cr.hidden = true; setPaused(false);
        outcome('<b>Resumed.</b> Each site and project is back on its previous approval mode' + (d.approval ? ' (automatic approval stays off for now: ' + esc(d.approval.message || '') + ')' : '') + '. ' +
          '<b>Not changed:</b> anything you undid while paused stays undone.');
        $('#digest-status').textContent = 'Resumed. Each site and project is back on its previous approval mode.';
      });
    });
  }

  function outcome(html) { var o = $('#kill-outcome'); if (!o) return; o.innerHTML = html; o.hidden = !html; }
  function killError(msg) { var e = $('#kill-error'); e.hidden = !msg; $('p', e).textContent = msg; if (msg) e.focus(); }

  function setPaused(p, initial) {
    d.paused = p;
    var was = OR.paused; OR.paused = p;
    // Pause stops every go link, so nothing counts as live while paused (PR #21 placement_counts_as_live).
    if (!initial && was !== p) $all('digest-item').forEach(function (el) { if (el.paint && el._rendered) el.paint(); });
    $('#kill').hidden = p; $('#paused-banner').hidden = !p;
    sections().forEach(function (s) { var m = $('[data-mode="' + refOf(s) + '"]'); if (m) m.textContent = MODE_LABEL[s.mode] || ''; });
    if (!initial) (p ? $('#resume-all') : $('#pause-all')).focus();
  }

  // Undo toast with Redo. The revoke already happened; the toast is only the Redo window.
  // "Live now" (section) and "Live items" (account) follow every change in what counts as live.
  function bumpLive(el, delta) {
    var secEl = el.closest('[data-section]'); if (!secEl) return;
    var ref = secEl.getAttribute('data-section');
    var sec = sections().filter(function (s) { return refOf(s) === ref; })[0];
    if (sec) { sec.live_count = Math.max(0, (sec.live_count || 0) + delta); var b = $('[data-stat=live_count]', secEl); if (b) b.textContent = num(sec.live_count); }
    var t = d.totals || (d.totals = {});
    t.live_items = Math.max(0, (t.live_items || 0) + delta);
    var tb = $('[data-total=live_items] b'); if (tb) tb.textContent = num(t.live_items);
  }
  function placeName(it) { return (it.site || {}).name || 'the page'; }
  function showToast(el) {
    if (toastItem && toastItem !== el) toastItem.closeWindow();
    toastItem = el;
    var t = $('#toast'), it = el.item;
    // Lesson 5: say what was reverted and what was not (yet).
    $('#toast-text').textContent = 'Undone. Clicks are off now. Not yet reverted: the box stays in the post until ' + placeName(it) +
      ' takes it out, and cached copies of the page may show it a little longer.';
    var dash = OR.signedIn === false;
    if (dash) $('#toast-text').textContent += ' Redo needs the dashboard.';
    $('#toast-redo').hidden = dash; $('#toast-redo').disabled = dash;
    t.style.setProperty('--toast-ms', TOAST_MS + 'ms');
    t.hidden = false;
    var bar = $('.bar', t); bar.hidden = false; bar.style.animation = 'none'; void bar.offsetWidth; bar.style.animation = '';
    clearTimeout(toastTimer);
    toastTimer = setTimeout(hideToast, TOAST_MS);
  }
  function toastMessage(msg) {
    clearTimeout(toastTimer);
    $('#toast-text').textContent = msg; $('#toast-redo').hidden = true; $('.bar', $('#toast')).hidden = true;
    $('#toast').hidden = false;
    toastTimer = setTimeout(hideToast, 5000);
  }
  function hideToast() {
    clearTimeout(toastTimer);
    $('#toast').hidden = true;
    if (toastItem) toastItem.closeWindow();
    toastItem = null;
  }

  document.addEventListener('offerrails:placement-result', function (e) {
    var el = e.target.closest ? e.target.closest('digest-item') : null; if (!el) return;
    var r = e.detail;
    if (r.action === 'undo' && r.outcome === 'revoked' && r.window_open) showToast(el);
    if (/^(revoked|restored|retried|not_failed)$/.test(r.outcome)) refreshNumbers();
    if (r.action === 'restore') {
      if (r.outcome === 'restored') { toastItem = null; hideToast(); }
      else { toastItem = null; toastMessage(r.message); }
    }
  });
  // Items report when they start/stop counting as live (undo, restore, failed restore, retried restore placed).
  document.addEventListener('offerrails:live-count', function (e) {
    var el = e.target.closest ? e.target.closest('digest-item') : null; if (el) bumpLive(el, e.detail.delta);
  });
  document.addEventListener('offerrails:plugin-state', function () { refreshNumbers(); });
  document.addEventListener('offerrails:account-result', function (e) { if (/^(paused|resumed)$/.test(e.detail.outcome)) refreshNumbers(); });
  document.addEventListener('DOMContentLoaded', function () {
    $('#toast-redo').addEventListener('click', function () {
      var el = toastItem; if (!el) return;
      this.disabled = true; $('#toast-text').textContent = 'Bringing it back…';
      clearTimeout(toastTimer);
      el.restore().then(function () { var b = el.querySelector('[data-action=toggle]'); if (b && !b.hidden) b.focus(); });
    });
  });

  var SKELETON = '';
  function boot() {
    if (!SKELETON) SKELETON = $('#digest-root').innerHTML;
    load().then(function (data) {
      d = data;
      render();
      emit('offerrails:digest-opened', { digest_id: d.digest_id, user_id: d.user_id, opened_at: new Date().toISOString(),
        opened_from: params.get('from') || 'web', signed_in: !!(d.session && d.session.signed_in) });
      // Recorded after GET /v1/digest read "since", so this open does not hide what it shows.
      if (REAL) OR.api.post('/v1/events', { kind: 'digest_opened' }); // endpoint: POST /v1/events
    }).catch(function () {
      var root = $('#digest-root'); root.removeAttribute('aria-busy');
      root.innerHTML = '<h1 class="sr-only">Your digest</h1><div class="alert" role="alert" data-test="load-error"><p><strong>We couldn\'t load your digest.</strong> Nothing changed on your sites. Try again; if it keeps failing, your agent can show the same numbers.</p>' +
        '<p style="margin-top:12px"><button type="button" class="btn secondary" id="retry-load">Try again</button></p></div>';
      $('#retry-load').addEventListener('click', function () { root.setAttribute('aria-busy', 'true'); root.innerHTML = SKELETON; boot(); });
    });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot); else boot();
})();
