import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import time
from urllib.error import HTTPError, URLError
from urllib.parse import parse_qs, urlparse
from urllib.request import Request, build_opener, HTTPRedirectHandler, urlopen


API = "http://127.0.0.1:8086"
ROOT = Path(__file__).resolve().parents[1]
STATE = Path(sys.argv[1]).resolve()
ADMIN = os.environ["ADMIN_API_KEY"]
STRIPE_SECRET = "whsec_" + secrets.token_urlsafe(32)
TRACE = []


def request(path, payload=None, headers=None, method="POST", expected=200):
    body = None if payload is None else json.dumps(payload).encode()
    req = Request(API + path, body, {"Content-Type": "application/json", **(headers or {})}, method=method)
    try:
        with urlopen(req, timeout=10) as response:
            status, data = response.status, json.load(response)
    except HTTPError as error:
        status, data = error.code, json.load(error)
    if status != expected:
        raise RuntimeError(f"{method} {path}: expected {expected}, received {status}: {data.get('error', data.get('status'))}")
    return data


def admin(path, payload=None, expected=200):
    return request("/v1/offer-review/" + path, payload, {"X-Admin-Key": ADMIN}, expected=expected)


def sql(statement):
    subprocess.run(["docker", "compose", "-f", str(ROOT / "docker-compose.poc.yml"), "exec", "-T", "db", "psql", "-U", "affilio", "-d", "affilio_poc", "-v", "ON_ERROR_STOP=1", "-c", statement], check=True, capture_output=True)


class Agent:
    def __init__(self, owner, maker_key="", publisher_key=""):
        env = {key: value for key, value in os.environ.items() if not key.startswith("DISTRIBUTOOR_") and key not in ("ADMIN_API_KEY", "CONNECTION_ENCRYPTION_KEY", "DATABASE_URL")}
        env.update(DISTRIBUTOOR_API_URL=API, DISTRIBUTOOR_API_KEY=maker_key, DISTRIBUTOOR_SITE_KEY=publisher_key)
        self.owner, self.sequence = owner, 0
        self.process = subprocess.Popen([str(STATE / "distributoor-mcp")], env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        self.rpc("initialize", {"protocolVersion": "2025-06-18", "capabilities": {}, "clientInfo": {"name": owner, "version": "0.1"}})
        self.process.stdin.write(json.dumps({"jsonrpc": "2.0", "method": "notifications/initialized"}) + "\n")
        self.process.stdin.flush()

    def rpc(self, method, params):
        self.sequence += 1
        self.process.stdin.write(json.dumps({"jsonrpc": "2.0", "id": self.sequence, "method": method, "params": params}) + "\n")
        self.process.stdin.flush()
        line = self.process.stdout.readline()
        if not line:
            raise RuntimeError(f"{self.owner}: MCP process stopped")
        response = json.loads(line)
        if "error" in response:
            raise RuntimeError(response["error"]["message"])
        return response["result"]

    def call(self, name, args=None):
        result = self.rpc("tools/call", {"name": name, "arguments": args or {}})
        TRACE.append({"owner": self.owner, "tool": name, "ok": not result.get("isError", False)})
        if result.get("isError"):
            raise RuntimeError(f"{self.owner}: {name}: {result['content'][0]['text']}")
        return json.loads(result["content"][0]["text"])

    def close(self):
        self.process.stdin.close()
        self.process.wait(timeout=10)
        if self.process.returncode:
            raise RuntimeError(f"{self.owner}: MCP process failed")


def stripe(product_id, event):
    raw = json.dumps(event).encode()
    timestamp = str(int(time.time()))
    signature = hmac.new(STRIPE_SECRET.encode(), timestamp.encode() + b"." + raw, hashlib.sha256).hexdigest()
    req = Request(API + "/v1/webhooks/stripe/" + product_id, raw, {"Content-Type": "application/json", "Stripe-Signature": f"t={timestamp},v1={signature}"})
    with urlopen(req, timeout=10) as response:
        return json.load(response)


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def click(tracked):
    try:
        build_opener(NoRedirect).open(API + tracked, timeout=10)
    except HTTPError as response:
        if response.code != 302:
            raise
        return parse_qs(urlparse(response.headers["Location"]).query)["via"][0]
    raise RuntimeError("tracked link did not redirect")


def main():
    for _ in range(100):
        try:
            request("/health", method="GET")
            break
        except (URLError, TimeoutError):
            time.sleep(0.1)
    else:
        raise RuntimeError("POC API did not start; inspect .local/agent-poc/server.log")
    owner = request("/v1/makers/register", {"email": "builder@poc.example", "name": "Independent builder"})
    publisher_owner = request("/v1/sites/register", {"domain": "publisher.poc.example", "name": "Independent developer educator"})
    access = admin("agent-access/sites/" + publisher_owner["site_id"], expected=201)
    maker = Agent("Maker agent", maker_key=owner["api_key"])
    publisher = Agent("Publisher agent", publisher_key=access["api_key"])
    try:
        product = maker.call("register_product", {"name": "InboxPilot", "website_url": "https://inboxpilot.poc.example", "product_type": "saas", "payment_provider": "stripe", "tagline": "Email inbox productivity for developers", "keywords": ["email", "inbox", "productivity", "developers"]})
        product_id = product["id"]
        sql(f"UPDATE products SET readiness_status='ready',readiness_score=100 WHERE id='{product_id}'")
        admin("products/" + product_id + "/connection", {"signing_secret": STRIPE_SECRET, "live_mode": False, "currency": "USD"}, expected=201)
        offer = maker.call("create_offer", {"product_id": product_id, "commission_type": "percentage", "commission_value": 20, "refund_window_days": 30, "terms": "POC fixture: 20% of first paid checkout excluding tax/shipping; direct merchant payout after a 30-day hold. Proposed service fee: maker pays 10% of commission separately."})
        offer_id = offer["id"]
        maker.call("submit_offer", {"offer_id": offer_id})
        admin(offer_id + "/approve")
        context = "A developer tutorial about email inbox productivity and evaluating InboxPilot for email workflows."
        opportunities = publisher.call("find_opportunities", {"context": context})
        no_match = publisher.call("find_opportunities", {"context": "Growing tomatoes in garden soil and watering flowers."})
        assert no_match["status"] == "no_match"
        prepared = publisher.call("prepare_placement", {"offer_id": offer_id, "surface_url": "https://publisher.poc.example/email-tutorial", "context": context, "copy": "Consider InboxPilot when evaluating tools for your email inbox productivity workflow."})["placement"]
        placement_id = prepared["id"]
        request("/v1/placements/" + placement_id + "/commit", {"confirm": True}, {"X-API-Key": access["api_key"]}, expected=403)
        admin("placements/" + placement_id + "/approve", {"site_id": publisher_owner["site_id"], "owner_email": "educator@poc.example", "evidence": "SIMULATED owner approval for the local fixture. Exact copy, article URL, disclosure and frozen commercial terms reviewed; no real owner consent claimed."})
        committed = publisher.call("commit_placement", {"placement_id": placement_id})["placement"]
        receipt = committed["receipt"]
        text = prepared["copy"] + "\n\n" + API + receipt["tracked_link"] + "\n\n" + prepared["disclosure"]
        (STATE / "approved-placement.md").write_text(text)
        publisher.call("record_placement_receipt", {"placement_id": placement_id, "surface_url": prepared["surface_url"], "published_text": text, "evidence": "Local approved-placement.md artifact; no public publication."})
        click_id = click(receipt["tracked_link"])
        sql(f"UPDATE clicks SET clicked_at=NOW()-INTERVAL '2 hours' WHERE id='{click_id}'")
        payment = {"id": "evt_poc_paid", "livemode": False, "type": "checkout.session.completed", "created": int(time.time()), "data": {"object": {"id": "cs_poc", "payment_intent": "pi_poc", "payment_status": "paid", "amount_total": 4900, "currency": "usd", "client_reference_id": click_id, "metadata": {"dtr_product_id": product_id}}}}
        assert stripe(product_id, payment)["status"] == "recorded"
        assert stripe(product_id, payment)["status"] == "duplicate"
        paid = publisher.call("get_balance")["balances"]
        earnings = maker.call("get_earnings")
        assert paid[0]["pending"] == 9.8
        refund = {"id": "evt_poc_partial", "livemode": False, "type": "charge.refunded", "created": int(time.time()), "data": {"object": {"id": "ch_poc", "payment_intent": "pi_poc", "amount_refunded": 2450, "currency": "usd"}}}
        assert stripe(product_id, refund)["status"] == "refunded"
        partial = publisher.call("get_balance")["balances"]
        assert partial[0]["pending"] == 4.9
        refund["id"], refund["data"]["object"]["amount_refunded"] = "evt_poc_full", 4900
        assert stripe(product_id, refund)["status"] == "refunded"
        reversed_balance = publisher.call("get_balance")["balances"]
        assert reversed_balance[0]["pending"] == 0
        report = {"title": "From shipped product to attributable revenue", "mode": "Local proof · synthetic signed Stripe events", "product": product, "opportunities": opportunities, "no_match": no_match, "placement": publisher.call("get_placement", {"placement_id": placement_id})["placement"], "balances": {"paid_event": paid, "half_refund": partial, "full_refund": reversed_balance}, "maker_earnings_before_refund": earnings, "trace": TRACE, "proof": ["Two separately keyed MCP processes", "Unapproved commit rejected", "Exact approved copy and disclosure saved as a local artifact", "One $49 paid-event fixture produces $9.80 held commission", "Duplicate event does not double-book", "50% refund leaves $4.90; full refund leaves $0"], "limits": ["No real Stripe checkout or bank payout", "Owner approval simulated by the local operator", "No public article or social post was published", "Publication evidence is publisher-reported", "Readiness was seeded for the fixture; no live product assessment", "Client transport tested with local MCP processes, not Cursor/Grok/Muse/dots/OpenClaw"]}
        (STATE / "report.json").write_text(json.dumps(report, indent=2))
        encoded = base64.b64encode(json.dumps(report).encode()).decode()
        template = (ROOT / "affilio-wp/admin/build/agent-poc.html").read_text()
        (STATE / "report.html").write_text(template.replace("__POC_REPORT_BASE64__", encoded))
        config = {"mcpServers": {"distributoor-revenue": {"command": str(STATE / "distributoor-mcp"), "env": {"DISTRIBUTOOR_API_URL": API, "DISTRIBUTOOR_SITE_KEY": access["api_key"]}}}}
        (STATE / "publisher.mcp.json").write_text(json.dumps(config, indent=2))
        print("Two MCP agents completed the revenue loop. Proof and local placement saved in .local/agent-poc/.")
    finally:
        maker.close()
        publisher.close()


if __name__ == "__main__":
    main()
