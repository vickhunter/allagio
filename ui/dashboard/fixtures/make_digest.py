"""Generates digest-*.json fixtures. Run: python3 make_digest.py"""
import json, copy

LONG = "Rail Tool turns your dev tool into a referral offer in one afternoon: Stripe checkout, tracked links and payouts set up by your agent today."
assert len(LONG) == 140, len(LONG)

RULE_TEXT = {
    "hard_checks_passed": "Passed all checks",
    "auto:category_invoicing+relevance_40": "Matched your rules: invoicing is allowed and relevance 64 is above your 40",
    "auto:category_devtools+relevance_40": "Matched your rules: developer tools are allowed and relevance 58 is above your 40",
    None: None,
}

def item(i, kind, site, domain, article, url, pos, copy_, by, rule, clicks, live_at, owner):
    return {
        "item_id": f"itm_{i:03d}", "kind": kind, "placement_id": f"plc_{i:03d}",
        "owner_ref": owner,
        "site": {"name": site, "domain": domain},
        "article": {"title": article, "url": url},
        "position": pos, "copy": copy_, "link_text": "Vedi il prodotto",
        "disclosure_label": "(link sponsorizzato)",
        "approved_by": by, "policy_rule_id": rule, "policy_rule_text": RULE_TEXT.get(rule), "policy_version": 3,
        "side": "publisher" if owner.startswith("site_") else "maker", "plugin_state": "placed",
        "went_live_at": live_at, "clicks": clicks, "status": "live",
        "preview_href": "#preview"
    }

def eur(minor):
    return {"amount_minor": minor, "currency": "EUR", "test_mode": True}

def normal():
    p_items = [
        item(1, "placement", "victormotricala.it", "victormotricala.it", "Come scelgo gli strumenti per il mio SaaS", "https://victormotricala.it/strumenti-saas", "end",
             "Rail Tool: un'offerta referral per il tuo tool, pronta in un pomeriggio.", "policy:yolo", "hard_checks_passed", 41, "2026-10-08T09:12:00Z", "prj_rail"),
        item(2, "listing", "Indie Dev Weekly", "indiedevweekly.example", "Issue 112: tools we actually pay for", "https://indiedevweekly.example/112", "section",
             "Rail Tool sets up referral offers for dev tools without a sales call.", "owner", None, 18, "2026-10-06T16:40:00Z", "prj_rail"),
    ]
    s_items = [
        item(3, "placement", "victormotricala.it", "victormotricala.it", "Come scelgo gli strumenti per il mio SaaS", "https://victormotricala.it/strumenti-saas", "end",
             "Rail Tool: un'offerta referral per il tuo tool, pronta in un pomeriggio.", "policy:yolo", "hard_checks_passed", 41, "2026-10-08T09:12:00Z", "site_vm"),
        item(4, "placement", "victormotricala.it", "victormotricala.it", "Fatturare da freelance: il mio setup 2026", "https://victormotricala.it/fatturare-freelance", "section",
             "Fattura Lampo crea e invia fatture elettroniche dal telefono, da €9 al mese.", "policy:auto", "auto:category_invoicing+relevance_40", 27, "2026-10-07T11:03:00Z", "site_vm"),
        item(5, "placement", "victormotricala.it", "victormotricala.it", "Backup del sito WordPress senza plugin pesanti", "https://victormotricala.it/backup-wordpress", "start",
             "SnapVault salva il tuo sito ogni notte e lo ripristina in un clic.", "owner", None, 9, "2026-10-05T08:30:00Z", "site_vm"),
        item(6, "placement", "newsletter.victormotricala.it", "newsletter.victormotricala.it", "Lettera #31: cosa ho spedito a settembre", "https://newsletter.victormotricala.it/31", "end",
             "Fattura Lampo crea e invia fatture elettroniche dal telefono, da €9 al mese.", "policy:yolo", "hard_checks_passed", 6, "2026-10-09T07:00:00Z", "site_nl"),
    ]
    projects = [{
        "project_id": "prj_rail", "name": "Rail Tool", "url": "https://rail-tool.example", "role": "maker",
        "mode": "yolo", "mode_change_url": "/owner/settings/approval?project=prj_rail",
        "went_live_count": 2, "live_count": 2, "clicks": 59, "earnings": eur(1140),
        "top_sources": [{"source": "victormotricala.it", "clicks": 41}, {"source": "indiedevweekly.example", "clicks": 18}],
        "items": p_items }]
    sites = [
        {"site_id": "site_vm", "name": "victormotricala.it", "url": "https://victormotricala.it", "role": "publisher",
         "mode": "yolo", "mode_change_url": "/owner/settings/approval?site=site_vm",
         "went_live_count": 2, "live_count": 3, "clicks": 77, "earnings": eur(2350),
         "top_sources": [{"source": "google", "clicks": 52}, {"source": "newsletter", "clicks": 14}, {"source": "direct", "clicks": 11}],
         "items": s_items[:3]},
        {"site_id": "site_nl", "name": "newsletter.victormotricala.it", "url": "https://newsletter.victormotricala.it", "role": "publisher",
         "mode": "auto", "mode_change_url": "/owner/settings/approval?site=site_nl",
         "went_live_count": 1, "live_count": 1, "clicks": 6, "earnings": eur(0),
         "top_sources": [{"source": "email", "clicks": 6}],
         "items": s_items[3:]},
    ]
    return {
        "digest_id": "dg_2026w41_usr8f2c", "user_id": "usr_8f2c",
        "period": {"since": "2026-10-03T08:00:00Z", "until": "2026-10-10T08:00:00Z"},
        "session": {"signed_in": True, "user_id": "usr_8f2c"},
        "test_mode": True, "paused": False,
        "totals": {"live_items": 6, "clicks": 142, "earnings": eur(3490), "projects": 1, "sites": 2, "went_live": 5},
        "projects": projects, "sites": sites,
    }

def write(name, d):
    json.dump(d, open(f"digest-{name}.json", "w"), indent=2, ensure_ascii=False)

n = normal(); write("normal", n)

e = normal()
for s in e["projects"] + e["sites"]:
    s.update(items=[], went_live_count=0, live_count=0, clicks=0, earnings=None, top_sources=[])
e["totals"] = {"live_items": 0, "clicks": 0, "earnings": None, "projects": 1, "sites": 2, "went_live": 0}
write("empty", e)

ne = normal()
for s in ne["projects"] + ne["sites"]:
    s["earnings"] = None
ne["totals"]["earnings"] = None
write("no-earnings", ne)

m = normal()
articles = ["Strumenti SaaS 2026", "Guida a Stripe per indie", "Il mio stack da solo founder", "Come prezzo un prodotto digitale",
            "Newsletter: cosa funziona", "Hosting economico per side project", "SEO per piccoli siti", "Automatizzare le fatture",
            "Analytics senza cookie", "Lanciare su Product Hunt", "Email transazionali", "Pagamenti ricorrenti in Italia"]
vm = m["sites"][0]
for k, a in enumerate(articles):
    vm["items"].append(item(100 + k, "placement", "victormotricala.it", "victormotricala.it", a,
        "https://victormotricala.it/" + a.lower().replace(" ", "-").replace(":", ""), ["start", "section", "end"][k % 3],
        LONG, ["policy:yolo", "policy:auto", "owner"][k % 3],
        ["hard_checks_passed", "auto:category_devtools+relevance_40", None][k % 3], 3 + k, "2026-10-0%dT10:00:00Z" % (3 + k % 7), "site_vm"))
vm["live_count"] = len(vm["items"]); vm["went_live_count"] = len(vm["items"])
m["totals"]["live_items"] = sum(len(s["items"]) for s in m["projects"] + m["sites"])
m["totals"]["went_live"] = m["totals"]["live_items"]
write("many", m)

nos = normal(); nos["session"] = {"signed_in": False, "user_id": "usr_8f2c"}; nos["paused"] = True
for s in nos["projects"] + nos["sites"]:
    s["mode_before_pause"] = s["mode"]; s["mode"] = "manual"
write("paused-signed-out", nos)

ps = normal()
vm = ps["sites"][0]
for it, st in zip(vm["items"], ["pending", "removed", "offline"]):
    it["status"] = "undone"; it["plugin_state"] = st
vm["live_count"] = 0
ps["totals"]["live_items"] = 3
ps["sites"][1]["items"][0].update(status="undone", plugin_state="failed", failed_action="removed", plugin_failed_reason=None)
ps["sites"][1]["live_count"] = 0
ps["projects"][0]["items"][1]["plugin_state"] = None
ps["totals"]["live_items"] = 2
write("plugin-states", ps)
print("ok")

fl = normal()
vm = fl["sites"][0]
vm["items"][0].update(status="undone", plugin_state="failed", failed_action="removed",
    plugin_failed_reason="The block was edited in WordPress after it was inserted, so the plugin left the post alone.")
vm["items"][1].update(status="live", plugin_state="failed", failed_action="restored", plugin_failed_reason=None)
vm["live_count"] = 1  # a failed restore is not live (PL 03:43)
fl["totals"]["live_items"] = 4
write("failed", fl)
