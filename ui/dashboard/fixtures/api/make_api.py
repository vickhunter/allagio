"""Writes fixtures/api/*.json in the exact response shapes of PR #21 (vickhunter/affilio @ 2d3aed7; v9 re-read).
Each body mirrors the gin.H the handler writes (handlers/agent_entry.go, agent_account.go, entry/*.go, plugin_keys.go at 6042dcb);
gin.H is a Go map, so the server writes keys in sorted order and so do we (sort_keys).
See tests/contract_spec.json for the source of each shape. Ids are UUIDs because the API rejects anything else."""
import json, os
HERE = os.path.dirname(os.path.abspath(__file__))
SITE_VM, OFFER_RAIL = '5a1e0001-0000-4000-8000-00000000a001', '0ffe0001-0000-4000-8000-00000000b001'
USER = '0e0e0001-0000-4000-8000-00000000c001'
PLC = '9c000001-0000-4000-8000-000000000001'   # mock replaces it with the requested id
P = {n: '9c00000%d-0000-4000-8000-00000000000%d' % (n, n) for n in (3, 4, 5, 6, 7)}
CATALOG = [('disclosure', 'Add a disclosure.'), ('rel_sponsored', 'Add rel=sponsored.'), ('claims_from_offer', 'Use only claims from the offer.'),
           ('copy_length', 'Keep the copy to 140 characters.'), ('one_per_post', 'Keep one box on this post.'),
           ('relevance_min', 'Relevance is below the minimum.'), ('offer_live', 'The offer is not live.'),
           ('revoke_path', 'The revoke path is not available.'), ('cap', 'The placement cap is full.')]
CHECK = dict(CATALOG)
REFUSAL = {'window_expired': 'The restore window has expired.', 'paused': 'Pause or the kill switch was turned on after the undo.',
           'not_failed': 'The placement is not failed.', 'not_undone': 'The placement was not undone.'}
CLAIM_REASON, CLAIM_MESSAGE = 'claim_check_unavailable', 'Automatic approval is off until claim checks are live.'
CHANGE_URL, SITE_URL = 'https://offerrails.example/owner', 'https://victormotricala.it'
REASON = 'The block was edited in WordPress after it was inserted, so the plugin left the post alone.'
FALLBACK = 'https://fattura-lampo.example/'
MODES = ['manual', 'auto', 'yolo']
NOTE = ('Earnings are a list of currency and amount for recorded test-mode commission. The list is null when that scope has no test-mode rows, '
        'and includes a zero amount when those rows add up to zero. Publisher amounts are ledger rows joined to test conversions. Maker commission '
        'stays on the offer and is not added to publisher income. Live and legacy rows are omitted. Earnings from a click on a placement whose restore '
        'failed are omitted until that restore is confirmed placed. A placement counts as live only when plugin_state is placed and its go link is '
        'redirecting. Digest totals, site totals, and Live now exclude a failed restore. Click source is the referrer host; a click with no host counts as direct.')

def money(*pairs): return [{'currency': c, 'amount': a} for c, a in pairs]
def poll(status, state, reason='', action='', rule=('category:invoicing', 'The category is invoicing.')):
    # handlers/placements.go writePlacementStatus
    return {'id': PLC, 'status': status, 'plugin_state': state, 'plugin_failed_reason': reason, 'failed_action': action, 'fallback_url': FALLBACK,
            'policy_rule_id': rule[0], 'policy_rule_text': rule[1],
            # 2d3aed7 (580edd1): the poll adds surface_url and placement.surface_url (the article URL)
            'surface_url': SURFACE, 'placement': {'surface_url': SURFACE}}
SURFACE = 'https://victormotricala.it/come-fatturare-da-freelance-2026/'
def placement(pid, summary, at, state, live, title, pos, link, label, clicks, by, rule=('', ''), ver=0, reason='', action='', undone_by=''):
    # digestJSON(): every item key, plus plugin_failed_reason and failed_action for placements
    return {'kind': 'placement', 'id': pid, 'summary': summary, 'went_live_at': at, 'undo_token': 'ut_' + pid[:8],
            'plugin_state': state, 'live_now': live, 'post_title': title, 'position': pos, 'link_text': link, 'label': label,
            'clicks': clicks, 'approved_by': by, 'undone_by': undone_by, 'policy_rule_id': rule[0], 'policy_rule_text': rule[1], 'rules_version': ver,
            'plugin_failed_reason': reason, 'failed_action': action}
def offer_item(oid, name, at, clicks):
    # scanDigest(): plugin_state "" (no plugin), live_now true for a listed offer, label = name, no failure fields
    return {'kind': 'offer', 'id': oid, 'summary': name, 'went_live_at': at, 'undo_token': 'ut_' + oid[:8],
            'plugin_state': '', 'live_now': True, 'post_title': '', 'position': '', 'link_text': '', 'label': name,
            'clicks': clicks, 'approved_by': '', 'undone_by': '', 'policy_rule_id': '', 'policy_rule_text': '', 'rules_version': 0}
def meta(stub=True, paused=(False, False, False), since_n=1):
    # attachDigestMeta(): same envelope on GET /v1/digest and GET /v1/account/digest
    b = {'paused': {'account': paused[0], 'site': paused[1], 'offer': paused[2]}, 'went_live_since_last': since_n,
         'change_url': CHANGE_URL, 'site_url': SITE_URL, 'available_modes': MODES}
    if stub: b.update(approval_reason=CLAIM_REASON, approval_message=CLAIM_MESSAGE, effective_mode='manual')
    return b
RULE_CAT = ('category:invoicing', 'The category is invoicing.')
RULE_HARD = ('hard_checks_passed', 'Passed every hard check.')
SITE_ITEMS = [
    placement(P[3], 'Fattura Lampo crea e invia fatture elettroniche dal telefono, da €9 al mese.', '2026-10-08T09:12:00Z', 'placed', True,
              'Come fatturare da freelance nel 2026', 'matched_section', 'Prova Fattura Lampo', '(link sponsorizzato)', 23, 'policy:auto', RULE_CAT, 3),
    placement(P[4], 'SnapVault salva il tuo sito ogni notte e lo ripristina in un clic.', '2026-10-07T16:40:00Z', 'placed', True,
              'Backup WordPress: la guida completa', 'end', 'Vedi SnapVault', 'Pubblicità', 14, 'owner', ('', ''), 3),
    placement(P[5], 'Ledgerly tiene la prima nota per te, collegata alla tua banca.', '2026-10-06T08:00:00Z', 'pending', False,
              'Prima nota senza fatica', 'after_intro', 'Scopri Ledgerly', '(link sponsorizzato)', 0, 'policy:yolo', RULE_HARD, 2),
    placement(P[6], 'Rail Tool porta un programma referral nel tuo prodotto in un pomeriggio.', '2026-10-05T11:00:00Z', 'failed', False,
              'Referral per SaaS piccoli', 'end', 'Prova Rail Tool', '(link sponsorizzato)', 2, 'owner', ('', ''), 2, REASON, 'removed'),
    placement(P[7], 'InvoiceKit fa i preventivi in due tap.', '2026-10-04T10:00:00Z', 'failed', False,
              'Preventivi veloci', 'after_intro', 'Vedi InvoiceKit', '(link sponsorizzato)', 0, 'policy:auto', RULE_CAT, 2, '', 'restored'),
]
def account(stub=True, paused=(False, False, False), items=None, site_live=2, total_live=3, site_earn=money(('EUR', 9.5), ('USD', 3.0)),
            offer_earn=money(('EUR', 4.0)), total_earn=money(('EUR', 9.5), ('USD', 3.0)), modes=('auto', 'yolo'), offer_live=1, site_total=None):
    site_mode, offer_mode = ('manual', 'manual') if stub else modes
    b = {'totals': {'went_live': total_live, 'clicks': 41, 'earnings': total_earn,
                    'clicks_by_source': [{'source': 'google.com', 'clicks': 32}, {'source': 'direct', 'clicks': 9}]},
         'projects': [
             {'kind': 'site', 'id': SITE_VM, 'name': 'victormotricala.it', 'mode': site_mode, 'went_live': site_live, 'clicks': 37, 'earnings': site_earn,
              'clicks_by_source': [{'source': 'google.com', 'clicks': 29}, {'source': 'direct', 'clicks': 8}], 'items': items if items is not None else SITE_ITEMS,
              'total_items': site_total if site_total is not None else len(items if items is not None else SITE_ITEMS)},
             {'kind': 'offer', 'id': OFFER_RAIL, 'name': 'Rail Tool', 'mode': offer_mode, 'went_live': offer_live, 'clicks': 4, 'earnings': offer_earn,
              'clicks_by_source': [{'source': 'google.com', 'clicks': 3}, {'source': 'direct', 'clicks': 1}],
              'items': [offer_item(OFFER_RAIL, 'Rail Tool', '2026-10-02T12:00:00Z', 4)], 'total_items': 1}],
         'earnings_note': NOTE, 'session': {'signed_in': True}}
    b.update(meta(stub, paused)); return b
def setup(role, path, saved_mode, extra, source_default, saved=True, stub=True):
    # setupJSON() + finishSetup(): mode is the effective mode (manual under the stub claim checker, with a reason)
    eff = 'manual' if stub and saved_mode in ('auto', 'yolo') else saved_mode
    b = {'saved': saved, 'role': role, 'install_path': path, 'mode': eff, 'mode_shown': saved_mode, 'mode_preselected': source_default,
         'available_modes': MODES, 'promote': '', 'avoid_topics': [], 'commission': None, 'project_name': '', 'listing_url': '',
         'listing_price': None, 'sites_channels': [], 'never_topics': [], 'box_position': '', 'human_login': True,
         'session': {'signed_in': True}, 'change_url': CHANGE_URL, 'site_url': SITE_URL if role == 'publisher' else ''}
    if eff != saved_mode: b.update(approval_reason=CLAIM_REASON, approval_message=CLAIM_MESSAGE)
    b.update(extra)
    if saved: b.update(saved_by=USER, saved_at='2026-10-10T06:58:00Z')
    return b

def eff(row, stub=True):
    # withEffectiveMode(): every applied row carries effective_mode, approval_reason, approval_message ("" when the checker is real)
    r = dict(row)
    if stub: r.update(effective_mode='manual', approval_reason=CLAIM_REASON, approval_message=CLAIM_MESSAGE)
    else: r.update(effective_mode=row['mode'], approval_reason='', approval_message='')
    return r
def applied(rows, stub=True): return {'scope': 'all', 'id': '', 'applied': [eff(r, stub) for r in rows]}
PAUSE_ROWS = [{'kind': 'site', 'id': SITE_VM, 'mode': 'manual', 'kill_switch': True, 'paused': 0},
              {'kind': 'offer', 'id': OFFER_RAIL, 'mode': 'manual', 'kill_switch': True, 'paused': 1}]
RESUME_ROWS = [{'kind': 'site', 'id': SITE_VM, 'mode': 'auto', 'kill_switch': False, 'unpaused': 0},
               {'kind': 'offer', 'id': OFFER_RAIL, 'mode': 'yolo', 'kill_switch': False, 'unpaused': 1}]
# plugin_keys.go: generateAPIKey() = 48 hex chars, KeyPrefix = first 8. Fake values, never a real key.
PK1, PK2 = '0f1e2d3c4b5a69788796a5b4c3d2e1f00112233445566778', '7a8b9c0d1e2f30415263748596a7b8c9dadbecfd0e1f2a3b'
SESSION_ONLY = 'plugin keys can only be managed from a signed-in dashboard session'
F = {
  'revoke-200': {'status': 'revoked', 'revoked_by_side': 'publisher', 'undo': True, 'undone_at': '2026-10-10T07:00:00Z'},
  'restore-200': {'status': 'committed', 'approved_by': 'policy:yolo', 'restored_by': USER, 'actor': 'owner_ui'},
  'restore-403': {'error': 'only the identity that undid this placement can restore it'},
  'restore-409-window': {'reason': 'window_expired', 'message': REFUSAL['window_expired']},
  'restore-409-paused': {'reason': 'paused', 'message': REFUSAL['paused']},
  'restore-409-not-undone': {'reason': 'not_undone', 'message': REFUSAL['not_undone']},
  'restore-422-cap': {'check_id': 'cap', 'check_text': CHECK['cap']},
  'restore-422-relevance_min': {'check_id': 'relevance_min', 'check_text': CHECK['relevance_min']},
  'restore-422-offer_live': {'check_id': 'offer_live', 'check_text': CHECK['offer_live']},
  'restore-422-disclosure': {'check_id': 'disclosure', 'check_text': CHECK['disclosure']},
  'retry-200': {'plugin_state': 'pending', 'plugin_failed_reason': '', 'failed_action': ''},
  'retry-403': {'error': 'publisher side required'},
  'retry-409-not_failed': {'reason': 'not_failed', 'message': REFUSAL['not_failed']},
  'retry-422-relevance_min': {'check_id': 'relevance_min', 'check_text': CHECK['relevance_min']},
  'placement-pending': poll('revoked', 'pending'),
  'placement-pending-live': poll('committed', 'pending'),
  'placement-placed': poll('committed', 'placed'),
  'placement-removed': poll('revoked', 'removed'),
  'placement-offline': poll('revoked', 'offline'),
  'placement-failed': poll('revoked', 'failed', REASON, 'removed'),
  'placement-failed-restored': poll('committed', 'failed', '', 'restored'),
  'placement-429': {'error': 'rate limit exceeded'},
  'placement-403-maker': {'error': 'offer is not on this key'},
  'pause-200': applied(PAUSE_ROWS),
  'pause-200-claims-live': applied(PAUSE_ROWS, stub=False),
  'resume-200': applied(RESUME_ROWS),
  'resume-200-claims-live': applied(RESUME_ROWS, stub=False),
  # fix B (6042dcb): decide/restore refuse any credential header, incl. the legacy site and maker keys
  'restore-403-session': {'error': 'decide and restore require a signed-in dashboard session'},
  # fix C (6042dcb): restore and retry check claims in the same transaction; the production stub refuses
  'restore-422-claims_from_offer': {'check_id': 'claims_from_offer', 'check_text': CHECK['claims_from_offer']},
  'retry-422-claims_from_offer': {'check_id': 'claims_from_offer', 'check_text': CHECK['claims_from_offer']},
  'account-projects': {'projects': [{'kind': 'site', 'id': SITE_VM, 'name': 'victormotricala.it', 'mode': 'manual'},
                                    {'kind': 'offer', 'id': OFFER_RAIL, 'name': 'Rail Tool', 'mode': 'manual'}]},
  'plugin-key-201': {'plugin_key': PK1, 'key_prefix': PK1[:8]},
  'plugin-key-rotate-200': {'plugin_key': PK2, 'key_prefix': PK2[:8]},
  # 2d3aed7 (80824ab) writePluginKeyError: {error (compat), reason, message}
  'plugin-key-409': {'error': 'a plugin key already exists', 'reason': 'key_exists', 'message': 'A plugin key already exists.'},
  'plugin-key-403': {'error': SESSION_ONLY, 'reason': 'forbidden_key_type', 'message': 'A site, maker, project, or account key, or a bearer token, cannot manage plugin keys.'},
  'plugin-key-403-forbidden': {'error': SESSION_ONLY, 'reason': 'forbidden', 'message': SESSION_ONLY},
  'plugin-key-401': {'error': 'sign in at /auth/login', 'reason': 'reauth_required', 'message': 'Sign in again to manage plugin keys.'},
  'plugin-key-401-stale': {'error': 'sign in at /auth/login', 'reason': 'reauth_required', 'message': 'This session is too old. Sign in again to manage plugin keys.'},
  'plugin-key-revoke-200': {'revoked': True},
  'plugin-key-list': {'keys': [{'id': '9e000001-0000-4000-8000-0000000000a1', 'key_prefix': PK1[:8], 'created_at': '2026-10-11T09:30:00Z', 'revoked': False},
                               {'id': '9e000002-0000-4000-8000-0000000000a2', 'key_prefix': 'c0ffee12', 'created_at': '2026-10-02T08:00:00Z', 'revoked': True,
                                'revoked_at': '2026-10-11T09:30:00Z'}]},
  'plugin-key-list-none': {'keys': []},
  'plugin-key-list-revoked': {'keys': [{'id': '9e000001-0000-4000-8000-0000000000a1', 'key_prefix': PK1[:8], 'created_at': '2026-10-11T09:30:00Z', 'revoked': True,
                                        'revoked_at': '2026-10-12T10:00:00Z'}]},
  'resume-403': {'error': 'resuming requires a signed-in dashboard session'},
  'events-201': {'kind': 'digest_opened', 'user_id': USER, 'recorded_at': '2026-10-10T07:01:00Z'},
  'hard-checks': {'checks': [{'check_id': k, 'check_text': v} for k, v in CATALOG]},
  'account-digest': account(),
  'account-digest-claims-live': account(stub=False),
  # 2d3aed7: undone_by is the owner id on an undone placement, '' otherwise
  'account-digest-undone-by': account(items=[dict(SITE_ITEMS[2], undone_by=USER)] + SITE_ITEMS[:2] + SITE_ITEMS[3:]),
  # placement_counts_as_live: a paused site/offer/account has no redirecting go link, so nothing there counts
  'account-digest-paused': account(paused=(True, True, True), items=[dict(i, live_now=False) for i in SITE_ITEMS], site_live=0, total_live=0, offer_live=0),
  'account-digest-site-paused': account(paused=(False, True, False), items=[dict(i, live_now=False) for i in SITE_ITEMS], site_live=0, total_live=1),
  'account-digest-zero': account(site_earn=None, offer_earn=None, total_earn=None),
  'account-digest-zero-amount': account(site_earn=money(('EUR', 0)), offer_earn=None, total_earn=money(('EUR', 0))),
  'account-digest-long': account(items=[placement('9c1%05d-0000-4000-8000-%012d' % (i, i), 'Offerta %d: un box di prova per una lista lunga.' % (i + 1),
        '2026-10-%02dT%02d:00:00Z' % (9 - i // 24 % 6, i % 24), 'pending' if i % 12 == 5 else 'placed', i % 12 != 5,
        'Articolo %d' % (i + 1), 'end', 'Vedi offerta', '(link sponsorizzato)', i % 7, 'policy:auto', RULE_CAT, 3) for i in range(50)],
        site_live=128, total_live=129, site_total=212),
  'digest': dict({'since': '2026-10-03T07:00:00Z', 'items': [SITE_ITEMS[0]], 'went_live': 2, 'session': {'signed_in': True}}, **meta()),
  'setup-settings-maker': setup('maker', 'cli', 'yolo', {'promote': 'Ship a referral offer for your dev tool in one afternoon.', 'avoid_topics': ['gambling', 'adult'],
      'commission': {'type': 'percent', 'value': 20, 'currency': ''}, 'project_name': 'Rail Tool', 'listing_url': 'https://railtool.example',
      'listing_price': {'value': 19, 'currency': 'EUR'}}, 'yolo'),
  'setup-settings-maker-fixed': setup('maker', 'cursor', 'manual', {'promote': 'Ship a referral offer for your dev tool in one afternoon.',
      'commission': {'type': 'fixed', 'value': 15, 'currency': 'EUR'}, 'project_name': 'Rail Tool', 'listing_url': 'https://railtool.example',
      'listing_price': None}, 'yolo'),
  'setup-settings-publisher': setup('publisher', 'claude_dir', 'manual', {'sites_channels': ['victormotricala.it', 'newsletter.victormotricala.it'],
      'never_topics': ['gambling', 'crypto'], 'box_position': 'end'}, 'manual'),
  'setup-settings-new': setup('publisher', 'cursor', 'yolo', {}, 'yolo', saved=False),
  'setup-settings-claims-live': setup('publisher', 'cursor', 'yolo', {}, 'yolo', saved=False, stub=False),
  'setup-settings-source-manual': setup('publisher', 'cli', 'manual', {}, 'manual', saved=False, stub=False),
  'setup-403': {'error': 'setup is saved from a signed-in dashboard session'},
  'setup-400': {'error': 'role, install_path, mode, and mode_shown required'},
}
for name in list(os.listdir(HERE)):
    if name.endswith('.json'): os.remove(os.path.join(HERE, name))
for name, body in F.items():
    with open(os.path.join(HERE, name + '.json'), 'w') as f:
        json.dump(body, f, indent=1, ensure_ascii=False, sort_keys=True); f.write('\n')
print(len(F), 'fixtures written (2d3aed7 shapes, keys sorted like Go maps)')
