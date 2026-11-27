"""Playwright walk of features/*.feature at 1440 and 390. Run: python3 tests/run_tests.py
Serves the folder with python3 -m http.server on a free port, system Chrome, aborts **/v1/go/**."""
import json, os, re, socket, subprocess, sys, time, traceback
from playwright.sync_api import sync_playwright

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
PROOF = '/workspace/builds/offer-rails/proof/d032-ui/v9-2d3aed7'
API = os.path.join(ROOT, 'fixtures/api')
os.makedirs(PROOF, exist_ok=True)
AXE = open(os.path.join(ROOT, 'vendor/axe.min.js')).read()
EVENTS = ['setup-submit', 'undo', 'restore', 'retry', 'placement-result', 'plugin-state', 'pause-all', 'resume-all', 'account-result', 'digest-opened', 'numbers-refreshed']
TEXTLOG = """window.__texts=[];document.addEventListener('DOMContentLoaded',()=>{new MutationObserver(ms=>{for(const m of ms){
const el=(m.target.nodeType===3?m.target.parentElement:m.target);if(!el||!el.closest)continue;
const t=el.closest('[data-test=state],[data-test=note],[data-test=note-error],#toast-text,[data-mode]');if(!t)continue;
const it=t.closest('digest-item');window.__texts.push({item:it&&it.item?it.item.item_id:null,kind:t.dataset.test||t.id||'mode',text:t.textContent});}})
.observe(document.body,{subtree:true,childList:true,characterData:true});});"""
RECORD = TEXTLOG + "window.__ev=[];" + "".join(
    "document.addEventListener('offerrails:%s',e=>window.__ev.push({name:e.type,detail:JSON.parse(JSON.stringify(e.detail))}));" % n for n in EVENTS)

def free_port():
    s = socket.socket(); s.bind(('127.0.0.1', 0)); p = s.getsockname()[1]; s.close(); return p

PORT = free_port()
BASE = 'http://127.0.0.1:%d/' % PORT
go_hits = []

def fx(name):
    return open(os.path.join(API, name + '.json')).read()

class MockAPI:
    """Fake D-032 backend: records every call; revoke is final on the server as soon as it lands."""
    def __init__(self):
        self.calls = []; self.revoked = {}; self.restore = None; self.plugin = ['pending']; self.revoke_body = None
        self.retry = None; self.hold_retry = False; self.held = []
        self.plugin_by = {}; self.hold_account = False; self.held_acct = []; self.resume = 200
        # real mode (PR #21 reads)
        self.acct = json.loads(fx('account-digest')); self.dig = 'digest'; self.hard = (200, fx('hard-checks'))
        self.resume_body = None; self.setup_get = 'setup-settings-publisher'; self.setup_put = (200, None); self.put_bodies = []
    def install(self, ctx):
        ctx.route(re.compile(r'.*/v1/placements/[^/]+/revoke$'), self.on_revoke)
        ctx.route(re.compile(r'.*/v1/placements/[^/]+/restore$'), self.on_restore)
        ctx.route(re.compile(r'.*/v1/placements/[^/]+/retry$'), self.on_retry)
        ctx.route(re.compile(r'.*/v1/placements/[^/?]+$'), self.on_get)
        ctx.route(re.compile(r'.*/v1/account/(pause|resume)$'), self.on_account)
        ctx.route(re.compile(r'.*/v1/account/digest$'), lambda r: (self.rec(r), self.reply(r, 200, json.dumps(self.acct))))
        ctx.route(re.compile(r'.*/v1/digest$'), lambda r: (self.rec(r), self.reply(r, 200, fx(self.dig))))
        ctx.route(re.compile(r'.*/v1/policy/hard-checks$'), lambda r: (self.rec(r), self.reply(r, self.hard[0], self.hard[1])))
        ctx.route(re.compile(r'.*/v1/events$'), lambda r: (self.rec(r), self.reply(r, 201, fx('events-201'))))
        ctx.route(re.compile(r'.*/v1/setup/settings(\?.*)?$'), self.on_setup)
        # v8: plugin key (6042dcb plugin_keys.go) + account projects
        self.pk = {'list': (200, 'plugin-key-list'), 'issue': (201, 'plugin-key-201'), 'rotate': (200, 'plugin-key-rotate-200'), 'revoke': (200, 'plugin-key-revoke-200')}
        self.pk_after = {}  # action -> list fixture served after that action succeeded
        ctx.route(re.compile(r'.*/v1/sites/[^/]+/plugin-key(/rotate|/revoke)?$'), self.on_pk)
        ctx.route(re.compile(r'.*/v1/account/projects$'), lambda r: (self.rec(r), self.reply(r, 200, fx('account-projects'))))
    def on_setup(self, route):
        c = self.rec(route)
        if c['method'] == 'GET': return self.reply(route, 200, fx(self.setup_get))
        self.put_bodies.append(c['body']); st, name = self.setup_put
        self.reply(route, st, fx(name) if name else json.dumps(dict(json.loads(fx(self.setup_get)), **c['body'])))
    def on_pk(self, route):
        c = self.rec(route); tail = c['path'].split('/plugin-key', 1)[1]
        act = 'list' if c['method'] == 'GET' else {'': 'issue', '/rotate': 'rotate', '/revoke': 'revoke'}[tail]
        st, name = self.pk[act]
        if st < 300 and act in self.pk_after: self.pk['list'] = (200, self.pk_after[act])
        self.reply(route, st, fx(name))
    def pk_calls(self):
        return [(c['method'], c['path'].split('/plugin-key', 1)[1] or '/', c['body']) for c in self.calls if '/plugin-key' in c['path']]
    def rec(self, route):
        r = route.request; body = r.post_data
        c = {'method': r.method, 'path': re.sub(r'^/backend', '', '/' + r.url.split('/', 3)[3]), 'url': r.url, 'body': json.loads(body) if body else None, 't': time.time(), 'headers': dict(r.headers)}
        self.calls.append(c); return c
    def reply(self, route, status, body):
        route.fulfill(status=status, content_type='application/json', body=body)
    def on_revoke(self, route):
        c = self.rec(route); pid = c['path'].split('/')[3]; self.revoked[pid] = c['t']
        self.reply(route, 200, self.revoke_body or fx('revoke-200').replace('plc_001', pid))
    def on_restore(self, route):
        c = self.rec(route); pid = c['path'].split('/')[3]
        if self.restore:
            st, name = self.restore; return self.reply(route, st, fx(name))
        if time.time() - self.revoked.get(pid, 0) > 6:  # server-side 6 s window
            return self.reply(route, 409, fx('restore-409-window'))
        self.revoked.pop(pid, None); self.reply(route, 200, fx('restore-200'))
    def on_retry(self, route):
        self.rec(route)
        if self.hold_retry: self.held.append(route); return
        st, name = self.retry or (200, 'retry-200'); self.reply(route, st, fx(name))
    def release_account(self):
        r, path = self.held_acct.pop(0)
        self.reply(r, 200, fx('pause-200' if path.endswith('pause') else 'resume-200'))
    def release_retry(self, status, body):
        r = self.held.pop(0); self.reply(r, status, body)
    def on_get(self, route):
        c = self.rec(route); pid = c['path'].split('/')[3]
        st = self.plugin_by[pid] if pid in self.plugin_by else (self.plugin.pop(0) if len(self.plugin) > 1 else self.plugin[0])
        if st == '429': return self.reply(route, 429, fx('placement-429'))
        self.reply(route, 200, fx('placement-' + st).replace('9c000001-0000-4000-8000-000000000001', pid) if st in ('pending', 'placed', 'removed', 'offline', 'failed', 'failed-restored') else json.dumps({'plugin_state': st}))
    def on_account(self, route):
        c = self.rec(route)
        if self.hold_account: self.held_acct.append((route, c['path'])); return
        if c['path'].endswith('pause'): return self.reply(route, 200, fx('pause-200'))
        self.reply(route, self.resume, self.resume_body or fx('resume-200' if self.resume == 200 else 'resume-403'))
    def find(self, suffix):
        return [c for c in self.calls if c['path'].split('?')[0].endswith(suffix)]


def ev(page, name=None):
    e = page.evaluate('window.__ev')
    return [x for x in e if name is None or x['name'] == 'offerrails:' + name]

def no_overflow(page):
    w = page.evaluate('[document.documentElement.scrollWidth, window.innerWidth]')
    assert w[0] <= w[1], 'horizontal overflow %s' % w

def visible_text(page):
    return page.evaluate('document.body.innerText')

SETTLED = "document.getAnimations().every(a => a.playState !== 'running' || !a.effect || a.effect.getTiming().iterations === Infinity || a.effect.getTiming().duration > 1000)"

def shot(page, name, width, full=True):
    try: page.wait_for_function(SETTLED, timeout=3000)  # v7: no screenshots mid first-load fade
    except Exception: pass
    page.screenshot(path=os.path.join(PROOF, '%s-%d.png' % (name, width)), full_page=full)

def axe(page):
    # v7: let finite entrance animations (700ms first-load fade, panel/modal entrances) finish so axe
    # measures settled colours; loops (BLIP blink, skeleton shimmer) and timers (the toast's drain bar, > 1s) are not waited on.
    try: page.wait_for_function("document.getAnimations().every(a => a.playState !== 'running' || !a.effect || a.effect.getTiming().iterations === Infinity || a.effect.getTiming().duration > 1000)", timeout=5000)
    except Exception:
        raise AssertionError('animations still running: %s' % page.evaluate("document.getAnimations().filter(a=>a.playState==='running').map(a=>[a.animationName||a.transitionProperty,a.effect&&a.effect.target&&(a.effect.target.id||a.effect.target.className),a.effect&&a.effect.getTiming().iterations,a.effect&&a.effect.getTiming().duration,a.currentTime])"))
    page.add_script_tag(content=AXE)
    r = page.evaluate("axe.run(document,{runOnly:{type:'tag',values:['wcag2a','wcag2aa','wcag21aa','best-practice']}})")
    bad = [(v['id'], v['impact'], len(v['nodes']), v['nodes'][0]['target']) for v in r['violations']]
    assert not bad, 'axe: %s' % bad
    return len(r['passes'])

# ---------------- setup ----------------
def setup_maker_happy(page, w):
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#continue')
    assert 'Your agent is connected' in page.inner_text('h1')
    assert 'npx offer-rails' in page.inner_text('[data-test=agent-source]')
    assert 'Rail Tool' in page.inner_text('[data-test=project-name]')
    assert page.is_checked('input[name=role][value=maker]')
    assert page.locator('fieldset.q').count() == 4
    for sel in ['#listing-name', '#avoid-topics', '#avoid-types', '#commission-value']:
        assert page.input_value(sel) != '', sel + ' not prefilled'
    assert page.is_checked('input[name=mode][value=yolo]')
    assert page.locator('label.choice:has(input[value=yolo])').inner_text().startswith('Approve anything that passes the checks')
    assert not re.search(r'\b(manual|auto|yolo)\b', visible_text(page), re.I), 'internal mode name on screen'
    page.wait_for_selector('section.checks[aria-busy=false]')
    assert page.locator('#h-checks').text_content() == 'Rules every listing must follow'
    assert 'checked' not in page.locator('#h-checks').text_content().lower()
    assert page.locator('[data-test=hard-checks] li').all_inner_texts() == [c['check_text'] for c in json.loads(fx('hard-checks'))['checks']]
    no_overflow(page); shot(page, 'setup-maker', w)
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    e = ev(page, 'setup-submit'); assert len(e) == 1
    d = e[0]['detail']
    assert d['roles'] == ['maker'] and d['mode_shown'] == 'yolo' and d['mode_chosen'] == 'yolo', d
    assert d['edited_fields'] == [], d['edited_fields']
    assert d['answers']['maker']['listing']['name'] == 'Rail Tool' and d['session_only'] is True
    assert 'Your agent can take it from here' in page.inner_text('[data-test=success]')
    assert page.locator('#back-to-agent').is_visible()
    shot(page, 'setup-maker-success', w)

def setup_publisher_happy(page, w):
    page.goto(BASE + 'setup.html?fixture=publisher'); page.wait_for_selector('#continue')
    assert page.is_checked('input[name=role][value=publisher]')
    assert 'Cursor' in page.inner_text('[data-test=agent-source]')
    assert page.locator('fieldset.q').count() == 4
    assert page.is_checked('input[name=position][value=end]')
    assert not re.search(r'\b(manual|auto|yolo)\b', visible_text(page), re.I)
    no_overflow(page); shot(page, 'setup-publisher', w)
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    d = ev(page, 'setup-submit')[0]['detail']
    assert d['roles'] == ['publisher'] and d['mode_chosen'] == 'yolo'
    p = d['answers']['publisher']
    assert [s['domain'] for s in p['sites']] == ['victormotricala.it', 'newsletter.victormotricala.it'], p
    assert p['box_position'] == 'end' and p['never_topics'] == ['gambling', 'crypto']

def setup_edit_prefilled(page, w):
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#continue')
    page.click('#edit-listing'); assert page.get_attribute('#edit-listing', 'aria-expanded') == 'true'
    page.fill('#listing-pitch', 'Referral offers for dev tools, set up by your agent.')
    new = 'Referral offers for dev tools, set up by your agent.'
    assert page.text_content('[data-bind=pitch]') == new, page.text_content('[data-bind=pitch]')
    assert page.text_content('#listing-pitch-count') == '%d/140' % len(new), page.text_content('#listing-pitch-count')
    page.click('label.choice:has(input[value=manual])')
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    d = ev(page, 'setup-submit')[0]['detail']
    assert d['answers']['maker']['listing']['pitch'] == 'Referral offers for dev tools, set up by your agent.'
    assert d['mode_shown'] == 'yolo' and d['mode_chosen'] == 'manual'
    assert 'listing.pitch' in d['edited_fields'] and 'mode' in d['edited_fields'], d['edited_fields']

def setup_failed_signal(page, w):
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#continue')
    li = page.locator('[data-signal=webhook]')
    assert 'fail' in li.get_attribute('class') and li.is_visible()
    assert 'No handler for a completed checkout' in li.inner_text()
    assert '72' in page.inner_text('[data-test=readiness-score]')
    assert '1 check failed' in page.inner_text('[data-test=fail-note]')
    assert page.is_enabled('#continue')
    li.scroll_into_view_if_needed(); shot(page, 'setup-maker-failed-signal', w, full=False)

def setup_empty_scan(page, w):
    page.goto(BASE + 'setup.html?fixture=empty'); page.wait_for_selector('#continue')
    t = visible_text(page)
    assert 'No pitch found' in t and 'No commission found' in t and "We couldn't tell" in t
    assert page.is_checked('input[name=mode][value=yolo]')
    no_overflow(page); shot(page, 'setup-empty', w)
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    d = ev(page, 'setup-submit')[0]['detail']
    assert d['answers']['maker']['avoid_topics'] == [] and d['answers']['maker']['commission']['value'] is None

def setup_directory_default(page, w):
    page.goto(BASE + 'setup.html?fixture=directory'); page.wait_for_selector('#continue')
    assert page.is_checked('input[name=mode][value=manual]')
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    d = ev(page, 'setup-submit')[0]['detail']
    assert d['mode_shown'] == 'manual' and d['install_path'] == 'chatgpt_directory'

def setup_nomode(page, w):
    page.goto(BASE + 'setup.html?fixture=nomode'); page.wait_for_selector('#continue')
    assert page.locator('input[name=mode]:checked').count() == 0
    assert 'Suggested' not in page.inner_text('fieldset[data-q=mode]')
    page.click('#continue')
    assert page.is_visible('#mode-error') and ev(page, 'setup-submit') == []
    assert page.evaluate('document.activeElement.name') == 'mode'
    page.click('label.choice:has(input[value=auto])'); page.click('#continue'); page.wait_for_selector('[data-test=success]')
    d = ev(page, 'setup-submit')[0]['detail']
    assert d['mode_shown'] is None and d['mode_chosen'] == 'auto'

def setup_save_error(page, w):
    page.goto(BASE + 'setup.html?fixture=maker&fail=save'); page.wait_for_selector('#continue')
    page.fill('#avoid-topics', 'gambling, adult, crypto')
    page.click('#continue'); page.wait_for_selector('#save-error:not([hidden])')
    assert "We couldn't save your settings" in page.inner_text('#save-error')
    assert page.input_value('#avoid-topics') == 'gambling, adult, crypto'
    assert page.is_enabled('#continue') and page.locator('[data-test=success]').count() == 0
    assert len(ev(page, 'setup-submit')) == 1
    shot(page, 'setup-save-error', w, full=False)

def setup_also_role(page, w):
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#continue')
    page.check('#also-role'); page.click('#continue'); page.wait_for_selector('[data-test=success]')
    d = ev(page, 'setup-submit')[0]['detail']
    assert d['roles'] == ['maker', 'publisher'] and 'publisher' in d['answers'], d['roles']

def setup_a11y(page, w):
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#continue')
    axe(page)
    page.keyboard.press('Tab'); assert page.evaluate('document.activeElement.className') == 'skip'
    order = []
    for _ in range(40):
        page.keyboard.press('Tab')
        order.append(page.evaluate("(()=>{const a=document.activeElement;return a.id||a.name||a.textContent.trim().slice(0,20)})()"))
        if order[-1] == 'continue': break
    want = ['role', 'also-role', 'edit-listing', 'avoid-topics', 'avoid-types', 'commission-value', 'commission-type', 'mode', 'continue']
    assert [o for o in order if o in want] == want, order
    page.goto(BASE + 'setup.html?fixture=publisher'); page.wait_for_selector('#continue'); axe(page)

# ---------------- digest ----------------
def digest_opened(page, w):
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    e = ev(page, 'digest-opened'); assert len(e) == 1 and e[0]['detail']['digest_id'] == 'dg_2026w41_usr8f2c'
    totals_y = page.locator('[data-test=totals]').bounding_box()['y']
    assert totals_y < page.locator('section.project').first.bounding_box()['y']
    for k in ['live_items', 'clicks', 'earnings', 'projects', 'sites']:
        assert page.locator('[data-total=%s]' % k).count() == 1
    assert page.inner_text('[data-total=earnings]').count('€34.90') == 1
    assert 'Test mode' in page.inner_text('[data-total=earnings]')
    assert page.locator('section.project').count() == 3
    hrefs = page.eval_on_selector_all('a', 'as=>as.map(a=>a.getAttribute("href"))')
    assert not [h for h in hrefs if h and '/v1/go/' in h], hrefs
    assert set(page.eval_on_selector_all('a[data-preview]', 'as=>as.map(a=>a.getAttribute("href"))')) == {'#preview'}
    t = page.locator('digest-item').first.inner_text()
    assert '(link sponsorizzato)' in t and 'Approved by your rule: Passed all checks' in ' '.join(t.split())
    assert not re.search(r'\b(manual|yolo|policy:)\b', visible_text(page), re.I)
    no_overflow(page); shot(page, 'digest-normal', w)

def site_item(page, n=0):
    return page.locator('[data-section=site_vm] digest-item').nth(n)

def state(item):
    return item.locator('[data-test=state]').text_content()

def note(item):
    return item.locator('[data-test=note]').text_content()

def undo(page, item):
    item.locator('button[data-action=toggle]').click()
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:placement-result")')

def digest_undo_redo(page, w):
    page.goto(BASE + 'digest.html?fixture=normal&toast_ms=30000'); page.wait_for_selector('digest-item')
    item = site_item(page)
    t0 = time.time(); undo(page, item)
    rv = page.mock.find('/revoke'); assert len(rv) == 1, page.mock.calls
    assert rv[0]['path'] == '/v1/placements/plc_003/revoke' and rv[0]['body'] == {'reason': 'undo'} and rv[0]['t'] - t0 < 4
    e = ev(page, 'undo'); assert len(e) == 1 and e[0]['detail']['item_id'] == 'itm_003' and e[0]['detail']['request']['body'] == {'reason': 'undo'}
    assert state(item) == 'Leaving your post shortly', state(item)
    assert page.is_visible('#toast') and 'Redo' in page.inner_text('#toast')
    assert page.locator('[role=dialog],dialog[open]').count() == 0
    shot(page, 'digest-undo-toast', w, full=False)
    page.click('#toast-redo'); page.wait_for_function('window.__ev.filter(e=>e.name=="offerrails:placement-result").length==2')
    assert len(page.mock.find('/restore')) == 1 and len(ev(page, 'restore')) == 1
    assert state(item) == 'Going back into your post', state(item)
    assert page.is_hidden('#toast')

def digest_undo_then_close(page, w):
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    with page.expect_request(re.compile(r'.*/revoke$')) as req:
        t0 = time.time(); site_item(page, 1).locator('button[data-action=toggle]').click()
    assert time.time() - t0 < 4 and json.loads(req.value.post_data) == {'reason': 'undo'}
    page.close()  # tab closed right after the tap
    assert 'plc_004' in page.mock.revoked, 'placement should stay revoked on the server'
    assert not page.mock.find('/restore')

def digest_window_closes(page, w):
    page.goto(BASE + 'digest.html?fixture=normal&toast_ms=600'); page.wait_for_selector('digest-item')
    item = site_item(page); undo(page, item)
    page.wait_for_selector('#toast[hidden]', state='attached', timeout=3000)
    assert item.locator('button[data-action=toggle]').is_hidden()
    assert state(item) == 'Leaving your post shortly'

def restore_case(page, w, status_fixture, expect, shot_name=None, wait=0, toast_ms=30000):
    if status_fixture: page.mock.restore = status_fixture
    page.goto(BASE + 'digest.html?fixture=normal&toast_ms=%d' % toast_ms); page.wait_for_selector('digest-item')
    item = site_item(page); undo(page, item)
    if wait: page.wait_for_timeout(wait)
    page.click('#toast-redo'); page.wait_for_function('window.__ev.filter(e=>e.name=="offerrails:placement-result").length==2')
    n, t = note(item), page.inner_text('#toast-text')
    assert expect in n and expect in t, (n, t)
    assert '{' not in n and '"' not in n
    assert item.locator('.digest-item').get_attribute('data-status') == 'undone'
    assert state(item) == 'Leaving your post shortly'
    assert item.locator('button[data-action=toggle]').is_hidden() and page.is_hidden('#toast-redo')
    if shot_name: item.scroll_into_view_if_needed(); shot(page, shot_name, w, full=False)

def restore_409_window_7s(page, w):
    # client toast set longer than the server window to simulate a late tap; server mock enforces 6 s
    restore_case(page, w, None, 'Too late to redo. The restore window has expired. It stays undone.', 'restore-409-too-late', wait=7000, toast_ms=10000)
    r = [e for e in ev(page, 'placement-result') if e['detail']['action'] == 'restore'][0]['detail']
    assert r['http_status'] == 409 and r['outcome'] == 'window_expired'

def restore_409_paused(page, w):
    restore_case(page, w, (409, 'restore-409-paused'), "Can't redo. Pause or the kill switch was turned on after the undo. It stays undone.", 'restore-409-paused')

def restore_409_noreason(page, w):
    restore_case(page, w, (409, 'restore-409-not-undone'), "Can't redo. The placement was not undone. Showing the latest status.")

def restore_403_other_side(page, w):
    restore_case(page, w, (403, 'restore-403'), 'Only the person who undid this box can bring it back.', 'restore-403')

def restore_422_cap(page, w):
    restore_case(page, w, (422, 'restore-422-cap'), "Can't redo. The placement cap is full. It stays undone.", 'restore-422-cap')

def restore_422_relevance_min(page, w):
    restore_case(page, w, (422, 'restore-422-relevance_min'), "Can't redo. Relevance is below the minimum. It stays undone.")

def restore_422_offer_live(page, w):
    restore_case(page, w, (422, 'restore-422-offer_live'), "Can't redo. The offer is not live. It stays undone.")

def restore_422_fallbacks(page, w):
    restore_case(page, w, (422, 'synthetic/restore-422-no-text'), "Can't redo. The disclosure label could not be added. It stays undone.")
    page.mock.restore = (422, 'synthetic/restore-422-unknown')
    restore_case(page, w, None, "Can't redo: it no longer passes the checks. It stays undone.")

def plugin_states_fixture(page, w):
    page.goto(BASE + 'digest.html?fixture=plugin-states'); page.wait_for_selector('digest-item')
    a, b, c = site_item(page, 0), site_item(page, 1), site_item(page, 2)
    assert state(a) == 'Leaving your post shortly', state(a)
    assert state(b) == 'Box removed', state(b)
    assert state(c) == 'Clicks off · site offline', state(c)
    assert note(c) == 'Your site is offline. Clicks already stop at the link; the box will come out when it reconnects.', note(c)
    f = page.locator('[data-section=site_nl] digest-item').first
    assert state(f) == 'Clicks off · box still in your post', state(f)
    assert note(f) == "Your site couldn't update the post. Check the notice in your WordPress admin. Clicks already stop at the link.", note(f)
    assert f.locator('[data-test=note-reason]').is_hidden() and f.locator('button[data-action=retry]').text_content() == 'Try removing again'
    assert 'caution' in f.locator('[data-test=note-box]').get_attribute('class') and 'caution' not in c.locator('[data-test=note-box]').get_attribute('class')
    u = page.locator('[data-section=prj_rail] digest-item').nth(1)
    assert state(u) == 'Status unknown', state(u)
    live = page.locator('[data-section=prj_rail] digest-item').first
    assert state(live).startswith('Live · ') and 'unknown' not in state(live)
    for it in (a, b, c, f): assert it.locator('button[data-action=toggle]').is_hidden()
    no_overflow(page)
    page.locator('[data-section=site_vm]').scroll_into_view_if_needed(); shot(page, 'digest-plugin-states', w, full=False)
    page.locator('[data-section=site_nl]').scroll_into_view_if_needed(); shot(page, 'digest-plugin-failed', w, full=False)

def polls(page, item):
    return item.evaluate('e => e._polls || 0')

def gets(page):
    return len([c for c in page.mock.calls if c['method'] == 'GET'])

def advance(page, item, ms, expect_polls):
    page.clock.run_for(ms)
    for _ in range(400):
        if polls(page, item) >= expect_polls: break
        page.wait_for_timeout(25)
    page.wait_for_timeout(30)
    assert polls(page, item) == expect_polls, (ms, polls(page, item), expect_polls)

def fake_clock_undo(page, plugin):
    page.mock.plugin = plugin
    page.clock.install()
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    item = site_item(page); undo(page, item)
    return item

def plugin_pending_then_removed(page, w):
    item = fake_clock_undo(page, ['pending', 'removed'])
    assert state(item) == 'Leaving your post shortly'
    advance(page, item, 4000, 1); assert state(item) == 'Leaving your post shortly'
    advance(page, item, 4000, 2); assert state(item) == 'Box removed', state(item)
    page.clock.run_for(120000); page.wait_for_timeout(100); assert polls(page, item) == 2, 'stops once removed'
    assert all(c['path'] == '/v1/placements/plc_003' for c in page.mock.calls if c['method'] == 'GET')

def plugin_offline_after_undo(page, w):
    item = fake_clock_undo(page, ['offline'])
    advance(page, item, 4000, 1)
    assert note(item) == 'Your site is offline. Clicks already stop at the link; the box will come out when it reconnects.'

def plugin_failed_after_undo(page, w):
    item = fake_clock_undo(page, ['failed'])
    advance(page, item, 4000, 1)
    assert note(item) == "Your site couldn't update the post. Check the notice in your WordPress admin. Clicks already stop at the link."
    assert item.locator('[data-test=note-reason]').text_content().startswith('The block was edited')
    assert 'caution' in item.locator('[data-test=note-box]').get_attribute('class')
    page.clock.run_for(60000); page.wait_for_timeout(100); assert polls(page, item) == 1, 'failed needs the owner, no more polling'

def plugin_null_is_unknown(page, w):
    page.mock.revoke_body = json.dumps({'placement_id': 'plc_003', 'status': 'revoked'})
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    item = site_item(page); undo(page, item)
    # PR #21: revoke answers without plugin_state; the contract says a revoke sets plugin_state pending.
    assert state(item) == 'Leaving your post shortly', state(item)
    item.evaluate("e => e.setPluginState('weird_value')"); assert state(item) == 'Status unknown'
    item.evaluate("e => e.setPluginState(null)"); assert state(item) == 'Status unknown'

def plugin_restore_going_back(page, w):
    item = fake_clock_undo(page, ['pending'])
    page.click('#toast-redo'); page.wait_for_function('window.__ev.filter(e=>e.name=="offerrails:placement-result").length==2')
    assert state(item) == 'Going back into your post', state(item)
    item.scroll_into_view_if_needed(); shot(page, 'digest-restore-going-back', w, full=False)
    page.mock.plugin = ['placed']
    n = polls(page, item); advance(page, item, 4000, n + 1)
    assert state(item).startswith('Live · ') and item.locator('.digest-item').get_attribute('data-status') == 'live', state(item)

def poll_schedule(page, w):
    item = fake_clock_undo(page, ['pending'])
    for i in range(1, 16): advance(page, item, 4000, i)          # every 4 s for 60 s
    advance(page, item, 4000, 15); advance(page, item, 26000, 16)  # then every 30 s
    for i in range(17, 34): advance(page, item, 30000, i)         # ... until 10 min
    assert note(item) == 'Still waiting for your site.', note(item)
    assert state(item) == 'Leaving your post shortly'
    page.clock.run_for(300000); page.wait_for_timeout(100); assert polls(page, item) == 33
    item.scroll_into_view_if_needed(); shot(page, 'digest-still-waiting', w, full=False)

def poll_only_when_visible(page, w):
    page.add_init_script("Object.defineProperty(document,'visibilityState',{configurable:true,get(){return window.__vis||'visible'}})")
    item = fake_clock_undo(page, ['pending'])
    page.evaluate("window.__vis='hidden'; document.dispatchEvent(new Event('visibilitychange'))")
    page.clock.run_for(4000); page.wait_for_timeout(100)
    page.clock.run_for(60000); page.wait_for_timeout(100)
    assert gets(page) == 0 and polls(page, item) == 0, gets(page)
    page.evaluate("window.__vis='visible'; document.dispatchEvent(new Event('visibilitychange'))")
    for _ in range(200):
        if polls(page, item) >= 1: break
        page.wait_for_timeout(25)
    assert gets(page) == 1 and polls(page, item) == 1
    advance(page, item, 4000, 1); advance(page, item, 26000, 2)  # past 60 s, so the 30 s pace applies

def digest_live_now_updates(page, w):
    page.goto(BASE + 'digest.html?fixture=normal&toast_ms=30000'); page.wait_for_selector('digest-item')
    def live(ref): return page.text_content('[data-section=%s] [data-stat=live_count]' % ref)
    def total(): return page.text_content('[data-total=live_items] b')
    assert (live('site_vm'), live('prj_rail'), total()) == ('3', '2', '6')
    undo(page, site_item(page))
    assert (live('site_vm'), live('prj_rail'), total()) == ('2', '2', '5')
    p = page.locator('[data-section=prj_rail] digest-item').first
    p.locator('button[data-action=toggle]').click()
    page.wait_for_function('window.__ev.filter(e=>e.name=="offerrails:placement-result").length==2')
    assert (live('site_vm'), live('prj_rail'), total()) == ('2', '1', '4')
    page.click('#toast-redo'); page.wait_for_function('window.__ev.filter(e=>e.name=="offerrails:placement-result").length==3')
    assert (live('site_vm'), live('prj_rail'), total()) == ('2', '1', '4'), 'restore pending does not count yet'
    p.evaluate("e => e.setPluginState('placed')")
    assert (live('site_vm'), live('prj_rail'), total()) == ('2', '2', '5')
    page.mock.restore = (409, 'restore-409-window')
    site_item(page, 1).locator('button[data-action=toggle]').click()
    page.wait_for_function('window.__ev.filter(e=>e.name=="offerrails:placement-result").length==4')
    page.click('#toast-redo'); page.wait_for_function('window.__ev.filter(e=>e.name=="offerrails:placement-result").length==5')
    assert (live('site_vm'), total()) == ('1', '4'), 'a refused restore does not add back'
    page.locator('[data-total=live_items]').scroll_into_view_if_needed(); shot(page, 'digest-live-now-updated', w, full=False)

FAILED_SENTENCE = "Your site couldn't update the post. Check the notice in your WordPress admin."
FAILED_REASON = "The block was edited in WordPress after it was inserted, so the plugin left the post alone."

def failed_page(page, clock=False):
    if clock: page.clock.install()
    page.goto(BASE + 'digest.html?fixture=failed'); page.wait_for_selector('digest-item')
    return site_item(page, 0), site_item(page, 1)

def live_counts(page):
    return (page.text_content('[data-section=site_vm] [data-stat=live_count]'), page.text_content('[data-total=live_items] b'))

def failed_note_retry_button(page, w):
    rem, res = failed_page(page)
    assert state(rem) == 'Clicks off · box still in your post', state(rem)
    assert note(rem) == FAILED_SENTENCE + ' Clicks already stop at the link.', note(rem)
    assert rem.locator('[data-test=note-reason]').text_content() == FAILED_REASON
    box = rem.locator('[data-test=note-box]')
    assert 'caution' in box.get_attribute('class')
    btn = box.locator('button[data-action=retry]')
    assert btn.text_content() == 'Try removing again' and btn.bounding_box()['height'] >= 44, btn.bounding_box()
    assert state(res) == 'Not back in your post yet', state(res)
    assert note(res) == FAILED_SENTENCE + " Clicks stay off until it's back in your post." and res.locator('[data-test=note-reason]').is_hidden()
    assert res.locator('button[data-action=toggle]').text_content() == 'Leave it removed'
    assert rem.locator('button[data-action=toggle]').is_hidden()
    assert live_counts(page) == ('1', '4'), live_counts(page)
    assert res.locator('button[data-action=retry]').text_content() == 'Try putting it back again'
    assert res.locator('button[data-action=retry]').bounding_box()['height'] >= 44
    no_overflow(page)
    rem.scroll_into_view_if_needed(); shot(page, 'digest-failed-removal-retry', w, full=False)

def retry_failed_removal(page, w):
    page.mock.hold_retry = True; page.mock.plugin = ['removed']
    rem, _ = failed_page(page, clock=True)
    before = live_counts(page)
    btn = rem.locator('button[data-action=retry]')
    btn.click()
    for _ in range(200):
        if page.mock.held: break
        page.wait_for_timeout(25)
    assert btn.get_attribute('aria-busy') == 'true' and btn.is_disabled() and btn.text_content() == 'Trying again…'
    sty = btn.evaluate("b => { const c = getComputedStyle(b), p = getComputedStyle(b, '::before'); return [c.opacity, c.color, p.content, p.animationName] }")
    assert sty[0] == '1' and sty[1] == 'rgb(107, 69, 0)' and sty[2] == '""' and sty[3] == 'spin', sty
    assert state(rem) == 'Trying again…'
    rem.evaluate("e => { e.querySelector('[data-action=retry]').click(); e.retry(); }")  # double send attempts
    page.wait_for_timeout(100)
    assert len(page.mock.find('/retry')) == 1, page.mock.find('/retry')
    assert btn.bounding_box()['height'] >= 44
    rem.scroll_into_view_if_needed(); shot(page, 'digest-retry-in-progress', w, full=False)
    page.mock.release_retry(200, fx('retry-200'))
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:placement-result"&&e.detail.action=="retry")')
    assert len(ev(page, 'retry')) == 1 and ev(page, 'retry')[0]['detail']['failed_action'] == 'removed'
    assert state(rem) == 'Leaving your post shortly', state(rem)
    assert rem.locator('button[data-action=retry]').is_hidden() and live_counts(page) == before
    advance(page, rem, 4000, 1)
    assert state(rem) == 'Box removed' and live_counts(page) == before, (state(rem), live_counts(page))
    assert 'Live' not in state(rem)

def retry_removal_never_placed(page, w):
    page.mock.plugin = ['placed']
    rem, _ = failed_page(page, clock=True)
    before = live_counts(page)
    rem.locator('button[data-action=retry]').click()
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:placement-result"&&e.detail.action=="retry")')
    advance(page, rem, 4000, 1)
    assert state(rem) == 'Status unknown', state(rem)
    assert rem.locator('.digest-item').get_attribute('data-status') == 'undone' and live_counts(page) == before

def retry_failed_restore_422(page, w):
    page.mock.retry = (422, 'retry-422-relevance_min')
    _, res = failed_page(page)
    res.locator('button[data-action=retry]').click()
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:placement-result"&&e.detail.action=="retry")')
    # (a) expected after must-fix: the retry re-ran the hard checks and one failed. It stays removed; Retry is gone.
    assert note(res) == "Can't put it back. Relevance is below the minimum. It stays removed.", note(res)
    assert state(res) == 'Stays removed · not put back', state(res)
    assert res.locator('button[data-action=retry]').is_hidden() and res.locator('button[data-action=toggle]').is_hidden()
    assert res.locator('[data-test=note-error]').is_hidden() and not res.evaluate('e=>e.counted()')
    res.scroll_into_view_if_needed(); shot(page, 'digest-retry-422', w, full=False)

def retry_409_not_failed(page, w):
    page.mock.retry = (409, 'retry-409-not_failed'); page.mock.plugin = ['removed']
    rem, _ = failed_page(page)
    rem.locator('button[data-action=retry]').click()
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:placement-result"&&e.detail.action=="retry")')
    assert note(rem) == 'This box already changed. Showing the latest status.', note(rem)
    assert [c['path'] for c in page.mock.calls if c['method'] == 'GET'] == ['/v1/placements/plc_003']
    assert state(rem) == 'Box removed', state(rem)
    assert rem.locator('button[data-action=retry]').is_hidden()
    rem.scroll_into_view_if_needed(); shot(page, 'digest-retry-not-failed', w, full=False)

def retry_409_unknown_text(page, w):
    page.mock.retry = (409, 'synthetic/retry-409-unknown')
    rem, _ = failed_page(page)
    rem.locator('button[data-action=retry]').click()
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:placement-result"&&e.detail.action=="retry")')
    assert rem.locator('[data-test=note-error]').text_content() == "Couldn't try again. Your site key changed. Reconnect the plugin, then try again."

def restore_409_unknown_text(page, w):
    restore_case(page, w, (409, 'synthetic/restore-409-unknown'), "Can't redo. This placement is being reviewed by the maker right now. It stays undone.")

def failed_restore_not_live(page, w):
    page.mock.plugin = ['pending']
    page.clock.install()
    page.goto(BASE + 'digest.html?fixture=normal&toast_ms=30000'); page.wait_for_selector('digest-item')
    item = site_item(page)
    assert live_counts(page) == ('3', '6')
    undo(page, item); undone = live_counts(page); assert undone == ('2', '5'), undone
    page.click('#toast-redo'); page.wait_for_function('window.__ev.filter(e=>e.name=="offerrails:placement-result").length==2')
    assert live_counts(page) == undone and state(item) == 'Going back into your post', live_counts(page)
    page.mock.plugin = ['failed-restored']
    n = polls(page, item); advance(page, item, 4000, n + 1)
    assert state(item) == 'Not back in your post yet', state(item)
    assert live_counts(page) == undone, live_counts(page)
    assert note(item) == FAILED_SENTENCE + " Clicks stay off until it's back in your post."
    item.scroll_into_view_if_needed(); shot(page, 'digest-failed-restore-not-live', w, full=False)
    page.mock.plugin = ['pending']
    item.locator('button[data-action=retry]').click()
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:placement-result"&&e.detail.action=="retry")')
    assert state(item) == 'Going back into your post' and live_counts(page) == undone, live_counts(page)
    n = polls(page, item); advance(page, item, 4000, n + 1); assert live_counts(page) == undone
    page.mock.plugin = ['placed']
    n = polls(page, item); advance(page, item, 4000, n + 1)
    assert state(item).startswith('Live · ') and live_counts(page) == ('3', '6'), (state(item), live_counts(page))

def failed_restore_preview_not_live(page, w):
    rem, res = failed_page(page)
    live = site_item(page, 2)
    js = "e => { const b = e.querySelector('.di-box'), a = b.querySelector('a'), c = getComputedStyle(b); return [c.borderTopStyle, c.color, c.backgroundColor, getComputedStyle(b.querySelector('.copy')).textDecorationLine, getComputedStyle(a).color] }"
    r, x, l = res.evaluate(js), rem.evaluate(js), live.evaluate(js)
    assert l[0] == 'solid' and l[1] == 'rgb(35, 39, 32)', l
    assert r[0] == 'dashed' and r[1] == 'rgb(92, 98, 87)' and r[2] == 'rgb(253, 253, 247)' and r[4] == 'rgb(92, 98, 87)', r
    assert 'line-through' not in r[3], r
    assert x[0] == 'dashed' and x[1] == 'rgb(92, 98, 87)', x
    assert page.evaluate("getComputedStyle(document.querySelectorAll('[data-section=site_vm] digest-item')[0].querySelector('.di-box')).textDecorationLine").find('line-through') >= 0
    res.scroll_into_view_if_needed(); shot(page, 'digest-failed-restore-preview', w, full=False)

def texts(page, item_id=None, reset=False):
    t = page.evaluate('window.__texts')
    if reset: page.evaluate('window.__texts=[]')
    return [x for x in t if item_id is None or x['item'] == item_id or (x['kind'] == 'toast-text')]

def no_claims(page, item_id, removed=True, live=True, why=''):
    bad = [x for x in texts(page, item_id) if (removed and re.search(r'\bremoved\b', x['text'], re.I)) or
           (live and x['kind'] == 'state' and x['text'].startswith('Live'))]
    assert not bad, (why, bad)

def wait_result(page, n):
    page.wait_for_function('window.__ev.filter(e=>e.name=="offerrails:placement-result").length>=%d' % n)

def no_optimistic_audit(page, w):
    """Walks every action and fails if removed/placed is shown before the server confirms it."""
    page.mock.plugin = ['pending']
    rem, res = failed_page(page, clock=True)
    item2 = site_item(page, 2)                      # itm_005 / plc_005, live + placed
    texts(page, reset=True)
    # 1. undo -> pending -> removed
    item2.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    assert state(item2) == 'Leaving your post shortly'
    n = polls(page, item2); advance(page, item2, 4000, n + 1)
    no_claims(page, 'itm_005', why='undo before confirm')
    # 2. redo (restore) -> pending -> placed
    page.click('#toast-redo'); wait_result(page, 2)
    n = polls(page, item2); advance(page, item2, 4000, n + 1)
    assert state(item2) == 'Going back into your post'
    no_claims(page, 'itm_005', why='restore before confirm')
    page.mock.plugin_by['plc_005'] = 'placed'
    n = polls(page, item2); advance(page, item2, 4000, n + 1)
    assert state(item2).startswith('Live · ')
    # 3. retry a failed removal -> pending -> removed
    texts(page, reset=True)
    rem.locator('button[data-action=retry]').click(); wait_result(page, 3)
    assert state(rem) == 'Leaving your post shortly'
    n = polls(page, rem); advance(page, rem, 4000, n + 1)
    no_claims(page, 'itm_003', why='retry removal before confirm')
    page.mock.plugin_by['plc_003'] = 'removed'
    n = polls(page, rem); advance(page, rem, 4000, n + 1)
    assert state(rem) == 'Box removed'
    # 4. retry a failed restore -> pending (no Live) -> placed
    texts(page, reset=True)
    res.locator('button[data-action=retry]').click(); wait_result(page, 4)
    assert state(res) == 'Going back into your post'
    n = polls(page, res); advance(page, res, 4000, n + 1)
    no_claims(page, 'itm_004', why='retry restore before confirm')
    page.mock.plugin_by['plc_004'] = 'placed'
    n = polls(page, res); advance(page, res, 4000, n + 1)
    assert state(res).startswith('Live · ')
    # 5. pause and resume: nothing changes until the server answers
    page.mock.hold_account = True
    modes = page.eval_on_selector_all('[data-mode]', 'xs=>xs.map(x=>x.textContent)')
    page.click('#pause-all'); page.click('#confirm-pause-yes')
    for _ in range(200):
        if page.mock.held_acct: break
        page.wait_for_timeout(25)
    assert page.text_content('#confirm-pause-yes') == 'Pausing…'
    assert page.eval_on_selector_all('[data-mode]', 'xs=>xs.map(x=>x.textContent)') == modes and page.is_hidden('#paused-banner')
    page.mock.release_account(); page.wait_for_selector('#paused-banner:not([hidden])')
    page.click('#resume-all'); page.click('#confirm-resume-yes')
    for _ in range(200):
        if page.mock.held_acct: break
        page.wait_for_timeout(25)
    assert page.text_content('#confirm-resume-yes') == 'Resuming…' and page.is_visible('#paused-banner')
    page.mock.release_account(); page.wait_for_selector('#paused-banner[hidden]', state='attached')

def no_optimistic_leave(page, w):
    page.mock.plugin = ['pending']
    _, res = failed_page(page, clock=True)
    before = live_counts(page)
    texts(page, reset=True)
    res.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    assert state(res) == 'Taking it out of your post' and live_counts(page) == before
    res.scroll_into_view_if_needed(); shot(page, 'digest-leave-taking-out', w, full=False)
    n = polls(page, res); advance(page, res, 4000, n + 1)
    no_claims(page, 'itm_004', why='leave it removed before confirm')
    page.mock.plugin_by['plc_004'] = 'removed'
    n = polls(page, res); advance(page, res, 4000, n + 1)
    assert state(res) == 'Box removed' and live_counts(page) == before

def leave_then_failed_removal(page, w):
    page.mock.plugin = ['pending']
    _, res = failed_page(page, clock=True)
    res.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    page.mock.plugin_by['plc_004'] = 'failed'   # placement-failed.json: failed_action removed
    n = polls(page, res); advance(page, res, 4000, n + 1)
    assert state(res) == 'Clicks off · box still in your post', state(res)
    assert res.locator('button[data-action=toggle]').is_hidden()
    assert res.locator('button[data-action=retry]').text_content() == 'Try removing again'
    res.scroll_into_view_if_needed(); shot(page, 'digest-leave-then-failed-removal', w, full=False)

def leave_never_answered_offline(page, w):
    page.mock.plugin = ['pending']
    _, res = failed_page(page, clock=True)
    res.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    page.mock.plugin_by['plc_004'] = 'offline'  # server marks offline when no ack within 10 min
    n = polls(page, res); advance(page, res, 4000, n + 1)
    assert state(res) == 'Clicks off · site offline', state(res)
    no_claims(page, 'itm_004', why='offline is not removed')

def leave_it_removed(page, w):
    _, res = failed_page(page)
    before = live_counts(page)
    res.locator('button[data-action=toggle]').click()
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:placement-result"&&e.detail.action=="undo")')
    rv = page.mock.find('/revoke'); assert len(rv) == 1 and rv[0]['body'] == {'reason': 'undo'} and rv[0]['path'].endswith('plc_004/revoke')
    assert state(res) == 'Taking it out of your post', state(res)
    assert live_counts(page) == before and page.is_hidden('#toast')
    assert res.locator('button[data-action=toggle]').is_hidden() and res.locator('button[data-action=retry]').is_hidden()

def digest_rule_ids(page, w):
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    a, b = site_item(page, 0), site_item(page, 1)
    assert a.locator('.digest-item').get_attribute('data-policy-rule-id') == 'hard_checks_passed'
    assert b.locator('.digest-item').get_attribute('data-policy-rule-id') == 'auto:category_invoicing+relevance_40'
    assert 'passed all checks' in b.page.locator('[data-section=site_vm] digest-item [data-test=rule]').first.text_content().lower()
    assert 'invoicing is allowed and relevance 64' in b.locator('[data-test=rule]').text_content()
    assert 'auto:' not in visible_text(page)

def digest_earnings_zero(page, w):
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    t = page.locator('[data-section=site_nl] .stat').nth(3).inner_text()
    assert '€0.00' in t and 'No earnings data yet' not in t, t

def digest_pause_all(page, w):
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    page.click('#pause-all'); assert page.is_visible('#confirm-pause')
    assert ev(page, 'pause-all') == []
    shot(page, 'digest-pause-confirm', w, full=False)
    page.click('#confirm-pause-yes')
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:account-result")')
    assert [c['body'] for c in page.mock.find('/v1/account/pause')] == [{'scope': 'all'}]
    d = ev(page, 'pause-all')[0]['detail']
    assert d['mode_after'] == 'manual' and d['session_only'] is False and d['modes_before']['site_nl'] == 'auto'
    modes = page.eval_on_selector_all('[data-mode]', 'xs=>xs.map(x=>x.textContent)')
    assert modes and set(modes) == {'Ask me every time'}, modes
    assert page.is_visible('#resume-all') and 'Signed-in only' in page.inner_text('#paused-banner')

def digest_resume(page, w):
    page.mock.resume_body = fx('resume-200-claims-live')  # rows name server ids the fixture page doesn't use: fall back to the saved modes
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    page.click('#pause-all'); page.click('#confirm-pause-yes'); page.wait_for_selector('#resume-all:visible')
    page.click('#resume-all'); assert page.is_visible('#confirm-resume') and ev(page, 'resume-all') == []
    page.click('#confirm-resume-yes'); page.wait_for_selector('#pause-all:visible')
    assert [c['body'] for c in page.mock.find('/v1/account/resume')] == [{'scope': 'all'}]
    d = ev(page, 'resume-all')[0]['detail']
    assert d['session_only'] is True and d['restore_modes'] == {'prj_rail': 'yolo', 'site_vm': 'yolo', 'site_nl': 'auto'}, d
    assert page.inner_text('[data-mode=site_nl]') == 'Approve within my rules'

def digest_resume_signed_out(page, w):
    page.goto(BASE + 'digest.html?fixture=paused-signed-out'); page.wait_for_selector('digest-item')
    assert page.is_visible('#paused-banner') and page.is_disabled('#resume-all')
    assert 'Sign in to the dashboard to resume' in page.inner_text('#paused-banner')

def digest_resume_403(page, w):
    page.mock.resume = 403
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    page.click('#pause-all'); page.click('#confirm-pause-yes'); page.wait_for_selector('#resume-all:visible')
    page.click('#resume-all'); page.click('#confirm-resume-yes'); page.wait_for_selector('#kill-error:not([hidden])')
    t = page.inner_text('#kill-error')
    assert 'Resume needs you signed in to the dashboard' in t and '{' not in t, t
    assert page.is_visible('#paused-banner') and set(page.eval_on_selector_all('[data-mode]', 'xs=>xs.map(x=>x.textContent)')) == {'Ask me every time'}
    shot(page, 'digest-resume-403', w, full=False)

def digest_empty(page, w):
    page.goto(BASE + 'digest.html?fixture=empty'); page.wait_for_selector('[data-test=totals]')
    t = page.inner_text('[data-test=empty]')
    assert 'All clear.' in t and re.search(r'Your next digest arrives on Sat 17 Oct at \d\d:\d\d', t), t
    assert page.locator('[data-test=empty] button').count() == 1, 'one action'
    assert page.locator('digest-item').count() == 0
    assert len(ev(page, 'digest-opened')) == 1
    no_overflow(page); shot(page, 'digest-empty', w)

def digest_many_140(page, w):
    page.goto(BASE + 'digest.html?fixture=many'); page.wait_for_selector('digest-item')
    page.click('[data-more=site_vm]')
    n = page.locator('digest-item').count(); assert n >= 12, n
    lens = page.eval_on_selector_all('.di-box .copy', 'xs=>xs.map(x=>x.textContent.length)')
    assert max(lens) == 140, max(lens)
    bad = page.eval_on_selector_all('.di-box', 'xs=>xs.filter(x=>x.scrollWidth>x.clientWidth+1||x.getBoundingClientRect().right>innerWidth).length')
    assert bad == 0, bad
    no_overflow(page); shot(page, 'digest-many', w)

def digest_no_earnings(page, w):
    page.goto(BASE + 'digest.html?fixture=no-earnings'); page.wait_for_selector('digest-item')
    tile = page.inner_text('[data-total=earnings]')
    assert 'No earnings data yet' in tile and '€' not in tile and not re.search(r'\b0\b', tile), tile
    cells = page.eval_on_selector_all('.stat', 'xs=>xs.filter(x=>/Earnings/i.test(x.textContent)).map(x=>x.textContent)')
    assert len(cells) == 3 and all('No earnings data yet' in c and '€' not in c for c in cells), cells
    shot(page, 'digest-no-earnings', w, full=False)

def digest_a11y(page, w):
    for f in ['normal', 'empty', 'many', 'plugin-states', 'no-earnings', 'failed']:
        page.goto(BASE + 'digest.html?fixture=' + f); page.wait_for_selector('[data-test=totals]'); axe(page)
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    page.keyboard.press('Tab'); assert page.evaluate('document.activeElement.className') == 'skip'
    page.keyboard.press('Tab'); page.keyboard.press('Tab')
    assert page.evaluate('document.activeElement.id') == 'pause-all', 'kill switch should be first control after brand'
    page.click('#pause-all'); assert page.evaluate('document.activeElement.id') == 'confirm-pause-yes'
    axe(page)

# ---------- v6: real API mode against PR #21 (fb493f0), base URL /backend ----------
P3, P4, P5, P6, P7 = ['9c00000%d-0000-4000-8000-00000000000%d' % (i, i) for i in (3, 4, 5, 6, 7)]
SITE_VM, OFFER_RAIL = '5a1e0001-0000-4000-8000-00000000a001', '0ffe0001-0000-4000-8000-00000000b001'
CHANGE_URL = 'https://offerrails.example/owner'
CLAIM_MSG = 'Automatic approval is off until claim checks are live.'
SPEC = json.load(open(os.path.join(ROOT, 'tests/contract_spec.json')))

def real_digest(page, clock=False, acct=None):
    if acct: page.mock.acct = json.loads(fx(acct)) if isinstance(acct, str) else acct
    if clock: page.clock.install()
    page.goto(BASE + 'digest.html?api=real&api_base=/backend&toast_ms=30000'); page.wait_for_selector('digest-item')
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:digest-opened")')

def ritem(page, pid):
    return page.locator('digest-item:has(#w-%s)' % pid)

def wait_state(item, text, ms=5000):
    for _ in range(ms // 25):
        if state(item) == text: return
        item.page.wait_for_timeout(25)
    assert state(item) == text, (state(item), text)

def real_calls(page):
    return [(c['method'], c['path']) for c in page.mock.calls]

def placement_gets(page):
    return [c for c in page.mock.calls if c['method'] == 'GET' and '/v1/placements/' in c['path']]

def money_lines(page, sel):
    return [x.strip() for x in page.locator(sel + ' [data-money]').all_inner_texts()]

def real_digest_loads(page, w):
    real_digest(page)
    calls = real_calls(page)
    assert ('GET', '/v1/account/digest') in calls and ('GET', '/v1/digest') in calls, calls
    assert not [c for c in calls if c[1].endswith('/mode')], 'paused comes with the digest: no mode reads'
    assert placement_gets(page) == [], 'plugin_state and live_now come with the digest: no status read on first load'
    for _ in range(100):
        if page.mock.find('/v1/events'): break
        page.wait_for_timeout(25)
    ev_ = page.mock.find('/v1/events'); assert len(ev_) == 1 and ev_[0]['body'] == {'kind': 'digest_opened'}, ev_
    assert calls.index(('GET', '/v1/digest')) < [c['path'] for c in page.mock.calls].index('/v1/events'), 'since read before the open is recorded'
    assert all(c['url'].startswith(BASE + 'backend/v1/') for c in page.mock.calls), 'every call goes to the configured base'
    # numbers are the server's
    assert page.text_content('[data-total=live_items] b') == '3'
    assert page.text_content('[data-total=live_items] small') == '1 item went live since last digest'
    assert page.text_content('[data-section="%s"] [data-stat=live_count]' % SITE_VM) == '2'
    assert page.text_content('[data-section="%s"] [data-stat=live_count]' % OFFER_RAIL) == '1'
    counted = page.eval_on_selector_all('[data-section="%s"] digest-item' % SITE_VM, 'xs=>xs.filter(x=>x.counted()).map(x=>x.item.item_id)')
    assert counted == [P3, P4], ('Live now = live_now=true only', counted)
    # money: one line per currency, never added; offer = commission to pay, not income; no maker_commission tile
    assert money_lines(page, '[data-total=earnings]') == ['€9.50', 'US$3.00']
    site = '[data-section="%s"] [data-stat-earnings]' % SITE_VM; off = '[data-section="%s"] [data-stat-earnings]' % OFFER_RAIL
    assert money_lines(page, site) == ['€9.50', 'US$3.00'] and page.get_attribute(site, 'data-stat-earnings') == 'income'
    assert money_lines(page, off) == ['€4.00'] and page.get_attribute(off, 'data-stat-earnings') == 'commission'
    assert 'Commission to pay' in page.text_content(off) and 'Not income' in page.text_content(off)
    assert page.locator('[data-total=maker_commission]').count() == 0
    main = page.inner_text('main'); assert '12.50' not in main and '13.50' not in main, 'no sum across currencies or roles'
    # item fields
    i3 = ritem(page, P3)
    where = ' '.join(i3.locator('.di-where').inner_text().split())
    assert where == 'victormotricala.it · “Come fatturare da freelance nel 2026” · Matching section', where
    assert i3.locator('[data-preview]').text_content() == 'Prova Fattura Lampo' and i3.locator('.clicks').text_content() == '23 clicks'
    assert ' '.join(i3.locator('[data-test=rule]').inner_text().split()) == 'Approved by your rule: The category is invoicing. · rules v3'
    i4 = ritem(page, P4)
    assert i4.locator('.label').text_content() == 'Pubblicità' and i4.locator('.clicks').text_content() == '14 clicks'
    assert i4.locator('[data-test=rule]').inner_text().strip() == 'Approved by you' and 'End of article' in i4.locator('.di-where').inner_text()
    assert state(i3).startswith('Live · ') and state(i4).startswith('Live · ')
    i5 = ritem(page, P5)
    assert state(i5) == 'Waiting for your post to update' and i5.locator('button[data-action=toggle]').is_hidden()
    assert 'After the intro' in i5.locator('.di-where').inner_text()
    i6 = ritem(page, P6)
    assert state(i6) == 'Clicks off · box still in your post' and note(i6).endswith('Clicks already stop at the link.')
    i7 = ritem(page, P7)
    assert state(i7) == 'Not back in your post yet' and i7.locator('[data-test=note-reason]').is_hidden(), 'empty reason string shows nothing'
    assert i7.locator('button[data-action=toggle]').text_content() == 'Leave it removed'
    lst = ritem(page, OFFER_RAIL)
    assert state(lst).startswith('Listed · ') and lst.locator('button[data-action=toggle]').is_hidden()
    # envelope: change_url, site_url, approval reason under the stub
    hrefs = page.eval_on_selector_all('[data-test=change]', 'xs=>xs.map(x=>x.getAttribute("href"))')
    assert hrefs == [CHANGE_URL, CHANGE_URL], hrefs
    assert page.text_content('[data-section="%s"] [data-test=site-url]' % SITE_VM) == 'victormotricala.it'
    assert CLAIM_MSG in page.text_content('[data-test=approval-note]')
    assert page.eval_on_selector_all('[data-test=mode-why]', 'xs=>xs.map(x=>x.textContent)') == [CLAIM_MSG, CLAIM_MSG]
    assert page.locator('[data-test=key-note]').count() == 0, 'signed in: no key line'
    assert page.locator('[data-test=page-note]').count() == 0
    assert page.locator('[data-test=section-paused]').count() == 0 and page.is_hidden('#paused-banner')
    assert page.locator('[data-test=expires]').count() == 0, '(e) committed/placed boxes never show an expiry'
    no_overflow(page); axe(page)
    shot(page, 'real-digest', w)

def real_pending_resolves_by_poll(page, w):
    page.mock.plugin_by[P5] = 'removed'
    real_digest(page, clock=True)
    i5 = ritem(page, P5)
    assert state(i5) == 'Waiting for your post to update' and placement_gets(page) == []
    no_claims(page, P5, why='pending digest item before the poll answers')
    i5.scroll_into_view_if_needed(); shot(page, 'real-pending-waiting', w, full=False)
    advance(page, i5, 4000, 1)
    assert [c['path'] for c in placement_gets(page)] == ['/v1/placements/' + P5], 'only the pending item is polled, on the 4 s schedule'
    assert state(i5) == 'Box removed', state(i5)

def real_undo_redo(page, w):
    real_digest(page, clock=True)
    i3 = ritem(page, P3)
    i3.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    rv = page.mock.find('/%s/revoke' % P3)
    assert len(rv) == 1 and rv[0]['body'] == {'reason': 'undo'} and rv[0]['url'] == BASE + 'backend/v1/placements/%s/revoke' % P3
    assert state(i3) == 'Leaving your post shortly'
    page.mock.acct['projects'][0]['went_live'] = 1; page.mock.acct['totals']['went_live'] = 9
    page.clock.run_for(500)
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:numbers-refreshed")')
    assert page.text_content('[data-total=live_items] b') == '9' and page.text_content('[data-section="%s"] [data-stat=live_count]' % SITE_VM) == '1'
    page.click('#toast-redo'); wait_result(page, 2)
    assert len(page.mock.find('/%s/restore' % P3)) == 1
    assert state(i3) == 'Going back into your post', state(i3)
    no_claims(page, P3, removed=True, live=True, why='real undo/redo before confirmation')
    page.mock.plugin_by[P3] = 'placed'
    n = polls(page, i3); advance(page, i3, 4000, n + 1)
    assert state(i3).startswith('Live · ')
    i3.scroll_into_view_if_needed(); shot(page, 'real-redo-placed', w, full=False)

def real_undone_by(page, w):
    page.mock.revoke_body = json.dumps(dict(json.loads(fx('revoke-200')), undone_by='agent'))
    real_digest(page)
    i3 = ritem(page, P3); i4 = ritem(page, P4)
    assert i3.locator('[data-test=undone-by]').is_hidden(), '(d) nothing before an undo'
    i3.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    assert i3.locator('[data-test=undone-by]').text_content() == 'Undone by your agent'
    page.mock.revoke_body = None
    i4.locator('button[data-action=toggle]').click(); wait_result(page, 2)
    assert i4.locator('[data-test=undone-by]').is_hidden(), '(d) field absent: nothing shown'

def real_poll_429(page, w):
    page.mock.plugin_by[P3] = '429'
    real_digest(page, clock=True)
    i3 = ritem(page, P3)
    i3.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    advance(page, i3, 4000, 1); advance(page, i3, 4000, 2)
    assert state(i3) == 'Leaving your post shortly' and note(i3) == '', (state(i3), note(i3))
    page.mock.plugin_by[P3] = 'removed'
    advance(page, i3, 4000, 3)
    assert state(i3) == 'Box removed'

def real_restore_409_not_undone(page, w):
    page.mock.restore = (409, 'restore-409-not-undone')
    real_digest(page)
    i3 = ritem(page, P3)
    i3.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    page.mock.plugin_by[P3] = 'placed'
    page.click('#toast-redo'); wait_result(page, 2)
    assert page.text_content('#toast-text') == "Can't redo. The placement was not undone. Showing the latest status."
    r = [e['detail'] for e in ev(page, 'placement-result') if e['detail']['action'] == 'restore'][0]
    assert r['outcome'] == 'not_undone' and r['reason'] == 'not_undone'
    for _ in range(200):
        if state(i3).startswith('Live · '): break
        page.wait_for_timeout(25)
    assert state(i3).startswith('Live · '), ('resynced from GET /v1/placements/:id', state(i3))
    assert ('GET', '/v1/placements/' + P3) in real_calls(page)

def real_restore_409_server_message(page, w):
    page.mock.restore = (409, 'restore-409-paused')
    real_digest(page)
    ritem(page, P3).locator('button[data-action=toggle]').click(); wait_result(page, 1)
    page.click('#toast-redo'); wait_result(page, 2)
    assert page.text_content('#toast-text') == "Can't redo. Pause or the kill switch was turned on after the undo. It stays undone."

def real_failed_flow(page, w):
    page.mock.plugin_by[P7] = 'pending'
    real_digest(page, clock=True)
    i7 = ritem(page, P7)
    i7.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    assert page.mock.find('/%s/revoke' % P7)[0]['body'] == {'reason': 'undo'}
    assert state(i7) == 'Taking it out of your post'
    page.mock.plugin_by[P7] = 'failed'   # failed_action removed
    n = polls(page, i7); advance(page, i7, 4000, n + 1)
    assert state(i7) == 'Clicks off · box still in your post' and i7.locator('button[data-action=toggle]').is_hidden()
    assert i7.locator('button[data-action=retry]').text_content() == 'Try removing again'
    i6 = ritem(page, P6); page.mock.plugin_by[P6] = 'pending'
    i6.locator('button[data-action=retry]').click(); wait_result(page, 2)
    assert state(i6) == 'Leaving your post shortly' and i6.locator('[data-test=note-reason]').is_hidden()

def real_retry_restore_422(page, w):
    """(a) expected after must-fix: retrying a failed restore re-runs the hard checks; a 422 leaves it removed, no Retry."""
    page.mock.retry = (422, 'retry-422-relevance_min')
    real_digest(page)
    i7 = ritem(page, P7)
    assert i7.locator('button[data-action=retry]').text_content() == 'Try putting it back again'
    i7.locator('button[data-action=retry]').click()
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:placement-result"&&e.detail.action=="retry")')
    r = [e['detail'] for e in ev(page, 'placement-result') if e['detail']['action'] == 'retry'][0]
    assert r['http_status'] == 422 and r['outcome'] == 'check_failed' and r['reason'] == 'relevance_min', r
    assert note(i7) == "Can't put it back. Relevance is below the minimum. It stays removed.", note(i7)
    assert state(i7) == 'Stays removed · not put back', state(i7)
    assert i7.locator('button[data-action=retry]').is_hidden() and i7.locator('button[data-action=toggle]').is_hidden()
    assert not i7.evaluate('e=>e.counted()') and i7.locator('.digest-item').get_attribute('data-status') == 'undone'
    assert len(page.mock.find('/%s/retry' % P7)) == 1
    i7.scroll_into_view_if_needed(); axe(page); shot(page, 'real-retry-restore-422', w, full=False)

def real_maker_no_retry(page, w):
    acct = json.loads(fx('account-digest'))
    failed = dict(next(i for i in acct['projects'][0]['items'] if i['id'] == P6), id='9c000008-0000-4000-8000-000000000008')
    acct['projects'][1]['items'].append(failed)
    real_digest(page, acct=acct)
    m = ritem(page, failed['id'])
    assert m.locator('button[data-action=retry]').is_hidden(), 'retry is 403 for a maker key: no Retry for makers'
    assert 'Only the publisher can try again.' in note(m) and note(m).startswith('Their site')
    assert ritem(page, P6).locator('button[data-action=retry]').is_visible(), 'publisher still gets Retry'
    m.scroll_into_view_if_needed(); shot(page, 'real-maker-no-retry', w, full=False)

def real_key_context(page, w):
    """(c) expected after must-fix: keys can undo/pause; restoring and deciding need the dashboard."""
    acct = json.loads(fx('account-digest')); acct['session'] = {'signed_in': False}
    real_digest(page, acct=acct)
    assert 'need the dashboard' in page.text_content('[data-test=key-note]')
    i7 = ritem(page, P7)
    assert i7.locator('button[data-action=retry]').is_hidden(), 'retry of a failed restore = restoring: session only'
    assert 'Putting it back needs the dashboard.' in note(i7)
    assert i7.locator('button[data-action=toggle]').text_content() == 'Leave it removed' and i7.locator('button[data-action=toggle]').is_visible(), 'undo stays'
    assert ritem(page, P6).locator('button[data-action=retry]').is_visible(), 'retrying a removal is an undo: allowed'
    i3 = ritem(page, P3)
    i3.locator('button[data-action=toggle]').click(); wait_result(page, 1)
    assert len(page.mock.find('/%s/revoke' % P3)) == 1, 'undo works with a key'
    assert page.is_hidden('#toast-redo') and 'Redo needs the dashboard.' in page.text_content('#toast-text')
    assert i3.locator('button[data-action=toggle]').is_hidden() and 'Putting it back needs the dashboard.' in note(i3)
    i3.evaluate('e=>e.restore()'); page.wait_for_timeout(100)
    assert page.mock.find('/restore') == [], 'no restore call without a session'
    assert page.is_enabled('#pause-all'), 'pause and the kill switch stay available'
    page.click('#pause-all'); page.click('#confirm-pause-yes'); page.wait_for_selector('#resume-all:visible')
    assert page.is_disabled('#resume-all') and len(page.mock.find('/v1/account/pause')) == 1
    no_overflow(page); axe(page); shot(page, 'real-key-context', w)

def real_pause_resume(page, w):
    real_digest(page, acct='account-digest-claims-live'); page.mock.resume_body = fx('resume-200-claims-live')
    assert page.text_content('[data-mode="%s"]' % SITE_VM) == 'Approve within my rules'
    assert page.locator('[data-test=approval-note]').count() == 0 and page.locator('[data-test=mode-why]').count() == 0
    page.mock.acct['totals']['went_live'] = 0
    for p_ in page.mock.acct['projects']: p_['went_live'] = 0
    page.click('#pause-all'); page.click('#confirm-pause-yes'); page.wait_for_selector('#resume-all:visible')
    assert page.mock.find('/v1/account/pause')[0]['body'] == {'scope': 'all'}
    assert set(page.eval_on_selector_all('[data-mode]', 'xs=>xs.map(x=>x.textContent)')) == {'Ask me every time'}
    page.wait_for_function('window.__ev.some(e=>e.name=="offerrails:numbers-refreshed")')
    assert page.text_content('[data-total=live_items] b') == '0', 'paused: no go link redirects, nothing is live'
    page.click('#resume-all'); page.click('#confirm-resume-yes'); page.wait_for_selector('#pause-all:visible')
    assert page.text_content('[data-mode="%s"]' % SITE_VM) == 'Approve within my rules', 'mode from applied[]'
    assert page.text_content('[data-mode="%s"]' % OFFER_RAIL) == 'Approve anything that passes the checks'
    shot(page, 'real-resumed-modes', w, full=False)

def real_resume_under_stub(page, w):
    real_digest(page)
    page.click('#pause-all'); page.click('#confirm-pause-yes'); page.wait_for_selector('#resume-all:visible')
    page.click('#resume-all'); page.click('#confirm-resume-yes'); page.wait_for_selector('#pause-all:visible')
    assert set(page.eval_on_selector_all('[data-mode]', 'xs=>xs.map(x=>x.textContent)')) == {'Ask me every time'}, 'stub: effective mode stays manual'

def real_paused_envelope(page, w):
    real_digest(page, acct='account-digest-paused')
    assert page.is_visible('#paused-banner') and page.is_hidden('#kill')
    assert page.text_content('[data-total=live_items] b') == '0'
    real_digest(page, acct='account-digest-site-paused')
    assert page.is_hidden('#paused-banner') and page.is_visible('#pause-all')
    assert page.locator('[data-section="%s"] [data-test=section-paused]' % SITE_VM).text_content() == 'Paused'
    assert page.locator('[data-section="%s"] [data-test=section-paused]' % OFFER_RAIL).count() == 0
    axe(page); shot(page, 'real-site-paused', w, full=False)

def real_earnings_zero_null(page, w):
    real_digest(page, acct='account-digest-zero')
    for sel in ['[data-total=earnings]', '[data-section="%s"] [data-stat-earnings]' % SITE_VM, '[data-section="%s"] [data-stat-earnings]' % OFFER_RAIL]:
        t = page.text_content(sel); assert 'No earnings data yet' in t and '€' not in t, (sel, t)
    real_digest(page, acct='account-digest-zero-amount')
    assert money_lines(page, '[data-total=earnings]') == ['€0.00']
    assert money_lines(page, '[data-section="%s"] [data-stat-earnings]' % SITE_VM) == ['€0.00']
    assert 'No earnings data yet' in page.text_content('[data-section="%s"] [data-stat-earnings]' % OFFER_RAIL)
    acct = json.loads(fx('account-digest')); acct['totals']['earnings'] = []
    real_digest(page, acct=acct)
    assert 'No earnings data yet' in page.text_content('[data-total=earnings]'), 'empty list = no earnings data'
    axe(page)

def real_long_list(page, w):
    real_digest(page, acct='account-digest-long')
    sec = '[data-section="%s"]' % SITE_VM
    assert page.text_content(sec + ' [data-stat=live_count]') == '128' and page.text_content('[data-total=live_items] b') == '129'
    assert page.locator(sec + ' li:not([hidden]) digest-item').count() == 6
    assert ' '.join(page.inner_text(sec + ' [data-test=page-note]').split()) == 'Showing the latest 50 of 212. See them all in the dashboard'
    page.click(sec + ' [data-more]'); assert page.locator(sec + ' li:not([hidden]) digest-item').count() == 50
    assert placement_gets(page) == [], 'no per-item status reads'
    no_overflow(page); axe(page); shot(page, 'real-long-list', w)

def real_api_base_guard(page, w):
    page.goto(BASE + 'digest.html?api=real&api_base=https://evil.example/x'); page.wait_for_load_state('domcontentloaded')
    assert page.evaluate('window.OfferRails.apiBase') == ''
    page.goto(BASE + 'digest.html?api=real&api_base=//evil.example'); page.wait_for_load_state('domcontentloaded')
    assert page.evaluate('window.OfferRails.apiBase') == ''
    page.goto(BASE + 'digest.html?api=real&api_base=http://127.0.0.1:8787/api'); page.wait_for_load_state('domcontentloaded')
    assert page.evaluate('window.OfferRails.apiBase') == 'http://127.0.0.1:8787/api'

def real_setup(page, name, role=''):
    page.mock.setup_get = name
    page.goto(BASE + 'setup.html?api=real&api_base=/backend' + ('&role=' + role if role else '')); page.wait_for_selector('#setup-form')
    page.wait_for_selector('section.checks[aria-busy=false]')

def real_setup_hard_checks(page, w):
    real_setup(page, 'setup-settings-publisher')
    want = [c['check_text'] for c in json.loads(fx('hard-checks'))['checks']]
    assert page.locator('[data-test=hard-checks] li').all_inner_texts() == want, 'rendered exactly as GET /v1/policy/hard-checks returns'
    assert ('GET', '/v1/policy/hard-checks') in real_calls(page)
    page.mock.hard = (200, json.dumps({'checks': [{'check_id': 'x_new', 'check_text': 'A brand-new check.'}]}))
    page.reload(); page.wait_for_selector('section.checks[aria-busy=false]')
    assert page.locator('[data-test=hard-checks] li').all_inner_texts() == ['A brand-new check.'], 'no hardcoded list'
    page.mock.hard = (500, '{}')
    page.reload(); page.wait_for_selector('section.checks[aria-busy=false]')
    assert page.locator('[data-test=hard-checks] li').count() == 0 and page.is_visible('[data-test=hard-checks-error]')
    axe(page)

def real_setup_publisher(page, w):
    real_setup(page, 'setup-settings-publisher')
    assert page.mock.find('/v1/setup/settings')[0]['method'] == 'GET'
    assert page.is_checked('input[name=mode][value=manual]') and 'Suggested' in page.text_content('[data-mode-choice=manual]')
    assert page.is_checked('input[name=position][value=end]') and page.input_value('#never-topics') == 'gambling, crypto'
    axe(page); shot(page, 'real-setup-publisher', w)
    page.click('input[name=position][value=matched_section]')
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    b = page.mock.put_bodies
    assert len(b) == 1 and sorted(b[0]) == sorted(SPEC['setup_put_keys']), b
    assert b[0]['role'] == 'publisher' and b[0]['install_path'] == 'claude_dir' and b[0]['mode'] == 'manual' and b[0]['mode_shown'] == 'manual'
    assert b[0]['box_position'] == 'matched_section' and b[0]['sites_channels'] == ['victormotricala.it', 'newsletter.victormotricala.it']
    assert b[0]['commission'] is None and b[0]['listing_price'] is None and b[0]['project_name'] == ''

def real_setup_stub_modes(page, w):
    """Maker, stub on: mode=manual + mode_preselected=yolo + approval_reason. v9 (design offer-rails.md:147): YOLO is picked and waits
    for the first approved placement; auto can't be picked (reason = approval_message)."""
    real_setup(page, 'setup-settings-maker', 'maker')
    assert page.is_disabled('input[name=mode][value=auto]') and page.inner_text('#why-auto') == CLAIM_MSG
    assert page.get_attribute('input[name=mode][value=auto]', 'aria-describedby') == 'why-auto'
    assert page.is_checked('input[name=mode][value=yolo]'), 'YOLO shown as picked, never as a lost choice'
    assert page.locator('#why-yolo').count() == 0 and page.locator('#why-manual').count() == 0
    assert not page.is_checked('input[name=mode][value=manual]')
    axe(page); shot(page, 'real-setup-stub-modes', w)

def real_setup_maker_auto_off(page, w):
    """(b) expected after must-fix: while the claim check is a stub a maker can't auto-accept publishers."""
    s = dict(json.loads(fx('setup-settings-maker-fixed')), approval_reason='claim_check_unavailable', approval_message=CLAIM_MSG)
    page.route('**/backend/v1/setup/settings*', lambda r: r.fulfill(status=200, content_type='application/json', body=json.dumps(s)) if r.request.method == 'GET' else r.fallback())
    real_setup(page, 'setup-settings-maker-fixed', 'maker')
    assert page.is_checked('input[name=role][value=maker]')
    assert page.is_disabled('input[name=mode][value=auto]') and page.inner_text('#why-auto') == CLAIM_MSG
    assert page.is_checked('input[name=mode][value=yolo]') and CLAIM_MSG in page.inner_text('[data-test=mode-pending]')
    assert 'publishers get approved' in page.inner_text('fieldset[data-q=mode] legend')
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    assert page.mock.put_bodies[0]['mode'] == 'yolo', 'the owner pick is sent back as is; the server holds it until the first approved placement'

def real_setup_maker_put(page, w):
    real_setup(page, 'setup-settings-maker', 'maker')
    f = page.mock.find('/v1/setup/settings'); assert f[0]['url'].endswith('/v1/setup/settings?role=maker')
    assert page.input_value('#commission-value') == '20' and page.input_value('#commission-type') == 'percent'
    assert page.input_value('#listing-name') == 'Rail Tool' and page.input_value('#listing-price') == '19'
    assert page.locator('#avoid-types').count() == 0, 'no server field for publisher types'
    page.check('#also-role'); page.click('#continue'); page.wait_for_selector('[data-test=success]')
    b = page.mock.put_bodies
    assert [x['role'] for x in b] == ['maker', 'publisher'] and all(sorted(x) == sorted(SPEC['setup_put_keys']) for x in b), b
    m = b[0]
    assert m['project_name'] == 'Rail Tool' and m['promote'] == 'Ship a referral offer for your dev tool in one afternoon.'
    assert m['listing_url'] == 'https://railtool.example' and m['commission'] == {'type': 'percent', 'value': 20, 'currency': ''}
    assert m['listing_price'] == {'value': 19, 'currency': 'EUR'} and m['avoid_topics'] == ['gambling', 'adult']
    assert m['install_path'] == 'cli' and m['mode'] == 'yolo' and m['mode_shown'] == 'yolo'
    assert b[1]['commission'] is None and b[1]['listing_price'] is None

def real_setup_maker_fixed(page, w):
    real_setup(page, 'setup-settings-maker-fixed', 'maker')
    assert page.input_value('#commission-type') == 'fixed' and page.input_value('#commission-value') == '15'
    assert page.input_value('#listing-price') == '', 'listing_price null: empty, optional'
    assert page.is_checked('input[name=mode][value=yolo]') and 'Suggested' in page.text_content('[data-mode-choice=yolo]')
    page.select_option('#commission-currency', 'USD')
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    m = page.mock.put_bodies[0]
    assert m['commission'] == {'type': 'fixed', 'value': 15, 'currency': 'USD'} and m['listing_price'] is None, m

def real_setup_claims_live(page, w):
    real_setup(page, 'setup-settings-claims-live')
    assert page.is_enabled('input[name=mode][value=yolo]') and page.is_checked('input[name=mode][value=yolo]')
    assert page.locator('[data-test=mode-unavailable]').count() == 0 and 'Suggested' in page.text_content('[data-mode-choice=yolo]')

def real_setup_source_manual(page, w):
    real_setup(page, 'setup-settings-source-manual')
    assert page.is_checked('input[name=mode][value=manual]') and 'Suggested' in page.text_content('[data-mode-choice=manual]')
    assert 'Suggested' not in page.text_content('[data-mode-choice=yolo]'), 'install path cli would give yolo if we guessed'

def real_setup_new(page, w):
    real_setup(page, 'setup-settings-new')
    # v9 (design offer-rails.md:147): manual + mode_preselected yolo = YOLO picked, on after the first approved placement
    assert page.is_checked('input[name=mode][value=yolo]') and page.is_enabled('input[name=mode][value=yolo]')
    assert page.locator('#why-yolo').count() == 0 and page.inner_text('#why-auto') == CLAIM_MSG

def real_setup_key_context(page, w):
    """(c) expected after must-fix: opened with a key, saving needs the dashboard."""
    s = dict(json.loads(fx('setup-settings-publisher')), session={'signed_in': False})
    page.route('**/backend/v1/setup/settings*', lambda r: r.fulfill(status=200, content_type='application/json', body=json.dumps(s)))
    real_setup(page, 'setup-settings-publisher')
    assert page.is_disabled('#continue') and 'needs the dashboard' in page.text_content('[data-test=key-note]')
    page.evaluate("document.getElementById('setup-form').requestSubmit()"); page.wait_for_timeout(100)
    assert page.mock.put_bodies == [] and ev(page, 'setup-submit') == []
    axe(page); shot(page, 'real-setup-key-context', w)

def real_setup_403(page, w):
    page.mock.setup_put = (403, 'setup-403')
    real_setup(page, 'setup-settings-publisher')
    page.click('#continue'); page.wait_for_selector('#save-error:not([hidden])')
    assert page.is_visible('#setup-form') and '{' not in page.inner_text('#save-error')

def totals_grid_layout(page, w):
    """Any number of tiles: every row filled edge to edge, no empty grey cell."""
    real_digest(page)
    def check(n):
        page.evaluate("""n => { const t = document.querySelector('[data-test=totals]'); const tiles = [...t.children];
          while (t.children.length < n) t.appendChild(tiles[t.children.length % tiles.length].cloneNode(true));
          while (t.children.length > n) t.lastElementChild.remove(); }""", n)
        rows = page.evaluate("""() => { const t = document.querySelector('[data-test=totals]'); const inner = t.clientWidth; const rows = {};
          for (const c of t.children) { const r = c.getBoundingClientRect(); (rows[Math.round(r.top)] = rows[Math.round(r.top)] || []).push(r.width); }
          return { inner, rows: Object.values(rows).map(ws => ws.reduce((a, b) => a + b, 0)), bg: getComputedStyle(t).backgroundColor,
                   panel: getComputedStyle(t.children[0]).backgroundColor }; }""")
        for total in rows['rows']: assert abs(total - rows['inner']) <= 1.5, (n, rows)
        assert rows['bg'] == rows['panel'], 'no grey filler behind the tiles'
    for n in (4, 5, 6, 7, 8):
        check(n)
        if n == 6: page.locator('[data-test=totals]').screenshot(path=os.path.join(PROOF, 'totals-6-tiles-%d.png' % w))
    check(5); no_overflow(page)
    page.locator('[data-test=totals]').screenshot(path=os.path.join(PROOF, 'totals-5-tiles-%d.png' % w))

# ---------------- v7 design system (DESIGN.md) ----------------
def active(page):
    return page.evaluate("(()=>{const a=document.activeElement;const d=a&&a.closest('digest-item');return {tag:a.tagName,id:a.id,test:a.dataset?a.dataset.test:null,item:d&&d.item?(d.item.id||d.item.placement_id||d.item.rule_id||''):null,cls:a.className,inOverlay:!!a.closest('[data-overlay]')}})()")

def focused_item(page):
    return page.evaluate("(()=>{const a=document.activeElement;return a&&a.classList.contains('digest-item')?[...document.querySelectorAll('digest-item')].indexOf(a.closest('digest-item')):-1})()")

def no_writes(page):
    return [c for c in page.mock.calls if c['method'] != 'GET' and not c['path'].startswith('/v1/events')]

def trap(page, sel, n=12):
    for _ in range(n):
        page.keyboard.press('Tab'); assert page.evaluate("s=>!!document.activeElement.closest(s)", sel), 'focus left ' + sel
    for _ in range(4):
        page.keyboard.press('Shift+Tab'); assert page.evaluate("s=>!!document.activeElement.closest(s)", sel), 'focus left ' + sel

def kb_list_navigation(page, w):
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    page.keyboard.press('j'); assert focused_item(page) == 0
    page.keyboard.press('j'); assert focused_item(page) == 1
    page.keyboard.press('k'); assert focused_item(page) == 0
    page.keyboard.press('k'); assert focused_item(page) == 0, 'stays on the first item'
    ring = page.evaluate("getComputedStyle(document.activeElement).outlineStyle + ' ' + getComputedStyle(document.activeElement).outlineWidth")
    assert ring.startswith('solid 3px'), ring
    page.keyboard.press('j'); shot(page, 'v7-focus-ring', w, full=False)
    assert no_writes(page) == []

def kb_guards(page, w):
    """R only where an Undo / Leave it removed button is showing; A never calls; typing is never intercepted."""
    real_digest(page)
    page.locator('#w-%s' % P3).locator('xpath=ancestor::article[1]').focus() if False else None
    page.evaluate("id=>document.querySelector('digest-item:has(#w-'+id+') .digest-item').focus()", P3)
    page.keyboard.press('a'); page.wait_for_timeout(120)
    assert no_writes(page) == [] and 'Approving happens in the dashboard' in page.text_content('#digest-status')
    page.keyboard.press('r'); wait_result(page, 1)
    assert len(page.mock.find('/%s/revoke' % P3)) == 1 and ev(page, 'undo')[0]['detail'].get('placement_id', P3)
    page.evaluate("id=>document.querySelector('digest-item:has(#w-'+id+') .digest-item').focus()", P3)
    page.keyboard.press('r'); page.wait_for_timeout(200)
    assert page.mock.find('/restore') == [] and len(page.mock.find('/revoke')) == 1, 'R never redoes'
    # a listing: R does nothing
    page.evaluate("()=>[...document.querySelectorAll('digest-item')].find(d=>d.isListing()).querySelector('.digest-item').focus()")
    page.keyboard.press('r'); page.wait_for_timeout(150)
    assert len(no_writes(page)) == 1 and 'Listings can' in page.text_content('#digest-status')
    # typing in the palette input never moves or acts
    before = focused_item(page)
    page.keyboard.press('Control+k'); page.wait_for_selector('[data-test=palette-input]')
    page.keyboard.type('jkra?')
    assert page.input_value('[data-test=palette-input]') == 'jkra?' and page.locator('[data-test=help]').count() == 0
    page.keyboard.press('Escape'); assert len(no_writes(page)) == 1

def kb_key_context(page, w):
    """(c) with a key: R undoes (allowed); the failed restore offers 'Leave it removed' only; never restores."""
    acct = json.loads(fx('account-digest')); acct['session'] = {'signed_in': False}
    real_digest(page, acct=acct)
    page.evaluate("id=>document.querySelector('digest-item:has(#w-'+id+') .digest-item').focus()", P7)
    page.keyboard.press('r'); page.wait_for_timeout(400)
    assert page.mock.find('/restore') == [] and page.mock.find('/%s/retry' % P7) == [], 'restoring stays session-only'
    page.evaluate("id=>document.querySelector('digest-item:has(#w-'+id+') .digest-item').focus()", P3)
    page.keyboard.press('r'); wait_result(page, 1)
    assert len(page.mock.find('/%s/revoke' % P3)) == 1
    page.keyboard.press('Control+k'); page.wait_for_selector('[data-test=palette]')
    page.keyboard.type('resume')
    assert page.locator('[data-test=palette-list] li').count() in (0, 1)

def kb_typing_setup(page, w):
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#setup-form')
    f = page.locator('#setup-form input[type=text]:visible, #setup-form textarea:visible').first
    f.click(); page.keyboard.type('jk?ar')
    assert 'jk?ar' in f.input_value() and page.locator('[data-overlay]').count() == 0
    assert page.mock.put_bodies == [] and ev(page, 'setup-submit') == []

def palette_open_run_close(page, w):
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    page.keyboard.press('j'); opener = active(page)
    page.keyboard.press('Control+k'); page.wait_for_selector('[data-test=palette]')
    assert active(page)['test'] == 'palette-input'
    rows = page.locator('[data-test=palette-list] li')
    txt = page.inner_text('[data-test=palette-list]')
    for k in ('J', 'K', 'R', 'A', '?'):
        assert page.locator('[data-test=palette-list] kbd', has_text=k).count() >= 1, k
    assert page.get_attribute('li[data-cmd=approve]', 'aria-disabled') == 'true' and 'Approving happens in the dashboard' in page.inner_text('li[data-cmd=approve]')
    axe(page); shot(page, 'v7-palette', w, full=False)
    page.keyboard.press('Escape'); page.wait_for_selector('[data-test=palette]', state='detached')
    assert active(page) == opener, 'focus returns'
    page.keyboard.press('Meta+k'); page.wait_for_selector('[data-test=palette]')
    page.keyboard.type('pause'); assert rows.count() == 1
    page.keyboard.press('Enter'); page.wait_for_selector('[data-test=palette]', state='detached')
    page.wait_for_selector('#confirm-pause:not([hidden])')
    assert page.mock.find('/v1/account/pause') == [], 'the confirm still guards pausing'
    page.keyboard.press('Control+k'); page.keyboard.type('zzzz'); page.wait_for_selector('[data-test=palette-empty]')
    page.keyboard.press('Escape')
    page.keyboard.press('?'); page.wait_for_selector('[data-test=help]')
    h = page.inner_text('[data-test=help-list]')
    assert 'Next item' in h and 'Approve' in h and 'dashboard' in h
    trap(page, '[data-test=help]'); page.keyboard.press('Escape'); page.wait_for_selector('[data-test=help]', state='detached')

def panel_focus_trap(page, w):
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    page.keyboard.press('j'); opener = active(page)
    page.keyboard.press('Enter'); page.wait_for_selector('[data-test=panel]')
    p = page.locator('[data-test=panel]')
    assert p.get_attribute('role') == 'dialog' and p.get_attribute('aria-modal') == 'true'
    assert page.evaluate("getComputedStyle(document.querySelector('[data-test=panel] .overlay-body')||document.querySelector('[data-test=panel]')).overflowY") in ('auto', 'scroll')
    assert p.locator('a[href]').count() == 0, 'box preview never links'
    assert p.locator('[data-money]').count() == 0 and p.locator('[data-test=blip]').count() == 0
    assert 'Status' in p.locator('[data-test=panel-facts]').text_content()
    box = p.bounding_box()
    if w == 390: assert box['x'] <= 0.5 and box['width'] >= 389, box
    else: assert box['width'] <= 480 and box['x'] + box['width'] >= 1439, box
    page.wait_for_timeout(350); axe(page); shot(page, 'v7-panel', w, full=False)
    trap(page, '[data-test=panel]')
    page.keyboard.press('Escape'); page.wait_for_selector('[data-test=panel]', state='detached')
    assert active(page) == opener
    page.locator('[data-action=details]').first.click(); page.wait_for_selector('[data-test=panel]')
    page.keyboard.press('Escape'); assert active(page)['cls'].find('di-open') >= 0
    assert no_writes(page) == []

def modal_focus_trap(page, w):
    real_digest(page)
    link = page.locator('[data-section="%s"] [data-test=change]' % SITE_VM)
    link.click(); page.wait_for_selector('[data-test=modes-modal]')
    assert 'blur' in page.evaluate("getComputedStyle(document.querySelector('[data-test=scrim]')).backdropFilter")
    page.wait_for_timeout(350)
    m = page.locator('[data-test=modes-modal]').bounding_box()
    if w == 1440: assert abs((m['x'] + m['width'] / 2) - 720) < 2, m
    assert 'Ask me every time' in page.inner_text('[data-test=modes-modal]')
    axe(page); shot(page, 'v7-modal', w, full=False)
    trap(page, '[data-test=modes-modal]')
    page.keyboard.press('Escape'); page.wait_for_selector('[data-test=modes-modal]', state='detached')
    assert page.evaluate("document.activeElement.dataset.test") == 'change'
    assert no_writes(page) == [] and page.mock.put_bodies == []

def reduced_motion(page, w):
    page.goto(BASE + 'digest.html?fixture=empty'); page.wait_for_selector('[data-test=blip]')
    assert page.evaluate("getComputedStyle(document.querySelector('.blip .blip-bar')).animationName") == 'blip'
    page.emulate_media(reduced_motion='reduce'); page.reload(); page.wait_for_selector('[data-test=blip]')
    assert page.evaluate("getComputedStyle(document.querySelector('.blip .blip-bar')).animationName") == 'none'
    page.keyboard.press('Control+k'); page.wait_for_selector('[data-test=palette]')
    d = page.evaluate("parseFloat(getComputedStyle(document.querySelector('[data-test=palette]')).animationDuration)")
    assert d <= 0.002, d
    page.keyboard.press('Escape'); shot(page, 'v7-reduced-motion-empty', w)
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#continue')
    t = page.evaluate("parseFloat(getComputedStyle(document.querySelector('#continue .keycap-face')).transitionDuration)")
    assert t <= 0.002, t
    shot(page, 'v7-reduced-motion-setup', w)

def keycap_continue(page, w):
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#continue')
    assert page.locator('.keycap').count() == 1 and page.text_content('#continue') == 'Continue'
    ty = lambda: page.evaluate("new DOMMatrix(getComputedStyle(document.querySelector('#continue .keycap-face')).transform).m42")
    assert ty() == -4, ty()
    page.hover('#continue'); page.wait_for_timeout(260)
    if w == 1440: assert ty() == -6, ty()
    assert 'Nothing goes live until it follows the rules above' in page.inner_text('.actions')
    assert 'checked' not in page.locator('#continue-reassure').inner_text().lower()
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    assert page.locator('.keycap').count() == 0, 'one primary per screen: setup Continue only'

def continue_setup_pill(page, w):
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#skip-setup')
    page.click('#skip-setup'); page.wait_for_selector('[data-test=continue-setup]')
    assert 'digest.html' in page.url and page.mock.put_bodies == []
    m = json.loads(page.evaluate("localStorage.getItem('offerrails.setup')"))
    assert m['state'] == 'skipped' and 1 <= m['resume'] <= 4
    t = page.inner_text('[data-test=continue-row]')
    assert 'Continue setup' in t and 'question %d of 4' % m['resume'] in t
    assert page.locator('[data-test=continue-setup]').count() == 1
    shot(page, 'v7-continue-setup-pill', w, full=False)
    page.click('[data-test=continue-setup]'); page.wait_for_selector('[data-test=resume-note]')
    assert 'question %d of 4' % m['resume'] in page.text_content('[data-test=resume-note]')
    assert page.evaluate("n=>[...document.querySelectorAll('#questions fieldset.q')][n-1].contains(document.activeElement)", m['resume'])
    page.goto(BASE + 'setup.html?fixture=maker'); page.wait_for_selector('#continue')
    page.click('#continue'); page.wait_for_function("window.__ev.some(e=>e.name=='offerrails:setup-submit')")
    page.wait_for_function("localStorage.getItem('offerrails.setup')===null")
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    assert page.locator('[data-test=continue-setup]').count() == 0

def empty_state_action(page, w):
    page.goto(BASE + 'digest.html?fixture=empty'); page.wait_for_selector('[data-test=empty]')
    e = page.locator('[data-test=empty]')
    assert e.locator('h2').inner_text() == 'All clear.'
    assert e.locator('[data-test=blip]').count() == 1 and page.locator('[data-test=blip]').count() == 1
    assert e.locator('[data-money]').count() == 0 and e.locator('.alert').count() == 0
    page.click('[data-test=empty-action]')
    page.wait_for_function("document.getElementById('copy-note').textContent.length>0")
    assert page.text_content('#copy-note').startswith('Copied: npx offer-rails.')
    assert no_writes(page) == []
    page.goto(BASE + 'digest.html?fixture=normal'); page.wait_for_selector('digest-item')
    assert page.locator('[data-test=blip]').count() == 0, 'BLIP only in empty states'

def undo_message_text(page, w):
    real_digest(page)
    name = page.evaluate("id=>document.querySelector('digest-item:has(#w-'+id+')').item.site.name", P3)
    ritem(page, P3).locator('button[data-action=toggle]').click(); wait_result(page, 1)
    assert page.text_content('#toast-text') == ('Undone. Clicks are off now. Not yet reverted: the box stays in the post until ' + name +
        ' takes it out, and cached copies of the page may show it a little longer.'), page.text_content('#toast-text')
    page.click('#pause-all'); page.click('#confirm-pause-yes'); page.wait_for_selector('[data-test=kill-outcome]:not([hidden])')
    assert page.inner_text('[data-test=kill-outcome]') == ('Paused 1 site and 1 project. They ask you before anything new goes live, and tracked links stop counting clicks. '
        'Not changed: boxes already in posts stay there until you undo them.'), page.inner_text('[data-test=kill-outcome]')
    page.click('#resume-all'); page.click('#confirm-resume-yes'); page.wait_for_selector('#pause-all:visible')
    t = page.inner_text('[data-test=kill-outcome]')
    assert t.startswith('Resumed. Each site and project is back on its previous approval mode') and t.endswith('Not changed: anything you undid while paused stays undone.'), t

def skeleton_shape(page, w):
    held = []
    page.route('**/backend/v1/account/digest', lambda r: held.append(r))
    page.goto(BASE + 'digest.html?api=real&api_base=/backend'); page.wait_for_selector('[data-test=skeleton]')
    page.wait_for_function("document.querySelector('#digest-root').getAttribute('aria-busy')==='true'")
    for _ in range(100):
        if held: break
        page.wait_for_timeout(50)
    geo = "s=>{const t=document.querySelector(s);const r=t.getBoundingClientRect();return {w:r.width,x:r.x,cols:new Set([...t.children].map(c=>Math.round(c.getBoundingClientRect().left))).size,n:t.children.length}}"
    sk = page.evaluate(geo, '[data-test=skeleton-totals]')
    shot(page, 'v7-skeleton', w, full=False)
    held[0].fulfill(status=200, content_type='application/json', body=json.dumps(page.mock.acct))
    page.wait_for_selector('digest-item')
    real = page.evaluate(geo, '[data-test=totals]')
    assert abs(sk['w'] - real['w']) <= 2 and abs(sk['x'] - real['x']) <= 2 and sk['cols'] == real['cols'], (sk, real)
    assert page.get_attribute('#digest-root', 'aria-busy') is None

def load_error_next_step(page, w):
    st = {'n': 0}
    def h(r):
        st['n'] += 1
        if st['n'] == 1: return r.fulfill(status=500, content_type='application/json', body='{}')
        r.fulfill(status=200, content_type='application/json', body=json.dumps(page.mock.acct))
    page.route('**/backend/v1/account/digest', h)
    page.goto(BASE + 'digest.html?api=real&api_base=/backend'); page.wait_for_selector('[data-test=load-error]')
    t = page.inner_text('[data-test=load-error]'); assert 'Nothing changed' in t and '{' not in t, t
    assert page.locator('[data-test=blip]').count() == 0, 'no BLIP next to errors'
    axe(page)
    page.click('#retry-load'); page.wait_for_selector('digest-item')


TESTS = [setup_maker_happy, setup_publisher_happy, setup_edit_prefilled, setup_failed_signal, setup_empty_scan,
         setup_directory_default, setup_save_error, setup_also_role, setup_a11y,
         setup_nomode,
         digest_opened, digest_undo_redo, digest_undo_then_close, digest_window_closes,
         restore_409_window_7s, restore_409_paused, restore_409_noreason, restore_403_other_side,
         restore_422_cap, restore_422_relevance_min, restore_422_offer_live, restore_422_fallbacks,
         plugin_states_fixture, plugin_pending_then_removed, plugin_offline_after_undo, plugin_failed_after_undo,
         plugin_null_is_unknown, plugin_restore_going_back, poll_schedule, poll_only_when_visible, digest_live_now_updates,
         failed_note_retry_button, retry_failed_removal, retry_removal_never_placed, retry_failed_restore_422,
         retry_409_not_failed, retry_409_unknown_text, restore_409_unknown_text, failed_restore_not_live, failed_restore_preview_not_live, leave_it_removed,
         no_optimistic_audit, no_optimistic_leave, leave_then_failed_removal, leave_never_answered_offline,
         real_digest_loads, real_pending_resolves_by_poll, real_undo_redo, real_undone_by, real_poll_429,
         real_restore_409_not_undone, real_restore_409_server_message, real_failed_flow, real_retry_restore_422,
         real_maker_no_retry, real_key_context, real_pause_resume, real_resume_under_stub, real_paused_envelope,
         real_earnings_zero_null, real_long_list, real_api_base_guard,
         real_setup_hard_checks, real_setup_publisher, real_setup_stub_modes, real_setup_maker_auto_off, real_setup_maker_put,
         real_setup_maker_fixed, real_setup_claims_live, real_setup_source_manual, real_setup_new, real_setup_key_context, real_setup_403,
         totals_grid_layout,
         digest_rule_ids, digest_earnings_zero,
         digest_pause_all, digest_resume, digest_resume_403,
         digest_resume_signed_out, digest_empty, digest_many_140, digest_no_earnings, digest_a11y,
         kb_list_navigation, kb_guards, kb_key_context, kb_typing_setup, palette_open_run_close, panel_focus_trap, modal_focus_trap,
         reduced_motion, keycap_continue, continue_setup_pill, empty_state_action, undo_message_text, skeleton_shape, load_error_next_step]

# ---------------- v8: plugin key (PR #21 @ 6042dcb) ----------------
PK1 = json.loads(fx('plugin-key-201'))['plugin_key']; PK2 = json.loads(fx('plugin-key-rotate-200'))['plugin_key']
def no_key_leak(page, *keys):
    store = page.evaluate('JSON.stringify(Object.assign({}, localStorage)) + JSON.stringify(Object.assign({}, sessionStorage)) + location.href + JSON.stringify(window.__ev)')
    for k in keys: assert k not in store, 'raw key leaked to storage/url/events'

def setup_to_key_step(page, listing='plugin-key-list-none'):
    page.mock.pk['list'] = (200, listing)
    real_setup(page, 'setup-settings-publisher')
    page.click('#continue')

def key_setup_shown_once(page, w):
    setup_to_key_step(page); page.wait_for_selector('[data-test=key-step]')
    assert page.input_value('#plugin-key-value') == PK1 and 'Mono' in page.eval_on_selector('#plugin-key-value', 'e=>getComputedStyle(e).fontFamily') or 'mono' in page.eval_on_selector('#plugin-key-value', 'e=>getComputedStyle(e).fontFamily').lower()
    t = ' '.join(page.inner_text('[data-test=key-once]').split())
    assert "We won't show this key again." in t and 'Offer Rails › Settings › Plugin key' in t, t
    assert page.is_disabled('#key-continue') and page.locator('#key-continue.keycap').count() == 1
    assert [x[:2] for x in page.mock.pk_calls()] == [('GET', '/'), ('POST', '/')] and page.mock.pk_calls()[1][2] == {}
    axe(page); no_overflow(page); shot(page, 'v8-key-shown-once', w)
    page.ctx.grant_permissions(['clipboard-read', 'clipboard-write'], origin=BASE.rstrip('/'))
    page.click('#copy-key'); page.wait_for_selector('#copy-key[data-copied=true]')
    assert page.text_content('#copy-key') == 'Copied ✓' and page.is_enabled('#key-continue')
    assert page.evaluate('navigator.clipboard.readText()') == PK1
    shot(page, 'v8-key-copied', w, full=False)
    page.click('#key-continue'); page.wait_for_selector('[data-test=success]')
    assert 'starts %s' % PK1[:8] in page.text_content('[data-test=plugin-key-note]') and PK1 not in page.content()
    no_key_leak(page, PK1); axe(page)

def key_setup_checkbox(page, w):
    setup_to_key_step(page); page.wait_for_selector('[data-test=key-step]')
    page.check('#key-saved'); assert page.is_enabled('#key-continue') and 'Saved.' in page.text_content('#key-gate')
    page.uncheck('#key-saved'); assert page.is_disabled('#key-continue')
    page.check('#key-saved'); page.click('#key-continue'); page.wait_for_selector('[data-test=success]')

def key_setup_already_has_key(page, w):
    setup_to_key_step(page, 'plugin-key-list'); page.wait_for_selector('[data-test=success]')
    assert page.locator('[data-test=key-step]').count() == 0 and page.get_attribute('[data-test=plugin-key-note]', 'data-kind') == 'has'
    assert [x[:2] for x in page.mock.pk_calls()] == [('GET', '/')], 'no second key when one is active'

def key_setup_409(page, w):
    page.mock.pk['issue'] = (409, 'plugin-key-409')
    setup_to_key_step(page); page.wait_for_selector('[data-test=success]')
    t = page.text_content('[data-test=plugin-key-note]'); assert 'A plugin key already exists.' in t and 'Rotate' in t and '{' not in t

def key_setup_sign_in_again(page, w):
    page.mock.pk['list'] = (401, 'plugin-key-401')  # v9: 403 is forbidden_key_type (own copy, key_err_setup_reasons)
    real_setup(page, 'setup-settings-publisher'); page.click('#continue'); page.wait_for_selector('[data-test=reauth]')
    assert 'Sign in again to continue.' in page.text_content('[data-test=reauth]') and 'Nothing was changed.' in page.text_content('[data-test=reauth]')
    assert [x[:2] for x in page.mock.pk_calls()] == [('GET', '/')]
    axe(page); shot(page, 'v8-key-setup-sign-in-again', w, full=False)
    page.click('#key-later'); page.wait_for_selector('[data-test=success]')

def key_setup_maker_no_step(page, w):
    real_setup(page, 'setup-settings-maker', 'maker'); page.click('#continue'); page.wait_for_selector('[data-test=success]')
    assert page.mock.pk_calls() == [] and page.mock.find('/v1/account/projects') == []

def setup_saved_mode_kept(page, w):
    real_setup(page, 'setup-settings-publisher')
    assert page.is_visible('[data-test=mode-kept]') and all(page.eval_on_selector_all('input[name=mode]', 'xs=>xs.map(x=>x.disabled)'))
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    assert 'your saved approval mode' in page.text_content('[data-test=success]')
    real_setup(page, 'setup-settings-new'); assert page.locator('[data-test=mode-kept]').count() == 0, 'a new setup still picks a mode'

def open_settings(page, acct=None):
    real_digest(page, acct=acct)
    page.click('[data-site-settings="%s"]' % SITE_VM); page.wait_for_selector('[data-test=site-settings]')

def key_settings_rotate(page, w):
    page.mock.pk_after['rotate'] = 'plugin-key-list'
    open_settings(page); page.wait_for_selector('[data-test=key-prefix]')
    assert page.text_content('[data-test=key-prefix]') == PK1[:8] and PK1 not in page.content()
    assert page.locator('[data-key-act=generate]').count() == 0 and page.is_visible('[data-key-act=rotate]') and page.is_visible('[data-key-act=revoke]')
    axe(page); shot(page, 'v8-settings-panel', w, full=False)
    page.click('[data-key-act=rotate]'); page.wait_for_selector('[data-test=rotate-confirm]')
    assert [x[:2] for x in page.mock.pk_calls()] == [('GET', '/')], 'rotate asks first'
    page.click('[data-key-act=rotate-yes]'); page.wait_for_selector('[data-test=key-once]')
    assert page.input_value('#plugin-key-value') == PK2 and page.is_disabled('#key-continue')
    axe(page); shot(page, 'v8-rotate-shown-once', w, full=False)
    page.check('#key-saved'); page.click('#key-continue'); page.wait_for_selector('[data-test=key-msg]')
    assert 'Key rotated' in page.text_content('[data-test=key-msg]') and PK2 not in page.content()
    assert ('POST', '/rotate', {}) in page.mock.pk_calls(); no_key_leak(page, PK1, PK2)

def key_settings_generate_409(page, w):
    page.mock.pk['list'] = (200, 'plugin-key-list-revoked'); page.mock.pk['issue'] = (409, 'plugin-key-409')
    open_settings(page); page.wait_for_selector('[data-key-act=generate]')
    assert 'No active plugin key.' in page.text_content('[data-test=key-status]')
    page.click('[data-key-act=generate]'); page.wait_for_selector('[data-test=key-msg]')
    page.wait_for_function("document.activeElement && document.activeElement.dataset.keyAct==='rotate'")
    t = page.text_content('[data-test=key-msg]'); assert 'A plugin key already exists.' in t and 'rotate it' in t and '{' not in t
    axe(page); shot(page, 'v8-generate-409', w, full=False)

def key_settings_generate_201(page, w):
    page.mock.pk['list'] = (200, 'plugin-key-list-none'); page.mock.pk_after['issue'] = 'plugin-key-list'
    open_settings(page); page.click('[data-key-act=generate]'); page.wait_for_selector('[data-test=key-once]')
    assert page.input_value('#plugin-key-value') == PK1
    page.check('#key-saved'); page.click('#key-continue'); page.wait_for_selector('[data-test=key-prefix]')

def key_revoke_modal(page, w):
    page.mock.pk_after['revoke'] = 'plugin-key-list-revoked'
    open_settings(page); page.click('[data-key-act=revoke]'); page.wait_for_selector('[data-test=revoke-modal]')
    t = ' '.join(page.inner_text('[data-test=revoke-modal]').split())
    for want in ["The plugin can't confirm placements any more.", "New boxes won't count as live, or earn, until you paste a new key in Offer Rails › Settings › Plugin key.",
                 'boxes already live stay live.']:
        assert want in t.replace('\u2019', "'"), (want, t)
    assert page.evaluate('document.activeElement.id') == 'revoke-no', 'focus starts on Cancel'
    axe(page); shot(page, 'v8-revoke-modal', w, full=False)
    page.click('#revoke-no'); page.wait_for_selector('[data-test=site-settings]')
    assert ('POST', '/revoke', {}) not in page.mock.pk_calls()
    page.click('[data-key-act=revoke]'); page.click('#revoke-yes'); page.wait_for_selector('[data-test=site-settings] [data-key-act=generate]')
    assert ('POST', '/revoke', {}) in page.mock.pk_calls() and 'Key revoked.' in page.text_content('[data-test=key-msg]')
    assert 'No active plugin key.' in page.text_content('[data-test=key-status]')

def key_revoke_403(page, w):
    open_settings(page); page.mock.pk['revoke'] = (403, 'plugin-key-403')
    page.click('[data-key-act=revoke]'); page.click('#revoke-yes'); page.wait_for_selector('[data-test=site-settings] [data-test=reauth]')
    assert 'This needs the dashboard, not a key.' in page.text_content('[data-test=reauth]') and 'Nothing was changed.' in page.text_content('[data-test=reauth]')

def key_rotate_401_sign_in_again(page, w):
    open_settings(page); page.mock.pk['rotate'] = (401, 'plugin-key-401')
    page.click('[data-key-act=rotate]'); page.click('[data-key-act=rotate-yes]'); page.wait_for_selector('[data-test=reauth]')
    assert page.get_attribute('[data-test=reauth-link]', 'href') == json.loads(fx('account-digest'))['change_url']
    axe(page); shot(page, 'v8-sign-in-again', w, full=False)

def key_context_hidden(page, w):
    acct = json.loads(fx('account-digest')); acct['session'] = {'signed_in': False}
    open_settings(page, acct)
    assert 'need the dashboard' in page.text_content('[data-test=key-context-line]')
    assert page.locator('[data-key-act]').count() == 0 and page.mock.pk_calls() == [], 'no session: no key calls, no buttons'
    assert 'plugin keys need the dashboard' in page.text_content('#digest-root [data-test=key-note]')
    axe(page); shot(page, 'v8-key-context', w, full=False)

META = "document.addEventListener('DOMContentLoaded',()=>{const m=document.createElement('meta');m.name='csrf-token';m.content='%s';document.head.appendChild(m);});"
def csrf_on_writes(page, w):
    page.add_init_script(META % 'tok-meta-123')
    open_settings(page); page.click('[data-key-act=rotate]'); page.click('[data-key-act=rotate-yes]'); page.wait_for_selector('[data-test=key-once]')
    hs = [c for c in page.mock.calls if c['path'].endswith('/rotate')]
    assert hs and hs[0]['headers'].get('x-csrf-token') == 'tok-meta-123', 'CSRF token from <meta name=csrf-token> on owner writes'
    gets = [c for c in page.mock.calls if c['method'] == 'GET']
    assert gets and all('x-csrf-token' not in c['headers'] for c in gets), 'never on reads'

def csrf_window_global_ignored(page, w):
    # v9 / 2d3aed7: the server sets no window global; a stray window.OfferRails.csrf must never be sent
    page.add_init_script("window.OfferRails={csrf:'tok-window'}")
    open_settings(page); page.click('[data-key-act=rotate]'); page.click('[data-key-act=rotate-yes]'); page.wait_for_selector('[data-test=key-once]')
    hs = [c for c in page.mock.calls if c['path'].endswith('/rotate')]
    assert hs and 'x-csrf-token' not in hs[0]['headers'], 'no meta, no token: the window fallback is gone'

def digest_paid_clicks_and_truncation(page, w):
    acct = json.loads(fx('account-digest'))
    offer = next(p for p in acct['projects'] if p['id'] == OFFER_RAIL)
    offer['truncated'] = True
    offer['items'][0]['clicks'] = 99
    offer['items'][0]['paid_clicks'] = 3
    real_digest(page, acct=acct)
    sec = '[data-section="%s"]' % OFFER_RAIL
    note = ' '.join(page.inner_text(sec + ' [data-test=page-note]').split())
    assert 'The list was cut' in note, note
    assert page.locator(sec + ' .clicks').first.inner_text() == '3 clicks'

TESTS.append(digest_paid_clicks_and_truncation)

def real_latest_50_of_n(page, w):
    real_digest(page, acct='account-digest-long')
    sec = '[data-section="%s"]' % SITE_VM
    assert ' '.join(page.inner_text(sec + ' [data-test=page-note]').split()) == 'Showing the latest 50 of 212. See them all in the dashboard'
    assert page.locator('[data-section="%s"] [data-test=page-note]' % OFFER_RAIL).count() == 0, 'total_items == items: no note'
    page.click(sec + ' [data-more]'); page.locator(sec + ' [data-test=page-note]').scroll_into_view_if_needed()
    no_overflow(page); axe(page); shot(page, 'v8-latest-50-of-N', w, full=False)

def real_resume_effective_mode(page, w):
    real_digest(page, acct='account-digest-claims-live')
    page.mock.resume_body = fx('resume-200')  # stub back on: rows say effective manual + reason
    page.click('#pause-all'); page.click('#confirm-pause-yes'); page.wait_for_selector('#resume-all:visible')
    page.click('#resume-all'); page.click('#confirm-resume-yes'); page.wait_for_selector('#pause-all:visible')
    assert set(page.eval_on_selector_all('[data-mode]', 'xs=>xs.map(x=>x.textContent)')) == {'Ask me every time'}, 'effective_mode wins over mode'
    assert json.loads(fx('resume-200'))['applied'][0]['approval_message'] in page.text_content('[data-test=kill-outcome]')
    real_digest(page, acct='account-digest-claims-live'); page.mock.resume_body = fx('resume-200-claims-live')
    page.click('#pause-all'); page.click('#confirm-pause-yes'); page.wait_for_selector('#resume-all:visible')
    page.click('#resume-all'); page.click('#confirm-resume-yes'); page.wait_for_selector('#pause-all:visible')
    assert page.text_content('[data-mode="%s"]' % SITE_VM) == 'Approve within my rules'
    assert 'stays off' not in page.text_content('[data-test=kill-outcome]')

def real_setup_install_path(page, w):
    # v9 / 2d3aed7: the server defaults install_path from owners.source; the UI never forwards ?install_path=
    page.mock.setup_get = 'setup-settings-new'
    page.goto(BASE + 'setup.html?api=real&api_base=/backend&role=publisher&install_path=cli'); page.wait_for_selector('#setup-form')
    u = page.mock.find('/v1/setup/settings')[0]['url']
    assert u.endswith('/v1/setup/settings?role=publisher') and 'install_path' not in u, u
    page.goto(BASE + 'setup.html?api=real&api_base=/backend'); page.wait_for_selector('#setup-form')
    assert page.mock.find('/v1/setup/settings')[-1]['url'].endswith('/v1/setup/settings'), 'no query at all'
    page.click('#continue'); page.wait_for_selector('[data-test=success],[data-test=key-step]')
    put = page.mock.put_bodies[-1]; assert put['install_path'] == json.loads(fx('setup-settings-new'))['install_path'], put

V8 = [real_setup_install_path, key_setup_shown_once, key_setup_checkbox, key_setup_already_has_key, key_setup_409, key_setup_sign_in_again, key_setup_maker_no_step,
      setup_saved_mode_kept, key_settings_rotate, key_settings_generate_409, key_settings_generate_201, key_revoke_modal, key_revoke_403,
      key_rotate_401_sign_in_again, key_context_hidden, csrf_on_writes, real_latest_50_of_n, real_resume_effective_mode]
TESTS += V8
# ---------------- v9: PR #21 @ 2d3aed7 ----------------
USER_ID = '0e0e0001-0000-4000-8000-00000000c001'
OFFER2 = '0ffe0002-0000-4000-8000-00000000b002'
REVIEW_COPY = {
    'claims': ('Claims are not checked yet.', 'Use only claims from the offer.', 'Nothing to do for now.'),
    'head': ('The offer page did not answer.', 'The offer page did not answer.', 'Make sure the offer link opens'),
    'relevance': ('The offer scored below the minimum relevance.', 'Relevance is below the minimum.', 'Make the offer\u2019s description'),
    'caps': ('You already have as many offers listed as the cap allows.', 'The placement cap is full.', 'End or pause a listed offer')}
def in_review_case(page, w, reason, shoot):
    real_digest(page, acct='synthetic/account-digest-in-review-' + reason)
    it = ritem(page, OFFER2); it.scroll_into_view_if_needed()
    assert state(it) == 'In review', state(it)
    check, msg, nxt = REVIEW_COPY[reason]
    assert it.locator('[data-test=review]').get_attribute('data-reason') == reason
    assert check in it.locator('[data-test=review-check]').text_content()
    assert it.locator('[data-test=review-message]').text_content() == 'Server says: \u201c%s\u201d' % msg, 'server message word for word'
    t = it.locator('[data-test=review-next]').text_content(); assert t.startswith('Next step:') and nxt in t, t
    txt = it.inner_text().lower()
    for bad in ('minute', 'hour', 'today', 'tomorrow', 'soon', 'within'): assert bad not in txt, ('no time promised', bad)
    assert it.locator('button[data-action=toggle]').is_hidden(), 'no undo on a listing in review'
    listed = ritem(page, OFFER_RAIL); assert state(listed).startswith('Listed') and listed.locator('[data-test=review]').count() == 0
    no_overflow(page); axe(page)
    if shoot: shot(page, 'v9-in-review-' + reason, w, full=False)
def listing_in_review_claims(page, w): in_review_case(page, w, 'claims', True)   # hosted CLAIMS_CHECK=stub state
def listing_in_review_head(page, w): in_review_case(page, w, 'head', True)
def listing_in_review_relevance(page, w): in_review_case(page, w, 'relevance', False)
def listing_in_review_caps(page, w): in_review_case(page, w, 'caps', True)
def listing_review_optional(page, w):
    acct = json.loads(fx('synthetic/account-digest-in-review-claims'))
    for p in acct['projects']:
        for i in p['items']:
            if i['id'] == OFFER2: i['message'] = None
    real_digest(page, acct=acct)
    assert ritem(page, OFFER2).locator('[data-test=review]').count() == 0, 'no message: no review block'
    real_digest(page)  # 2d3aed7 owner digest: no status/review fields at all -> plain listing
    assert state(ritem(page, OFFER_RAIL)).startswith('Listed') and page.locator('[data-test=review]').count() == 0

def key_err(page, w, act, st, name, want_lead, want_msg, shot_name=None):
    open_settings(page); page.wait_for_selector('[data-key-act]')
    if act == 'rotate':
        page.mock.pk['rotate'] = (st, name); page.click('[data-key-act=rotate]'); page.click('[data-key-act=rotate-yes]')
    page.wait_for_selector('[data-test=reauth]')
    t = ' '.join(page.inner_text('[data-test=reauth]').split())
    assert want_lead in t and 'Nothing was changed.' in t, t
    if want_msg: assert page.text_content('[data-test=reauth-server]') == want_msg
    else: assert page.locator('[data-test=reauth-server]').count() == 0
    axe(page)
    if shot_name: shot(page, shot_name, w, full=False)
    return t
def key_err_reauth_required(page, w):
    t = key_err(page, w, 'rotate', 401, 'plugin-key-401', 'Sign in again to continue.', 'Sign in again to manage plugin keys.', 'v9-key-error-reauth')
    assert page.get_attribute('[data-test=reauth]', 'data-reason') == 'reauth_required' and page.text_content('[data-test=reauth-link]') == 'Sign in again'
def key_err_reauth_stale(page, w):
    key_err(page, w, 'rotate', 401, 'plugin-key-401-stale', 'Sign in again to continue.', 'This session is too old. Sign in again to manage plugin keys.')
def key_err_forbidden_key_type(page, w):
    key_err(page, w, 'rotate', 403, 'plugin-key-403', 'This needs the dashboard, not a key.',
            'A site, maker, project, or account key, or a bearer token, cannot manage plugin keys.', 'v9-key-error-forbidden-key-type')
    assert page.text_content('[data-test=reauth-link]') == 'Open the dashboard'
def key_err_forbidden_other(page, w):
    key_err(page, w, 'rotate', 403, 'plugin-key-403-forbidden', 'Sign in again to continue.', json.loads(fx('plugin-key-403-forbidden'))['message'])
def key_err_legacy_fallback(page, w):
    key_err(page, w, 'rotate', 401, 'synthetic/plugin-key-401-legacy', 'Sign in again to continue.', None)
    page.mock.pk['list'] = (200, 'plugin-key-list-revoked'); page.mock.pk['issue'] = (409, 'synthetic/plugin-key-409-legacy')
    open_settings(page); page.click('[data-key-act=generate]'); page.wait_for_selector('[data-test=key-msg]')
    t = page.text_content('[data-test=key-msg]'); assert t.startswith('This site already has a plugin key.') and 'rotate it' in t, t
def key_err_key_exists(page, w):
    page.mock.pk['list'] = (200, 'plugin-key-list-revoked'); page.mock.pk['issue'] = (409, 'plugin-key-409')
    open_settings(page); page.click('[data-key-act=generate]'); page.wait_for_selector('[data-test=key-msg]')
    t = page.text_content('[data-test=key-msg]'); assert t.startswith('A plugin key already exists.') and 'rotate it' in t, t
    page.wait_for_function("document.activeElement && document.activeElement.dataset.keyAct==='rotate'")
    axe(page); shot(page, 'v9-key-error-key-exists', w, full=False)
def key_err_setup_reasons(page, w):
    page.mock.pk['issue'] = (409, 'plugin-key-409'); setup_to_key_step(page); page.wait_for_selector('[data-test=success]')
    t = page.text_content('[data-test=plugin-key-note]'); assert t.startswith('A plugin key already exists.') and 'Rotate' in t, t
    page.mock.pk['list'] = (403, 'plugin-key-403'); real_setup(page, 'setup-settings-publisher'); page.click('#continue'); page.wait_for_selector('[data-test=reauth]')
    assert page.get_attribute('[data-test=reauth]', 'data-reason') == 'forbidden_key_type' and 'cannot manage plugin keys' in page.text_content('[data-test=reauth]')
    axe(page); shot(page, 'v9-key-error-setup-forbidden', w, full=False)

def digest_undone_by_owner_id(page, w):
    real_digest(page, acct='account-digest-undone-by')
    it = ritem(page, P5); it.scroll_into_view_if_needed()
    ub = it.locator('[data-test=undone-by]'); assert ub.is_visible() and ub.text_content() == 'Undone by an owner on this account', ub.text_content()
    assert USER_ID not in page.inner_text('body'), 'owner id never shown'
    assert ritem(page, P3).locator('[data-test=undone-by]').is_hidden(), "'' = nothing shown"
    axe(page); shot(page, 'v9-undone-by-digest', w, full=False)
    it.locator('[data-action=details]').click(); page.wait_for_selector('[role=dialog]')
    assert 'an owner on this account' in page.inner_text('[role=dialog]') and USER_ID not in page.inner_text('[role=dialog]')
    axe(page); shot(page, 'v9-undone-by-panel', w, full=False)

def mode_preselected_from_server(page, w):
    # The UI never picks a mode itself: checked/suggested = server mode_preselected (or the saved mode), nothing else.
    for name in ('setup-settings-new', 'setup-settings-source-manual', 'setup-settings-claims-live'):
        real_setup(page, name); b = json.loads(fx(name))
        sug = page.eval_on_selector_all('input[name=mode]', 'xs=>xs.filter(x=>x.checked).map(x=>x.value)')
        assert sug in ([b['mode_preselected']], [b['mode']]), (name, sug, b['mode_preselected'])
    b = json.loads(fx('setup-settings-new')); b.update(mode_preselected=None, install_path='cursor', mode='', mode_shown='')
    page.mock.setup_get = 'setup-settings-new'
    page.route(re.compile(r'.*/v1/setup/settings(\?.*)?$'), lambda r: r.fulfill(status=200, content_type='application/json', body=json.dumps(b)) if r.request.method == 'GET' else r.fallback())
    page.goto(BASE + 'setup.html?api=real&api_base=/backend'); page.wait_for_selector('#setup-form')
    assert page.eval_on_selector_all('input[name=mode]', 'xs=>xs.filter(x=>x.checked).length') == 0, 'no mode_preselected and no server mode: nothing picked, even for cursor'

def post_title_fallback(page, w):
    acct = json.loads(fx('account-digest')); n = 0
    for p in acct['projects']:
        for i in p['items']:
            if i.get('kind') == 'placement':
                n += 1; i['post_title'] = ''
                if n == 1: i['surface_url'] = 'https://victormotricala.it/blog/fatture-2026/'
                else: i.pop('surface_url', None)
    real_digest(page, acct=acct)
    txt = page.locator('digest-item').all_inner_texts(); joined = ' '.join(txt)
    assert '/blog/fatture-2026' in joined, 'surface_url path used when post_title is empty'
    assert 'Untitled post' in joined, 'Untitled post when neither is there'
    assert '\u201c\u201d' not in joined, 'never an empty title'
    no_overflow(page); axe(page)
    if w == 390: shot(page, 'v9-post-title-fallback', w, full=False)
PENDING_COPY = 'It turns on after your first approved placement.'
def yolo_pending_server_reason(page, w):
    """mode manual + mode_preselected yolo + approval_reason/message: YOLO picked, pending note carries the server's message."""
    real_setup(page, 'setup-settings-new')
    assert page.is_checked('input[name=mode][value=yolo]') and page.is_enabled('input[name=mode][value=yolo]')
    t = ' '.join(page.inner_text('[data-test=mode-pending]').split())
    assert t.startswith('Picked.') and PENDING_COPY in t and CLAIM_MSG in t and 'Until then, you approve' not in t, t
    assert page.get_attribute('[data-test=mode-pending]', 'data-reason') == 'claim_check_unavailable'
    for bad in ('lost', 'reset', 'changed to'): assert bad not in t.lower()
    no_overflow(page); axe(page); shot(page, 'v9-yolo-pending-server-reason', w)
def yolo_pending_fallback(page, w):
    """Same state, server sends no approval_reason (1b85f07 today): our own wording."""
    b = json.loads(fx('setup-settings-new')); b.update(approval_reason=None, approval_message=None)
    page.route('**/backend/v1/setup/settings*', lambda r: r.fulfill(status=200, content_type='application/json', body=json.dumps(b)) if r.request.method == 'GET' else r.fallback())
    real_setup(page, 'setup-settings-new')
    assert page.is_checked('input[name=mode][value=yolo]') and page.locator('[data-test=mode-unavailable]').count() == 0
    t = ' '.join(page.inner_text('[data-test=mode-pending]').split())
    assert t == 'Picked. ' + PENDING_COPY + ' Until then, you approve each new box yourself.', t
    page.click('#continue'); page.wait_for_selector('[data-test=success]')
    assert page.mock.put_bodies[-1]['mode'] == 'yolo'
    no_overflow(page); axe(page); shot(page, 'v9-yolo-pending-fallback', w)
PENDING_MSG = 'YOLO is picked and turns on after the first approved placement.'
def yolo_pending_f249a77(page, w):
    """f249a77 shape: approval_reason yolo_after_first_placement + its message: YOLO picked, server sentence shown, auto NOT locked."""
    b = json.loads(fx('setup-settings-new')); b.update(approval_reason='yolo_after_first_placement', approval_message=PENDING_MSG)
    page.route('**/backend/v1/setup/settings*', lambda r: r.fulfill(status=200, content_type='application/json', body=json.dumps(b)) if r.request.method == 'GET' else r.fallback())
    real_setup(page, 'setup-settings-new')
    assert page.is_checked('input[name=mode][value=yolo]') and page.is_enabled('input[name=mode][value=auto]')
    assert page.locator('[data-test=mode-unavailable]').count() == 0, 'this reason locks nothing'
    t = ' '.join(page.inner_text('[data-test=mode-pending]').split())
    assert t == 'Picked. ' + PENDING_MSG, t
    assert page.get_attribute('[data-test=mode-pending]', 'data-reason') == 'yolo_after_first_placement'
    no_overflow(page); axe(page); shot(page, 'v9-yolo-pending-f249a77', w)
def in_review_from_account_digest(page, w):
    """65a1871 live shape: the held offer is in its /v1/account/digest project items; /v1/digest is read only for "since"
    (an offer item only there is not merged in any more)."""
    pid = '0ffe0009-0000-4000-8000-00000000b009'
    held = {'approved_by': '', 'clicks': 0, 'id': pid, 'kind': 'offer', 'label': 'Northwind Builder', 'link_text': '', 'live_now': False, 'message': 'Claims are not checked yet.',
        'plugin_state': '', 'policy_rule_id': '', 'policy_rule_text': '', 'position': '', 'post_title': '', 'review_reason': 'claims', 'rules_version': 0, 'status': 'submitted',
        'summary': 'Northwind Builder', 'undo_token': 'x', 'undone_by': '', 'went_live_at': '2026-10-13T22:24:00Z'}
    acct = json.loads(fx('account-digest'))
    acct['projects'].append({'clicks': 0, 'clicks_by_source': [], 'earnings': None, 'id': pid, 'items': [held], 'kind': 'offer', 'mode': 'manual', 'name': 'Northwind Builder', 'total_items': 1, 'went_live': 0})
    ghost = dict(held, id='0ffe0009-0000-4000-8000-00000000b00a', label='Only In V1 Digest', summary='Only In V1 Digest')
    acct['projects'].append({'clicks': 0, 'clicks_by_source': [], 'earnings': None, 'id': ghost['id'], 'items': [], 'kind': 'offer', 'mode': 'manual', 'name': 'Only In V1 Digest', 'total_items': 0, 'went_live': 0})
    dig = json.loads(fx('digest')); dig['items'] = [it for it in dig.get('items', []) if it.get('id') not in (pid, ghost['id'])] + [ghost]
    page.route(re.compile(r'.*/v1/digest$'), lambda r: r.fulfill(status=200, content_type='application/json', body=json.dumps(dig)))
    real_digest(page, acct=acct)
    it = ritem(page, pid); it.scroll_into_view_if_needed()
    assert it.count() == 1 and state(it) == 'In review', 'the held offer appears once, in its own section'
    assert it.locator('[data-test=review-message]').text_content() == 'Server says: \u201cClaims are not checked yet.\u201d'
    assert ritem(page, ghost['id']).count() == 0, '/v1/digest items are no longer merged into offer sections'
    no_overflow(page); axe(page); shot(page, 'v9-in-review-from-account-digest', w, full=False)
V9 = [in_review_from_account_digest, yolo_pending_f249a77, yolo_pending_server_reason, yolo_pending_fallback, post_title_fallback, csrf_window_global_ignored, listing_in_review_claims, listing_in_review_head, listing_in_review_relevance, listing_in_review_caps, listing_review_optional,
      key_err_reauth_required, key_err_reauth_stale, key_err_forbidden_key_type, key_err_forbidden_other, key_err_legacy_fallback, key_err_key_exists,
      key_err_setup_reasons, digest_undone_by_owner_id, mode_preselected_from_server]
TESTS += V9

def main():
    only = os.environ.get('ONLY')
    tests = [t for t in TESTS if not only or re.search(only, t.__name__)]
    srv = subprocess.Popen([sys.executable, '-m', 'http.server', str(PORT), '--bind', '127.0.0.1'], cwd=ROOT,
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    time.sleep(0.8)
    results = {}
    try:
        with sync_playwright() as p:
            b = p.chromium.launch(executable_path='/usr/bin/google-chrome', args=['--headless=new'])
            for w in (1440, 390):
                results[w] = [0, 0]
                for t in tests:
                    ctx = b.new_context(viewport={'width': w, 'height': 900}, ignore_https_errors=True,
                                        is_mobile=(w == 390), has_touch=(w == 390), device_scale_factor=1)
                    ctx.route('**/v1/go/**', lambda r: (go_hits.append(r.request.url), r.abort()))
                    ctx.add_init_script(RECORD)
                    mock = MockAPI(); mock.install(ctx)
                    page = ctx.new_page(); errs = []
                    page.ctx = ctx
                    ctx.set_default_timeout(60000)
                    _goto = page.goto
                    def goto(url, _g=_goto):  # one retry: python http.server can stall a keep-alive socket
                        try: return _g(url, wait_until='domcontentloaded', timeout=60000)
                        except Exception: return _g(url, wait_until='domcontentloaded', timeout=60000)
                    page.goto = goto
                    page.on('pageerror', lambda e: errs.append(str(e)))
                    try:
                        page.mock = mock; t(page, w); assert not errs, errs
                        results[w][0] += 1; print('PASS %4d %s' % (w, t.__name__))
                    except Exception as e:
                        results[w][1] += 1; print('FAIL %4d %s: %s' % (w, t.__name__, e))
                        traceback.print_exc(limit=-2)
                    ctx.close()
            b.close()
    finally:
        srv.terminate()
    print('go-link requests:', len(go_hits))
    for w, (ok, bad) in results.items():
        print('width %d: %d passed, %d failed' % (w, ok, bad))
    sys.exit(1 if any(r[1] for r in results.values()) or go_hits else 0)

if __name__ == '__main__':
    main()
