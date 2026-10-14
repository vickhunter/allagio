"""Live smoke (v9), CLAIMS_CHECK=stub server: python3 tests/live_smoke_stub.py <sha> <port> <oidc_port> <proof_subdir>
Follows d024-local/checks-<sha>-claims.py (read, not edited): register site (plugin_key), owner via fake OIDC, untrusted maker
(email != owner) -> submitted/claims/'The claims check is not live yet.'; trusted maker (admin trust) -> listed. Product readiness is set
'ready' in the local test DB the same way the d024 check does, so only the claims check can hold the offer.
UI: the owner digest at these SHAs does not carry review_reason (maker routes only), so the page gets the server's real /v1/account/digest
plus one offer section whose item is the untrusted maker's real GET /v1/maker/offers/:id (status, review_reason, message verbatim).
Never prints keys; /v1/go/** aborted + counted."""
import os, re, sys, json, uuid, subprocess, socket, time, requests, urllib.parse as up
from playwright.sync_api import sync_playwright
SHA, PORT_API, OIDC, SUB = sys.argv[1], sys.argv[2], sys.argv[3], sys.argv[4]
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__))); L = '/workspace/builds/offer-rails/d024-local'
B = 'http://127.0.0.1:' + PORT_API; V = B + '/v1'; CA = '%s/tls-%s/fakeoidc-ca.pem' % (L, OIDC)
PROOF = '/workspace/builds/offer-rails/proof/d032-ui/v9-2d3aed7/' + SUB; os.makedirs(PROOF, exist_ok=True)
ENV = dict(l.strip().split('=', 1) for l in open('%s/run/%s.env' % (L, SHA)) if '=' in l and not l.startswith('#'))
R = []; go = []
def rec(n, ok, info=''): R.append((n, bool(ok))); print(('PASS ' if ok else 'FAIL ') + n + (' | ' + str(info) if info else ''), flush=True)
def sql(q): return subprocess.run([L + '/tools/pgexec/pgexec', q], env={'PGURL': ENV['DATABASE_URL']}, capture_output=True, text=True)
tag = uuid.uuid4().hex[:6]; OWNER = 'owner-%s@uismoke.test' % tag
j = requests.post(V + '/sites/register', json={'site_url': 'https://ui-stub-%s.test' % tag, 'plugin_version': '0.1.20'}).json()
rec('POST /v1/sites/register returns plugin_key', bool(j.get('plugin_key')), 'len=%d' % len(j.get('plugin_key') or ''))
s = requests.Session(); r = s.get(B + '/auth/login', params={'return_to': '/owner'}, allow_redirects=False)
q = dict(up.parse_qsl(up.urlsplit(r.headers['Location']).query))
r2 = requests.post('https://127.0.0.1:%s/authorize' % OIDC, data={**q, 'email': OWNER, 'name': 'UI Stub Owner'}, verify=CA, allow_redirects=False); s.get(r2.headers['Location'], allow_redirects=False)
url = requests.post(V + '/sites/owner-connection', headers={'X-API-Key': j['api_key'], 'Content-Type': 'application/json'}, data='{}').json()['connect_url']
m = re.search(r'name="csrf" value="([^"]+)"', s.get(url).text); tok = dict(up.parse_qsl(up.urlsplit(url).query))['token']
s.post(B + '/owner/connect-site', data={'token': tok, 'csrf': m.group(1) if m else ''}, headers={'Origin': B}, allow_redirects=False)
mm = re.search(r'<meta name="csrf-token" content="([^"]+)"', s.get(B + '/owner').text); CSRF = mm.group(1) if mm else ''
rec('owner session + <meta csrf-token>', bool(CSRF))
pw = s.post(V + '/account/pause', headers={'Origin': B, 'X-CSRF-Token': CSRF, 'Content-Type': 'application/json'}, data='{"scope":"all"}')
pr = s.post(V + '/account/resume', headers={'Origin': B, 'X-CSRF-Token': CSRF, 'Content-Type': 'application/json'}, data='{"scope":"all"}')
rec('owner POST with Origin + X-CSRF-Token accepted (pause/resume all)', pw.status_code < 300 and pr.status_code < 300, '%d/%d' % (pw.status_code, pr.status_code))
def maker(email, pname, trusted):
    k = requests.post(V + '/makers/register', json={'email': email, 'name': pname + ' Maker'}).json(); mh = {'X-Maker-Key': k['api_key']}
    p = requests.post(V + '/maker/products', headers=mh, json={'name': pname, 'website_url': 'https://example.com', 'product_type': 'saas', 'primary_category': 'software'}).json()
    sql("UPDATE products SET readiness_status='ready' WHERE id='%s'" % p['id'])
    o = requests.post(V + '/maker/offers', headers=mh, json={'product_id': p['id'], 'commission_type': 'percentage', 'commission_value': 20, 'landing_url': 'https://example.com', 'terms': '%s pays 20 percent to publishers.' % pname}).json()
    if trusted: requests.post(V + '/offer-review/makers/%s/trust' % k['maker_id'], headers={'X-Admin-Key': ENV['ADMIN_API_KEY']}, json={'trusted': True, 'actor': 'd032-ui-v9-smoke'})
    r = requests.post(V + '/maker/offers/%s/submit' % o['id'], headers=mh); g = requests.get(V + '/maker/offers/%s' % o['id'], headers=mh)
    return r, g, o, p
mail_u = 'maker-%s@uismoke-maker.test' % tag
r, g, o, p = maker(mail_u, 'Northwind Ledger', False); ju = r.json(); gu = g.json()
rec('untrusted maker (%s != owner) -> submitted, review_reason claims, "The claims check is not live yet."' % mail_u.split('@')[1],
    r.status_code == 200 and ju.get('status') == 'submitted' and ju.get('review_reason') == 'claims' and ju.get('message') == 'The claims check is not live yet.' and gu.get('status') == 'submitted',
    '%d %s' % (r.status_code, {k: ju.get(k) for k in ('status', 'review_reason', 'message')}))
r3, g3, o3, p3 = maker('trusted-%s@uismoke-maker.test' % tag, 'Southwind Ledger', True); j3 = r3.json()
rec('trusted maker still lists directly', r3.status_code == 200 and j3.get('status') == 'listed' and not j3.get('review_reason'), '%d %s' % (r3.status_code, {k: j3.get(k) for k in ('status', 'review_reason')}))
def item(oid, name, src):
    return {'approved_by': '', 'clicks': 0, 'id': oid, 'kind': 'offer', 'label': name, 'link_text': '', 'live_now': src.get('status') == 'listed', 'message': src.get('message'),
            'plugin_state': '', 'policy_rule_id': '', 'policy_rule_text': '', 'position': '', 'post_title': '', 'review_reason': src.get('review_reason'), 'rules_version': 0,
            'status': src.get('status'), 'summary': name, 'undo_token': '', 'undone_by': '', 'went_live_at': src.get('updated_at') or '2026-10-13T00:00:00Z'}
INJ = {'clicks': 0, 'clicks_by_source': [], 'earnings': None, 'id': p['id'], 'kind': 'offer', 'mode': 'manual', 'name': 'Northwind Ledger (maker view)', 'total_items': 2, 'went_live': 1,
       'items': [item(o['id'], 'Northwind Ledger', gu), item(o3['id'], 'Southwind Ledger', g3.json())]}
def fp():
    x = socket.socket(); x.bind(('127.0.0.1', 0)); n = x.getsockname()[1]; x.close(); return n
PP = fp(); BASE = 'http://127.0.0.1:%d/' % PP
srv = subprocess.Popen([sys.executable, '-m', 'http.server', str(PP), '--bind', '127.0.0.1'], cwd=ROOT, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); time.sleep(0.8)
def proxy(route):
    rq = route.request; path = '/' + rq.url.split('/backend/', 1)[1]
    if '/v1/go/' in path: go.append(path); return route.abort()
    hd = {k: v for k, v in rq.headers.items() if k.lower() in ('content-type', 'accept', 'x-csrf-token')}; hd['Origin'] = B
    x = s.request(rq.method, B + path, headers=hd, data=rq.post_data, allow_redirects=False); body = x.text
    if path.split('?')[0] == '/v1/account/digest' and x.status_code == 200:
        d = x.json(); d['projects'] = (d.get('projects') or []) + [INJ]; body = json.dumps(d)
    route.fulfill(status=x.status_code, content_type='application/json', body=body)
try:
    with sync_playwright() as pw_:
        b = pw_.chromium.launch(executable_path='/usr/bin/google-chrome', args=['--headless=new'])
        for w in (390, 1440):
            ctx = b.new_context(viewport={'width': w, 'height': 900}, is_mobile=(w == 390), has_touch=(w == 390))
            ctx.route('**/v1/go/**', lambda rt: (go.append(rt.request.url), rt.abort())); ctx.route('**/backend/**', proxy)
            ctx.add_init_script("document.addEventListener('DOMContentLoaded',()=>{const m=document.createElement('meta');m.name='csrf-token';m.content=%s;document.head.appendChild(m);});" % json.dumps(CSRF))
            pg = ctx.new_page(); pg.goto(BASE + 'digest.html?api=real&api_base=/backend', wait_until='domcontentloaded')
            sel = '[data-test=review][data-reason=claims]'
            try: pg.wait_for_selector(sel, timeout=20000)
            except Exception: pass
            ok = pg.locator(sel).count() == 1
            t = ' '.join(pg.locator(sel).inner_text().split()) if ok else ''
            rec('%d in-review screen: claims in plain words + server message + next step, no time' % w,
                ok and 'checking the offer\u2019s claims against the merchant page' in t and '\u201cThe claims check is not live yet.\u201d' in t and 'Next step:' in t
                and not any(x in t.lower() for x in ('minute', 'hour', 'today', 'tomorrow', 'soon', 'within')), t[:220])
            if ok: pg.locator(sel).scroll_into_view_if_needed()
            pg.screenshot(path='%s/live-in-review-claims-%d.png' % (PROOF, w))
            lst = pg.locator('digest-item').filter(has_text='Southwind Ledger')
            rec('%d trusted maker offer shows as listed, no review box' % w, lst.count() == 1 and lst.locator('[data-test=review]').count() == 0, (lst.inner_text()[:80] if lst.count() else 'missing'))
            ctx.close()
        b.close()
finally:
    srv.terminate()
ok = sum(1 for _, x in R if x)
print('live smoke stub vs %s @ :%s: %d passed, %d failed; go-link requests: %d' % (SHA, PORT_API, ok, len(R) - ok, len(go)))
sys.exit(0 if ok == len(R) and not go else 1)
