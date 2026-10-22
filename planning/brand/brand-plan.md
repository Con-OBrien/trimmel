# Brand plan: from "PC Optimizer" to a name and identity we can own

Status: draft, 2026-10-22. Nothing in the app has been renamed or restyled yet.
Open `brand-board.html` in a browser to see the identity. Assets are in this folder.

## 1. Why change

- **The current name cannot be owned.** "PC Optimizer" is a description, not a name. A search for the exact term turns up several products and at least one USPTO record for PCOPTIMIZER (I have not checked its owner or status). A generic name is hard to trademark, hard to search for, and easy to confuse with the many scareware "PC optimizer" tools, which is exactly the company we do not want to keep.
- **The look is generic.** Cream background, teal buttons, a lightning bolt in a gauge. It says "utility", not who we are.
- **The product already has a point of view** that the brand never states: plain English, nothing done behind your back, everything undoable. The brand should be that.

## 2. Positioning

> For people who own a Windows PC and are not sure what is safe to remove, Fettle is a calm, plain-spoken tidy-up tool that shows you everything first and lets you undo anything. Unlike the cleaners that scare you into clicking, it only does what you choose.

Three pillars, used everywhere from the website to error messages:

1. **Plain English.** Say what a thing is and what happens if it goes.
2. **Nothing behind your back.** Show first, act only on what is ticked.
3. **Undo anything.** Every change says how to reverse it.

Personality: a capable friend who has seen this before. Calm, concrete, a little dry. It never panics, never apologises, never sells.

## 3. The name

**Working name: Fettle.** To fettle is to trim the rough edges off a casting until it is finished (a foundry and pottery word). "In fine fettle" means in good condition. Short, easy to say and spell, specific in meaning, and it sounds nothing like "optimizer" or "booster".

### Clearance status: not cleared

What I found (quick checks only, not legal advice):

| Check | Result |
| --- | --- |
| Software search for "Fettle" PC cleaner | No Windows cleaner by that name turned up. |
| US trademark | A live **FETTLE** record exists in Class 42 (software design and development services) and another in Class 44 (health and beauty). Class 42 is close enough to software that this needs a proper look before we commit. |
| Domains | fettle.com, fettle.app, getfettle.com and usefettle.com are registered by others. **fettlepc.com, fettle.pc and fettlewin.com looked unregistered** (a DNS check; confirm with a registrar). |
| UK/EU marks | Not checked. |

So Fettle is a strong candidate with one real risk. Before spending money on a logo roll-out: run an official search (USPTO, UKIPO, EUIPO) in Classes 9 and 42, and ask a trademark attorney for a one-hour opinion. If Fettle fails, the mark and palette do not change (see section 4), only the wordmark.

### Alternatives to test if Fettle fails

| Name | Idea | Notes |
| --- | --- | --- |
| Kempt | "Unkempt" turned around: neat and tidy | Clever but easy to misread; all main domains taken |
| Pared | Pared down, trimmed | Calm and clear; probably crowded |
| Tend | To look after | Very generic, hard to own |
| Burnish | To polish and finish | A little too shiny for a tool about safety |
| A coined word | Built for ownership | Easiest to trademark, needs the most explaining |

Whatever we choose must pass three tests: it can be said on the phone and spelled back, it does not contain "PC", "optimizer", "cleaner" or "booster", and it passes the clearance above.

## 4. The mark

**A brass shaving curling off a clean board.** The board is your drive or your PC; the curl is the rough stuff coming away. It is the visual form of "fettling".

- It does not depend on a letter, so a name change does not mean a new logo.
- It is two shapes (a slab and one stroke), so it holds up at 16 px in the taskbar.
- Brass-on-iron is distinctive in a category full of blue shields and green leaves.

Files in this folder:

| File | Use |
| --- | --- |
| `mark-tile.svg` | App icon (iron tile, cream board, brass curl) |
| `mark.svg` | Bare mark for light backgrounds (iron board, glaze curl) |
| `mark-mono.svg` | One colour, uses `currentColor` |
| `lockup.svg` | Icon plus the wordmark. **The text is live**: convert to outlines before sending to a printer or another person, or it falls back to a system font |
| `icons/mark-tile-256.png`, `-512.png` | Raster app icon |
| `icons/mark.ico` | Windows icon, 16 to 256 px |

Rules: keep clear space of one board-height; never add gradients or shine; never rotate it; brass is only for the curl and never for text or thin lines on light backgrounds (2.0 : 1 contrast).

Open work on the mark: a hand-tuned 16 px version (the curl will be thicker), a monochrome favicon, and an animated version of the curl for the welcome screen. Have a human designer review the curve before it is final.

## 5. Colour

| Name | Hex | Use |
| --- | --- | --- |
| Iron | `#1D2125` | Text, sidebar, icon tile |
| Plaster | `#F3F4F1` | Backgrounds |
| Glaze | `#0E6B73` | Buttons, links, the health score |
| Glaze mist | `#D8EBEC` | Tips, selected rows |
| Brass | `#E0A526` | The curl, the current tab |
| Slate | `#4F5A60` | Secondary text |
| Done / Check first / Problem | `#1E7B4F` / `#8A5300` / `#B42318` | Status |

Contrast (WCAG): iron on plaster 14.7, slate on plaster 6.4, glaze on plaster 5.7, plaster on glaze 5.7, brass on iron 7.4, all passing AA. Brass on plaster is 2.0 and is never used for text.

This replaces the current cream, teal and honey, which sit close to a very common AI-and-wellness look. Cooler plaster and iron read more like a tool; glaze keeps some of the existing teal so the change is not jarring.

## 6. Type

- **Bricolage Grotesque** (ExtraBold, Bold): the name, headlines, big numbers. Has the character of a hand-cut sign.
- **Atkinson Hyperlegible Next** (Regular, Medium, Bold): everything people read and act on. Designed by the Braille Institute for maximum letter clarity, which fits the product story and the accessibility work already done.
- Sentence case everywhere. No all-caps labels. No eyebrow text above headings.
- Both are SIL OFL, available as variable fonts via `@fontsource`, and can be bundled so the app makes no network request. Today's app uses Nunito.

## 7. Voice

Say what happened and what to do next. One idea per sentence. Plain verbs. No apology, no exclamation marks, no fear.

| Today | New |
| --- | --- |
| No startup programs found. | Nothing starts with Windows on this PC. |
| uninstaller failed: | The uninstaller closed before it finished, so nothing was removed. Try again, or remove it in Settings. |
| Freed 2.1 GB | Freed 2.1 GB. Nothing else was touched. |
| Memory is 88% used. Check Processes for heavy apps. | Memory is nearly full, so Windows is leaning on the slower disk. These three apps use the most. |

Taglines to test: **Keep your PC in fine fettle.** (warm, teaches the word), **Tidy up. Speed up. Undo anything.** (literal, for the first line of the site), **Nothing deleted behind your back.** (the trust line, already true).

## 8. What a rename touches in the product

Do this only after the name is cleared. It is a contained change but has more places than expected.

| Area | Items |
| --- | --- |
| App | `wails.json` (name, outputfilename, product name), `main.go` window title, `build/windows/info.json`, `appicon.png`, `icon.ico`, manifest identity, frontend title and sidebar brand, login screen text |
| Look | Replace the palette tokens in `frontend/src/style.css`; swap Nunito for the two new fonts; update the inline logo SVGs in `index.html` and the favicon; the loopback sign-in page in `internal/auth/auth.go` has its own inline logo and colours |
| Data | Config folder `%APPDATA%\pcoptimizer`, Credential Manager service `pcoptimizer`, state file. **Keep reading the old locations and migrate**, or existing users are signed out and lose preferences |
| Installer | Install folder, Start Menu shortcut, uninstall registry key `PCOptimizer`, `DisplayName`, `Publisher`, uninstaller text, installer window. Upgrading over an old install must remove the old shortcut and registry key |
| Code | Go module path `pcoptimizer` (internal only; can stay), `Moved Programs` folder name (keep: existing moved apps depend on it) |
| Site | `docs/` pages, SVG banner and screens, logo, `version.json`, domain, Search Console verification, privacy policy |
| Distribution | Download file names, GitHub repository name and README, Google OAuth consent screen (name and logo) |
| Legal | Privacy policy and licence text; trademark filing once cleared |

## 9. Rollout

1. **Clear the name** (1 to 2 weeks): official searches, a short attorney opinion, secure domain and handles. Decide among Fettle or an alternative.
2. **Polish the identity** (about 1 week): have a designer review the mark, produce the 16 px and favicon variants and outlined wordmark, and finish the website art.
3. **Restyle the app** (about 1 week): tokens, fonts, icon, copy pass on all strings in the voice above. Reversible and can ship as a normal update.
4. **Rename** (about 1 week): installer, data migration, site, repository, with a clear "formerly PC Optimizer" note on the site and in the update notes for existing users.
5. **Launch assets**: a short product page, three screenshots, one 30-second screen recording, the tagline test.

## 10. Decisions for you

1. Go ahead with **Fettle** as the working name while clearance runs, or start from the alternatives list?
2. Is a **brass shaving on a board** the right feeling, or do you want a different metaphor? (I can draw 2 to 3 other directions, for example a stylised drive, a soft tool-handle shape, or a pure lettermark.)
3. Do you want to keep **today's teal and honey** for continuity, or move to **iron, plaster and glaze**?
4. Is a **hand-drawn designer review** in the budget before launch? I recommend it for the final mark.
