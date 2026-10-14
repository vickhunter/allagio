/* PR #21 (vickhunter/affilio @ fb493f0) <-> UI view model. Pure functions, no I/O.
   Used only in real mode (?api=real). See PARITY.md for every field.
   Rules kept: nothing is shown placed or removed before the plugin confirms it; an item's status is never guessed;
   Live now counts only what the server says counts (live_now / went_live). */
(function () {
  'use strict';
  var OR = (window.OfferRails = window.OfferRails || {});
  var MODES = ['manual', 'auto', 'yolo'];
  // placement_proposals.status as returned by GET /v1/placements/:id and restore. Anything else = unknown.
  function statusFromServer(s) {
    if (s === 'revoked') return 'undone';
    if (s === 'approved' || s === 'committed') return 'live';
    return null;
  }
  // Digest items have no status field. plugin_state + live_now settle it except while the plugin is still
  // working (pending/offline), where the direction is not in the item: 'awaiting' until the regular poll answers.
  // A missing or unknown plugin_state is 'unknown' (shown as "Status unknown").
  function itemStatus(it) {
    var p = it.plugin_state;
    if (it.live_now === true) return 'live';
    if (p === 'placed') return 'live';                     // placed but not counting (e.g. paused): still in the post
    if (p === 'removed') return 'undone';
    if (p === 'failed') return it.failed_action === 'restored' ? 'live' : it.failed_action === 'removed' ? 'undone' : 'unknown';
    if (p === 'pending' || p === 'offline') return 'awaiting';
    return 'unknown';
  }
  // Earnings: a list of {currency, amount}, or null. One line per currency, never added together.
  function money(list) {
    if (!Array.isArray(list)) return null;
    var out = list.filter(function (x) { return x && typeof x.currency === 'string' && /^[A-Z]{3}$/.test(x.currency) && isFinite(Number(x.amount)); })
      .map(function (x) { return { currency: x.currency, amount_minor: Math.round(Number(x.amount) * 100) }; });
    return out.length ? out : null;
  }
  function sources(list) { return (list || []).slice().sort(function (a, b) { return b.clicks - a.clicks; }); }
  // Only http(s) links or same-site paths ever become an href.
  function safeUrl(u) { return typeof u === 'string' && (/^https?:\/\/[^\s"'<>]+$/.test(u) || /^\/[^\s"'<>]*$/.test(u)) ? u : null; }
  function str(s) { return typeof s === 'string' && s ? s : null; }
  // 1b85f07 (Site Keeper): digest items can arrive with post_title "". Fall back to surface_url's path, then 'Untitled post'.
  function postTitle(it) {
    var t = str(it.post_title) && it.post_title.trim(); if (t) return t;
    var u = typeof it.surface_url === 'string' ? it.surface_url : '', m = /^https?:\/\/[^\/?#\s]+(\/[^?#\s]*)?/.exec(u), p = m ? (m[1] || '') : '';
    try { p = decodeURIComponent(p); } catch (e) {}
    p = p.replace(/\/+$/, ''); return p ? p : 'Untitled post';
  }
  var REVIEW = ['claims', 'head', 'relevance', 'caps'];
  function viewItem(it, p) {
    var side = p.kind === 'site' ? 'publisher' : 'maker';
    var common = { item_id: it.id, side: side, site: { name: p.name }, copy: it.summary, went_live_at: it.went_live_at, undo_token: it.undo_token,
      clicks: typeof it.clicks === 'number' ? it.clicks : null, live_now: it.live_now === true };
    if (it.kind !== 'placement') {
      // 2d3aed7: an untrusted maker's offer that failed a server check stays `submitted` with review_reason + message.
      // Both fields are optional (null or absent when listed); without them the row stays a plain listing.
      var inReview = (it.status === 'submitted' || it.status === 'in_review') && REVIEW.indexOf(it.review_reason) >= 0 && str(it.message);
      return Object.assign(common, { placement_id: null, kind: 'listing', status: inReview ? 'in_review' : 'listed', link_text: str(it.link_text), disclosure_label: null,
        review_reason: inReview ? it.review_reason : null, review_message: inReview ? it.message.trim() : null });
    }
    return Object.assign(common, { placement_id: it.id, kind: 'placement', status: itemStatus(it),
      article: { title: postTitle(it) }, position: str(it.position), link_text: str(it.link_text), disclosure_label: str(it.label),
      approved_by: str(it.approved_by), undone_by: str(it.undone_by), policy_rule_id: str(it.policy_rule_id), policy_rule_text: str(it.policy_rule_text),
      policy_version: it.rules_version > 0 ? it.rules_version : null,
      plugin_state: it.plugin_state, plugin_failed_reason: it.plugin_failed_reason || '', failed_action: it.failed_action || '' });
  }
  function section(p, env) {
    var paused = env.paused || {};
    var s = { name: p.name, mode: MODES.indexOf(p.mode) >= 0 ? p.mode : null, went_live_count: null, live_count: p.went_live, clicks: p.clicks,
      earnings: money(p.earnings), earnings_kind: p.kind === 'site' ? 'income' : 'commission', // an offer's amount is commission the maker pays
      top_sources: sources(p.clicks_by_source), items: (p.items || []).map(function (it) { return viewItem(it, p); }),
      server_kind: p.kind, server_id: p.id, mode_change_url: safeUrl(env.change_url),
      total_items: typeof p.total_items === 'number' ? p.total_items : null, // 6042dcb: all items, not capped at 50
      paused: p.kind === 'site' ? paused.site === true : paused.offer === true };
    if (p.kind === 'site') { s.site_id = p.id; s.url = safeUrl(env.site_url); } else s.project_id = p.id;
    return s;
  }
  // GET /v1/account/digest (+ GET /v1/digest for "since")
  function digestFromServer(acct, dig) {
    var projects = acct.projects || [], t = acct.totals || {}, paused = acct.paused || {};
    var sites = projects.filter(function (p) { return p.kind === 'site'; }).map(function (p) { return section(p, acct); });
    // site_url is one value for the account: shown only when there is exactly one site, never guessed for several.
    if (sites.length !== 1) sites.forEach(function (s) { s.url = null; });
    var offers = projects.filter(function (p) { return p.kind !== 'site'; }).map(function (p) { return section(p, acct); });
    // 65a1871: held offers come from /v1/account/digest itself; /v1/digest is read only for "since".
    return {
      digest_id: null, user_id: null, test_mode: true, // earnings are test-mode only (earnings_note)
      period: { since: dig && dig.since ? dig.since : null }, session: acct.session || { signed_in: false },
      paused: paused.account === true,
      approval: str(acct.approval_reason) ? { reason: acct.approval_reason, message: str(acct.approval_message), effective_mode: str(acct.effective_mode) } : null,
      available_modes: Array.isArray(acct.available_modes) ? acct.available_modes : MODES,
      totals: { live_items: t.went_live, went_live: typeof acct.went_live_since_last === 'number' ? acct.went_live_since_last : null,
        clicks: t.clicks, earnings: money(t.earnings), projects: offers.length, sites: sites.length, top_sources: sources(t.clicks_by_source) },
      projects: offers, sites: sites, earnings_note: acct.earnings_note || '', change_url: safeUrl(acct.change_url)
    };
  }

  // Setup. GET /v1/setup/settings -> view model the setup page renders.
  function setupFromServer(s) {
    var c = s.commission, lp = s.listing_price;
    var currency = (c && c.currency) || (lp && lp.currency) || 'EUR';
    return {
      setup_id: null, _server: s, real: true, saved: s.saved === true, // 6042dcb: a save keeps the stored mode
      session: { user_id: s.saved_by || null, signed_in: !!(s.session && s.session.signed_in), human_login: !!s.human_login },
      agent: { source: s.install_path, install_path: s.install_path },
      detected_roles: s.role ? [s.role] : [],
      project: { name: str(s.project_name), url: safeUrl(s.listing_url) || safeUrl(s.site_url) },
      mode_preselected: MODES.indexOf(s.mode_preselected) >= 0 ? s.mode_preselected : null, // from the server only; never derived here
      mode_effective: MODES.indexOf(s.mode) >= 0 ? s.mode : null,
      available_modes: Array.isArray(s.available_modes) ? s.available_modes : MODES,
      approval: str(s.approval_reason) ? { reason: s.approval_reason, message: str(s.approval_message) } : null,
      change_url: safeUrl(s.change_url), site_url: safeUrl(s.site_url), currency: currency,
      maker: { listing: { name: str(s.project_name) || '', pitch: s.promote || '', price_value: lp ? lp.value : null,
          price: lp ? fmt(lp.value, lp.currency) : '', url: s.listing_url || '' },
        avoid_topics: s.avoid_topics || [], commission: c ? { type: c.type, value: c.value, currency: c.currency || '' } : { type: 'percent', value: null, currency: '' } },
      publisher: { sites: (s.sites_channels || []).map(function (d) { return { site_id: null, domain: d, selected: true }; }),
        never_topics: s.never_topics || [], box_position: s.box_position || 'end' }
    };
  }
  function fmt(v, cur) { try { return new Intl.NumberFormat('en-GB', { style: 'currency', currency: cur }).format(v); } catch (e) { return v + ' ' + cur; } }
  // offerrails:setup-submit detail -> one PUT /v1/setup/settings body per role, exactly the keys PutSetup accepts.
  function setupPutBodies(detail, server) {
    server = server || {};
    var currency = detail.currency || 'EUR';
    return detail.roles.map(function (role) {
      var a = detail.answers[role] || {}, b = {
        role: role, install_path: server.install_path || detail.install_path, mode: detail.mode_chosen,
        mode_shown: detail.mode_shown || server.mode_shown || server.mode_preselected,
        promote: '', avoid_topics: [], project_name: '', listing_url: '', commission: null, listing_price: null,
        sites_channels: [], never_topics: [], box_position: '' };
      if (role === 'maker') {
        var l = a.listing || {}, c = a.commission || {};
        b.project_name = (l.name || '').slice(0, 120);
        b.promote = (l.pitch || '').slice(0, 280);
        b.listing_url = l.url || '';
        b.avoid_topics = a.avoid_topics || [];
        if (c.value != null && (c.type === 'percent' || c.type === 'fixed'))
          b.commission = { type: c.type, value: c.value, currency: c.type === 'fixed' ? currency : (c.currency || '') };
        if (l.price_value != null && l.price_value !== '') b.listing_price = { value: Number(l.price_value), currency: currency };
      } else {
        b.sites_channels = (a.sites || []).map(function (x) { return x.domain; });
        b.never_topics = a.never_topics || [];
        b.box_position = a.box_position || '';
      }
      return b;
    });
  }
  OR.contract = { statusFromServer: statusFromServer, itemStatus: itemStatus, digestFromServer: digestFromServer, money: money,
    setupFromServer: setupFromServer, setupPutBodies: setupPutBodies, safeUrl: safeUrl };
})();
