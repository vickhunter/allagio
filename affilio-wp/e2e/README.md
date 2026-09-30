# Affilio WP Browser E2E

Browser-level Playwright tests for WordPress admin flows.

## Prerequisites
- Running WordPress site with Affilio plugin installed and activated.
- Admin credentials for that site.
- Node.js 18+.

## Environment
- `WP_BASE_URL` (example: `https://yourblog.com`)
- `WP_ADMIN_USER`
- `WP_ADMIN_PASS`
- Optional: `AFFILIO_E2E_API_KEY` for connected-flow test.

## Run
```bash
cd /Users/victormotricala/Desktop/Developer/affilio/affilio-wp/e2e
npm install
npm run install:browsers
WP_BASE_URL="https://yourblog.com" \
WP_ADMIN_USER="admin" \
WP_ADMIN_PASS="password" \
AFFILIO_E2E_API_KEY="your-site-api-key" \
npm test
```

## Coverage
- Settings/onboarding page visibility.
- Disconnected banner path.
- Connected dashboard render path (when API key env is provided).

The settings test now checks the Connect workflow and opens the manual-settings
accordion only for the legacy credential tests. Tests run serially because they
share WordPress options. Use an isolated WordPress fixture.

`AFFILIO_E2E_SETUP=1` additionally verifies Connect, published-post scan progress
and the resulting dashboard without copying a credential. Only enable it for a
fresh isolated site with this branch's backend. The fixture needs a post titled
`Best developer hosting tools`. WordPress cron must run; an optional
`AFFILIO_E2E_CRON_COMPOSE` points at the isolated `affilio-onboarding-qa` Docker
Compose file and runs the queued event through its `cli` service. An optional
`AFFILIO_E2E_HOSTMAP` supplies Chromium host-resolution rules for a local test
domain. No live publishing or payments occur in this test.
