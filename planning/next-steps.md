# Next steps (written 2026-10-28)

State: Plus checkout works end to end in Stripe **test mode** (Google sign-in, Stripe Managed Payments checkout, webhook, key on the account page, key bound to the Google account, activation in the app, refund hides the key). PR #21 (Upgrade button, price and Plus copy, support email) is open and not merged. Setup details: `planning/checkout-setup.md`.

## 1. Before taking real money

- [ ] **Back up `plus-private.key`** (password manager or offline copy). It is the only copy of the signing key; losing it orphans every key sold. Also stored as `LICENCE_PRIVATE_KEY` in Vercel.
- [ ] **Wrong-account test:** remove the key in the app, sign in as a different Google account, paste the same key. Expect "that key belongs to a***@... Sign in to Trimmel with that Google account".
- [ ] **Test on a real PC (never verified):**
  - [ ] Scheduled cleanup: switching it on creates the `Trimmel\Scheduled cleanup` task; run it from Task Scheduler.
  - [ ] Alerts: `Trimmel\Alerts check` task is created, the toast appears, its button opens Trimmel.
  - [ ] Move apps allowance: free build refuses a second move or more than 10 GB, with the message pointing to Plus.
- [ ] **Stripe, live mode** (separate from test mode; do each in live):
  - [ ] Complete account activation if Stripe still asks for anything.
  - [ ] Activate Managed Payments in live mode.
  - [ ] Create the Trimmel Plus product and £30 one-time price with an "Eligible for Managed Payments" tax code. Note the live `price_...` ID.
  - [ ] Create a restricted key (Checkout Sessions: Write). Note the `rk_live_...` key.
  - [ ] Add a live webhook destination (Snapshot, API `2026-04-22.dahlia`, events `checkout.session.completed`, `checkout.session.async_payment_succeeded`, `charge.refunded`, URL `https://trimmel.app/api/webhooks/stripe`). Note its `whsec_...`.
  - [ ] Set `support@trimmel.app` as the support email in business settings.
  - [ ] Check the price shows £30 including tax at checkout for a UK address.
- [ ] **Vercel Production variables:** replace `STRIPE_SECRET_KEY`, `STRIPE_PRICE_ID`, `STRIPE_WEBHOOK_SECRET` with the live values, then redeploy. Keep the test values written down for later testing.
- [ ] **Make one real purchase, then refund it** (pay £30 with your own card, check the key appears and works in the app, then refund in the live dashboard and confirm the key disappears).
- [ ] **Merge PR #21** (Upgrade button and copy).
- [ ] Check CI on the merged `main` is green.

## 2. Release the app with Plus

The move allowance is already on `main`: any build limits free users, so release only once the checkout above is live.

- [ ] Add GitHub repo secrets `TRIMMEL_GOOGLE_CLIENT_ID` and `TRIMMEL_GOOGLE_CLIENT_SECRET` (the desktop OAuth client, same values as `.env.local`).
- [ ] Bump `productVersion` in `wails.json` and `installer/wails.json` (the release workflow checks the tag matches both). Suggested `0.4.0`; the licence covers up to major 1 (`LICENCE_MAJOR`).
- [ ] Update `docs/changelog.html` and the version shown in the README badge and `docs/index.html` JSON-LD.
- [ ] Merge to `main`, then `git tag v0.4.0 && git push origin v0.4.0`.
- [ ] Watch the Release workflow, then check the GitHub Release has `Trimmel-Setup.exe` and `Trimmel-Portable.zip`, `docs/downloads/version.json` updated on `main`, and trimmel.app offers the new version.
- [ ] Install the released build on a clean machine or VM and run the whole flow once: sign in, Settings, Upgrade, buy, paste key.

## 3. After launch

- [ ] **Replying as support@trimmel.app:** set up Resend (or Brevo) SMTP and add a Gmail "Send mail as" address.
- [ ] **Emailing the key** after purchase (Stripe only emails the receipt). Needs the same email service.
- [ ] **Avast and other antivirus warnings:** submit false-positive reports for each release; signing helps most. Certum Open Source signing was about €49 a year; deferred until Plus has sold (see memory `defer-large-spend`).
- [ ] **Remove the old SignPath mentions** if any remain (none in the repo as of this date).
- [ ] Consider rate limiting on `/api/*` if abuse shows up in Vercel logs.
- [ ] Phase 2 features from `planning/premium-plan.md` (history log, duplicate finder, large and old files, driver installs with a restore point) once there are sales.
- [ ] Decide the open-core question (`premium-plan.md` section 5). The Plus code is in the MIT repo today, and the site says so.

## 4. Local housekeeping

- [ ] Delete `build\bin\trimmel-plus-test.exe` and the stale `build\bin\trimmel.exe` (23 Oct build) once the release is out. The test exe contains your Google desktop client secret.
- [ ] Keep `.env.local` and `plus-private.key` out of git (both are ignored; do not move them).
- [ ] When testing again later, switch Vercel back to the test Stripe values or use a Vercel Preview environment with test keys.

## Known limits (accepted)

- Keys work offline, so a refunded or shared key keeps working on a PC that already activated it. The 3-PC limit and account binding are checked at activation only.
- One order per Google account on the website.
- No rate limiting beyond Vercel's platform limits.
