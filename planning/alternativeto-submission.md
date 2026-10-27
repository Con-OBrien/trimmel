# AlternativeTo submission: Trimmel

Fields to fill in under **My submissions → Trimmel → Edit**. Everything below comes from the repo and website, so it matches what a reviewer will see at https://www.trimmel.app.

## Basics

| Field | Value |
| --- | --- |
| Name | Trimmel |
| Official website | https://www.trimmel.app |
| Tagline (short description) | Free Windows tune-up: clean junk, trim startup, move big apps to another drive and update drivers, with undo. |
| Platforms | Windows (10 and 11, 64-bit) |
| License | Open Source (MIT) |
| Price | Free |
| Source code | https://github.com/Con-OBrien/trimmel |
| Download | https://www.trimmel.app/#download |
| Latest version | 0.3.0 (23 Oct 2026) |
| Changelog | https://www.trimmel.app/changelog.html |
| Privacy | https://www.trimmel.app/privacy.html |

## Description (paste as the long description)

Trimmel is a free, open-source tune-up tool for Windows 10 and 11. It finds what is filling your drive, what slows your PC down at startup, and which drivers are out of date. It explains each item in plain English and deletes nothing you have not ticked.

- **Cleanup**: scans temp folders, Windows Update downloads, browser caches (Chrome, Edge, Firefox), crash dumps, shader caches and the Recycle Bin. Shows the size of each item before anything is removed.
- **Storage**: shows which folders and files take the most space on a drive. Read-only.
- **Startup**: lists everything that launches at sign-in, including scheduled tasks and Store apps, with an impact rating and a plain-English note. Switching items off uses the same flag as Task Manager, so it is fully reversible.
- **Move apps**: moves a large program to another drive and leaves a link behind so it keeps working. It copies and verifies first, and can move the program back.
- **Old versions**: finds older duplicate installs and opens each program's own uninstaller so you confirm what is removed.
- **Drivers**: lists installed drivers and checks Windows Update for newer, signed ones. You choose which to install.
- **Processes and Overview**: live CPU and memory per process with a note on what each belongs to, and a health score with suggestions that jump to the right tab.

Trimmel contains no analytics and sends no usage data. It is a small native app (Go and Wails) rather than a bundled browser. The installer is not code signed, so Windows SmartScreen may show a warning on first run (choose More info, then Run anyway). The full source is public and every release is built from it.

## Categories / tags

Disk cleaner, System optimizer, Startup manager, Driver updater, Uninstaller, Disk space analyzer, Open source, Privacy-friendly, No telemetry.

## Suggested alternatives to list Trimmel against

CCleaner, BleachBit, Microsoft PC Manager, Wise Disk Cleaner, Glary Utilities, Driver Booster, Revo Uninstaller, WizTree (for the Storage view).
The site already has a comparison page: https://www.trimmel.app/guides/trimmel-vs-microsoft-pc-manager-vs-bleachbit.html

## Features checklist (tick these in the form)

- Free, Open Source, No registration required to use the cleanup tools, No tracking or telemetry
- Undo / reversible changes (startup items, app moves)
- Plain-English explanations for each item

## Images

| Use | File in repo |
| --- | --- |
| Logo / icon | `build/appicon.png` (512+ px) or `docs/logo.svg` |
| Screenshot: Overview | `docs/screens/overview.png` |
| Screenshot: Cleanup | `docs/screens/cleanup.png` |
| Screenshot: Startup | `docs/screens/startup.png` |
| Screenshot: Move apps | `docs/screens/move.png` |
| Screenshot: Drivers | `docs/screens/drivers.png` |
| Screenshot: Storage | `docs/screens/storage.png` |
| Screenshot: Apps | `docs/screens/apps.png` |

The screenshots are filled with sample data, so they are fine to publish.

## Before resubmitting

- Check each field above is filled in on AlternativeTo, especially description, platform, license, pricing, tags and at least three screenshots.
- Make sure the website itself clearly shows what the app does, the download, the licence and the version. A reviewer judges the site, not just the form.
- Use the "ask us to take another look" button after saving, and mention what you added.
