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
