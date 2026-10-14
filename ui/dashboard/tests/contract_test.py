"""Contract test: fixtures/api vs PR #21 at f249a77. Facts come from PR21-body-f249a77.md (tables, text, inline JSON)
and tests/contract_spec.json (exact handler keys, read from the code at 6042dcb, re-read at 2d3aed7). Also checks the UI's own calls and copy
against the PR. Run: python3 tests/contract_test.py   (no browser, no network)."""
import json, os, re, sys
ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HEAD = 'f249a77c9b4d42985878c776079a09e8ea35c2ea'
PR = open(os.path.join(ROOT, 'PR21-body-f249a77.md')).read()
SPEC = json.load(open(os.path.join(ROOT, 'tests/contract_spec.json')))
API = os.path.join(ROOT, 'fixtures/api')
results = []
def jsons(): return sorted(f for f in os.listdir(API) if f.endswith('.json'))
def load(f): return json.load(open(os.path.join(API, f)))
def check(name, fn):
    try: fn(); results.append((name, None))
    except AssertionError as e: results.append((name, str(e) or 'failed'))
def table(heading):
    sec = PR.split('## ' + heading, 1)[1].split('\n## ', 1)[0]
    return [[c.strip().strip('`') for c in l.strip().strip('|').split('|')] for l in sec.splitlines()
            if l.startswith('|') and not l.startswith('| ---') and not re.match(r'\|\s*(check_id|reason|Screen|Method)\s*\|', l)]

CATALOG = [(r[0], r[1]) for r in table('Hard checks')]
REFUSALS = {r[0]: r[1] for r in table('409 reasons')}
ENDPOINTS = {(r[0], r[1]) for r in table('Endpoint table (for the CLI/MCP in part B)')}
PARITY_ROWS = table('Parity table')
PARITY = set()
for row in PARITY_ROWS:
    m = re.match(r'(GET|POST|PUT) (\S+)', row[2]); PARITY.add((m.group(1), m.group(2))) if m else None
PLUGIN_STATES = re.findall(r'`(\w+)`', re.search(r'The only values are (.+?)\. A brand-new', PR).group(1))
def _j(x):
    try: return json.loads(x)
    except ValueError: return None
INLINE = [j for j in (_j(x) for x in re.findall(r'`(\{"[^`]*?\})`', PR)) if isinstance(j, dict)]
CLAIM_MSG = re.search(r'`approval_message` `([^`]+)`', PR).group(1)

def typ(v, t):
    ok = {'str': lambda x: isinstance(x, str), 'bool': lambda x: isinstance(x, bool), 'int': lambda x: isinstance(x, int) and not isinstance(x, bool),
          'num': lambda x: isinstance(x, (int, float)) and not isinstance(x, bool), 'null': lambda x: x is None,
          'list': lambda x: isinstance(x, list), 'dict': lambda x: isinstance(x, dict)}
    return any(ok[p](v) for p in t.split('|'))
def keys_match(obj, keys, where, optional=None):
    optional = optional or {}
    extra = set(obj) - set(keys) - set(optional)
    missing = set(keys) - set(obj)
    assert not extra and not missing, '%s: missing %s, unexpected %s' % (where, sorted(missing), sorted(extra))
    for k, t in dict(keys, **optional).items():
        if k in obj: assert typ(obj[k], t), '%s.%s=%r is not %s' % (where, k, obj[k], t)
def money_ok(v, where):
    if v is None: return
    assert isinstance(v, list) and v, '%s: earnings is a non-empty list or null, got %r' % (where, v)
    for x in v: assert set(x) == {'currency', 'amount'} and re.match(r'^[A-Z]{3}$', x['currency']) and typ(x['amount'], 'num'), (where, x)
    assert len({x['currency'] for x in v}) == len(v), '%s: one entry per currency' % where

def fixture_shapes():
    files = sorted(f[:-5] for f in jsons())
    assert set(files) == set(SPEC['shapes']), 'fixtures vs spec: %s' % sorted(set(files) ^ set(SPEC['shapes']))
    for n in files:
        b, s = load(n + '.json'), SPEC['shapes'][n]
        keys_match(b, s['keys'], n, s.get('optional'))
        for k, sub in s.get('nested', {}).items(): keys_match(b[k], sub, n + '.' + k)
        for k, sub in s.get('each', {}).items():
            for i, x in enumerate(b[k]): keys_match(x, sub, '%s.%s[%d]' % (n, k, i))
check('every fixture has exactly the 2d3aed7 handler keys and types (optional keys only where the handler adds them)', fixture_shapes)

def sorted_keys():
    for f in jsons():
        raw = open(os.path.join(API, f)).read()
        def walk(o, where):
            if isinstance(o, dict):
                assert list(o) == sorted(o), '%s: keys not in Go map order at %s' % (f, where)
                for k, v in o.items(): walk(v, where + '.' + k)
            elif isinstance(o, list):
                for i, v in enumerate(o): walk(v, '%s[%d]' % (where, i))
        walk(json.loads(raw), '$')
check('fixture keys are in sorted order, the way Go writes a gin.H', sorted_keys)

def catalog():
    assert [(c['check_id'], c['check_text']) for c in load('hard-checks.json')['checks']] == CATALOG, 'hard-checks fixture != PR table, in order'
    for f in jsons():
        b = load(f)
        if 'check_id' in b: assert (b['check_id'], b['check_text']) in CATALOG, (f, b)
check('GET /v1/policy/hard-checks and every 422 = PR hard-check table, in catalog order', catalog)

def refusals():
    assert 'not_undone' in REFUSALS and 'one_per_post' in REFUSALS and len(REFUSALS) == 16, sorted(REFUSALS)  # 2d3aed7 adds one_per_post
    seen = set()
    for f in jsons():
        b = load(f)
        if 'reason' in b and 'message' in b and not f.startswith('plugin-key-'):  # plugin-key errors: own reasons, checked in v9
            assert REFUSALS.get(b['reason']) == b['message'] and b['message'], (f, b); seen.add(b['reason'])
        if f.startswith(('restore-409', 'retry-409')): assert set(b) == {'reason', 'message'}, (f, b)
    assert {'window_expired', 'paused', 'not_failed', 'not_undone'} <= seen, seen
check('every 409 is {reason, message} from the PR 409 table (incl. not_undone), message never empty', refusals)

def inline_examples():
    by_reason = {load(f)['reason']: load(f) for f in jsons() if 'reason' in load(f) and not f.startswith('plugin-key-')}
    used = 0
    for ex in INLINE:
        if 'reason' in ex and 'message' in ex:
            assert by_reason.get(ex['reason']) == ex, ('inline example', ex); used += 1
    assert used >= 2, used
check('fixtures equal the PR inline JSON examples', inline_examples)

def poll_fields():
    want = re.search(r'cheap poll\..*?`\{([^}]*)\}`', PR).group(1).replace(' ', '').split(',')
    for f in jsons():
        if f.startswith('placement-') and SPEC['shapes'][f[:-5]]['status'] == 200:
            b = load(f); opt = {'surface_url', 'placement.surface_url'}  # v9: optional in the spec
            core = set(want) - opt; got = set(b) | ({'placement.surface_url'} if isinstance(b.get('placement'), dict) and 'surface_url' in b['placement'] else set())
            got -= {'placement'}; assert core <= got <= set(want), (f, sorted(b), want)
            assert b['plugin_state'] in PLUGIN_STATES and b['failed_action'] in ('removed', 'restored', ''), (f, b)
    assert '2 seconds per placement' in PR and 'every 4 seconds still succeeds' in PR
check('poll body = PR field list (now with policy_rule_id/text); plugin_state in the PR enum; 2 s limit leaves room for 4 s', poll_fields)

def digest_items():
    ui_fields = re.search(r'Each digest item .*? includes (.+?)\. Placement items', PR).group(1)
    named = set(re.findall(r'`(\w+)`', ui_fields))
    for f in jsons():
        if not (f.startswith('account-digest') or f == 'digest.json'): continue
        b = load(f)
        items = b['items'] if 'items' in b else [i for p in b['projects'] for i in p['items']]
        for it in items:
            opt = SPEC['digest_item_optional']
            assert sorted(set(it) - set(opt)) == SPEC['digest_item_keys'][it['kind']], (f, sorted(it))
            for k, t in opt.items(): assert k not in it or typ(it[k], t), (f, k, it[k])
            assert named <= set(it), (f, named - set(it))
            if it['kind'] == 'placement':
                assert it['plugin_state'] in PLUGIN_STATES, it
                assert not it['live_now'] or it['plugin_state'] == 'placed', ('live_now needs placed', f, it['id'])
                assert it['position'] in SPEC['box_positions'] + [''], it['position']
                assert it['approved_by'] in ('owner', 'policy:auto', 'policy:yolo', ''), it['approved_by']
            else:
                assert it['plugin_state'] == '' and it['live_now'] is True, ('offer item', f, it)
        for p in b.get('projects', []):
            assert p['mode'] in SPEC['modes'] and p['kind'] in ('site', 'offer')
            money_ok(p['earnings'], f + ' ' + p['name'])
        if 'totals' in b: money_ok(b['totals']['earnings'], f + ' totals')
        if 'approval_reason' in b:
            assert b['approval_reason'] == SPEC['claim_check']['reason'] and b['approval_message'] == CLAIM_MSG == SPEC['claim_check']['message']
            assert b['effective_mode'] == 'manual' and all(p['mode'] == 'manual' for p in b.get('projects', []))
        assert b['available_modes'] == SPEC['modes']
        assert 'maker_commission' not in json.dumps(b), 'placeholder maker_commission is gone'
check('digest items carry every UI field; earnings are per-currency lists or null; effective_mode + reason under the stub', digest_items)

def setup_fixtures():
    for f in jsons():
        if not f.startswith('setup-settings'): continue
        b = load(f)
        assert b['install_path'] in SPEC['install_paths'] and b['mode_preselected'] in SPEC['modes'] and b['mode'] in SPEC['modes']
        assert b['box_position'] in SPEC['box_positions'] + [''], b['box_position']
        assert b['available_modes'] == SPEC['modes']
        c, lp = b['commission'], b['listing_price']
        if c is not None:
            assert set(c) == {'type', 'value', 'currency'} and c['type'] in SPEC['commission_types'], c
            assert c['type'] != 'fixed' or re.match(r'^[A-Z]{3}$', c['currency']), 'fixed needs a currency'
        if lp is not None:
            assert set(lp) == {'value', 'currency'} and re.match(r'^[A-Z]{3}$', lp['currency'])
            assert not (c and c['currency']) or c['currency'] == lp['currency'], 'listing price uses the commission currency'
        stubbed = b['mode'] == 'manual' and b['mode_shown'] in ('auto', 'yolo')
        assert stubbed == ('approval_reason' in b), (f, 'approval_reason only when the stub turned auto/yolo into manual')
        if stubbed: assert b['approval_message'] == CLAIM_MSG
check('setup fixtures: PR enums, commission {type percent|fixed, value, currency}, listing_price, claim-check reason', setup_fixtures)

def ui_endpoints():
    src = ''.join(open(os.path.join(ROOT, 'assets', f)).read() for f in os.listdir(os.path.join(ROOT, 'assets')) if f.endswith('.js'))
    used = set()
    for m in re.finditer(r"OR\.api\.(get|post|put)\(\s*'(/v1/[^']*)'", src): used.add((m.group(1).upper(), m.group(2)))
    for m in re.finditer(r"// endpoint: (GET|POST|PUT) (\S+)", src): used.add((m.group(1), m.group(2)))
    assert len(used) >= 16, used
    missing = [u for u in used if u not in ENDPOINTS and u not in PARITY]
    assert not missing, 'UI calls endpoints not in the PR tables: %s' % missing
    assert not [u for u in used if u[1].startswith('/v1/go')], 'UI must never call /v1/go/'
    assert ('GET', '/v1/sites/:id/mode') not in used and ('GET', '/v1/offers/:id/mode') not in used, 'paused now comes with the digest'
    assert ('GET', '/v1/policy/hard-checks') in used
    print('  UI endpoints (%d): %s' % (len(used), ', '.join('%s %s' % u for u in sorted(used, key=lambda x: x[1]))))
check('every endpoint the UI calls is in the PR tables; no mode reads; hard checks read from the API', ui_endpoints)

def no_hardcoded_checks():
    for f in os.listdir(os.path.join(ROOT, 'assets')):
        if not f.endswith('.js'): continue
        s = open(os.path.join(ROOT, 'assets', f)).read()
        for old in ['Disclosure label and sponsored link', 'Undo always works', 'Relevance at least 25/100']:
            assert old not in s, '%s still has the old hard-check list (%s)' % (f, old)
        if f in ('setup.js', 'contract.js'):
            for _, text in CATALOG: assert text not in s, '%s hardcodes catalog text %r' % (f, text)
    assert ('digest', 'Undo a listing (no redo in v1)', 'POST /v1/digest/undo') in [tuple(r[:3]) for r in PARITY_ROWS]
check('no hardcoded hard-check list in the UI; parity row "Undo a listing (no redo in v1)" exists', no_hardcoded_checks)

def plugin_keys():
    # PR body "Plugin keys" paragraph + parity rows + handlers/plugin_keys.go at 6042dcb
    for path in ['POST /v1/sites/:id/plugin-key', 'GET /v1/sites/:id/plugin-key', 'POST /v1/sites/:id/plugin-key/rotate', 'POST /v1/sites/:id/plugin-key/revoke']:
        m, pth = path.split(' '); assert (m, pth) in PARITY, ('parity row missing', path)
        assert '`%s`' % pth in PR or '`%s' % path in PR, path
    assert '201 `{plugin_key, key_prefix}`' in PR and '409 `{error, reason: key_exists, message}`' in PR
    assert load('plugin-key-409.json')['error'] == 'a plugin key already exists' and load('plugin-key-409.json')['reason'] == 'key_exists'
    assert 'accept only a signed-in dashboard session' in PR and 'lists prefixes and never the secret' in PR
    for n in ('plugin-key-list.json', 'plugin-key-list-none.json', 'plugin-key-list-revoked.json'):
        for i, k in enumerate(load(n)['keys']):
            keys_match(k, SPEC['plugin_key_item']['keys'], '%s.keys[%d]' % (n, i), SPEC['plugin_key_item']['optional'])
            assert ('revoked_at' in k) == k['revoked'], k
            assert 'plugin_key' not in k, 'GET never returns the secret'
    for n in ('plugin-key-201.json', 'plugin-key-rotate-200.json'):
        b = load(n); assert re.match(r'^[0-9a-f]{48}$', b['plugin_key']) and b['key_prefix'] == b['plugin_key'][:8], b
    src = open(os.path.join(ROOT, 'assets/api.js')).read()
    for r in ['GET /v1/sites/:id/plugin-key', 'POST /v1/sites/:id/plugin-key', 'POST /v1/sites/:id/plugin-key/rotate', 'POST /v1/sites/:id/plugin-key/revoke']:
        assert ('// endpoint: ' + r + '\n') in src, ('api.js route', r)
    assert "'/rotate'" in src and "'/revoke'" in src and "'/plugin-key'" in src
    assert 'placeholder' not in src.lower(), 'no placeholder plugin-key names left in api.js'
check('plugin-key routes, 201/200/409/403 bodies and the prefix list = PR body + plugin_keys.go (session-only, secret never read back)', plugin_keys)

def mustfixes_6042dcb():
    for f in jsons():
        b = load(f)
        for p in b.get('projects', []) if f.startswith('account-digest') else []:
            assert isinstance(p.get('total_items'), int) and p['total_items'] >= len(p['items']), (f, p['name'])
        for a in b.get('applied', []):
            assert {'effective_mode', 'approval_reason', 'approval_message'} <= set(a), (f, a)
            assert (a['approval_reason'] == '') == (a['effective_mode'] == a['mode']) or a['mode'] == 'manual', (f, a)
    assert 'adds `total_items` on each digest project' in PR and 'returns `effective_mode` from pause and resume' in PR and 'keeps a stored setup mode' in PR
    assert 'decide and restore refuse a site key and a maker key' in PR
    assert load('restore-403-session.json')['error'] == 'decide and restore require a signed-in dashboard session'
check('6042dcb must-fixes: total_items per project, effective_mode on pause/resume rows, stored setup mode kept, site/maker key 403 on restore', mustfixes_6042dcb)

def v9_2d3aed7():
    # 1 CSRF from the meta tag only
    api = open(os.path.join(ROOT, 'assets/api.js')).read()
    assert 'meta[name="csrf-token"]' in api and 'OR.csrf' not in api and '<meta name="csrf-token" content="...">' in PR and 'does not set a `window` global' in PR
    # 2 install_path defaulted by the server; the UI never sends it as a query
    setup = open(os.path.join(ROOT, 'assets/setup.js')).read()
    assert "install_path=' +" not in setup and "params.get('install_path')" not in setup and 'returns the install path for `owners.source`' in PR
    # 3 plugin-key error reasons
    for r in ('reauth_required', 'forbidden_key_type', 'key_exists'): assert '`%s`' % r in PR and r in api, r
    for f in jsons():
        if f.startswith('plugin-key-') and SPEC['shapes'][f[:-5]]['status'] >= 400:
            b = load(f); assert b['reason'] in SPEC['plugin_key_error_reasons'] and b['message'] and b['error'], (f, b)
    assert load('plugin-key-401.json')['reason'] == 'reauth_required' and load('plugin-key-403.json')['reason'] == 'forbidden_key_type'
    # 4 review_reason values = PR = the UI's copy table
    pr_r = re.search(r'The listing carries `review_reason` \(([^)]*)\)', PR).group(1)
    assert re.findall(r'`(\w+)`', pr_r) == SPEC['review_reasons'], pr_r
    item = open(os.path.join(ROOT, 'assets/digest-item.js')).read(); con = open(os.path.join(ROOT, 'assets/contract.js')).read()
    for r in SPEC['review_reasons']: assert ('    %s: {' % r) in item and ("'%s'" % r) in con, r
    assert 'Both are null when the offer is listed' in PR and '`GET /v1/listings`' in PR
    # 5 undone_by in the PR; owner id never shown raw
    assert 'include `undone_by`. It is the owner id' in PR
    assert any(it.get('undone_by') for p in load('account-digest-undone-by.json')['projects'] for it in p['items'])
    # 6 mode_preselected comes from the server; the UI has no install_path -> mode table
    assert 'mode_preselected' in setup and 'function defaultMode(d) { return' in setup and '`mode_preselected` is unchanged' in PR
check('v9 2d3aed7: CSRF meta only, no ?install_path=, plugin-key reason/message, review_reason set, undone_by owner id, mode_preselected from server', v9_2d3aed7)

for n, err in results: print(('PASS ' if err is None else 'FAIL ') + n + ('' if err is None else ': ' + err))
bad = sum(1 for _, e in results if e)
print('contract (PR #21 @ %s): %d passed, %d failed' % (HEAD[:7], len(results) - bad, bad))
sys.exit(1 if bad else 0)
