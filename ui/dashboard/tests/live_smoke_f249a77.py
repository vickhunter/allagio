"""Live smoke (v9) vs PR #21 f249a77 (default :18128, OIDC :18447; SMOKE_PORT/SMOKE_OIDC/SMOKE_SHA/SMOKE_PROOF override).
No injection: every screen shows what the server sends. Flow:
 1 site owner registers a site (plugin_key), connects it; trusted maker offer lists; one live placement on that site (prepare ->
   owner approve -> commit -> plugin-ack placed) so the site digest has an item -> post_title checked.
 2 untrusted maker (email != site owner) offer -> submitted / claims / "Claims are not checked yet."; the maker owner signs in
   with the maker's email; if that does not link the maker (known server bug) the owner binds it through the dashboard form
   POST /owner/accounts (role=maker + maker key). The real /v1/account/digest then drives the in-review screen at 390 + 1440.
 3 trusted maker owner likewise -> listed, no review box.
 4 yolo save -> GET mode manual + mode_preselected yolo + approval_reason; setup screen shows YOLO picked with the server's reason.
 5 GET /v1/go/<unknown uuid> by plain HTTP (no browser): 404, no Location, port kept.
Never clicks go links; **/v1/go/** aborted + counted. Never prints keys."""
import os, sys, json, uuid, subprocess, socket, time, requests
import urllib.parse
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from live_lib import *
from playwright.sync_api import sync_playwright
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PROOF = '/workspace/builds/offer-rails/proof/d032-ui/v9-2d3aed7/' + os.environ.get('SMOKE_PROOF', 'live-18128'); os.makedirs(PROOF, exist_ok=True)
R = []; go = []; FACTS = {}
def rec(n, ok, info=''): R.append((n, bool(ok))); print(('PASS ' if ok else 'FAIL ') + n + (' | ' + str(info) if info else ''), flush=True)
tag = uuid.uuid4().hex[:6]
CTX = "I migliori landing page builder senza codice: come creare una landing page con template, aumentare la conversione e la lead generation, e collegare l'email marketing. Confronto dei page builder no-code per iniziare."
COPY = 'Se cerchi un landing page builder senza codice, prova Smoke Demo Builder.'
# 1
reg, s1 = site_owner(tag, 'uisite'); O1 = owner_headers(s1)
rec('register returns plugin_key', bool(reg.get('plugin_key')), 'len=%d' % len(reg.get('plugin_key') or ''))
rec('owner page has <meta name="csrf-token">', bool(O1['X-CSRF-Token']))
tmail = 'trusted-%s@uismoke-maker.test' % tag; tmh, tmid = maker(tmail)
toid, tr = offer(tmh, tmid, True, 'Smoke Demo Builder %s' % tag, 'Commission paid directly by merchant.')
rec('trusted maker lists directly', tr.status_code == 200 and tr.json().get('status') == 'listed' and not tr.json().get('review_reason'), '%d %s' % (tr.status_code, tr.json().get('status')))
article = 'https://uisite-%s.test/come-scegliere-un-landing-page-builder/' % tag
pr = requests.post(V + '/placements', headers={'X-API-Key': reg['api_key'], 'Content-Type': 'application/json'}, json={'offer_id': toid, 'surface_url': article, 'context': CTX, 'copy': COPY})
pl = (pr.json().get('placement') or {}).get('id')
if pl:
    g = s1.get(B + '/owner/placements/%s' % pl); s1.post(B + '/owner/placements/%s/approve' % pl, data={'csrf': csrf(g.text)}, headers={'Origin': B}, allow_redirects=False)
    cm = requests.post(V + '/placements/%s/commit' % pl, headers={'X-API-Key': reg['api_key'], 'Content-Type': 'application/json'}, json={})
    ak = requests.post(V + '/placements/%s/plugin-ack' % pl, headers={'X-Plugin-Key': reg['plugin_key']}, json={'action': 'placed', 'rel_sponsored': True})
    print('  placement prepare %d commit %d ack %d' % (pr.status_code, cm.status_code, ak.status_code))
d1 = s1.get(V + '/account/digest', headers={'Origin': B}).json()
pitems = [i for p in d1.get('projects', []) for i in p.get('items', []) if i.get('kind') == 'placement']
FACTS['post_title'] = [(i.get('post_title'), i.get('surface_url')) for i in pitems]
rec('site digest placement post_title is a string', pitems and all(isinstance(i.get('post_title'), str) for i in pitems), FACTS['post_title'])
rec('site digest placement surface_url path is an article path', pitems and all(urllib.parse.urlparse(i.get('surface_url') or '').path not in ('', '/') for i in pitems), FACTS['post_title'])
# 2 + 3
def maker_owner(mail, mh):
    s = login(mail); d = s.get(V + '/account/projects', headers={'Origin': B}).json(); linked = any(p.get('kind') == 'offer' for p in d.get('projects', []))
    how = 'sign-in email'
    if not linked:
        b = bind_maker(s, mh['X-Maker-Key']); how = 'dashboard bind POST /owner/accounts (%d), sign-in email did not link' % b.status_code
    return s, how
def untrusted(n):
    mail = 'maker%d-%s@uismoke-maker.test' % (n, tag); mh, mid = maker(mail)
    oid, r = offer(mh, mid, False, 'Northwind Builder %d %s' % (n, tag), 'Commission paid directly by merchant.'); j = r.json()
    rec('untrusted maker #%d (%s != owner) -> submitted, claims, "Claims are not checked yet."' % (n, mail.split('@')[1]),
        r.status_code == 200 and j.get('status') == 'submitted' and j.get('review_reason') == 'claims' and j.get('message') == 'Claims are not checked yet.', {k: j.get(k) for k in ('status', 'review_reason', 'message')})
    s, how = maker_owner(mail, mh); FACTS['untrusted_link'] = how
    return s, oid
su, uoid = untrusted(1)
du = su.get(V + '/account/digest', headers={'Origin': B}).json()
aitems = [i for p in du.get('projects', []) for i in p.get('items', []) if i.get('kind') != 'placement']
FACTS['account_digest_offer_items'] = len(aitems); FACTS['account_digest_offer_sections'] = [(p.get('kind'), p.get('total_items')) for p in du.get('projects', [])]
rec('GET /v1/account/digest lists the held offer in its section (link: %s)' % FACTS['untrusted_link'], any(i.get('id') == uoid for i in aitems), FACTS['account_digest_offer_sections'])
dd = su.get(V + '/digest', headers={'Origin': B}).json()
ditems = [i for i in dd.get('items', []) if i.get('kind') == 'offer']
rec('GET /v1/digest offer items carry status, review_reason, message', ditems and all(all(k in i for k in ('status', 'review_reason', 'message')) for i in ditems)
    and any(i.get('id') == uoid and i['status'] == 'submitted' and i['review_reason'] == 'claims' and i['message'] == 'Claims are not checked yet.' for i in ditems), [(i.get('status'), i.get('review_reason'), i.get('message')) for i in ditems])
USESS = {390: su, 1440: untrusted(2)[0]}  # /v1/digest items are "since the last open": one fresh maker owner per width
st, how_t = maker_owner(tmail, tmh); FACTS['trusted_link'] = how_t
# 4
yreg, sy = site_owner(tag, 'uiyolo'); Oy = owner_headers(sy)
pu = sy.put(V + '/setup/settings', headers=Oy, json={'role': 'publisher', 'install_path': 'cli', 'mode': 'yolo', 'mode_shown': 'yolo'})
gy = sy.get(V + '/setup/settings', params={'role': 'publisher'}, headers={'Origin': B}).json()
FACTS['yolo_get'] = {k: gy.get(k) for k in ('mode', 'mode_preselected', 'approval_reason', 'approval_message')}
rec('yolo save -> GET mode manual, mode_preselected yolo, approval_reason', pu.status_code == 200 and gy.get('mode') == 'manual' and gy.get('mode_preselected') == 'yolo' and bool(gy.get('approval_reason')), FACTS['yolo_get'])
# 5
gz = requests.get(V + '/go/' + str(uuid.uuid4()), allow_redirects=False)
FACTS['go_unknown'] = (gz.status_code, gz.headers.get('Location'))
rec('GET /v1/go/<unknown id> (plain HTTP): 404, no redirect, port kept', gz.status_code == 404 and not gz.headers.get('Location'), FACTS['go_unknown'])
# UI
def fp():
    x = socket.socket(); x.bind(('127.0.0.1', 0)); n = x.getsockname()[1]; x.close(); return n
PP = fp(); BASE = 'http://127.0.0.1:%d/' % PP
srv = subprocess.Popen([sys.executable, '-m', 'http.server', str(PP), '--bind', '127.0.0.1'], cwd=ROOT, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL); time.sleep(0.8)
def ctx_for(b, s, w, writes=True):
    tok = csrf(s.get(B + '/owner').text)
    def proxy(route):
        rq = route.request; path = '/' + rq.url.split('/backend/', 1)[1]
        if '/v1/go/' in path: go.append(path); return route.abort()
        if rq.method != 'GET' and not writes: return route.abort()
        hd = {k: v for k, v in rq.headers.items() if k.lower() in ('content-type', 'accept', 'x-csrf-token')}; hd['Origin'] = B
        x = s.request(rq.method, B + path, headers=hd, data=rq.post_data, allow_redirects=False)
        route.fulfill(status=x.status_code, content_type='application/json', body=x.text)  # passed through untouched
    c = b.new_context(viewport={'width': w, 'height': 900}, is_mobile=(w == 390), has_touch=(w == 390))
    c.route('**/v1/go/**', lambda rt: (go.append(rt.request.url), rt.abort())); c.route('**/backend/**', proxy)
    c.add_init_script("document.addEventListener('DOMContentLoaded',()=>{const m=document.createElement('meta');m.name='csrf-token';m.content=%s;document.head.appendChild(m);});" % json.dumps(tok))
    return c
BAD = ('minute', 'hour', 'today', 'tomorrow', 'soon', 'within')
try:
    with sync_playwright() as p:
        b = p.chromium.launch(executable_path='/usr/bin/google-chrome', args=['--headless=new'])
        for w in (390, 1440):
            c = ctx_for(b, USESS[w], w); pg = c.new_page(); pg.goto(BASE + 'digest.html?api=real&api_base=/backend', wait_until='domcontentloaded')
            sel = '[data-test=review][data-reason=claims]'
            try: pg.wait_for_selector(sel, timeout=20000)
            except Exception: pass
            ok = pg.locator(sel).count() >= 1; t = ' '.join(pg.locator(sel).first.inner_text().split()) if ok else ''
            rec('%d in-review screen from the real digest: claims in plain words + server message + next step, no time' % w,
                ok and 'Claims are not checked yet.' in t and '\u201cClaims are not checked yet.\u201d' in t and 'Next step:' in t and not any(x in t.lower() for x in BAD), t[:200])
            if ok: pg.locator(sel).first.scroll_into_view_if_needed()
            pg.wait_for_timeout(1500); pg.screenshot(path='%s/live-in-review-claims-%d.png' % (PROOF, w)); c.close()
            c = ctx_for(b, st, w); pg = c.new_page(); pg.goto(BASE + 'digest.html?api=real&api_base=/backend', wait_until='domcontentloaded')
            try: pg.wait_for_selector('digest-item', timeout=20000)
            except Exception: pass
            it = pg.locator('digest-item').filter(has_text='Smoke Demo Builder')
            rec('%d trusted maker offer shows listed, no review box (real digest)' % w, it.count() >= 1 and it.first.locator('[data-test=review]').count() == 0 and 'LISTED' in it.first.inner_text().upper(), it.first.inner_text()[:80].replace('\n', ' ') if it.count() else 'missing')
            pg.wait_for_timeout(1500); pg.screenshot(path='%s/live-trusted-listed-%d.png' % (PROOF, w), full_page=True); c.close()
            c = ctx_for(b, s1, w); pg = c.new_page(); pg.goto(BASE + 'digest.html?api=real&api_base=/backend', wait_until='domcontentloaded')
            try: pg.wait_for_selector('digest-item', timeout=20000)
            except Exception: pass
            txt = ' '.join(pg.locator('digest-item').all_inner_texts())
            rec('%d site digest shows the post title (no "Untitled post")' % w, pitems and 'Untitled post' not in txt and '\u201c\u201d' not in txt, txt[:160].replace('\n', ' '))
            pg.wait_for_timeout(1500); pg.screenshot(path='%s/live-site-digest-post-title-%d.png' % (PROOF, w), full_page=True); c.close()
            c = ctx_for(b, sy, w, writes=False); pg = c.new_page(); pg.goto(BASE + 'setup.html?api=real&api_base=/backend&role=publisher', wait_until='domcontentloaded')
            pg.wait_for_selector('#setup-form', timeout=20000); pg.wait_for_timeout(600)
            chk = pg.eval_on_selector_all('input[name=mode]', 'xs=>xs.filter(x=>x.checked).map(x=>x.value)')
            note = ' '.join(pg.inner_text('[data-test=mode-pending]').split()) if pg.locator('[data-test=mode-pending]').count() else ''
            rec('%d setup after yolo save: YOLO picked + server reason' % w, chk == ['yolo'] and gy.get('approval_message') and gy['approval_message'] in note and note.startswith('Picked.'), 'checked=%s note=%r' % (chk, note))
            pg.locator('[data-mode-choice=yolo]').scroll_into_view_if_needed(); pg.screenshot(path='%s/live-setup-yolo-picked-%d.png' % (PROOF, w)); c.close()
        b.close()
finally:
    srv.terminate()
ok = sum(1 for _, x in R if x)
print('FACTS ' + json.dumps(FACTS))
print('live smoke vs %s @ %s: %d passed, %d failed; /v1/go requests from the browser: %d' % (SHA, B, ok, len(R) - ok, len(go)))
sys.exit(0 if ok == len(R) and not go else 1)
