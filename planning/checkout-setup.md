# Plus checkout: setup

How the website sells Trimmel Plus. Code: `docs/api/` (Vercel functions, no dependencies), page: `docs/account.html`, tests: `tests/site/api.test.mjs` (`node --test tests/site/api.test.mjs`).

## Flow

1. Buyer opens `/account.html` and signs in with Google (Google Identity Services). `POST /api/auth/google` verifies the ID token with Google and sets a signed `trimmel_session` cookie (HttpOnly, Secure, SameSite=Lax, 30 days).
2. "Continue to checkout" calls `POST /api/checkout`, which creates a Lemon Squeezy checkout carrying the Google account id (`custom.user_sub`) and email, and returns its URL.
3. Lemon Squeezy takes payment and tax, then sends `order_created` to `POST /api/webhooks/lemonsqueezy` (HMAC-verified). The function mints a lifetime key (`TRIM1-...`, 3 PCs, up to `LICENCE_MAJOR`) and stores it in Upstash Redis against the account.
4. The buyer returns to `/account.html?paid=1`, which polls `/api/me` until the key appears, then shows it with copy and activation steps.
5. `order_refunded` marks the order refunded: the account page stops showing the key and the user may buy again. A key already issued cannot be revoked (offline check); that is accepted.

## One-time setup

1. **Licence key pair.** Run `go run ./tools/licensegen keygen -out plus-private.key`. Back the file up. Paste the printed public key into the app build (`-X trimmel/internal/license.publicKeyB64=...` in `release.yml`, or `internal/license/publickey.go`). Losing or replacing the private key orphans every key already sold.
2. **Google.** Cloud console > Credentials > OAuth client ID, type *Web application*. Authorised JavaScript origins: `https://trimmel.app` (and the preview origin if you test there). Not the desktop client the app uses. Copy the client ID.
3. **Lemon Squeezy.** Create a store and a product with one *single payment* variant (the Plus price; set 30-day refunds in your policy). Note the store id and variant id. Create an API key. Add a webhook to `https://trimmel.app/api/webhooks/lemonsqueezy` for `order_created` and `order_refunded`, with a signing secret you choose. Start in *test mode*.
4. **Upstash.** In the Vercel project: Storage > add Upstash Redis (Marketplace). It sets `KV_REST_API_URL` and `KV_REST_API_TOKEN`.
5. **Vercel environment variables** (Production):

   | Name | Value |
   | --- | --- |
   | `SITE_URL` | `https://trimmel.app` |
   | `SESSION_SECRET` | 32+ random characters (`node -e "console.log(require('crypto').randomBytes(32).toString('hex'))"`) |
   | `GOOGLE_WEB_CLIENT_ID` | from step 2 |
   | `LS_API_KEY`, `LS_STORE_ID`, `LS_VARIANT_ID`, `LS_WEBHOOK_SECRET` | from step 3 |
   | `LICENCE_PRIVATE_KEY` | contents of `plus-private.key` (base64, one line) |
   | `LICENCE_MAJOR` | optional, default `1` (highest app major version a key covers) |

   The Vercel project's root directory must be `docs`, so `docs/api` becomes `/api`.
6. **Test** with Lemon Squeezy test mode and card 4242 4242 4242 4242: sign in, buy, see the key, paste it into a build that carries the public key. Test-mode orders mint real working keys, so only you can create them; check `testMode` in the stored order.

## Before going live

- Switch Lemon Squeezy to live mode and update `LS_*` values and the webhook secret.
- Rewrite `docs/plus.html` ("Not on sale yet", price, "how it is planned to work") and link `account.html` from it, the home page and the app's Settings (`OpenPlusPage` in `app.go` opens `plus.html`).
- Do not release an app that carries the move allowance (already in `main`) until the public key is embedded and this checkout is live, or free users get limited with no way to upgrade.
- Lemon Squeezy emails a receipt, not the key. The key lives on the account page. Add an email step (for example Resend) if you want the key sent too.

## Known gaps

- No rate limiting on the API (Vercel's platform limits only).
- One order per account; a second licence for another person needs another Google account.
- Refunded keys keep working offline.
