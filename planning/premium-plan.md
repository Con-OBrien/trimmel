# Trimmel: premium version plan

Status: draft for discussion, written 2026-10-22. Nothing here is built yet.
This file lives in `planning/`, not `docs/`, because `docs/` is published as the public website.

## 1. Where we start

What exists today (v0.1.0 plus the work on `feat/product-3-4`):

- Overview with a health score, Processes, Cleanup, Storage (analyser with suggestions), Startup (with scheduled tasks and impact), Apps, Move apps, Old versions, Drivers.
- Free, MIT licensed, and the website says so in three places ("free, open source, MIT", "How much does it cost? Nothing").
- Optional Google sign-in (not present in the published build).
- No code signing, no auto-update, no crash reporting, no payment or licensing code.
- No background component: the app only acts while it is open.

Two facts shape everything below:

1. **Trust is the product.** The app asks for admin rights and deletes files. Users will only pay a tool they trust, and the market's best-known paid cleaner is distrusted for bundling and upsells. Plain English, previews, undo and no nagging are the differentiator, so premium must never weaken them.
2. **We have already promised "free, open source, MIT".** Any premium plan has to respect that promise or change it openly, before anyone pays.

## 2. Principles

1. **Keep every safety feature free.** Previews, reversibility, protected processes, the health score, and the explanations stay free. Nobody should pay to be safe.
2. **Charge for time saved and depth, not for basics.** Free users can do everything once, by hand. Premium removes repetition (automation), adds depth (history, deeper analysis) and covers more than one PC.
3. **No dark patterns.** No scare scans, no fake "issues found" counts, no pop-ups, no nagging banners. One quiet "Plus" entry in Settings and a short, honest note where a locked feature would have been.
4. **The free app must stay good.** Microsoft PC Manager and BleachBit are free. If free Trimmel is worse than them, nobody reaches the paid tier.
5. **Never lock users out of their own data or undo.** If a licence lapses, anything already moved or changed keeps working and can still be undone.

## 3. What goes in each tier

### Free (always)

| Area | Included |
| --- | --- |
| Overview | Health score, suggestions, system summary, tidy reminder |
| Processes | Everything |
| Cleanup | All cleanup targets, run by hand, with sizes and previews |
| Startup | List, impact notes, switch on and off (including scheduled tasks) |
| Storage | Drive analysis, folder drill-down, biggest files, the top 3 suggestions |
| Apps and Old versions | List, sizes, launch each uninstaller |
| Drivers | List installed drivers, check Windows Update for updates |
| Move apps | Move and move back, up to a small allowance (see below) |

### Plus (paid) — the candidates

Grouped by why someone would pay. Priority marks: **A** build first, **B** next, **C** later.

**Automation (A):** the strongest reason to pay, because a free user has to remember.
- Scheduled cleanup (weekly or monthly) that runs quietly and reports what it freed. Needs a small background piece (Windows scheduled task running `trimmel.exe --scheduled`, no always-on service).
- Low-disk alert: a notification when a drive falls below a chosen threshold, with a "Clean now" button.
- Weekly health report notification.

**Move apps without limits (A):** this is the most technically valuable feature and the one with the clearest payoff ("my C: drive is full"). Proposal: free users can move one app, or up to 10 GB in total, with no time limit; Plus is unlimited. Move back is always free.

**Deeper storage tools (B):**
- All suggestions, not just the top 3, plus one-click handling for the safe ones (developer caches, Docker prune guidance, Windows upgrade leftovers).
- Duplicate file finder (hash-based, always previews, only moves to the Recycle Bin).
- Large and old files view (by size and last-used date) with "move to another drive" for personal folders.

**History and undo log (B):** a timeline of everything the app did (cleaned, moved, toggled, installed), exportable as a report. Also supports the support story.

**Driver installs with a safety net (B):** scanning stays free. One-click install includes a restore point, rollback guidance and a "driver quality" note. Reason to gate: it carries risk and support cost.

**Startup extras (B):** delayed start (launch heavy items a minute after sign-in instead of disabling them), boot-time history from the Windows event log, and a "what changed since last week" view.

**Privacy clean (C):** browser history, cookies and download lists. Competitors charge for this; it needs extra care, so it comes later.

**Multi-PC (C):** one licence for 3 PCs. A shared dashboard comes later and needs a cloud service, so it is a separate decision.

**Business/IT (later, separate product line):** silent install, command-line mode, JSON reports, policy-friendly behaviour on managed PCs.

### What I would not charge for

Anything that is a safety feature, plain-English explanations, the health score, ending a task, the welcome guide, accessibility, updates for security fixes, or the ability to undo.

## 4. Pricing and licensing

Open decisions are listed in section 8. My recommendation, to be tested:

- **Plus: about $19.99 per year, or $39 once ("lifetime") for the same major version.** One licence covers 3 PCs.
- **14-day free trial of everything**, no card needed. After the trial the app drops to Free; nothing is deleted or undone.
- A one-off option matters here: utility buyers dislike subscriptions, and the automation features have low server cost. Offer a subscription only if a cloud feature (multi-PC dashboard) ships.
- 30-day refund, no questions.

**How licensing would work (kept simple):**

1. User buys through a Merchant of Record (Paddle, Lemon Squeezy or similar). That handles VAT and sales tax, which a solo seller cannot sensibly do alone.
2. They receive a **signed licence key** (an Ed25519 signature over email, tier, expiry and device limit). The app verifies it offline with a public key built in. No server call is needed to run, so it works offline and respects the privacy policy.
3. An optional online check (at most weekly, never blocking) enforces the 3-PC limit and honours refunds. If it cannot reach the server, the app keeps working.
4. The existing Google sign-in is **not required** for Plus. Decide separately whether sign-in has any purpose (see section 8).

## 5. The open-source question (needs a decision first)

The code is MIT and the website promises "free, open source". A premium feature in MIT code can simply be compiled out of the check by anyone. Options:

1. **Open core.** Keep today's app MIT and public. Build Plus features in a separate private module that the official build links in. Honest and simple to explain. Cost: Plus code must be cleanly separable (a small `premium` interface in `app.go`), and the "free" tier is the auditable one.
2. **Source-available / change of licence for new code.** New Plus code under a source-available licence. Existing MIT code stays MIT forever, which cannot be undone. Needs a clear note on the site.
3. **Stay fully open and sell convenience.** Charge for the signed, supported, auto-updating build and the hosted pieces, not for code. Works if trust matters more than copy protection. Lowest friction with the existing promise.

I recommend **open core (1)**. It keeps the promise for everything that exists today, and it matches the principle that safety features stay free and inspectable. The website copy ("free, open source") changes to "the core is free and open source; Plus adds automation and extra tools" before any paid feature ships.

## 6. Roadmap

**Phase 0: foundations (before charging anyone)**
- Code-sign the exe and installer. Without this, paid conversion will suffer from SmartScreen and antivirus warnings.
- Auto-update and update notices (the settings branch already has an update checker).
- Opt-in crash and error reporting ("copy diagnostics" button first).
- Decide the sign-in question and the licence/open-core question (section 5 and 8).
- Rebuild and republish with the Google client ID if sign-in stays (QA item H3).
- Installer tests and CI.

**Phase 1: licensing and the first Plus feature**
- `internal/license`: verify signed keys, trial state, plan limits. Pure Go, well tested.
- Settings page: "Plus" section, enter key, trial status, a clear list of what Plus adds.
- Scheduled cleanup and low-disk alerts (the automation set), because it is the clearest "I'd pay for that".
- Move apps allowance and the unlimited upgrade.
- Merchant of Record account, checkout page, licence delivery email, refund process, privacy policy update.

**Phase 2: depth**
- History and undo log, duplicate finder, large and old files, full suggestions with one-click handling, driver installs with restore point, startup delay and boot-time history.

**Phase 3: growth**
- Microsoft Store listing as a second channel (handles signing and updates).
- Multi-PC dashboard (only if people ask), Business tier, privacy clean.

## 7. How we will know it works

- Free: installs, 7-day retention, and how many users run Cleanup at least once.
- Trust: crash-free sessions, uninstall rate, support emails about "it deleted something".
- Plus: trial starts, trial-to-paid conversion (a realistic target for a utility is 2 to 5 percent of trial starts), refund rate under 5 percent.
- Which locked feature people click on most (count the clicks locally and report them only with consent).

All telemetry stays opt-in and is described in the privacy policy.

## 8. Decisions needed from you

1. **Open core, source-available or fully open?** (Section 5.) This decides the architecture.
2. **What is Google sign-in for?** Options: drop it, keep it optional for future sync, or keep it as the account for licensing. My lean: drop it as a gate, keep the code for an optional account later.
3. **One-off, subscription, or both?** My lean is both, with the one-off at roughly two years of the annual price.
4. **Price point and currency**, and whether to launch a founder discount.
5. **Where will you sell?** A Merchant of Record account (Paddle, Lemon Squeezy) or the Microsoft Store.
6. **Is the Move apps allowance the right line?** It is the feature most likely to drive upgrades, but also the one free users most want. Alternatives: a time-based trial of unlimited moves, or no allowance.
7. **Do you want a business/IT tier at all** or is this a consumer product?

## 9. Risks

- **Trust loss from any mistake.** One report of "it deleted my files" costs more than all revenue. Keep previews, protection rules and the QA habit (see `qa/`) for every new feature, especially duplicate finder and privacy clean.
- **Licence changes upset the open-source audience.** Mitigate by changing nothing that already exists and saying so plainly.
- **Piracy.** Offline keys can be shared. Accept some loss; the optional online check and the 3-PC limit are enough for a utility.
- **Competing with free.** Microsoft PC Manager is free and built in. Our edge is explanation, reversibility and moving apps, so those must stay clearly better, not merely different.
- **Support load.** Driver installs and moves create support work. Gate them behind safety nets and write the support page before launch.
- **Code signing cost and delay.** Certificate validation can take weeks; start it now.
