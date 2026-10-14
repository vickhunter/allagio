"""Live smoke (v9): the plugin-key UI + setup against the local 2d3aed7 API (http://127.0.0.1:18125, fake OIDC :18444). Never prints key values,
never requests /v1/go/** (aborted + counted). Each site gets a fresh owner email. The page calls /backend/v1/...; a Playwright route
forwards it to :18125 with the owner session cookie and Origin; the page itself sends X-CSRF-Token,
read only from <meta name="csrf-token"> (the token is taken from the same meta on /owner and mirrored into the static page). Copied from
live_smoke_6042dcb.py; that file is kept as it was."""
import os, re, json, uuid, subprocess, sys, time, socket, requests, urllib.parse as up
from playwright.sync_api import sync_playwright
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
B = 'http://127.0.0.1:' + os.environ.get('SMOKE_PORT', '18125'); V = B + '/v1'; OIDC = os.environ.get('SMOKE_OIDC', '18444'); CA = '/workspace/builds/offer-rails/d024-local/tls-%s/fakeoidc-ca.pem' % OIDC
PROOF = '/workspace/builds/offer-rails/proof/d032-ui/v9-2d3aed7/' + os.environ.get('SMOKE_PROOF', 'live'); os.makedirs(PROOF, exist_ok=True)
R = []; go_hits = []
def rec(n, ok, info=''): R.append((n, bool(ok), info)); print(('PASS ' if ok else 'FAIL ') + n + (' | ' + info if info else ''), flush=True)

def owner_site(tag):
    r = requests.post(V + '/sites/register', json={'site_url': 'https://ui-smoke-%s-%s.test' % (tag, uuid.uuid4().hex[:6]), 'plugin_version': '0.1.19'}); j = r.json()
    site = {'api_key': j['api_key'], 'id': j['site_id'], 'pk0': j.get('plugin_key') or ''}
    s = requests.Session()
    r = s.get(B + '/auth/login', params={'return_to': '/owner'}, allow_redirects=False)
    q = dict(up.parse_qsl(up.urlsplit(r.headers['Location']).query))
    r2 = requests.post('https://127.0.0.1:%s/authorize' % OIDC, data={**q, 'email': 'ui-%s-%s@smoke.test' % (tag, uuid.uuid4().hex[:8]), 'name': 'UI Smoke'}, verify=CA, allow_redirects=False)
    s.get(r2.headers['Location'], allow_redirects=False)
    url = requests.post(V + '/sites/owner-connection', headers={'X-API-Key': site['api_key'], 'Content-Type': 'application/json'}, data='{}').json()['connect_url']
    page = s.get(url, allow_redirects=False).text; m = re.search(r'name="csrf" value="([^"]+)"', page)
    tok = dict(up.parse_qsl(up.urlsplit(url).query))['token']
    s.post(B + '/owner/connect-site', data={'token': tok, 'csrf': m.group(1) if m else ''}, headers={'Origin': B}, allow_redirects=False)
    pg = s.get(B + '/owner').text
    mm = re.search(r'<meta name="csrf-token" content="([^"]+)"', pg)  # 2d3aed7: the only source
    site['csrf'] = mm.group(1) if mm else ''; site['s'] = s; site['no_window_global'] = 'OfferRails' not in pg and 'window.csrf' not in pg
    return site

def free_port():
    x = socket.socket(); x.bind(('127.0.0.1', 0)); p = x.getsockname()[1]; x.close(); return p
PORT = free_port(); BASE = 'http://127.0.0.1:%d/' % PORT
srv = subprocess.Popen([sys.executable, '-m', 'http.server', str(PORT), '--bind', '127.0.0.1'], cwd=ROOT, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); time.sleep(0.8)

def proxy(site, flags):
    def h(route):
        r = route.request; path = '/' + r.url.split('/backend/', 1)[1]
        if '/v1/go/' in path: go_hits.append(path); return route.abort()
        hd = {k: v for k, v in r.headers.items() if k.lower() in ('content-type', 'accept', 'x-csrf-token')}
        hd['Origin'] = B
        if flags.get('with_site_key') and '/plugin-key' in path: hd['X-API-Key'] = site['api_key']  # a key next to the session -> 403
        sess = requests.Session() if flags.get('no_session') and '/plugin-key' in path else site['s']  # no cookie -> 401 reauth_required
        flags.setdefault('urls', []).append(path)
        x = sess.request(r.method, B + path, headers=hd, data=r.post_data, allow_redirects=False)
        flags.setdefault('log', []).append((r.method, path.split('?')[0], x.status_code, 'x-csrf-token' in {k.lower() for k in r.headers}, (x.text[:160] if x.status_code >= 400 and 'plugin-key' not in path else '')))
        route.fulfill(status=x.status_code, content_type='application/json', body=x.text)
    return h

def page_for(b, site, w, flags):
    ctx = b.new_context(viewport={'width': w, 'height': 900}, ignore_https_errors=True, is_mobile=(w == 390), has_touch=(w == 390))
    ctx.route('**/v1/go/**', lambda r: (go_hits.append(r.request.url), r.abort()))
    ctx.route('**/backend/**', proxy(site, flags))
    ctx.add_init_script("document.addEventListener('DOMContentLoaded',()=>{const m=document.createElement('meta');m.name='csrf-token';m.content=%s;document.head.appendChild(m);});" % json.dumps(site['csrf']))
    ctx.grant_permissions(['clipboard-read', 'clipboard-write'], origin=BASE.rstrip('/'))
    return ctx, ctx.new_page()

def leaked(page, *keys):
    st = page.evaluate('JSON.stringify(Object.assign({}, localStorage))+JSON.stringify(Object.assign({}, sessionStorage))+location.href')
    return any(k and k in st for k in keys)

try:
    with sync_playwright() as p:
        b = p.chromium.launch(executable_path='/usr/bin/google-chrome', args=['--headless=new'])
        for w in (1440, 390):
            # A: settings panel on a connected site (register already minted a key)
            site = owner_site('settings%d' % w); flags = {}
            rec('%d register returns plugin_key + owner session + <meta csrf-token> on /owner, no window global' % w, site['pk0'] and site['csrf'] and site['no_window_global'], 'csrf_meta=%s' % bool(site['csrf']))
            ctx, page = page_for(b, site, w, flags)
            page.goto(BASE + 'digest.html?api=real&api_base=/backend', wait_until='domcontentloaded')
            page.wait_for_selector('[data-site-settings]', timeout=20000)
            page.click('[data-site-settings]'); page.wait_for_selector('[data-test=key-prefix]', timeout=15000)
            rec('%d GET shows the prefix only' % w, page.text_content('[data-test=key-prefix]') == site['pk0'][:8] and site['pk0'] not in page.content())
            r = page.evaluate("OfferRails.pluginKey.issue(%s)" % json.dumps(site['id']))
            rec('%d second create -> 409 {error, reason key_exists, message}' % w, r['status'] == 409 and r['body'].get('reason') == 'key_exists' and r['body'].get('message') and r['body'].get('error') and 'plugin_key' not in json.dumps(r['body']), '%s %s' % (r['status'], r['body']))
            km = r['body'].get('message', '')
            page.click('[data-key-act=rotate]'); page.click('[data-key-act=rotate-yes]'); page.wait_for_selector('[data-test=key-once]', timeout=15000)
            k1 = page.input_value('#plugin-key-value')
            rec('%d rotate shows a new key once (48 hex, differs)' % w, re.match(r'^[0-9a-f]{48}$', k1) and k1 != site['pk0'] and page.is_disabled('#key-continue'))
            page.screenshot(path='%s/live-rotate-shown-once-%d.png' % (PROOF, w))
            page.click('#copy-key'); page.wait_for_selector('#copy-key[data-copied=true]')
            rec('%d copy unlocks Continue; clipboard holds the key' % w, page.is_enabled('#key-continue') and page.evaluate('navigator.clipboard.readText()') == k1)
            page.click('#key-continue'); page.wait_for_selector('[data-test=key-prefix]', timeout=15000)
            rec('%d after Continue only the new prefix is shown' % w, page.text_content('[data-test=key-prefix]') == k1[:8] and k1 not in page.content() and not leaked(page, k1, site['pk0']))
            ack_old = requests.post(V + '/placements/%s/plugin-ack' % uuid.uuid4(), headers={'X-Plugin-Key': site['pk0']}, json={'action': 'placed', 'rel_sponsored': True}).status_code
            ack_new = requests.post(V + '/placements/%s/plugin-ack' % uuid.uuid4(), headers={'X-Plugin-Key': k1}, json={'action': 'placed', 'rel_sponsored': True}).status_code
            rec('%d old key refused, new key authenticates (random placement)' % w, ack_old == 403 and ack_new == 404, 'old %d new %d' % (ack_old, ack_new))
            page.click('[data-key-act=revoke]'); page.wait_for_selector('[data-test=revoke-modal]')
            page.screenshot(path='%s/live-revoke-modal-%d.png' % (PROOF, w))
            page.click('#revoke-yes'); page.wait_for_selector('[data-test=site-settings] [data-key-act=generate]', timeout=15000)
            ack_rev = requests.post(V + '/placements/%s/plugin-ack' % uuid.uuid4(), headers={'X-Plugin-Key': k1}, json={'action': 'placed', 'rel_sponsored': True}).status_code
            rec('%d revoke via modal -> no active key; revoked key 403' % w, 'No active plugin key.' in page.text_content('[data-test=key-status]') and ack_rev == 403, 'ack %d' % ack_rev)
            # 2d3aed7 key_exists through the UI: generate while a key is active is not offered, so call generate twice: the 2nd is a 409
            page.click('[data-key-act=generate]'); page.wait_for_selector('[data-test=key-once]', timeout=15000)
            k2 = page.input_value('#plugin-key-value'); page.check('#key-saved'); page.click('#key-continue'); page.wait_for_selector('[data-test=key-prefix]', timeout=15000)
            rec('%d generate after revoke -> 201 shown once' % w, re.match(r'^[0-9a-f]{48}$', k2) and page.text_content('[data-test=key-prefix]') == k2[:8])
            flags['with_site_key'] = True
            page.click('[data-key-act=rotate]'); page.click('[data-key-act=rotate-yes]'); page.wait_for_selector('[data-test=reauth]', timeout=15000)
            last = [x for x in flags['log'] if '/plugin-key' in x[1]][-1]
            t = ' '.join(page.inner_text('[data-test=reauth]').split())
            rec('%d session + site key -> 403 forbidden_key_type -> UI "This needs the dashboard, not a key." + server message' % w,
                last[2] == 403 and page.get_attribute('[data-test=reauth]', 'data-reason') == 'forbidden_key_type' and 'This needs the dashboard, not a key.' in t and 'cannot manage plugin keys' in t, '%s | %s' % (last[:3], t[:160]))
            page.screenshot(path='%s/live-sign-in-again-%d.png' % (PROOF, w))
            page.screenshot(path='%s/live-forbidden-key-type-%d.png' % (PROOF, w))
            writes = [x for x in flags['log'] if x[0] == 'POST' and '/plugin-key' in x[1]]
            rec('%d every plugin-key write carried X-CSRF-Token (from the meta tag)' % w, writes and all(x[3] for x in writes), '%d writes' % len(writes))
            ctx.close()
            # A2: no session cookie -> 401 reauth_required -> "Sign in again" + server message; 409 key_exists message in the UI
            flags = {'no_session': True}; ctx, page = page_for(b, site, w, flags)
            page.goto(BASE + 'digest.html?api=real&api_base=/backend', wait_until='domcontentloaded'); page.wait_for_selector('[data-site-settings]', timeout=20000)
            page.click('[data-site-settings]'); page.wait_for_selector('[data-test=reauth]', timeout=15000)
            t = ' '.join(page.inner_text('[data-test=reauth]').split()); last = [x for x in flags['log'] if '/plugin-key' in x[1]][-1]
            rec('%d no session -> 401 reauth_required -> "Sign in again to continue." + server message' % w,
                last[2] == 401 and page.get_attribute('[data-test=reauth]', 'data-reason') == 'reauth_required' and 'Sign in again to manage plugin keys.' in t, '%s | %s' % (last[:3], t[:160]))
            page.screenshot(path='%s/live-reauth-required-%d.png' % (PROOF, w)); ctx.close()
            # 409 key_exists through the UI: the site has an active key; a stale list (served empty by the test) offers Generate, the real POST answers 409
            flags = {}; ctx, page = page_for(b, site, w, flags)
            page.route('**/backend/v1/sites/*/plugin-key', lambda r: r.fulfill(status=200, content_type='application/json', body='{"keys":[]}') if r.request.method == 'GET' else r.fallback())
            page.goto(BASE + 'digest.html?api=real&api_base=/backend', wait_until='domcontentloaded'); page.wait_for_selector('[data-site-settings]', timeout=20000)
            page.click('[data-site-settings]')
            try: page.wait_for_selector('[data-key-act=generate]', timeout=15000)
            except Exception: pass
            if page.locator('[data-key-act=generate]').count():
                page.click('[data-key-act=generate]'); page.wait_for_selector('[data-test=key-msg]', timeout=15000)
                t = page.text_content('[data-test=key-msg]')
                rec('%d 409 key_exists -> UI shows the server message + rotate next step' % w, t.startswith(km) and 'rotate it' in t, t[:160])
                page.screenshot(path='%s/live-key-exists-%d.png' % (PROOF, w))
            else: rec('%d 409 key_exists -> UI shows the server message + rotate next step' % w, False, 'could not reach Generate')
            ctx.close()
            # B: setup shown-once step on a site with no active key (revoked first through the owner session)
            site = owner_site('setup%d' % w); flags = {}
            rv = site['s'].post(V + '/sites/%s/plugin-key/revoke' % site['id'], headers={'Origin': B, 'X-CSRF-Token': site['csrf'], 'Content-Type': 'application/json'}, data='{}').status_code
            ctx, page = page_for(b, site, w, flags)
            page.goto(BASE + 'setup.html?api=real&api_base=/backend&role=publisher&install_path=cli', wait_until='domcontentloaded'); page.wait_for_selector('#setup-form', timeout=20000)
            gu = [u for u in flags.get('urls', []) if u.startswith('/v1/setup/settings')]
            sg = site['s'].get(V + '/setup/settings?role=publisher', headers={'Origin': B}).json()
            rec('%d setup GET has no ?install_path=; server defaults it' % w, gu and all('install_path' not in u for u in gu) and sg.get('install_path'), '%s -> install_path=%r' % (gu[:1], sg.get('install_path')))
            chk = page.eval_on_selector_all('input[name=mode]', 'xs=>xs.filter(x=>x.checked).map(x=>x.value)')
            rec('%d checked mode is the server\'s (mode_preselected %r / mode %r), not the UI\'s' % (w, sg.get('mode_preselected'), sg.get('mode')), chk in ([], [sg.get('mode_preselected')], [sg.get('mode')]), str(chk))
            if page.locator('input[name=mode]:checked').count() == 0: page.check('input[name=mode][value=manual]')
            page.click('#continue')
            try: page.wait_for_selector('[data-test=key-step], [data-test=success], #save-error:not([hidden])', timeout=20000)
            except Exception: pass
            got = 'step' if page.locator('[data-test=key-once]').count() else 'success' if page.locator('[data-test=success]').count() else 'save-error' if page.is_visible('#save-error') else 'none'
            k3 = page.input_value('#plugin-key-value') if got == 'step' else ''
            rec('%d setup first save (no install_path from the UI) -> shown-once step (after revoke %d)' % (w, rv), got == 'step' and re.match(r'^[0-9a-f]{48}$', k3), 'got %s; calls %s' % (got, [(m, pth, c, e) for m, pth, c, _, e in flags['log']]))
            if got == 'step':
                page.screenshot(path='%s/live-setup-shown-once-%d.png' % (PROOF, w), full_page=True)
                page.click('#copy-key'); page.click('#key-continue'); page.wait_for_selector('[data-test=success]')
                rec('%d setup Continue -> success, key gone from page' % w, k3 not in page.content() and not leaked(page, k3))
            ctx.close()
        b.close()
finally:
    srv.terminate()
ok = sum(1 for _, o, _ in R if o)
print('live smoke vs %s @ %s:' % (os.environ.get('SMOKE_SHA', '2d3aed7'), B) + ' %d passed, %d failed; go-link requests: %d' % (ok, len(R) - ok, len(go_hits)))
sys.exit(0 if ok == len(R) and not go_hits else 1)
