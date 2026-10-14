/* <digest-item> : one live placement or listing in the digest, with one-tap Undo.
   Light DOM (styled by offer-rails.css), so the Go template can also server-render it.
   Data in : element.item = {...}  or  <digest-item data-item='{"item_id":...}'>
   Server  : (PR #21 fb493f0 shapes; see PARITY.md). plugin_state/live_now come with the digest: no status read on load.
             Undo  -> POST /v1/placements/:id/revoke  {reason:"undo"}   (immediately, keepalive)
             Redo  -> POST /v1/placements/:id/restore                   (only while the window is open)
             Retry -> POST /v1/placements/:id/retry                     (only when plugin_state is "failed"; repeats failed_action;
                                                                         publishers only: a maker key gets 403, so makers see no Retry)
             Plugin progress -> GET /v1/placements/:id  (while plugin_state is pending|offline and the tab is visible:
                                every 4 s for 60 s, then every 30 s, stop at 10 min with "Still waiting for your site")
   Events  : offerrails:undo, offerrails:restore (request sent),
             offerrails:placement-result (server answered), offerrails:plugin-state (plugin progress).
   Methods : undo(), restore(), retry(), closeWindow(), setPluginState(state). Needs assets/api.js. */
(function () {
  'use strict';
  var OR = (window.OfferRails = window.OfferRails || {});
  var FAST_MS = 4000, FAST_FOR = 60000, SLOW_MS = 30000, GIVE_UP = 600000;
  var KNOWN = { pending: 1, placed: 1, removed: 1, offline: 1, failed: 1 };
  var POS = { start: 'Start of article', section: 'Matching section', end: 'End of article',
    after_intro: 'After the intro', matched_section: 'Matching section', main_post: 'In the main post', first_reply: 'First reply',
    early_mention: 'Early mention', description: 'Description', pinned_comment: 'Pinned comment' }; // PR #21 fb493f0 box positions
  // Fallback only when the server sends no check_text for a 422.
  var CHECK_FALLBACK = {
    disclosure: 'The disclosure label could not be added.',
    rel_sponsored: 'The sponsored link attribute could not be set.',
    claims_from_offer: "The copy no longer matches the offer's own data.",
    copy_length: 'The copy is longer than 140 characters.',
    one_per_post: 'This post already has a box.',
    relevance_min: 'The box is no longer relevant enough to this post.',
    offer_live: 'The offer is no longer live.',
    revoke_path: 'Undo could not be guaranteed for this box.',
    cap: 'This site reached the most new boxes your rules allow.'
  };
  function esc(s) { return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) { return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]; }); }
  // Paid clicks when the digest sends them. Older payloads only have the total.
  function clickCount(it) { return typeof it.paid_clicks === 'number' ? it.paid_clicks : it.clicks; }
  function day(iso) { try { return iso ? new Date(iso).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' }) : ''; } catch (e) { return ''; } }
  function plainString(s) { return typeof s === 'string' && s.length < 300 && !/[{}\[\]<>]/.test(s) ? s : null; }

  // Who approved, in plain words. A rule decision shows the server's own rule text (policy_rule_text,
  // expected after PR #21 fixes). When the server sends neither policy_rule_id nor policy_rule_text (and no
  // approved_by), the line is hidden: we never guess why something went live. Internal ids are never shown.
  function approvedText(it) {
    var txt = plainString(it.policy_rule_text);
    if (it.approved_by === 'owner') return { lead: 'Approved by you', rest: '', cls: 'owner' };
    if (txt) return { lead: 'Approved by your rule:', rest: txt, cls: '' };
    if (it.policy_rule_id) return { lead: 'Approved by your rule', rest: '', cls: '' };
    if (it.approved_by === 'policy:yolo') return { lead: 'Approved automatically:', rest: 'passed all checks', cls: '' };
    if (it.approved_by === 'policy:auto') return { lead: 'Approved automatically:', rest: 'matched your rules', cls: '' };
    var w = whoText(it.approved_by); // (d) any other approved_by, as plain text
    return w ? { lead: 'Approved by ' + w, rest: '', cls: '' } : null;
  }
  // (d) Who undid it, only when the server sends undone_by. Internal ids are never shown.
  var WHO = { owner: 'you', agent: 'your agent', key: 'your agent', site_key: 'your agent', project_key: 'your agent', publisher: 'the publisher', maker: 'the maker' };
  function whoText(v) {
    if (typeof v !== 'string' || !v) return null;
    if (WHO[v]) return WHO[v];
    // 2d3aed7: undone_by is the owner id. The digest has no viewer id to compare it with, so an id reads "an owner", never the raw id.
    if (/^[0-9a-f]{8}-[0-9a-f]{4}-|^[a-z]+_[0-9a-f]{4,}$/i.test(v)) return 'an owner on this account';
    return plainString(v) && !/[:_]/.test(v) ? v : null;
  }
  OR.whoText = whoText;
  // 2d3aed7 review_reason -> which check is holding the listing, in plain words, and the next step. No times are promised.
  var REVIEW = {
    claims: { check: 'Claims are not checked yet.',
      next: 'Nothing to do for now. If the copy uses a number the offer doesn\u2019t show, change the copy to match the offer and submit again.' },
    head: { check: 'The offer page did not answer.', next: 'Make sure the offer link opens in a browser, then submit again.' },
    relevance: { check: 'The offer scored below the minimum relevance.', next: 'Make the offer\u2019s description say plainly what the product does, then submit again.' },
    caps: { check: 'You already have as many offers listed as the cap allows.', next: 'End or pause a listed offer to make room, then submit again.' }
  };
  OR.reviewText = function (r) { return REVIEW[r] || null; };
  function signedOut() { return OR.signedIn === false; } // (c) key/agent context: restoring and deciding need the dashboard
  function lower(s) { return s.charAt(0).toLowerCase() + s.slice(1); }
  OR.approvedText = approvedText;
  OR.positionLabel = function (v) { return POS[v] || v || ''; };

  // Plain copy for every restore outcome. Never raw JSON.
  function restoreMessage(status, body) {
    body = body || {};
    if (status === 409) {
      // Always the server's message (fb493f0: every 409 is {reason, message}). Fallbacks only if it is missing.
      var t409 = serverText(body, 409);
      if (body.reason === 'not_undone') return "Can't redo. " + sentence(t409 || 'This box was not undone.') + ' Showing the latest status.';
      if (body.reason === 'window_expired') return 'Too late to redo. ' + (t409 ? sentence(t409) + ' ' : '') + 'It stays undone.';
      if (body.reason === 'paused') return t409 ? "Can't redo. " + sentence(t409) + ' It stays undone.' : "Can't redo: you paused after undoing. It stays undone.";
      return t409 ? "Can't redo. " + sentence(t409) + ' It stays undone.' : "Can't redo this one. It stays undone.";
    }
    if (status === 403) return 'Only the person who undid this box can bring it back. It stays undone.';
    if (status === 422) {
      var why = plainString(body.check_text) || CHECK_FALLBACK[body.check_id];
      return why ? "Can't redo. " + why.replace(/\s*$/, '').replace(/([^.!?])$/, '$1.') + ' It stays undone.' : "Can't redo: it no longer passes the checks. It stays undone.";
    }
    return "Couldn't redo right now. It stays undone.";
  }
  OR.restoreMessage = restoreMessage;
  // 409 bodies are {reason, message}; 422 bodies are {check_id, check_text}.
  function serverText(b, status) { b = b || {}; return status === 422 ? plainString(b.check_text) : plainString(b.message); }
  function sentence(t) { return t.replace(/\s*$/, '').replace(/([^.!?])$/, '$1.'); }

  // Retry outcomes. 409 not_failed is handled by the caller (info + refetch). Server text word for word when present.
  function retryMessage(status, body) {
    body = body || {};
    var t = serverText(body, status);
    if (status === 422) return t ? "Couldn't try again. " + sentence(t) : "Couldn't try again: it no longer passes the checks.";
    if (status === 409) {
      if (t) return "Couldn't try again. " + sentence(t);
      if (body.reason === 'window_expired') return 'Too late to try again.';
      if (body.reason === 'paused') return "Couldn't try again: everything is paused.";
      return "Couldn't try again right now.";
    }
    if (status === 403) return t ? "Couldn't try again. " + sentence(t) : 'Only the account owner can try this again.';
    return t ? "Couldn't try again. " + sentence(t) : "Couldn't try again right now. Wait a minute and press it once more.";
  }
  OR.retryMessage = retryMessage;

  class DigestItem extends HTMLElement {
    set item(v) { this._item = v; this.render(); }
    get item() { return this._item; }
    connectedCallback() {
      if (!this._item && this.dataset.item) this._item = JSON.parse(this.dataset.item);
      if (this._item && !this._rendered) this.render();
    }
    render() {
      var it = this._item; if (!it) return;
      var a = approvedText(it);
      var where = '<b>' + esc((it.site || {}).name || (it.site || {}).domain) + '</b>' +
        ((it.article || {}).title ? ' · “' + esc(it.article.title) + '”' : '') + ' · ' + esc(POS[it.position] || it.position || '');
      var href = it.preview_href && it.preview_href.charAt(0) === '#' ? it.preview_href : '#preview';
      this.innerHTML =
        '<article class="digest-item" tabindex="-1" data-status="' + esc(it.status || 'live') + '" data-policy-rule-id="' + esc(it.policy_rule_id || '') + '" aria-labelledby="w-' + esc(it.item_id) + '">' +
          '<div class="di-main">' +
            '<p class="di-where" id="w-' + esc(it.item_id) + '">' + where + '</p>' +
            '<div class="di-box" data-test="box-copy"><span class="copy">' + esc(it.copy) + '</span> ' +
              '<a href="' + href + '" rel="sponsored nofollow" data-preview tabindex="-1" aria-hidden="true">' + esc(it.link_text || 'Vedi il prodotto') + '</a> ' +
              (it.disclosure_label === null ? '' : '<span class="label">' + esc(it.disclosure_label || '(link sponsorizzato)') + '</span>') + '</div>' +
            (a ? '<p class="di-rule" data-test="rule"><b class="' + a.cls + '">' + esc(a.lead) + '</b> ' + esc(a.rest) +
              (it.policy_version != null && it.approved_by !== 'owner' ? ' <span class="caption">· rules v' + esc(it.policy_version) + '</span>' : '') + '</p>' : '') +
            '<p class="di-rule caption" data-test="undone-by" hidden></p>' +
            (it.status === 'in_review' && REVIEW[it.review_reason] ? '<div class="di-note di-review" data-test="review" data-reason="' + esc(it.review_reason) + '">' +
              '<p data-test="review-check"><b>Check holding it:</b> ' + esc(REVIEW[it.review_reason].check) + '</p>' +
              '<p class="di-note-reason" data-test="review-message">Server says: \u201c' + esc(it.review_message) + '\u201d</p>' +
              '<p data-test="review-next"><b>Next step:</b> ' + esc(REVIEW[it.review_reason].next) + '</p></div>' : '') +
            (it.status === 'prepared' && it.expires_at ? '<p class="di-rule caption" data-test="expires">Expires ' + esc(day(it.expires_at)) + ' if not approved</p>' : '') + // (e) prepared only
            '<div class="di-note" data-test="note-box" hidden>' +
              '<p class="di-note-text" data-test="note" role="status"></p>' +
              '<p class="di-note-reason" data-test="note-reason" hidden></p>' +
              '<p class="di-note-error" data-test="note-error" role="alert" hidden></p>' +
              '<button type="button" class="btn secondary di-retry" data-action="retry" hidden></button>' +
            '</div>' +
          '</div>' +
          '<div class="di-side">' +
            '<span class="di-state" data-test="state"></span>' +
            '<span class="clicks">' + (clickCount(it) == null ? '' : esc(clickCount(it)) + (clickCount(it) === 1 ? ' click' : ' clicks')) + '</span>' +
            '<button type="button" class="btn secondary" data-action="toggle"></button>' +
            '<button type="button" class="btn quiet di-open" data-action="details" aria-haspopup="dialog">Details</button>' +
          '</div>' +
        '</article>';
      this._rendered = true;
      var self = this;
      this.querySelector('[data-preview]').addEventListener('click', function (e) { e.preventDefault(); });
      this.querySelector('[data-action=toggle]').addEventListener('click', function () { self.status() === 'live' ? self.undo() : self.restore(); });
      this.querySelector('[data-action=retry]').addEventListener('click', function () { self.retry(); });
      this.paint();
      if (this.isListing()) return;
      // fb493f0: plugin_state and live_now come with the digest, so there is no extra status read on load.
      // pending/offline items ride the regular 4 s / 30 s poll, which also settles their direction.
      if (it.placement_id && (it.plugin_state === 'pending' || it.plugin_state === 'offline')) this.watchPlugin();
    }
    isListing() { return this._item.kind === 'listing' && (this._item.status === 'listed' || this._item.status === 'in_review'); } // PR #21 offer rows: no parity undo
    status() { return (this._item && this._item.status) || 'live'; }
    post() { return this._item.side === 'maker' ? 'their post' : 'your post'; }
    // A removal (status undone) can never show as placed; a live item can never show as removed. Impossible pairs = unknown.
    pluginState() {
      var p = this._item.plugin_state, st = this.status();
      if (!KNOWN[p]) return 'unknown';
      if (p === 'placed' && st !== 'live') return 'unknown';
      if (p === 'removed' && st === 'live') return 'unknown';
      return p;
    }
    // Counts in "Live now" only when live AND the plugin confirmed placed (v4.3: rises only on confirmed placed;
    // drops at once on undo / leave it removed because clicks stop at once).
    // v5: AND the go link is redirecting, which Pause / the kill switch stop (PR #21 placement_counts_as_live).
    // fb493f0: until anything changes on this page, the server's live_now decides (same rule, computed server-side).
    counted() {
      if (this.isListing()) return false;
      if (typeof this._item.live_now === 'boolean' && !this._touched) return this._item.live_now && !OR.paused;
      return this.status() === 'live' && this.pluginState() === 'placed' && !OR.paused;
    }
    failedAction() { var a = this._item.failed_action; return a === 'removed' || a === 'restored' ? a : (this.status() === 'live' ? 'restored' : 'removed'); }
    stateText() {
      var it = this._item, ps = this.pluginState(), live = this.status() === 'live';
      if (this._busy) return live ? 'Removing…' : 'Bringing it back…';
      if (this._retrying) return 'Trying again…';
      if (this._refused) return 'Stays removed · not put back';
      if (this.isListing()) return it.status === 'in_review' ? 'In review' : 'Listed · ' + day(it.went_live_at);
      if (ps === 'unknown' || this.status() === 'unknown') return 'Status unknown';
      // Direction not known yet (digest item pending/offline): say only what is happening, never placed or removed.
      if (this.status() === 'awaiting') return ps === 'offline' ? 'Site offline · waiting for it to reconnect' : 'Waiting for ' + this.post() + ' to update';
      // Never removed/placed before the server says so: pending and offline only describe what is in progress.
      if (ps === 'failed') return this.failedAction() === 'removed' ? 'Clicks off · box still in ' + this.post() : 'Not back in ' + this.post() + ' yet';
      if (ps === 'offline') return (live ? 'Going back' : 'Clicks off') + ' · site offline';
      if (live) return ps === 'pending' ? 'Going back into ' + this.post() : 'Live · ' + day(it.went_live_at);
      if (ps === 'pending') return this._leaving ? 'Taking it out of ' + this.post() : 'Leaving ' + this.post() + ' shortly';
      return 'Box removed';
    }
    pluginNote() {
      var ps = this.pluginState(), live = this.status() === 'live';
      var site = this._item.side === 'maker' ? 'Their site' : 'Your site';
      if (ps === 'offline' && this.status() === 'awaiting') return { text: site + ' is offline. This updates when it reconnects.', cls: '' };
      if (ps === 'offline') return { text: live ? site + ' is offline. The box will come back when it reconnects.'
        : site + ' is offline. Clicks already stop at the link; the box will come out when it reconnects.', cls: '' };
      if (ps === 'failed') return { text: site + " couldn't update the post. Check the notice in your WordPress admin." +
        (this.failedAction() === 'removed' ? ' Clicks already stop at the link.' : " Clicks stay off until it's back in " + this.post() + '.'), cls: ' caution' };
      if (ps === 'pending' && this._stillWaiting) return { text: 'Still waiting for your site.', cls: '' };
      return null;
    }
    paint() {
      var it = this._item, st = this.status(), btn = this.querySelector('[data-action=toggle]');
      var label = ((it.site || {}).name || '') + ((it.article || {}).title ? ', ' + it.article.title : '');
      var art = this.querySelector('.digest-item');
      art.dataset.status = st; art.dataset.plugin = this.pluginState();
      this.querySelector('[data-test=state]').textContent = this.stateText();
      var pn = this.pluginNote(), info = this._info || '';
      var maker = it.side === 'maker', failedNow = !this._message && this.pluginState() === 'failed';
      var needsDash = signedOut() && !this._message && ((failedNow && this.failedAction() === 'restored') || (st !== 'live' && this._windowOpen));
      var note = this._message || [info, pn ? pn.text : '', failedNow && maker ? 'Only the publisher can try again.' : '',
        needsDash ? 'Putting it back needs the dashboard. Sign in there to do it.' : ''].filter(Boolean).join(' ');
      var box = this.querySelector('[data-test=note-box]'); box.hidden = !note;
      box.className = 'di-note' + (this._message ? ' warn' : pn ? pn.cls : '');
      this.querySelector('[data-test=note]').textContent = note;
      var failed = !this._message && this.pluginState() === 'failed';
      var reason = failed ? plainString(it.plugin_failed_reason) : null;
      var rs = this.querySelector('[data-test=note-reason]'); rs.textContent = reason || ''; rs.hidden = !reason;
      var er = this.querySelector('[data-test=note-error]'); er.textContent = failed ? (this._retryError || '') : ''; er.hidden = !(failed && this._retryError);
      var rb = this.querySelector('[data-action=retry]');
      rb.hidden = !failed || maker || (signedOut() && this.failedAction() === 'restored'); // maker: 403; restoring is session-only
      rb.disabled = !!this._retrying;
      rb.setAttribute('aria-busy', this._retrying ? 'true' : 'false');
      rb.textContent = this._retrying ? 'Trying again…' : this.failedAction() === 'removed' ? 'Try removing again' : 'Try putting it back again';
      btn.hidden = (st !== 'live' && !this._windowOpen) || this.isListing() || st === 'unknown' || st === 'awaiting';
      btn.disabled = !!this._busy;
      var giveUp = st === 'live' && this.pluginState() === 'failed' && this.failedAction() === 'restored';
      if (this.pluginState() === 'failed' && this.failedAction() === 'removed') btn.hidden = true; // failed removal: only Retry
      if (st !== 'live' && signedOut()) btn.hidden = true; // Redo is a restore: session only
      var ub = this.querySelector('[data-test=undone-by]'), who = st !== 'live' ? whoText(it.undone_by) : null;
      ub.textContent = who ? 'Undone by ' + who : ''; ub.hidden = !who;
      btn.textContent = st === 'live' ? (giveUp ? 'Leave it removed' : 'Undo') : 'Redo';
      btn.setAttribute('aria-label', (st === 'live' ? (giveUp ? 'Leave it removed: ' : 'Undo ') : 'Redo ') + label);
      this.querySelector('[data-action=details]').setAttribute('aria-label', 'Details for ' + label);
      // "Live now" bookkeeping: tell the page whenever this item starts or stops counting.
      var c = this.counted();
      if (this._lastCounted !== undefined && c !== this._lastCounted)
        this.fire('offerrails:live-count', { item_id: it.item_id, placement_id: it.placement_id || null, delta: c ? 1 : -1 });
      this._lastCounted = c;
    }
    payload(action) {
      var it = this._item;
      return { action: action, item_id: it.item_id, placement_id: it.placement_id || null, kind: it.kind, side: it.side || null,
        owner_ref: it.owner_ref || null, approved_by: it.approved_by, policy_rule_id: it.policy_rule_id || null,
        policy_version: it.policy_version == null ? null : it.policy_version, actor: 'owner_ui', session_only: false, at: new Date().toISOString() };
    }
    fire(name, detail) { this.dispatchEvent(new CustomEvent(name, { detail: detail, bubbles: true, composed: true })); }
    // endpoint: POST /v1/placements/:id/revoke
    // endpoint: POST /v1/placements/:id/restore
    // endpoint: POST /v1/placements/:id/retry
    path(op) { return '/v1/placements/' + encodeURIComponent(this._item.placement_id) + (op ? '/' + op : ''); }

    // Undo = revoke right away. No client-side hold.
    undo() {
      if (this.status() !== 'live' || this._busy) return Promise.resolve();
      var self = this, body = { reason: 'undo' }, d = this.payload('undo');
      d.request = { method: 'POST', path: this.path('revoke'), body: body };
      this._busy = true; this._message = ''; this.paint();
      this.fire('offerrails:undo', d);
      return OR.api.post(this.path('revoke'), body, { keepalive: true }).then(function (r) {
        self._busy = false;
        var ok = r.status >= 200 && r.status < 300;
        if (ok) {
          var gaveUp = self.pluginState() === 'failed' && self.failedAction() === 'restored';
          self._leaving = gaveUp;
          self._item.status = 'undone'; self._awaitPlaced = false; self._touched = true;
          if (r.body && typeof r.body.undone_by === 'string') self._item.undone_by = r.body.undone_by; // (d) only if the server says
          // PR #21: the revoke body has no plugin_state; the contract says a revoke sets it to pending (not removed).
          self._item.plugin_state = r.body && 'plugin_state' in r.body ? r.body.plugin_state : 'pending';
          self._windowOpen = !gaveUp; // "Leave it removed" has no Redo: Retry was the way back
        } else {
          self._message = "Couldn't undo right now. The box is still live. Try again.";
        }
        self.paint();
        self.fire('offerrails:placement-result', { action: 'undo', item_id: self._item.item_id, placement_id: self._item.placement_id,
          http_status: r.status, outcome: ok ? 'revoked' : 'error', plugin_state: self._item.plugin_state || null, window_open: !!self._windowOpen });
        if (ok && (self._item.plugin_state === 'pending' || self._item.plugin_state === 'offline')) self.watchPlugin();
        return r;
      });
    }
    // Redo = restore on the server; the server decides (200/409/403/422).
    restore() {
      if (this.status() === 'live' || this._busy || !this._windowOpen || signedOut()) return Promise.resolve();
      var self = this, d = this.payload('restore');
      d.request = { method: 'POST', path: this.path('restore'), body: {} };
      this._busy = true; this._message = ''; this.paint();
      this.fire('offerrails:restore', d);
      return OR.api.post(this.path('restore'), {}).then(function (r) {
        self._busy = false;
        var ok = r.status >= 200 && r.status < 300, outcome;
        if (ok) {
          // PR #21: restore answers {status, approved_by, restored_by, actor}; success sets plugin_state to pending.
          self._item.status = 'live'; self._touched = true; self._item.plugin_state = r.body && 'plugin_state' in r.body ? r.body.plugin_state : 'pending'; outcome = 'restored';
        } else {
          self._message = restoreMessage(r.status, r.body);
          var rsn = (r.body || {}).reason;
          outcome = r.status === 409 ? (/^(paused|window_expired|not_undone)$/.test(rsn) ? rsn : 'conflict')
            : r.status === 403 ? 'forbidden' : r.status === 422 ? 'check_failed' : 'error';
        }
        self._windowOpen = false;
        self.paint();
        self.fire('offerrails:placement-result', { action: 'restore', item_id: self._item.item_id, placement_id: self._item.placement_id,
          http_status: r.status, outcome: outcome, reason: (r.body && (r.body.reason || r.body.check_id)) || null,
          message: self._message || null, plugin_state: self._item.plugin_state || null });
        if (ok && (self._item.plugin_state === 'pending' || self._item.plugin_state === 'offline')) self.watchPlugin();
        if (outcome === 'not_undone') self.resync(); // the box was not undone: show what the server has now
        return { ok: ok, status: r.status, message: self._message };
      });
    }
    // Retry repeats only the action that failed (failed_action). Never changes status, never counts in "Live now".
    retry() {
      if (this._retrying || this.pluginState() !== 'failed' || this._item.side === 'maker') return Promise.resolve();
      if (signedOut() && this.failedAction() === 'restored') return Promise.resolve();
      var self = this, action = this.failedAction(), d = this.payload('retry');
      d.failed_action = action;
      d.request = { method: 'POST', path: this.path('retry'), body: {} };
      this._retrying = true; this._retryError = ''; this._info = ''; this.paint();
      this.fire('offerrails:retry', d);
      return OR.api.post(this.path('retry'), {}).then(function (r) {
        var ok = r.status >= 200 && r.status < 300, body = r.body || {}, outcome;
        if (ok) {
          outcome = 'retried';
          self._item.failed_action = action;
          if (action === 'restored') self._awaitPlaced = true; // not live again until the plugin confirms placed
          self._item.plugin_state = 'plugin_state' in body ? body.plugin_state : null;
          self._retrying = false; self.paint();
          if (self._item.plugin_state === 'pending' || self._item.plugin_state === 'offline') self.watchPlugin();
        } else if (r.status === 409 && body.reason === 'not_failed') {
          outcome = 'not_failed';
          self._info = 'This box already changed. Showing the latest status.';
          return OR.api.get(self.path()).then(function (g) {
            self._retrying = false;
            if (g.status === 200 && g.body) self.apply(g.body); else self.paint();
            self.fire('offerrails:placement-result', { action: 'retry', failed_action: action, item_id: self._item.item_id, placement_id: self._item.placement_id,
              http_status: 409, outcome: outcome, reason: 'not_failed', message: self._info, plugin_state: self._item.plugin_state });
            return r;
          });
        } else if (r.status === 422 && action === 'restored') {
          // Must-fix: retrying a failed restore re-runs every hard check. A 422 means it can't go back: it stays out of the post.
          outcome = 'check_failed';
          var why = plainString(body.check_text) || CHECK_FALLBACK[body.check_id];
          self._item.status = 'undone'; self._refused = true; self._windowOpen = false; self._touched = true;
          self._message = "Can't put it back. " + (why ? sentence(why) + ' ' : 'It no longer passes the checks. ') + 'It stays removed.';
          self._retrying = false; self.paint();
        } else {
          outcome = r.status === 422 ? 'check_failed' : r.status === 409 ? 'conflict' : r.status === 403 ? 'forbidden' : 'error';
          self._retryError = retryMessage(r.status, body);
          self._retrying = false; self.paint();
        }
        self.fire('offerrails:placement-result', { action: 'retry', failed_action: action, item_id: self._item.item_id, placement_id: self._item.placement_id,
          http_status: r.status, outcome: outcome, reason: body.reason || body.check_id || null, message: self._retryError || null, plugin_state: self._item.plugin_state || null });
        return r;
      });
    }
    // Apply a fresh GET /v1/placements/:id body: plugin fields only. status (and so "Live now") changes only through undo/restore.
    apply(b) {
      if ((this._item.status === 'unknown' || this._item.status === 'awaiting') && OR.contract) { var st = OR.contract.statusFromServer(b.status); if (st) this._item.status = st; }
      if ('failed_action' in b) this._item.failed_action = b.failed_action;
      if ('plugin_failed_reason' in b) this._item.plugin_failed_reason = b.plugin_failed_reason;
      var before = this._item.plugin_state;
      this.setPluginState('plugin_state' in b ? b.plugin_state : null);
      if (before === this._item.plugin_state) this.paint();
    }
    // endpoint: GET /v1/placements/:id
    resync() {
      var self = this;
      return OR.api.get(this.path()).then(function (g) {
        if (g.status !== 200 || !g.body) return;
        var st = OR.contract ? OR.contract.statusFromServer(g.body.status) : null;
        if (st) { self._item.status = st; self._touched = true; }
        self.apply(g.body);
      });
    }
    closeWindow() { this._windowOpen = false; if (this._rendered) this.paint(); }
    setPluginState(s) {
      if (s === undefined) s = null;
      if (s === this._item.plugin_state) return;
      this._item.plugin_state = s; this._touched = true;
      if (s !== 'pending' && s !== 'offline') this._leaving = false;
      if (s === 'placed' || s === 'failed' || this.status() !== 'live') this._awaitPlaced = false;
      this.paint();
      this.fire('offerrails:plugin-state', { item_id: this._item.item_id, placement_id: this._item.placement_id, status: this.status(), plugin_state: s });
      if (s === 'pending' || s === 'offline') { if (!this._watching) this.watchPlugin(); } else this.stopWatch();
    }
    // Ask the server for plugin progress while it can still change: every 4 s for 60 s, then every 30 s,
    // stop at 10 min. Only while the tab is visible; picks up again when it becomes visible.
    watchPlugin() {
      var self = this;
      this.stopWatch();
      this._watching = true; this._stillWaiting = false; this._start = Date.now(); this._gen = (this._gen || 0) + 1;
      this._onVis = function () { if (document.visibilityState === 'visible' && self._hiddenSkip) { self._hiddenSkip = false; self.pollNow(); } };
      document.addEventListener('visibilitychange', this._onVis);
      this.paint();
      this.schedule();
    }
    schedule() {
      clearTimeout(this._poll);
      if (!this._watching) return;
      var ps = this._item.plugin_state;
      if (ps !== 'pending' && ps !== 'offline') return this.stopWatch();
      var elapsed = Date.now() - this._start;
      if (elapsed >= GIVE_UP) { this._stillWaiting = ps === 'pending'; this.stopWatch(); this.paint(); return; }
      var self = this;
      this._poll = setTimeout(function () { self.pollNow(); }, Math.min(elapsed < FAST_FOR ? FAST_MS : SLOW_MS, GIVE_UP - elapsed));
    }
    pollNow() {
      var self = this, gen = this._gen;
      if (!this._watching) return;
      if (document.visibilityState !== 'visible') { this._hiddenSkip = true; return; }
      OR.api.get(this.path()).then(function (r) {
        if (gen !== self._gen || !self._watching) return;
        if (r.status === 200 && r.body && 'plugin_state' in r.body) {
          if ('failed_action' in r.body) self._item.failed_action = r.body.failed_action;
          if ('plugin_failed_reason' in r.body) self._item.plugin_failed_reason = r.body.plugin_failed_reason;
          if (self._item.status === 'awaiting' && OR.contract) { var st = OR.contract.statusFromServer(r.body.status); if (st) self._item.status = st; }
          self.setPluginState(r.body.plugin_state);
          self.paint();
        }
        self.schedule();
        self._polls = (self._polls || 0) + 1; // read by tests
      });
    }
    stopWatch() {
      clearTimeout(this._poll); this._watching = false; this._hiddenSkip = false;
      if (this._onVis) { document.removeEventListener('visibilitychange', this._onVis); this._onVis = null; }
    }
    disconnectedCallback() { this.stopWatch(); }
  }
  if (!customElements.get('digest-item')) customElements.define('digest-item', DigestItem);
})();
