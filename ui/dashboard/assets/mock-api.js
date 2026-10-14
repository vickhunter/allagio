/* Preview-only API mock. Does nothing unless the URL has ?mock=1.
   Answers with fixtures/api/*.json. Pick outcomes with ?restore=200|409-window|409-paused|409-noreason|403|422-cap|422-offer,
   ?plugin=removed|offline|pending (what GET /v1/placements/:id returns), ?resume=200|403. Never used in production. */
(function () {
  'use strict';
  var q = new URLSearchParams(location.search);
  if (!q.get('mock')) return;
  var real = window.fetch.bind(window);
  function fx(name, status) {
    return real('fixtures/api/' + name + '.json').then(function (r) { return r.text(); })
      .then(function (t) { return new Response(t, { status: status, headers: { 'Content-Type': 'application/json' } }); });
  }
  window.fetch = function (url, init) {
    var u = String(url), m = (init && init.method) || 'GET';
    if (/\/v1\/placements\/[^/]+\/revoke$/.test(u)) return fx('revoke-200', 200);
    if (/\/v1\/placements\/[^/]+\/restore$/.test(u)) {
      var r = q.get('restore') || '200';
      return fx(r === '200' ? 'restore-200' : 'restore-' + r, Number(r.slice(0, 3)));
    }
    if (/\/v1\/placements\/[^/]+$/.test(u) && m === 'GET') return fx('placement-' + (q.get('plugin') || 'removed'), 200);
    if (/\/v1\/account\/pause$/.test(u)) return fx('pause-200', 200);
    if (/\/v1\/account\/resume$/.test(u)) return q.get('resume') === '403' ? fx('resume-403', 403) : fx('resume-200', 200);
    return real(url, init);
  };
})();
