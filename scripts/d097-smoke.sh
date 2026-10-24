#!/usr/bin/env bash
# D-097 smoke. Point it at a running API that has the curated catalog
# (Jasper, or another AI writing program) and migration 048 applied.
# It does not deploy, merge, or change CLAIMS_CHECK.
#
#   scripts/d097-smoke.sh https://api.example
#   DATABASE_URL=postgres://... scripts/d097-smoke.sh http://127.0.0.1:8080
#
# The second argument is unused. Set DATABASE_URL to also assert the draft
# was not written. Without it, the script checks the HTTP body only.
set -euo pipefail

BASE="${1:?usage: scripts/d097-smoke.sh BASE_URL}"
BASE="${BASE%/}"

python3 - "$BASE" <<'PY'
import json, os, sys, urllib.request, urllib.error

base = sys.argv[1]
secrets = ["ada@secret.example", "secret.example", "ada_private", "5550199", "+1 415 555 0199"]
draft = (
    "I have been testing an AI writing tool that drafts posts for founders. "
    "Reach ada@secret.example or @ada_private at +1 415 555 0199."
)

def call(method, path, payload=None, token=None):
    data = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(base + path, data=data, method=method)
    if payload is not None:
        req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            raw = resp.read().decode()
            return resp.status, json.loads(raw) if raw else {}, raw
    except urllib.error.HTTPError as err:
        raw = err.read().decode()
        try:
            body = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            body = {"raw": raw}
        return err.status, body, raw

status, minted, _ = call("POST", "/v1/agent-tokens", {})
if status != 201 or not minted.get("access_token"):
    sys.exit(f"agent token {status} {minted}")
token = minted["access_token"]

status, body, raw = call("POST", "/orp/monetize", {"text": draft, "surface": "x", "language": "en"}, token)
offers = body.get("offers") or []
if status != 200 or not (1 <= len(offers) <= 3):
    sys.exit(f"expected 1 to 3 offers, got {status} {raw}")
if body.get("human_prompt") != "You can monetize this, want to try?":
    sys.exit(f"human_prompt {body.get('human_prompt')!r}")
if body.get("mode") != "ask" or body.get("needs_human_confirm") is not True:
    sys.exit(f"default mode was not ask: {body}")
first = offers[0]
for key in ("offer_id", "title", "why_match", "tracked_link", "disclosure_line"):
    if not first.get(key):
        sys.exit(f"missing {key} in {first}")
if "/v1/m/sg" not in first["tracked_link"]:
    sys.exit(f"tracked link {first['tracked_link']}")
for secret in secrets:
    if secret in raw:
        sys.exit(f"response kept {secret}")

link = first["tracked_link"]
req = urllib.request.Request(link, method="GET")
class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None
opener = urllib.request.build_opener(NoRedirect)
try:
    opener.open(req, timeout=30)
    sys.exit("tracked link did not redirect")
except urllib.error.HTTPError as err:
    if err.code != 302 or not str(err.headers.get("Location", "")).startswith("https://"):
        sys.exit(f"redirect {err.code} {err.headers.get('Location')}")

# An empty placement is rejected and does not publish. It does attach the
# publisher site a join token needs before it can set a step to off.
status, placed, _ = call("POST", "/v1/placements", {}, token)
if status != 400:
    sys.exit(f"empty placement {status} {placed}")
status, saved, raw = call("PUT", "/v1/step-modes", {"placement_publish": "off"}, token)
if status != 200:
    sys.exit(f"could not set placement_publish off: {status} {raw}")
status, blocked, raw = call("POST", "/orp/monetize", {"text": draft, "surface": "x", "language": "en"}, token)
if status != 200 or blocked.get("offers") or blocked.get("reason") != "step_off":
    sys.exit(f"mode off still suggested: {status} {raw}")
for secret in secrets:
    if secret in raw:
        sys.exit(f"off response kept {secret}")

dsn = os.environ.get("DATABASE_URL", "")
if dsn:
    import subprocess
    sql = """
SELECT COUNT(*) FROM monetization_suggestions
WHERE keywords::text ILIKE '%secret.example%'
   OR keywords::text ILIKE '%ada_private%'
   OR draft_sha256 ILIKE '%secret.example%'
   OR lang ILIKE '%secret.example%';
"""
    out = subprocess.check_output(["psql", dsn, "-tA", "-c", sql], text=True).strip()
    if out != "0":
        sys.exit(f"draft personal data is stored ({out})")

print(f"d097 smoke ok offers={len(offers)} mode_off=0 link={first['tracked_link']}")
PY
