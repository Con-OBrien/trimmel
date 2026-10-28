# Plus checkout: setup

How the website sells Trimmel Plus. Code: `docs/api/` (Vercel functions, no dependencies), page: `docs/account.html`, tests: `tests/site/api.test.mjs` (`node --test tests/site/api.test.mjs`).

Payments use **Stripe Managed Payments**: Stripe is the merchant of record, so it collects and remits VAT and sales tax, handles fraud and disputes, and emails the receipt. The customer sees the purchase as "Sold through Onelink". API reference: <https://docs.stripe.com/payments/managed-payments/set-up>.

## Flow

1. Buyer opens `/account.html` and signs in with Google (Google Identity Services). `POST /api/auth/google` verifies the ID token with Google and sets a signed `trimmel_session` cookie (HttpOnly, Secure, SameSite=Lax, 30 days).
2. "Continue to checkout" calls `POST /api/checkout`, which creates a Stripe Checkout Session (`mode=payment`, `managed_payments[enabled]=true`, API version `2026-04-22.dahlia`) carrying the Google account id, and returns its URL.
3. Stripe takes payment and sends `checkout.session.completed` (or `checkout.session.async_payment_succeeded` for delayed methods) to `POST /api/webhooks/stripe`. The function verifies `Stripe-Signature`, checks the session is paid and tagged `metadata[product]=trimmel-plus`, mints a lifetime key (`TRIM1-...`, 3 PCs, up to `LICENCE_MAJOR`) and stores it in Upstash Redis against the account.
4. The buyer returns to `/account.html?paid=1`, which polls `/api/me` until the key appears, then shows it with copy and activation steps.
5. A full refund (`charge.refunded` with `refunded: true`) marks the order refunded: the account page stops showing the key and the user may buy again. A partial refund keeps the key. A key already issued cannot be revoked (offline check); that is accepted.

## One-time setup

1. **Licence key pair.** Done: the public key is in `internal/license/publickey.go`. The private key (`plus-private.key`, gitignored) must be backed up and is the value of `LICENCE_PRIVATE_KEY`. Replacing it orphans every key already sold.
2. **Google.** Cloud console > Credentials > OAuth client ID, type *Web application*. Authorised JavaScript origins: `https://trimmel.app` (and the preview origin if you test there). Not the desktop client the app uses.
3. **Stripe.**
   1. Activate Managed Payments at <https://dashboard.stripe.com/settings/managed-payments> and accept the terms. Check the eligibility rules (digital software is eligible).
   2. Product catalog > Create product "Trimmel Plus", one-time price, and pick a product tax code labelled *Eligible for Managed Payments* (software/downloads). Copy the **price id** (`price_...`).
   3. Developers > API keys: use a **restricted key** that can write *Checkout Sessions* only (or the secret key in test mode while trying it). This is `STRIPE_SECRET_KEY`.
   4. Developers > Webhooks > Add endpoint: URL `https://trimmel.app/api/webhooks/stripe`, API version `2026-04-22.dahlia`, events `checkout.session.completed`, `checkout.session.async_payment_succeeded`, `charge.refunded`. Copy the signing secret (`whsec_...`) as `STRIPE_WEBHOOK_SECRET`.
   5. Do all of this in **test mode / a sandbox** first. Test and live have separate keys, prices and webhook secrets.
4. **Upstash.** In the Vercel project: Storage > add Upstash Redis (Marketplace). It sets `KV_REST_API_URL` and `KV_REST_API_TOKEN`.
5. **Vercel environment variables** (Production):

   | Name | Value |
   | --- | --- |
   | `SITE_URL` | `https://trimmel.app` |
   | `SESSION_SECRET` | 32+ random characters (`node -e "console.log(require('crypto').randomBytes(32).toString('hex'))"`) |
   | `GOOGLE_WEB_CLIENT_ID` | from step 2 |
   | `STRIPE_SECRET_KEY`, `STRIPE_PRICE_ID`, `STRIPE_WEBHOOK_SECRET` | from step 3 |
   | `LICENCE_PRIVATE_KEY` | contents of `plus-private.key` (base64, one line) |
   | `LICENCE_MAJOR` | optional, default `1` (highest app major version a key covers) |

   The Vercel project's root directory must be `docs`, so `docs/api` becomes `/api`. Redeploy after changing variables.
6. **Test** in Stripe test mode with card 4242 4242 4242 4242: sign in, buy, see the key, paste it into a build that carries the public key. Test-mode orders mint real working keys, so only you can create them; the stored order has `testMode: true`. Then issue a refund in the Stripe dashboard and check the key disappears from the account page. If `charge.refunded` does not fire for Managed Payments refunds, tell me and I will switch to another refund event.

## Before going live

- Swap to live-mode Stripe keys, price id and webhook secret (the live webhook is a separate endpoint entry).
- Rewrite `docs/plus.html` ("Not on sale yet", price, "how it is planned to work") and link `account.html` from it, the home page and the app's Settings (`OpenPlusPage` in `app.go` opens `plus.html`).
- Do not release an app that carries the move allowance (already in `main`) until the checkout is live, or free users get limited with no way to upgrade.
- Receipts come from Stripe (Onelink). The key is on the account page only. Add an email step (for example Resend) if you want the key sent too.
- Stripe can refund customers within 60 days without your approval if you do not answer a support escalation within 48 hours, so keep the support email in the Stripe dashboard current.

## Known gaps

- No rate limiting on the API (Vercel's platform limits only).
- One order per account; a second licence for another person needs another Google account.
- Refunded keys keep working offline.
- Managed Payments checkout does not support custom domains, so buyers pay on a Stripe page.
