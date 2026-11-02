#!/usr/bin/env python3
"""Time the public cold path: /llms.txt, /skill.md, join, test deal, proof.

Prints seconds per step and the total. Does not print tokens or keys.
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request
import uuid

ORIGIN = os.environ.get("OFFER_RAILS_URL", "http://127.0.0.1:8080").rstrip("/")
TERMS = {
    "title": "cold path notes",
    "disclosure": {"label": "Affiliate link", "lang": "en"},
    "obligations": [{
        "id": "place",
        "by": "acceptor",
        "action": "place",
        "destination": "https://example.com/notes",
        "proof": ["proof_url"],
    }],
}


def stamp():
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def request(method, path, body=None, token=None):
    data = None if body is None else json.dumps(body).encode()
    req = urllib.request.Request(ORIGIN + path, data=data, method=method)
    if body is not None:
        req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=30) as res:
            raw = res.read()
            return res.status, raw
    except urllib.error.HTTPError as err:
        return err.code, err.read()


def public_error(status, raw):
    text = raw.decode("utf-8", "replace")
    try:
        doc = json.loads(text)
    except json.JSONDecodeError:
        snippet = text.replace("\n", " ")[:180]
        return "status %s %s" % (status, snippet)
    body = doc.get("body") if isinstance(doc, dict) else None
    if isinstance(body, dict) and body.get("code"):
        return "status %s %s %s" % (status, body.get("code"), body.get("message", ""))
    if isinstance(doc, dict) and doc.get("error"):
        return "status %s %s" % (status, doc.get("error"))
    return "status %s" % status


def expect(status, raw, want):
    if status != want:
        raise SystemExit(public_error(status, raw))
    if not raw:
        return {}
    text = raw.decode("utf-8", "replace")
    if text[:1] not in "{[":
        return {"text": text}
    return json.loads(text)


def envelope(typ, sender, body, deal=None):
    doc = {
        "orp": "0.3",
        "type": typ,
        "id": "m_" + uuid.uuid4().hex[:16],
        "ts": stamp(),
        "from": sender,
        "body": body,
    }
    if deal:
        doc["deal"] = deal
    return doc


def post_orp(token, doc):
    status, raw = request("POST", "/orp", doc, token)
    parsed = expect(status, raw, 200)
    return parsed


def main():
    started = time.perf_counter()
    rows = []

    def step(name, fn):
        t0 = time.perf_counter()
        fn()
        seconds = time.perf_counter() - t0
        rows.append((name, seconds))
        print("%s %.3f" % (name, seconds))

    def llms():
        status, raw = request("GET", "/llms.txt")
        doc = expect(status, raw, 200)
        text = doc.get("text", "")
        if "/skill.md" not in text:
            raise SystemExit("llms.txt has no skill pointer")

    def skill():
        status, raw = request("GET", "/skill.md")
        doc = expect(status, raw, 200)
        text = doc.get("text", "")
        if "llms-full.txt" not in text:
            raise SystemExit("skill.md has no full text pointer")

    parties = {}

    def join():
        for name in ("offerer", "acceptor"):
            status, raw = request("POST", "/orp/join", {})
            doc = expect(status, raw, 201)
            token = doc.get("token") or ""
            party = doc.get("party") or ""
            if not token or not party:
                raise SystemExit("join did not return a party")
            parties[name] = (token, party)

    deal = {}

    def test_deal():
        offerer_token, offerer = parties["offerer"]
        acceptor_token, acceptor = parties["acceptor"]
        post_orp(offerer_token, envelope("hello", offerer, {"versions": ["0.3"]}))
        post_orp(offerer_token, envelope("discover", offerer, {
            "want": "offers",
            "query": "cold path notes",
            "limit": 5,
        }))
        offered = envelope("offer", offerer, {"terms": TERMS})
        offered["to"] = acceptor
        offer = post_orp(offerer_token, offered)
        body = offer.get("body") or {}
        deal_id = offer.get("deal") or ""
        terms_hash = body.get("terms_hash") or ""
        if not deal_id or not terms_hash or body.get("state") != "proposed":
            raise SystemExit("directed offer did not open a proposed deal")
        accepted = envelope("accept", acceptor, {"terms_hash": terms_hash}, deal_id)
        accept = post_orp(acceptor_token, accepted)
        if (accept.get("body") or {}).get("state") != "awaiting":
            raise SystemExit("accept did not leave the deal awaiting")
        status, raw = request("POST", "/orp/deals/%s/activate" % deal_id, {}, offerer_token)
        active = expect(status, raw, 200)
        if (active.get("body") or {}).get("state") != "active":
            raise SystemExit("activate did not make the deal active")
        post_orp(acceptor_token, envelope("execute", acceptor, {"obligation": "place"}, deal_id))
        deal["id"] = deal_id

    def proof():
        status, raw = request("GET", "/orp/proof/" + deal["id"])
        doc = expect(status, raw, 200)
        text = doc.get("text", "")
        if "does not hold money" not in text:
            raise SystemExit("proof page was empty")

    step("llms.txt", llms)
    step("skill.md", skill)
    step("join", join)
    step("test deal", test_deal)
    step("proof", proof)
    print("total %.3f" % (time.perf_counter() - started))


if __name__ == "__main__":
    try:
        main()
    except urllib.error.URLError as err:
        print("cold path could not reach the server", file=sys.stderr)
        raise SystemExit(1) from err
