/* Offer Rails D-032 setup page. Data in: JSON in #or-setup-data (Go template) or
   fixtures/setup-<fixture>.json. Data out: CustomEvent "offerrails:setup-submit"
   on document (see INTERFACE.md). Saving: window.OfferRails.saveSetup(payload)
   returns a Promise; the host replaces it with the real API call.
   Real mode (?api=real, base URL per assets/api.js): GET /v1/setup/settings in, PUT /v1/setup/settings out,
   one PUT per role, exactly the keys PutSetup accepts (PR #21 fb493f0).
   Hard checks: GET /v1/policy/hard-checks, rendered as returned (no list in this file). */
(function () {
  'use strict';
  var OR = (window.OfferRails = window.OfferRails || {});
  var params = new URLSearchParams(location.search);

  // Plain labels only. Internal values never reach the screen.
  var MODES = [
    { value: 'manual', label: 'Ask me every time',
      maker: 'No publisher shows your offer until you approve them here or in the digest.',
      publisher: 'Nothing goes live on your pages until you approve it here or in the digest.' },
    { value: 'auto', label: 'Approve within my rules',
      maker: 'Your agent accepts publishers that fit your rules (site quality, niches). Anything else waits for you.',
      publisher: 'Your agent approves boxes that fit your rules (relevance 40+, up to 3 new boxes a month). Anything else waits for you.' },
    { value: 'yolo', label: 'Approve anything that passes the checks',
      maker: 'Any publisher that passes the checks below can show your offer right away. Undo any item from the digest.',
      publisher: 'A box goes live as soon as it passes the checks below. Undo any item from the digest.' }
  ];
  var POSITIONS = [
    { value: 'after_intro', label: 'Start of article', help: 'After the first paragraph.' },
    { value: 'matched_section', label: 'Matching section', help: 'Next to the paragraph it fits best.' },
    { value: 'end', label: 'End of article', help: 'Before your sign-off. Least intrusive.' }
  ];
  // PR #21 fb493f0 commission.type values. A fixed amount is in the chosen currency.
  var COMMISSION_TYPES = [
    { value: 'percent', label: '% per sale' },
    { value: 'fixed', label: 'Fixed amount per sale' }
  ];
  var CURRENCIES = ['EUR', 'USD', 'GBP'];
  // install_path values from PR #21 (cli, cursor, connector, grok, chatgpt_dir, claude_dir) + the earlier source names.
  var SOURCE_NAMES = { cli: 'npx offer-rails', mcp: 'MCP', cursor: 'Cursor', claude: 'Claude', chatgpt: 'ChatGPT', grok: 'Grok',
    connector: 'a connector', chatgpt_dir: 'ChatGPT', claude_dir: 'Claude' };
  var RETURN_TO = { cli: 'your terminal', mcp: 'your agent', cursor: 'Cursor', claude: 'Claude', chatgpt: 'ChatGPT', grok: 'Grok',
    connector: 'your agent', chatgpt_dir: 'ChatGPT', claude_dir: 'Claude' };
  // Old position values still accepted on input.
  var POS_IN = { start: 'after_intro', section: 'matched_section' };

  function esc(s) { return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); }
  function list(a) { return (a || []).join(', '); }
  function split(s) { return String(s || '').split(',').map(function (x) { return x.trim(); }).filter(Boolean); }
  function $(sel, el) { return (el || document).querySelector(sel); }
  function $all(sel, el) { return Array.prototype.slice.call((el || document).querySelectorAll(sel)); }

  var REAL = OR.apiMode === 'real';
  if (REAL && !OR.saveSetup) OR.saveSetup = function (detail) {
    var bodies = OR.contract.setupPutBodies(detail, data._server), i = 0;
    function next() {
      if (i >= bodies.length) return Promise.resolve({ ok: true });
      return OR.api.put('/v1/setup/settings', bodies[i++]).then(function (r) { // endpoint: PUT /v1/setup/settings
        if (r.status < 200 || r.status >= 300) throw new Error('save ' + r.status);
        return next();
      });
    }
    return next();
  };
  OR.saveSetup = OR.saveSetup || function () {
    // Preview only: ?fail=save simulates the API refusing.
    return new Promise(function (resolve, reject) {
      setTimeout(function () { params.get('fail') === 'save' ? reject(new Error('preview: save failed')) : resolve({ ok: true }); }, 250);
    });
  };

  function load() {
    if (REAL) {
      var role = (params.get('role') || '').replace(/[^a-z]/g, '');
      // 2d3aed7: the server defaults install_path from owners.source, so the UI never sends ?install_path= (or depends on it).
      var qs = [];
      if (role) qs.push('role=' + role);
      return OR.api.get('/v1/setup/settings' + (qs.length ? '?' + qs.join('&') : '')).then(function (r) { // endpoint: GET /v1/setup/settings
        if (r.status !== 200 || !r.body) throw new Error('setup ' + r.status);
        return OR.contract.setupFromServer(r.body);
      });
    }
    var inline = $('#or-setup-data');
    if (inline && inline.textContent.trim()) return Promise.resolve(JSON.parse(inline.textContent));
    var f = (params.get('fixture') || 'maker').replace(/[^a-z-]/g, '');
    return fetch('fixtures/setup-' + f + '.json').then(function (r) { if (!r.ok) throw new Error('fixture ' + f); return r.json(); });
  }

  var data, initial = {}, modeShown, modeChecked, modeSuggested;
  var UNAVAILABLE_WHY = 'Not available yet.';
  // PR #21 fb493f0: available_modes, plus approval_reason/approval_message while the claim check is a stub
  // (then mode is manual and auto/yolo can't be picked). The reason shown is the server's approval_message.
  function approvalOf(d) {
    if (d.approval) return d.approval;
    return typeof d.approval_reason === 'string' && d.approval_reason ? { reason: d.approval_reason, message: d.approval_message } : null;
  }
  // (c) Real mode without a signed-in session = a key/agent opened this page. Saving (PUT) is session-only.
  function keyContext(d) { return !!(d.real && !(d.session && d.session.signed_in)); }
  // Design offer-rails.md:147 (Product Lead, v9): a site starts in mode manual with mode_preselected yolo until its first approved
  // placement. That is YOLO picked and waiting, never a lost choice: show YOLO checked plus why it is not on yet.
  function yoloPending(d) {
    var eff = d.mode_effective || d.mode_saved;
    return d.mode_preselected === 'yolo' && (!eff || eff === 'manual') && !(Array.isArray(d.available_modes) && d.available_modes.indexOf('yolo') < 0);
  }
  var PENDING_REASON = 'yolo_after_first_placement';
  var PENDING_FALLBACK = 'Until then, you approve each new box yourself.';
  function modeAvailable(d, v) {
    if (v === 'yolo' && yoloPending(d)) return true;
    // f249a77: approval_reason yolo_after_first_placement only says YOLO is waiting; it does not lock the other modes.
    if (approvalOf(d) && approvalOf(d).reason === PENDING_REASON) return !(Array.isArray(d.available_modes) && d.available_modes.indexOf(v) < 0);
    if (Array.isArray(d.available_modes) && d.available_modes.indexOf(v) < 0) return false;
    return !(approvalOf(d) && v !== 'manual');
  }
  function plainNote(t) { return typeof t === 'string' && t.length && t.length < 200 && !/[{}\[\]<>]/.test(t) ? t : null; }

  // The server decides the preselected mode (install path rules live there). Missing = nothing preselected.
  function defaultMode(d) { return ['manual', 'auto', 'yolo'].indexOf(d.mode_preselected) >= 0 ? d.mode_preselected : null; }

  function render(d) {
    data = d;
    var agent = d.agent || {}, project = d.project || {};
    var roles = d.detected_roles && d.detected_roles.length ? d.detected_roles : [];
    var primary = roles[0] || 'maker';
    // Suggested = the server's mode_preselected, only if it can be picked. Checked = the effective (saved) mode if it can be
    // picked, else the suggestion. mode_shown sent back = what was checked when the page opened.
    modeSuggested = defaultMode(d);
    if (modeSuggested && !modeAvailable(d, modeSuggested)) modeSuggested = null; // never preselect an option you can't pick
    var eff = d.mode_effective || d.mode_saved;
    modeChecked = yoloPending(d) ? 'yolo' : ['manual', 'auto', 'yolo'].indexOf(eff) >= 0 && modeAvailable(d, eff) ? eff : modeSuggested;
    modeShown = modeChecked;
    var srcName = SOURCE_NAMES[agent.source] || agent.client_name || agent.source || 'your agent';
    var who = d.session && d.session.signed_in ? 'Signed in' + (d.session.device_code ? ' · code ' + esc(d.session.device_code) + ' approved' : '') : '';
    $('#signed-in').innerHTML = who;

    var root = $('#setup-root');
    root.innerHTML =
      '<section class="connected" aria-labelledby="h-connected">' +
        '<div class="rail-line" aria-hidden="true">' +
          '<div class="station"><span class="flag">Agent</span><strong data-test="agent-source">' + esc(srcName) + '</strong></div>' +
          '<div class="track mid"></div>' +
          '<div class="station mid"><span class="flag">Offer Rails</span><strong>Signed in</strong></div>' +
          '<div class="track"></div>' +
          '<div class="station"><span class="flag">' + (primary === 'publisher' ? 'Site' : 'Project') + '</span><strong data-test="project-name">' + esc(project.name || 'Not found yet') + '</strong></div>' +
        '</div>' +
        '<p class="eyebrow">Connected via ' + esc(srcName) + '</p>' +
        '<h1 id="h-connected">Your agent is connected</h1>' +
        '<p>' + (project.name
          ? 'It scanned <strong>' + esc(project.name) + '</strong> and filled in everything below. Check it, then press Continue.'
          : 'It could not find a project or site yet. Fill in what you know, or press Continue and let your agent finish later.') + '</p>' +
      '</section>' +
      '<div id="save-error" class="alert" role="alert" hidden><p><strong>We couldn\'t save your settings.</strong> Nothing changed and your answers are still here. Try again.</p></div>' +
      '<form id="setup-form" novalidate>' +
        '<div class="roles">' +
          '<div class="seg" role="radiogroup" aria-label="I am here as">' +
            roleRadio('maker', 'Maker', primary) + roleRadio('publisher', 'Publisher', primary) +
          '</div>' +
          '<span class="caption">' + (roles.length ? 'Detected from your scan' : 'We couldn\'t tell. Pick one.') + '</span>' +
        '</div>' +
        '<label class="also"><input type="checkbox" id="also-role"' + (roles.length > 1 ? ' checked' : '') + '> <span id="also-label"></span></label>' +
        '<div id="questions"></div>' +
        '<section class="checks" aria-labelledby="h-checks" aria-busy="true"><h2 id="h-checks">Rules every listing must follow</h2><ul data-test="hard-checks"></ul>' +
          '<p class="caption hc-error" data-test="hard-checks-error" hidden>We couldn\'t load the list of rules right now. They still apply to every listing.</p></section>' +
        (keyContext(d) ? '<p class="approval-note" data-test="key-note" id="key-note"><b>Opened from your agent.</b> Your agent can only make approval stricter. ' +
          'Choosing these answers and saving them needs the dashboard: sign in there, then open this page again.</p>' : '') +
        '<div class="actions"><button type="submit" class="keycap" id="continue" aria-describedby="continue-reassure' + (keyContext(d) ? ' key-note" disabled' : '"') + '>' +
          '<span class="keycap-face">Continue</span></button>' +
        '<span class="reassure" id="continue-reassure">Nothing goes live until it follows the rules above. You can change any of this later from the dashboard.</span>' +
        '<button type="button" class="btn quiet skip-setup" id="skip-setup" data-test="skip-setup">Skip for now</button></div>' +
      '</form>';

    $all('input[name=role]').forEach(function (r) { r.addEventListener('change', function () { renderQuestions(r.value); }); });
    $('#setup-form').addEventListener('submit', onSubmit);
    renderQuestions(primary);
    loadHardChecks();
    $('#skip-setup').addEventListener('click', skip);
    root.removeAttribute('aria-busy'); root.classList.add('first-load');
    resume();
  }
  function face(btn, text) { var f = btn.querySelector('.keycap-face'); (f || btn).textContent = text; }
  // Lesson 2: setup never blocks. Skip remembers the first unanswered question; the digest shows one "Continue setup" pill.
  function firstUnanswered() {
    var role = currentRole();
    if (role === 'maker') {
      if (!val('listing-name') && !val('listing-pitch')) return 1;
      if (val('commission-value') === '') return 3;
    } else if (!$all('input[name=site]:checked').length && !val('extra-site')) return 1;
    return $('input[name=mode]:checked') ? 4 : 4;
  }
  function skip() {
    var q = firstUnanswered(), keep = new URLSearchParams();
    ['api', 'api_base', 'mock'].forEach(function (k) { if (params.get(k)) keep.set(k, params.get(k)); });
    try { localStorage.setItem('offerrails.setup', JSON.stringify({ state: 'skipped', resume: q, at: new Date().toISOString() })); } catch (e) {}
    document.dispatchEvent(new CustomEvent('offerrails:setup-skipped', { detail: { resume: q }, bubbles: true }));
    location.href = 'digest.html' + (keep.toString() ? '?' + keep.toString() : '');
  }
  function resume() {
    var n = Number(params.get('resume')); if (!(n >= 1 && n <= 4)) return;
    var fs = $all('#questions fieldset.q')[n - 1]; if (!fs) return;
    fs.insertAdjacentHTML('beforebegin', '<p class="caption" data-test="resume-note" id="resume-note">Picking up where you left off: question ' + n + ' of 4.</p>');
    var target = $all('input:not([disabled]):not([type=hidden]),select,button', fs).filter(function (x) { return x.offsetParent !== null; })[0];
    if (target) { target.focus(); target.scrollIntoView({ block: 'center' }); }
  }
  if (OR.ui) {
    OR.ui.command({ id: 'save', label: 'Save and continue', group: 'Setup', hint: 'Same as the Continue button',
      available: function () { var b = $('#continue'); return b && !b.disabled ? true : 'Saving needs you signed in to the dashboard'; },
      run: function () { $('#continue').click(); } });
    OR.ui.command({ id: 'skip', label: 'Skip setup for now', group: 'Setup', hint: 'A “Continue setup” pill brings you back',
      available: function () { return $('#skip-setup') ? true : 'Setup is finished'; }, run: function () { skip(); } });
    [1, 2, 3, 4].forEach(function (n) {
      OR.ui.command({ id: 'q' + n, label: 'Go to question ' + n, group: 'Setup',
        available: function () { return $all('#questions fieldset.q')[n - 1] ? true : 'Not on this screen'; },
        run: function () { var fs = $all('#questions fieldset.q')[n - 1]; var t = $all('input:not([disabled]),select', fs).filter(function (x) { return x.offsetParent !== null; })[0]; if (t) { t.focus(); t.scrollIntoView({ block: 'center' }); } } });
    });
  }
  // The only source of the hard checks is the server's catalog. Render whatever it returns, in its order.
  function loadHardChecks() {
    var get = REAL ? OR.api.get('/v1/policy/hard-checks') // endpoint: GET /v1/policy/hard-checks
      : fetch('fixtures/api/hard-checks.json').then(function (r) { return r.json().then(function (b) { return { status: r.status, body: b }; }); });
    return Promise.resolve(get).then(function (r) {
      var checks = r && r.status === 200 && r.body && Array.isArray(r.body.checks) ? r.body.checks.filter(function (c) { return c && typeof c.check_text === 'string' && c.check_text; }) : null;
      if (!checks || !checks.length) throw new Error('hard checks');
      $('[data-test=hard-checks]').innerHTML = checks.map(function (c) { return '<li data-check="' + esc(c.check_id) + '">' + esc(c.check_text) + '</li>'; }).join('');
    }).catch(function () { $('[data-test=hard-checks-error]').hidden = false; })
      .then(function () { var sec = $('section.checks'); if (sec) sec.setAttribute('aria-busy', 'false'); });
  }

  function roleRadio(v, label, primary) {
    return '<label><input type="radio" name="role" value="' + v + '"' + (v === primary ? ' checked' : '') + '><span>' + label + '</span></label>';
  }

  function currentRole() { var r = $('input[name=role]:checked'); return r ? r.value : 'maker'; }

  function renderQuestions(role) {
    var q = $('#questions');
    $('#also-label').textContent = role === 'maker' ? 'I also publish content (blog, newsletter, channel)' : 'I also have a product to promote';
    q.innerHTML = role === 'maker' ? makerQuestions() : publisherQuestions();
    q.insertAdjacentHTML('beforeend', modeQuestion(role));
    wire(role);
    snapshot();
  }

  function head(n, title, id) {
    return '<legend id="' + id + '"><span class="q-num">' + n + ' of 4</span> ' + esc(title) + '</legend>';
  }

  function makerQuestions() {
    var m = data.maker || {}, l = m.listing || {}, r = m.readiness || {}, c = m.commission || {};
    var signals = r.signals || [];
    var failed = signals.filter(function (s) { return !s.pass; }).length;
    return '' +
      '<fieldset class="q" data-q="listing">' + head(1, 'What to promote', 'q1') +
        '<p class="hint">This listing card is what publishers and their agents see.</p>' +
        '<div class="listing" data-test="listing-card">' +
          '<div class="listing-top"><div>' +
            '<p class="listing-name" data-bind="name">' + (l.name ? esc(l.name) : '<span class="empty-hint">No name found</span>') + '</p>' +
            '<p class="pitch" data-bind="pitch">' + (l.pitch ? esc(l.pitch) : '<span class="empty-hint">No pitch found. Add one line, or leave it for your agent.</span>') + '</p>' +
            '<p class="meta"><span data-bind="price">' + (l.price ? esc(l.price) : 'Price not found') + '</span>' + (l.url ? ' · ' + esc(l.url.replace(/^https?:\/\//, '')) : '') + '</p>' +
          '</div>' +
          '<div class="score" data-test="readiness-score">' + (r.score == null ? '<span class="flag">Readiness</span><b>–</b><span class="caption">not scanned</span>'
            : '<span class="flag">Readiness</span><b>' + esc(r.score) + '</b><span class="caption">of 100</span>') + '</div></div>' +
          (signals.length ? '<ul class="signals" aria-label="Readiness signals' + (failed ? ', ' + failed + ' failed' : '') + '">' + signals.map(function (s) {
            return '<li class="' + (s.pass ? 'pass' : 'fail') + '" data-signal="' + esc(s.id) + '"><span class="mark" aria-hidden="true">' + (s.pass ? '✓' : '✕') + '</span>' +
              '<div><strong>' + esc(s.label) + ' <span class="sr-only">' + (s.pass ? 'passed' : 'failed') + '</span></strong><span class="reason">' + esc(s.reason) + '</span></div></li>';
          }).join('') + '</ul>' : '') +
          (failed ? '<p class="caption" data-test="fail-note" style="margin:10px 0 0">' + failed + ' check' + (failed > 1 ? 's' : '') + ' failed. You can still continue; publishers see the score.</p>' : '') +
          '<p class="boundary">Only this card and the pass/fail signals left your machine. Never source code, .env files, keys or anything in .gitignore.</p>' +
        '</div>' +
        '<button type="button" class="btn secondary edit-toggle" aria-expanded="false" aria-controls="listing-edit" id="edit-listing">Edit listing</button>' +
        '<div id="listing-edit" hidden style="margin-top:14px">' +
          field('listing-name', 'Name', l.name, 'text', 'maxlength="60"') +
          field('listing-pitch', 'One-line pitch', l.pitch, 'text', 'maxlength="140"', true) +
          (data.real ? field('listing-price', 'Price (optional)', l.price_value == null ? '' : l.price_value, 'number', 'min="0" step="0.01" inputmode="decimal" placeholder="Not set"')
            : field('listing-price', 'Price', l.price, 'text', 'maxlength="40" placeholder="e.g. €19/month"')) +
        '</div>' +
      '</fieldset>' +
      '<fieldset class="q" data-q="avoid">' + head(2, 'Who should not promote it', 'q2') +
        '<p class="hint">Comma separated. Leave empty to allow everyone who passes the checks.</p>' +
        field('avoid-topics', 'Topics to avoid', list(m.avoid_topics), 'text', 'placeholder="Nothing found. e.g. gambling, adult"') +
        (data.real ? '' : field('avoid-types', 'Publisher types to avoid', list(m.avoid_publisher_types), 'text', 'placeholder="Nothing found. e.g. coupon sites"')) +
      '</fieldset>' +
      '<fieldset class="q" data-q="commission">' + head(3, 'Commission you offer', 'q3') +
        '<div class="row">' + field('commission-value', 'Amount', c.value == null ? '' : c.value, 'number', 'min="0" step="0.01" inputmode="decimal" placeholder="20"') +
        '<div class="field"><label for="commission-type">Paid as</label><select id="commission-type">' +
          COMMISSION_TYPES.map(function (t) { return '<option value="' + t.value + '"' + (t.value === (c.type || 'percent') ? ' selected' : '') + '>' + t.label + '</option>'; }).join('') +
        '</select></div>' +
        '<div class="field"><label for="commission-currency">Currency</label><select id="commission-currency">' + currencyOptions() + '</select></div></div>' +
        '<p class="hint">The currency is used for a fixed amount and for the price.</p>' +
        (c.value == null ? '<p class="empty-hint">No commission found. Leave empty and your agent will suggest one from similar offers.</p>' : '') +
      '</fieldset>';
  }

  function currencyOptions() {
    var cur = data.currency || ((data.maker || {}).commission || {}).currency || 'EUR', list = CURRENCIES.slice();
    if (list.indexOf(cur) < 0) list.unshift(cur);
    return list.map(function (c) { return '<option value="' + c + '"' + (c === cur ? ' selected' : '') + '>' + c + '</option>'; }).join('');
  }
  function publisherQuestions() {
    var p = data.publisher || {}, sites = p.sites || [];
    return '' +
      '<fieldset class="q" data-q="sites">' + head(1, 'Where boxes can appear', 'q1') +
        (sites.length ? '<div class="choices">' + sites.map(function (s, i) {
          return '<label class="choice"><input type="checkbox" name="site" value="' + i + '"' + (s.selected ? ' checked' : '') + '>' +
            '<span><b>' + esc(s.domain) + '</b><small>' + esc(s.kind || 'site') + (s.found_by ? ' · found in your ' + esc(s.found_by) : '') + '</small></span></label>';
        }).join('') + '</div>' : '<p class="empty-hint">No sites found by the scan.</p>') +
        '<div style="margin-top:12px">' + field('extra-site', 'Add another site or channel', '', 'text', 'placeholder="example.com or youtube.com/@you"') + '</div>' +
      '</fieldset>' +
      '<fieldset class="q" data-q="never">' + head(2, 'Topics never to promote', 'q2') +
        '<p class="hint">Comma separated. Offers in these topics never reach your pages.</p>' +
        field('never-topics', 'Never promote', list(p.never_topics), 'text', 'placeholder="Nothing found. e.g. gambling, crypto"') +
      '</fieldset>' +
      '<fieldset class="q" data-q="position">' + head(3, 'Where the box sits in a post', 'q3') +
        '<div class="pos-grid">' + POSITIONS.map(function (o) {
          var bars = o.value === 'after_intro' ? '<i></i><i class="box"></i><i></i><i></i>' : o.value === 'matched_section' ? '<i></i><i></i><i class="box"></i><i></i>' : '<i></i><i></i><i></i><i class="box"></i>';
          return '<label class="choice"><input type="radio" name="position" value="' + o.value + '"' + (o.value === (POS_IN[p.box_position] || p.box_position || 'end') ? ' checked' : '') + '>' +
            '<span class="pos-mini" aria-hidden="true">' + bars + '</span><span><b>' + o.label + '</b><small>' + o.help + '</small></span></label>';
        }).join('') + '</div>' +
      '</fieldset>';
  }

  function modeQuestion(role) {
    return '<fieldset class="q" data-q="mode">' + head(4, 'How new ' + (role === 'maker' ? 'publishers' : 'boxes') + ' get approved', 'q4') +
      '<p class="hint"><span class="session-only">Signed-in only</span> Only you, signed in here, can make this looser. Your agent can only make it stricter.</p>' +
      (data.saved ? '<p class="hint" id="mode-kept" data-test="mode-kept">Saving here keeps your saved approval mode. Change it in the dashboard' +
        (data.change_url ? ' (<a href="' + esc(data.change_url) + '">Change</a>)' : '') + '.</p>' : '') +
      '<div class="choices">' + MODES.map(function (m) {
        var ap = approvalOf(data), off = !modeAvailable(data, m.value), why = off ? ((ap && plainNote(ap.message)) || UNAVAILABLE_WHY) : '';
        return '<label class="choice' + (off ? ' unavailable' : '') + '" data-mode-choice="' + m.value + '"><input type="radio" name="mode" value="' + m.value + '"' +
          (m.value === modeChecked ? ' checked' : '') + (off ? ' disabled aria-describedby="why-' + m.value + '"' : data.saved ? ' disabled aria-describedby="mode-kept"' : '') + '>' +
          '<span><b>' + esc(m.label) + (m.value === modeSuggested ? '<span class="tag">Suggested</span>' : '') + '</b><small>' + esc(m[role]) + '</small>' +
          (off ? '<small class="why" id="why-' + m.value + '" data-test="mode-unavailable">' + esc(why) + '</small>' : '') +
          (m.value === 'yolo' && yoloPending(data) ? '<small class="why pending" data-test="mode-pending" data-reason="' + esc((ap && ap.reason) || '') + '"><b>Picked.</b> ' +
            (ap && ap.reason === PENDING_REASON && plainNote(ap.message) ? esc(ap.message) // f249a77: the server's own sentence says it all
              : 'It turns on after your first approved placement. ' + esc((ap && plainNote(ap.message)) || PENDING_FALLBACK)) + '</small>' : '') + '</span></label>';
      }).join('') + '</div><p class="field-error" id="mode-error" role="alert" hidden>Pick how new ' + (role === 'maker' ? 'publishers' : 'boxes') + ' get approved.</p></fieldset>';
  }

  function field(id, label, value, type, attrs, counter) {
    return '<div class="field"><label for="' + id + '">' + esc(label) + '</label>' +
      '<input type="' + type + '" id="' + id + '" value="' + esc(value) + '" ' + (attrs || '') + (counter ? ' aria-describedby="' + id + '-count"' : '') + '>' +
      (counter ? '<span class="count" id="' + id + '-count">' + String(value || '').length + '/140</span>' : '') + '</div>';
  }

  function wire(role) {
    var edit = $('#edit-listing');
    if (edit) edit.addEventListener('click', function () {
      var box = $('#listing-edit'), open = box.hidden;
      box.hidden = !open; edit.setAttribute('aria-expanded', String(open));
      edit.textContent = open ? 'Done editing' : 'Edit listing';
      if (open) $('#listing-name').focus();
    });
    [['listing-name', 'name'], ['listing-pitch', 'pitch'], ['listing-price', 'price']].forEach(function (p) {
      var el = $('#' + p[0]); if (!el) return;
      el.addEventListener('input', function () {
        var t = $('[data-bind=' + p[1] + ']'); if (t) t.textContent = el.value || (p[1] === 'price' ? 'Price not found' : '');
        var c = $('#' + p[0] + '-count'); if (c) c.textContent = el.value.length + '/140';
      });
    });
  }

  // Remember the prefilled values so we can report which ones the human edited.
  function collect() {
    var role = currentRole(), a = {};
    if (role === 'maker') {
      var l = (data.maker || {}).listing || {};
      a.maker = {
        listing: data.real ? { name: val('listing-name'), pitch: val('listing-pitch'), price_value: val('listing-price') === '' ? null : Number(val('listing-price')), url: l.url || '' }
          : { name: val('listing-name'), pitch: val('listing-pitch'), price: val('listing-price'), url: l.url || '' },
        avoid_topics: split(val('avoid-topics')),
        commission: { type: val('commission-type'), value: val('commission-value') === '' ? null : Number(val('commission-value')),
          currency: val('commission-type') === 'fixed' ? val('commission-currency') : '' }
      };
      if (!data.real) a.maker.avoid_publisher_types = split(val('avoid-types'));
    } else {
      var sites = (data.publisher || {}).sites || [];
      var chosen = $all('input[name=site]:checked').map(function (c) { var s = sites[Number(c.value)]; return { site_id: s.site_id, domain: s.domain }; });
      if (val('extra-site')) chosen.push({ site_id: null, domain: val('extra-site') });
      var pos = $('input[name=position]:checked');
      a.publisher = { sites: chosen, never_topics: split(val('never-topics')), box_position: pos ? pos.value : null };
    }
    return a;
  }
  // Second role ticked via "I also...": send the scan's values unchanged (editable later in the dashboard).
  function defaultsFor(r) {
    if (r === 'maker') { var m = data.maker || {}; return { listing: m.listing || null, avoid_topics: m.avoid_topics || [], avoid_publisher_types: m.avoid_publisher_types || [], commission: m.commission || null }; }
    var p = data.publisher || {};
    return { sites: (p.sites || []).filter(function (s) { return s.selected; }).map(function (s) { return { site_id: s.site_id, domain: s.domain }; }), never_topics: p.never_topics || [], box_position: p.box_position || 'end' };
  }
  function val(id) { var el = $('#' + id); return el ? el.value.trim() : ''; }
  function flat(o, pre, out) {
    out = out || {};
    Object.keys(o || {}).forEach(function (k) {
      var v = o[k], key = pre ? pre + '.' + k : k;
      if (v && typeof v === 'object' && !Array.isArray(v)) flat(v, key, out); else out[key] = JSON.stringify(v);
    });
    return out;
  }
  function snapshot() { initial = flat(collect()); }

  function onSubmit(e) {
    e.preventDefault();
    var role = currentRole(), also = $('#also-role').checked;
    var roles = also ? (role === 'maker' ? ['maker', 'publisher'] : ['publisher', 'maker']) : [role];
    var answers = collect(), now = flat(answers);
    roles.forEach(function (r) { if (!answers[r]) answers[r] = defaultsFor(r); });
    var edited = Object.keys(now).filter(function (k) { return now[k] !== initial[k]; }).map(function (k) { return k.replace(/^(maker|publisher)\./, ''); });
    var modeEl = $('input[name=mode]:checked');
    if (keyContext(data)) return;
    if (!modeEl) {
      $('#mode-error').hidden = false;
      $all('input[name=mode]:not(:disabled)').forEach(function (r) { r.setAttribute('aria-describedby', 'mode-error'); });
      $('input[name=mode]').focus();
      return;
    }
    $('#mode-error').hidden = true;
    var modeChosen = modeEl.value;
    if (modeChosen !== modeShown) edited.push('mode');
    var detail = {
      setup_id: data.setup_id || null,
      user_id: (data.session || {}).user_id || null,
      source: (data.agent || {}).source || null,
      install_path: (data.agent || {}).install_path || null,
      currency: val('commission-currency') || data.currency || null,
      roles: roles,
      roles_detected: data.detected_roles || [],
      answers: answers,
      mode_shown: modeShown,
      mode_chosen: modeChosen,
      edited_fields: edited,
      actor: 'owner_ui',
      session_only: true,
      submitted_at: new Date().toISOString()
    };
    document.dispatchEvent(new CustomEvent('offerrails:setup-submit', { detail: detail, bubbles: true }));
    var btn = $('#continue');
    btn.disabled = true; face(btn, 'Saving…');
    $('#save-error').hidden = true;
    OR.saveSetup(detail).then(function () { afterSave(detail); }, function () {
      btn.disabled = false; face(btn, 'Continue');
      var err = $('#save-error'); err.hidden = false;
      $('#setup-status').textContent = "We couldn't save your settings. Try again.";
      err.scrollIntoView({ block: 'center' });
    });
  }

  // v8: after a site connects, its plugin key is shown once (PR #21 @ 6042dcb, session-only routes in assets/api.js).
  var UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
  function afterSave(detail) {
    if (!REAL || detail.roles.indexOf('publisher') < 0 || !OR.pluginKey) return success(detail);
    var q = params.get('site');
    var site = UUID.test(q || '') ? Promise.resolve({ id: q, name: (data.publisher.sites[0] || {}).domain || '' })
      : OR.api.get('/v1/account/projects').then(function (r) { // endpoint: GET /v1/account/projects
        var p = r.status === 200 && r.body && Array.isArray(r.body.projects) ? r.body.projects.filter(function (x) { return x.kind === 'site' && UUID.test(x.id); })[0] : null;
        return p ? { id: p.id, name: p.name } : null;
      });
    return site.then(function (st) {
      if (!st) return success(detail);
      return OR.pluginKey.list(st.id).then(function (r) {
        if (OR.pluginKey.needsSignIn(r)) return keySignIn(detail, r);
        var keys = r.status === 200 && r.body && Array.isArray(r.body.keys) ? r.body.keys : null;
        var active = keys && keys.filter(function (k) { return !k.revoked; })[0];
        if (active) return success(detail, { kind: 'has', text: 'Your plugin already has its key (starts ' + active.key_prefix + '). Nothing to paste.' });
        return OR.pluginKey.issue(st.id).then(function (r2) {
          if (r2.status === 201 && r2.body && typeof r2.body.plugin_key === 'string') return keyStep(detail, st, r2.body);
          if (OR.pluginKey.needsSignIn(r2) && OR.pluginKey.reason(r2) !== 'key_exists') return keySignIn(detail, r2);
          if (OR.pluginKey.reason(r2) === 'key_exists') { var m = OR.pluginKey.message(r2);
            return success(detail, { kind: 'exists', text: (m ? m + ' ' : 'This site already has a plugin key. ') + 'To get a new one, rotate it from the digest: site Settings › Plugin key › Rotate.' }); }
          return success(detail, { kind: 'error', text: 'We couldn\u2019t create the plugin key right now. Generate it later from the digest: site Settings › Plugin key.' });
        });
      });
    }).catch(function () { success(detail, { kind: 'error', text: 'We couldn\u2019t create the plugin key right now. Generate it later from the digest: site Settings › Plugin key.' }); });
  }
  function keyStep(detail, st, body) {
    $('#setup-root').innerHTML = '<section class="card key-step" data-test="key-step" tabindex="-1" id="key-step">' +
      '<p class="eyebrow">Saved · one last step</p>' +
      OR.keyUI.onceHTML({ key: body.plugin_key, site: st.name, level: 1, intro: 'Your settings are saved. ' + (st.name ? st.name + ' is connected. ' : '') + 'The WordPress plugin needs this key to confirm each box it puts in a post. Until it has it, new boxes won\u2019t count as live.' }) +
      '</section>';
    OR.keyUI.wireOnce($('#key-step'), function () { success(detail, { kind: 'done', text: 'Plugin key handed over (starts ' + body.key_prefix + '). New boxes count as live once the plugin confirms them.' }); });
    $('#setup-status').textContent = 'Saved. Copy your plugin key: it is shown once.';
    $('#key-once-h').setAttribute('tabindex', '-1'); $('#key-once-h').focus();
  }
  function keySignIn(detail, r) {
    $('#setup-root').innerHTML = '<section class="card key-step" data-test="key-step" tabindex="-1" id="key-step">' +
      '<p class="eyebrow">Saved · plugin key</p><h1>Your settings are saved</h1>' +
      OR.keyUI.signInHTML(data.change_url, 'Your plugin key', r) +
      '<div class="actions"><button type="button" class="btn quiet" id="key-later" data-test="key-later">Do it later from Settings</button></div></section>';
    $('#key-later').addEventListener('click', function () { success(detail, { kind: 'later', text: 'Get your plugin key later from the digest: site Settings › Plugin key.' }); });
    $('#setup-status').textContent = 'Saved. Sign in again to get your plugin key.';
    $('#key-step').focus();
  }

  function success(detail, key) {
    try { localStorage.removeItem('offerrails.setup'); } catch (e) {}
    var src = (data.agent || {}).source, back = RETURN_TO[src] || 'your agent';
    var name = (detail.answers.maker && detail.answers.maker.listing.name) || (data.project || {}).name || 'your project';
    var modeLabel = MODES.filter(function (m) { return m.value === detail.mode_chosen; })[0].label;
    $('#setup-root').innerHTML =
      '<section class="card done" data-test="success" tabindex="-1" id="done">' +
        '<div class="tick" aria-hidden="true">✓</div>' +
        '<p class="eyebrow">Saved</p>' +
        '<h1>Your agent can take it from here</h1>' +
        '<p>It will ' + (detail.roles[0] === 'maker' ? 'list <strong>' + esc(name) + '</strong> and look for publishers' : 'start matching offers to your posts') +
        ' using ' + (data.saved ? 'your saved approval mode (<strong>' + esc(modeLabel) + '</strong> right now)' : '<strong>' + esc(modeLabel) + '</strong>') + '. You\'ll get a digest when something goes live, with one-tap undo for every item.</p>' +
        (key ? '<p class="key-note" data-test="plugin-key-note" data-kind="' + esc(key.kind) + '">' + esc(key.text) + '</p>' : '') +
        '<p class="caption">These rules apply to every listing.</p>' +
        '<div class="actions"><button type="button" class="btn quiet" id="back-to-agent">Back to my agent</button>' +
        '<span class="caption">You can close this tab and return to ' + esc(back) + '.</span></div>' +
      '</section>';
    $('#setup-status').textContent = 'Saved. Your agent can take it from here.';
    $('#done').focus();
    $('#back-to-agent').addEventListener('click', function () { window.close(); });
  }

  load().then(render).catch(function () {
    $('#setup-root').removeAttribute('aria-busy'); $('#setup-root').innerHTML = '<div class="alert" role="alert"><p><strong>We couldn\'t load what your agent found.</strong> Go back to your agent and run the sign-in again.</p></div>';
  });
})();
