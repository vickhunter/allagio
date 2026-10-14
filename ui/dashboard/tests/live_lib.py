"""Shared helpers for the v9 live smokes. Target from SMOKE_PORT / SMOKE_OIDC / SMOKE_SHA. Never prints key values."""
import os, re, json, uuid, subprocess, requests, urllib.parse as up
L = '/workspace/builds/offer-rails/d024-local'
PORT = os.environ.get('SMOKE_PORT', '18128'); OIDC = os.environ.get('SMOKE_OIDC', '18447'); SHA = os.environ.get('SMOKE_SHA', 'f249a77')
B = 'http://127.0.0.1:' + PORT; V = B + '/v1'; CA = '%s/tls-%s/fakeoidc-ca.pem' % (L, OIDC)
ENV = dict(l.strip().split('=', 1) for l in open('%s/run/%s.env' % (L, SHA)) if '=' in l and not l.startswith('#'))
def sql(q): return subprocess.run([L + '/tools/pgexec/pgexec', q], env={'PGURL': ENV['DATABASE_URL']}, capture_output=True, text=True).stdout.strip()
def csrf(html):
    m = re.search(r'name="csrf" value="([^"]+)"', html) or re.search(r'name="csrf-token" content="([^"]+)"', html); return m.group(1) if m else ''
def login(email, name='UI Smoke'):
    s = requests.Session(); r = s.get(B + '/auth/login', params={'return_to': '/owner'}, allow_redirects=False)
    q = dict(up.parse_qsl(up.urlsplit(r.headers['Location']).query))
    r2 = requests.post('https://127.0.0.1:%s/authorize' % OIDC, data={**q, 'email': email, 'name': name}, verify=CA, allow_redirects=False)
    s.get(r2.headers['Location'], allow_redirects=False); return s
def owner_headers(s): return {'Origin': B, 'X-CSRF-Token': csrf(s.get(B + '/owner').text), 'Content-Type': 'application/json'}
def site_owner(tag, name):
    j = requests.post(V + '/sites/register', json={'site_url': 'https://%s-%s.test' % (name, tag), 'plugin_version': '0.1.20'}).json()
    s = login('%s-%s@uismoke.test' % (name, tag))
    url = requests.post(V + '/sites/owner-connection', headers={'X-API-Key': j['api_key'], 'Content-Type': 'application/json'}, data='{}').json()['connect_url']
    tok = dict(up.parse_qsl(up.urlsplit(url).query))['token']
    s.post(B + '/owner/connect-site', data={'token': tok, 'csrf': csrf(s.get(url).text)}, headers={'Origin': B}, allow_redirects=False)
    return j, s
def maker(email):
    j = requests.post(V + '/makers/register', json={'email': email, 'name': 'UI Smoke Maker'}).json(); return {'X-Maker-Key': j['api_key']}, j['maker_id']
def offer(mh, mid, trusted, pname, terms):
    land = 'https://example.com/' + pname.lower().replace(' ', '-')
    p = requests.post(V + '/maker/products', headers=mh, json={'name': pname, 'tagline': 'Demo landing page builder: launch a landing page without code', 'website_url': land,
        'primary_category': 'landing page builder', 'keywords': ['landing page', 'landing page builder', 'page builder', 'no-code', 'templates']}).json()
    sql("UPDATE products SET readiness_status='ready' WHERE id='%s'" % p['id'])  # same test-data step as d024-local/checks-*-claims.py
    o = requests.post(V + '/maker/offers', headers=mh, json={'product_id': p['id'], 'commission_type': 'percentage', 'commission_value': 30, 'commission_currency': 'USD',
        'cookie_duration_days': 30, 'refund_window_days': 30, 'landing_url': land, 'terms': terms}).json()
    if trusted: requests.post(V + '/offer-review/makers/%s/trust' % mid, headers={'X-Admin-Key': ENV['ADMIN_API_KEY']}, json={'trusted': True, 'actor': 'd032-ui-v9-smoke'})
    return o['id'], requests.post(V + '/maker/offers/%s/submit' % o['id'], headers=mh)
def bind_maker(s, maker_key):
    """Owner dashboard form POST /owner/accounts role=maker + the maker key (hosted_auth.go OwnerAccount)."""
    return s.post(B + '/owner/accounts', data={'role': 'maker', 'api_key': maker_key, 'csrf': csrf(s.get(B + '/owner').text), 'return_to': '/owner'}, headers={'Origin': B}, allow_redirects=False)
