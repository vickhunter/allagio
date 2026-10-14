"""Records what the owner setup screen shows after saving mode yolo on :18127 (1b85f07). Site Keeper reported the
server returns mode manual with no approval_reason. The UI must show the server's answer as-is."""
import os, re, sys, json, uuid, subprocess, socket, time, requests, urllib.parse as up
from playwright.sync_api import sync_playwright
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__))); B = 'http://127.0.0.1:' + os.environ.get('SMOKE_PORT', '18127'); V = B + '/v1'
OIDC = os.environ.get('SMOKE_OIDC', '18446'); CA = '/workspace/builds/offer-rails/d024-local/tls-%s/fakeoidc-ca.pem' % OIDC; PROOF = '/workspace/builds/offer-rails/proof/d032-ui/v9-2d3aed7/' + os.environ.get('SMOKE_PROOF', 'live-18127')
tag = uuid.uuid4().hex[:6]; go = []
j = requests.post(V + '/sites/register', json={'site_url': 'https://ui-yolo-%s.test' % tag, 'plugin_version': '0.1.20'}).json()
s = requests.Session(); r = s.get(B + '/auth/login', params={'return_to': '/owner'}, allow_redirects=False)
q = dict(up.parse_qsl(up.urlsplit(r.headers['Location']).query))
r2 = requests.post('https://127.0.0.1:%s/authorize' % OIDC, data={**q, 'email': 'yolo-%s@uismoke.test' % tag, 'name': 'Yolo'}, verify=CA, allow_redirects=False); s.get(r2.headers['Location'], allow_redirects=False)
url = requests.post(V + '/sites/owner-connection', headers={'X-API-Key': j['api_key'], 'Content-Type': 'application/json'}, data='{}').json()['connect_url']
m = re.search(r'name="csrf" value="([^"]+)"', s.get(url).text); tok = dict(up.parse_qsl(up.urlsplit(url).query))['token']
s.post(B + '/owner/connect-site', data={'token': tok, 'csrf': m.group(1) if m else ''}, headers={'Origin': B}, allow_redirects=False)
CSRF = re.search(r'<meta name="csrf-token" content="([^"]+)"', s.get(B + '/owner').text).group(1)
g0 = s.get(V + '/setup/settings?role=publisher', headers={'Origin': B}).json()
body = {'role': 'publisher', 'install_path': g0.get('install_path') or 'cli', 'mode': 'yolo', 'mode_shown': g0.get('mode_preselected') or 'manual'}
pu = s.put(V + '/setup/settings', headers={'Origin': B, 'X-CSRF-Token': CSRF, 'Content-Type': 'application/json'}, data=json.dumps(body))
g1 = s.get(V + '/setup/settings?role=publisher', headers={'Origin': B}).json()
pick = lambda d: {k: d.get(k) for k in ('mode', 'mode_preselected', 'available_modes', 'approval_reason', 'approval_message', 'effective_mode')}
print('PUT yolo ->', pu.status_code, pu.text[:300] if pu.status_code >= 400 else '', json.dumps(pick(pu.json() if pu.headers.get('content-type', '').startswith('application/json') else {})))
print('GET after ->', json.dumps(pick(g1)))
def fp():
    x = socket.socket(); x.bind(('127.0.0.1', 0)); n = x.getsockname()[1]; x.close(); return n
PP = fp(); BASE = 'http://127.0.0.1:%d/' % PP
srv = subprocess.Popen([sys.executable, '-m', 'http.server', str(PP), '--bind', '127.0.0.1'], cwd=ROOT, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); time.sleep(0.8)
def proxy(route):
    rq = route.request; path = '/' + rq.url.split('/backend/', 1)[1]
    if '/v1/go/' in path: go.append(path); return route.abort()
    if rq.method != 'GET': return route.abort()  # read-only: never re-save
    x = s.get(B + path, headers={'Origin': B}); route.fulfill(status=x.status_code, content_type='application/json', body=x.text)
try:
    with sync_playwright() as p:
        b = p.chromium.launch(executable_path='/usr/bin/google-chrome', args=['--headless=new'])
        for w in (390, 1440):
            ctx = b.new_context(viewport={'width': w, 'height': 900}); ctx.route('**/v1/go/**', lambda rt: (go.append(1), rt.abort())); ctx.route('**/backend/**', proxy)
            pg = ctx.new_page(); pg.goto(BASE + 'setup.html?api=real&api_base=/backend&role=publisher', wait_until='domcontentloaded'); pg.wait_for_selector('#setup-form', timeout=20000); pg.wait_for_timeout(800)
            chk = pg.eval_on_selector_all('input[name=mode]', 'xs=>xs.filter(x=>x.checked).map(x=>x.value)')
            note = pg.locator('[data-test=mode-pending]')
            print('%d owner screen: checked=%s note=%r' % (w, chk, ' '.join(note.first.inner_text().split())[:160] if note.count() else ''))
            pg.screenshot(path='%s/live-setup-after-yolo-save-%d.png' % (PROOF, w), full_page=True); ctx.close()
        b.close()
finally: srv.terminate()
print('go-link requests:', len(go))
