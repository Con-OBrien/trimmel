# Trimmel: SEO plan

Status: draft, written 2026-10-22. Nothing here is built yet.
This file lives in `planning/`, not `docs/`, because `docs/` is published as the public website.

## 1. Where we start

Audit of `docs/` (one page, `index.html`, plus `privacy.html`), live at https://trimmel.vercel.app:

| Item | State |
| --- | --- |
| `<title>` and meta description | Present and decent. Title leads with the brand, not a search term. |
| Headings | One `h1` ("Free up space. Speed up startup. Undo anything."), `h2` per section. No keyword in the `h1`. |
| Images | `alt` text present, width and height set, lazy loading. Screenshots are PNG (90 to 170 KB each). |
| `robots.txt` | Missing (404). |
| `sitemap.xml` | Missing (404). |
| Canonical URL | Missing. |
| Open Graph and Twitter card tags | Missing. Links shared on social or chat show no preview image. |
| Structured data (JSON-LD) | Missing. |
| Pages | One landing page. Everything competes for one URL. |
| Brand name | "Trimmel" has no search meaning yet. Old name "PC Optimizer" has history but is generic. |
| Search Console | The old verification file exists. The new URL is not yet a verified property (see the OAuth home page error). |
| Domain | `*.vercel.app`. Fine for now, but a custom domain would be better. Google's OAuth review also dislikes `vercel.app`. |
| Core Web Vitals | Not measured. Single file with inline CSS and a Google Fonts request, so probably good. |

## 2. Goals and honest expectations

1. Be found for the **brand** ("Trimmel", "Trimmel Windows") within weeks. Easy once the page is indexed.
2. Be found for **problem searches** ("free up disk space Windows 11", "move installed program to another drive", "disable startup programs Windows"). These are crowded, but long-tail versions are winnable with good guides.
3. Do not chase "PC optimizer" or "PC cleaner" head terms. They are owned by scareware vendors and large brands. Our trust story is the differentiator, so we should rank for honest, specific queries instead.

Realistic timeline: indexing in days, brand ranking in 2 to 4 weeks, first long-tail traffic in 2 to 4 months. SEO is slow.

## 3. Phase 0: fix the basics (one afternoon)

All in `docs/`. No design changes.

1. **Search Console.** Add `https://trimmel.vercel.app/` as a URL-prefix property, verify, submit the sitemap. Add Bing Webmaster Tools (import from Search Console).
2. **`robots.txt`**: allow all, point at the sitemap. Do not block `/downloads/`.
3. **`sitemap.xml`**: the home page, privacy page and any new guides, with `lastmod`.
4. **Canonical** `<link rel="canonical">` on every page, set to the final domain.
5. **Open Graph and Twitter tags**: `og:title`, `og:description`, `og:image` (1200 x 630, a banner with the logo and the three-line promise), `og:url`, `twitter:card=summary_large_image`.
6. **JSON-LD**: `SoftwareApplication` (name, operatingSystem "Windows 10, Windows 11", applicationCategory "UtilitiesApplication", offers price 0, downloadUrl, softwareVersion from `version.json`) and `FAQPage` for the existing "Quick answers" block. Keep them true: no rating or review markup until real reviews exist.
7. **Title and `h1`**: keep the brand, add the plain description, for example "Trimmel: free Windows disk cleanup, startup manager and app mover". Keep the current three-line promise as the visible hero, but make sure the page also says "Windows" and "disk space" in the first paragraph.
8. **Privacy page**: add a description meta tag and a link back to the home page.
9. **Fonts**: keep `preconnect`. Consider self-hosting the two fonts (same family the app uses) to remove a third-party request.
10. **Images**: convert screenshots to WebP or AVIF with PNG fallback, keep dimensions.
11. **Rename leftovers**: the FAQ still mentions "PC Optimizer 0.1". Keep one clear line (people searching the old name will land here), but nothing else.

## 4. Phase 1: content that earns search traffic (weeks 1 to 4)

Add `docs/guides/` with a handful of honest, specific pages. Each solves one problem, shows the manual way first, then how Trimmel does it safely. Short, accurate, screenshots from the real app.

Candidate pages, ordered by value:

1. How to move an installed program to another drive on Windows (junctions, risks, undo). Our strongest differentiator.
2. How to free up disk space on Windows 11 (what is safe to delete, what is not).
3. What to disable at startup on Windows (a plain-English list; ties to the startup advice data in `internal/startup/advice.go`).
4. Is it safe to delete `SoftwareDistribution\Download`, `Temp`, shader caches (one page per folder if traffic shows demand).
5. How to update drivers safely without a driver-updater tool (why Windows Update is enough).
6. Trimmel vs Microsoft PC Manager vs BleachBit (honest comparison; links to them).

Rules for every guide: one `h1` with the exact question, a short answer in the first paragraph, a "last checked on Windows 11 <build>" date, links to the app only where it fits, and no claims we have not tested.

Also: a **changelog** page (`/changelog`) from release notes. It gives fresh content on every release and a place for "what's new".

## 5. Phase 2: off-site (ongoing, small)

1. **GitHub**: complete the repo description, topics (`windows`, `disk-cleanup`, `startup-manager`, `wails`, `go`), a link to the site, and a good README with screenshots. GitHub ranks well and passes trust.
2. **Directories**: submit to AlternativeTo (as an alternative to CCleaner, BleachBit, PC Manager), Softpedia, MajorGeeks, Chocolatey and winget (winget needs a signed installer, so after code signing). Each gives a backlink and a discovery channel.
3. **Communities**: a "Show HN" and posts in relevant subreddits only when the app is signed and the page is solid. One honest launch post beats repeated promotion.
4. **Reviews**: ask early users for Trustpilot or AlternativeTo reviews only after they have used it, never in the app by default.
5. **Microsoft Store listing** (if we go that way): the Store page ranks on its own and brings branded search.

## 6. Technical decisions

| Decision | Recommendation |
| --- | --- |
| Custom domain | Buy one (about £10 to £15 a year, for example `trimmel.app`). Better for trust, for the Google OAuth consent screen and for the licence emails. Redirect the old `vercel.app` URL with a 301. Do it before the guides, so links accumulate on the final domain. |
| Static generation | Stay with hand-written HTML for now. Move to a small static site generator (Astro or Eleventy) when there are more than about five pages, so the head tags, nav and footer come from one template. |
| Analytics | Cookie-free and privacy-respecting (Plausible, Umami self-hosted, or Vercel Analytics). Update `privacy.html`. Do not add Google Analytics: it conflicts with the privacy promise. |
| Downloads | Keep `Trimmel-Setup.exe` as a plain link, add the SHA-256 on the page. Search engines and security scanners trust a page that shows a checksum and a signed installer. |
| `noindex` | Do not index `/downloads/`. The exe and zip are not pages. Add `X-Robots-Tag` headers in `vercel.json` for them. |

## 7. Measuring it

- Search Console: indexed pages, impressions and clicks for the brand and for each guide.
- Position tracking for 10 target queries, checked monthly by hand.
- Downloads per month from `version.json` plus analytics events on the download button.
- Share of visits that reach the download button, per guide. A guide that ranks but never converts needs a rewrite.

## 8. Risks

- **Thin or overclaiming content** hurts rankings and trust. Every claim in a guide must be something we have tested.
- **Unsigned installer warnings** drive visitors away even when SEO works. Code signing comes first (see the pricing discussion).
- **Brand collision**: check search results for "Trimmel" once indexed. If other products dominate, lean on "Trimmel for Windows" in titles.
- **Scareware neighbourhood**: the "PC optimizer" results are full of low-trust sites. Do not copy their patterns (fake scans, countdowns, inflated counts). Our plain tone is the edge.

## 9. Order of work

1. Search Console and Bing verification (needed anyway for the OAuth home page check).
2. Phase 0 items 2 to 8 and 11 (about half a day, I can do these).
3. Decide on the custom domain, then switch.
4. Write guides 1 to 3, publish, submit in Search Console.
5. GitHub polish and directory submissions.
6. After code signing and the next signed release: winget, Store, launch posts.

## 10. Progress log

### Phase 0 (done 2026-10-22, uncommitted when written)

Done in `docs/`: `robots.txt`, `sitemap.xml`, canonical tags, Open Graph and Twitter tags, `og.png` (1200 x 630), JSON-LD (`SoftwareApplication`, `FAQPage` copied from the visible FAQ), new title and hero line with "Windows", "disk cleanup", "startup manager", "app mover", WebP screenshots with PNG fallback (about 60 percent smaller), `fetchpriority="high"` on the hero image, `noindex` header for `/downloads/` in `vercel.json`, description and canonical on `privacy.html`.
Not done: self-hosted fonts (left as is), Search Console and Bing verification (needs the owner's account), analytics.
All absolute URLs use `https://trimmel.vercel.app`. When the custom domain is live, replace it in `docs/index.html`, `docs/privacy.html`, `docs/robots.txt`, `docs/sitemap.xml` and `internal/appinfo/appinfo.go` (`SiteURL`), and add a 301 from the old host.

### Domain availability (RDAP lookup, 2026-10-22; confirm at the registrar)

| Name | Result |
| --- | --- |
| trimmel.com, trimmel.net, trimmel.org, trimmel.co.uk | Taken |
| trimmel.app, trimmel.dev, trimmel.io, trimmel.co | No registration found, probably available |
| gettrimmel.com, usetrimmel.com, trimmelapp.com | No registration found, probably available |

`trimmel.app` is the best fit: short, clearly a software product, and `.app` is HTTPS-only. Check the live price on Cloudflare Registrar before buying.

### Domain (2026-10-22)

`trimmel.app` bought on Cloudflare, DNS only, pointed at Vercel. `www.trimmel.app` works. The apex `trimmel.app` returned no DNS answer when last checked (Cloudflare nameservers had no apex record), so the canonical host is **`https://www.trimmel.app`** for now. The old `trimmel.vercel.app` host redirects (301) to it through `docs/vercel.json`. Once the apex resolves and Vercel serves it directly, switch the canonical URLs to `https://trimmel.app` (same files as the earlier swap plus `internal/appinfo/appinfo.go`) and make `www` redirect to the apex.

### Progress (2026-10-22, later)

- Phase 1 started: three guides and a guides index in `docs/guides/` (move a program, free disk space, startup programs), with Article JSON-LD, added to the sitemap and linked from the home page nav and footer. The `robocopy` / `mklink /J` recipe was run once on a temp folder to check it works.
- GitHub: description, homepage (`https://www.trimmel.app`), topics and Discussions set. Discussion #1 asks about Trimmel Plus and is linked from the home page.
- Analytics: `docs/analytics.js` loads Umami (cookie-free) once an ID is set; it is inert now. Download buttons already carry `data-umami-event` attributes. Before enabling, add a "Website analytics" section to `privacy.html`.
- Directory copy and checklist: `planning/directory-submissions.md`.
