#!/usr/bin/env bash
# Cold-agent smoke for hosted ORP test deals.
# Usage: scripts/d092c-smoke.sh [base_url]
# The only input is the host URL. No pre-seeded key.
set -euo pipefail

BASE="${1:-${BASE_URL:-http://127.0.0.1:8080}}"
export BASE

python3 - <<'PY'
import json, os, re, sys, urllib.error, urllib.request

base = os.environ["BASE"].rstrip("/")
token_re = re.compile(r"orat_[A-Za-z0-9_\-]+")

def show(label, status, raw):
    text = token_re.sub("orat_REDACTED", raw)
    print(f"--- {label} {status}")
    print(text)

def call(method, url, payload=None, token=None):
    data = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(url, data=data, method=method)
    if payload is not None:
        req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            body = resp.read().decode()
            return resp.status, body
    except urllib.error.HTTPError as err:
        return err.code, err.read().decode()

def must(status, want, body, label):
    if status != want:
        show(label, status, body)
        sys.exit(1)

stamp = None

def envelope(typ, mid, sender, body):
    import datetime
    return {
        "orp": "0.3",
        "type": typ,
        "id": mid,
        "ts": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "from": sender,
        "body": body,
    }

status, raw = call("GET", base + "/.well-known/orp.json")
must(status, 200, raw, "discovery")
show("discovery", status, raw)
doc = json.loads(raw)
if doc.get("name") != "In testing" or "conformance" in doc:
    print("discovery is not the in-testing document")
    sys.exit(1)
endpoint = doc["endpoint"]
join_url = doc["join"]

def join(label):
    status, raw = call("POST", join_url, {})
    must(status, 201, raw, label)
    show(label, status, raw)
    body = json.loads(raw)
    return body["token"], body["party"]

maker_token, maker = join("join-maker")
pub_token, publisher = join("join-publisher")

terms = {
    "title": "zqtestmention notes",
    "kind": "sponsored",
    "disclosure": {"label": "Affiliate link", "lang": "en"},
    "obligations": [{
        "id": "place",
        "action": "place",
        "by": "acceptor",
        "content": {"link": True},
        "destination": "https://example.com/notes",
        "proof": ["proof_url"],
    }],
}

status, raw = call("POST", endpoint, envelope("discover", "m_d092c_smoke_d0", publisher, {
    "want": "offers", "query": "zqtestmention", "limit": 5,
}), pub_token)
must(status, 200, raw, "discover-before")
show("discover-before", status, raw)
before = json.loads(raw)
if before["body"].get("empty_reason") != "no_match":
    print("discover before the offer was not empty")
    sys.exit(1)

status, raw = call("POST", endpoint, envelope("offer", "m_d092c_smoke_of", maker, {"terms": terms}), maker_token)
must(status, 200, raw, "offer")
show("offer", status, raw)
offered = json.loads(raw)
offer_id = offered["body"]["id"]
terms_hash = offered["body"]["terms_hash"]

status, raw = call("POST", endpoint, envelope("discover", "m_d092c_smoke_d1", publisher, {
    "want": "offers", "query": "zqtestmention", "limit": 5,
}), pub_token)
must(status, 200, raw, "discover")
show("discover", status, raw)
found = json.loads(raw)
items = found["body"]["items"]
if not any(item.get("offer", {}).get("id") == offer_id and item.get("pool") == "test" for item in items):
    print("discover did not list the standing offer")
    sys.exit(1)

status, raw = call("POST", endpoint, envelope("accept", "m_d092c_smoke_ac", publisher, {
    "offer": offer_id, "terms_hash": terms_hash,
}), pub_token)
must(status, 200, raw, "accept")
show("accept", status, raw)
accepted = json.loads(raw)
deal = accepted["body"]
deal_id = accepted["deal"]
if deal["state"] != "awaiting" or "tracking" in deal:
    print("accept did not open an awaiting deal without tracking")
    sys.exit(1)
reasons = deal["waiting"]["reasons"]
if "step_ask" not in reasons or "claims_unchecked" not in reasons:
    print("waiting reasons missing")
    sys.exit(1)

origin = endpoint[: -len("/orp")] if endpoint.endswith("/orp") else base
proof_url = origin + "/orp/proof/" + deal_id
status, raw = call("GET", proof_url)
must(status, 200, raw, "proof-page")
show("proof-page", status, raw)
if "Affiliate link" not in raw or maker in raw or publisher in raw:
    print("proof page missing the disclosure or leaked a party id")
    sys.exit(1)

execute = envelope("execute", "m_d092c_smoke_ex", publisher, {
    "obligation": "place",
    "proof_url": proof_url,
})
execute["deal"] = deal_id
status, raw = call("POST", endpoint, execute, pub_token)
must(status, 200, raw, "execute")
show("execute", status, raw)
executed = json.loads(raw)
if executed["body"]["state"] != "awaiting":
    print("execute moved the deal off awaiting")
    sys.exit(1)
if executed["body"]["obligations"][0]["state"] != "verified":
    print("place obligation was not verified")
    sys.exit(1)

status, raw = call("GET", endpoint + "/inbox", token=pub_token)
must(status, 200, raw, "inbox")
show("inbox", status, raw)
inbox = json.loads(raw)
receipts = [m for m in inbox["messages"] if m.get("type") == "receipt"]
if not any(m["body"].get("method") == "proof_url" and m["body"].get("result") == "verified" for m in receipts):
    print("inbox has no verified proof_url receipt")
    sys.exit(1)
print("--- smoke ok")
PY
