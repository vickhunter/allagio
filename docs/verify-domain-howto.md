# Prove domain verification on a host you control

`POST /v1/sites/:id/verify` marks a publisher site verified when that site's domain serves its `verification_token`. This note is how to prove that path. Run it only against a domain you control. Do not point it at a domain you do not operate.

The local proof does not use the public network. `TestDomainVerificationTokenServedThenConflict` starts an `httptest` TLS server, serves the token at `/.well-known/offer-rails.txt`, and dials that server through the verification client. The first verify is HTTP 200 with `verified: true`. The second verify is HTTP 409 with `reason` `domain_verified`, and it does not fetch again.

```bash
cd affilio-api
# AFFILIO_TEST_DATABASE_URL must point at the test Postgres.
go test ./internal/handlers -count=1 -run TestDomainVerificationTokenServedThenConflict
```

## Hosted proof

Use a domain whose DNS you can change, and an HTTPS origin on that exact host. The checker requests `https://<domain>/.well-known/offer-rails.txt` and, only if that file does not contain the token, `https://<domain>/`. A redirect that changes host, scheme, or port is not proof.

1. Create a publisher account for that domain.

```bash
curl -sS -X POST "$BASE/v1/agent-accounts" \
  -H 'Content-Type: application/json' \
  -d '{"role":"publisher","name":"Proof","email":"you@your-domain.example","domain":"your-domain.example"}'
```

Keep the returned `site_id`, `api_key`, and `verification_token`. The token is 48 characters. Do not commit it or paste it into a shared log.

2. Serve the token on that domain. The body is `text/plain`. The token is the whole body, or one line of its own. A WordPress site with this plugin does that when both `affilio_api_key` and `affilio_verification_token` are set: `/.well-known/offer-rails.txt` returns the token and a newline. A static host can publish the same file. The home-page alternative is `<meta name="offer-rails-verification" content="THE_TOKEN">`.

3. Confirm the file before you call verify.

```bash
curl -sS -D- "https://your-domain.example/.well-known/offer-rails.txt"
```

The body must contain the token on its own line, and the final URL must still be that host on port 443.

4. Ask the API to check it. Send the site key. The path id must be that site.

```bash
curl -sS -D- -X POST "$BASE/v1/sites/$SITE_ID/verify" \
  -H "X-API-Key: $SITE_KEY" \
  -H 'Content-Type: application/json' \
  -d '{}'
```

A match is HTTP 200. `verified` is true and `verified_at` is an RFC3339 timestamp.

5. Call the same request again. The response is HTTP 409:

```json
{"reason":"domain_verified","message":"This domain is already verified."}
```

That second call does not fetch the domain. A second site signed up on the same host gets the same 409 and also does not fetch.

A missing token is HTTP 422 `verification_missing`. A page that cannot be read is HTTP 422 `verification_unreachable`. Neither is a 500, and neither includes a resolved address.

`GET /v1/sites/current` with the site key returns `verified` and `verified_at` after the 200.
