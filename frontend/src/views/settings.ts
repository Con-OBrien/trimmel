import { api, AppInfo, LicenseStatus, UpdateInfo } from "../api"
import { busy, errMsg, esc, toast, View } from "../ui"

const row = (title: string, sub: string, action = "") => `
  <div class="list-row">
    <div class="grow"><strong>${title}</strong><div class="muted small mono">${sub}</div></div>
    <div class="row-actions">${action}</div>
  </div>`

// Plus controls appear only in builds that can check keys. The free app is never locked.
const plusRows = (lic: LicenseStatus) => {
  const who = lic.tier === "plus" && lic.email ? ` Licensed to ${esc(lic.email)}.` : ""
  const upgrade = lic.canActivate && lic.tier !== "plus" ? `<button class="btn primary" id="se-plus-upgrade">Upgrade to Plus</button> ` : ""
  const rows = [row("Trimmel Plus", `${esc(lic.note)}${who}<br><span class="muted">Every feature you can use today stays free.</span>`,
    `${upgrade}<button class="btn" id="se-plus-about">About Plus</button>`)]
  if (!lic.canActivate) return rows.join("")
  if (lic.tier === "plus") {
    rows.push(row("Licence key", "Saved on this PC.", `<button class="btn" id="se-plus-remove">Remove key</button>`))
  } else {
    rows.push(`<div class="list-row"><div class="grow"><strong>Have a key?</strong>
      <div class="muted">Bought Plus on the website? Sign in here with the same Google account, then paste the key.</div>
      <div><input type="search" id="se-plus-key" placeholder="TRIM1-…" aria-label="Plus licence key" autocomplete="off" spellcheck="false"></div></div>
      <div class="row-actions"><button class="btn primary" id="se-plus-activate">Activate</button></div></div>`)
    if (lic.canTrial) rows.push(row("Free trial", "14 days of everything in Plus. No card needed. It ends by itself.",
      `<button class="btn" id="se-plus-trial">Start free trial</button>`))
  }
  return rows.join("")
}

export const settingsView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Settings</h1></header>
    <div id="se-body"><p class="muted">Loading…</p></div>`
  const body = root.querySelector<HTMLElement>("#se-body")!

  const render = async (info: AppInfo) => {
    const { install } = info
    const auth = await api.AuthStatus().catch(() => null)
    const lic = await api.LicenseStatus().catch(() => null)
    const canChange = install.installed && install.hasUninstall
    const where = install.installed ? "Installed with setup" : "Not installed with setup (portable or development copy)"

    body.innerHTML = `
      <h2 class="section-title">About</h2>
      <div class="panel list">
        ${row("Trimmel", `Version ${esc(info.version)} · ${info.admin ? "running as administrator" : "not running as administrator"}`,
          `<button class="btn" id="se-update">Check for updates</button>`)}
        <div id="se-update-msg" class="list-row" hidden></div>
      </div>

      <h2 class="section-title">Installation</h2>
      <div class="panel list">
        ${row("Install location", `${esc(install.dir)}<br><span class="muted">${where}</span>`,
          `<button class="btn" id="se-open-install">Open folder</button>
           <button class="btn" id="se-change" ${canChange ? "" : "disabled"} title="${canChange ? "" : "Only available for copies installed with setup"}">Change location…</button>`)}
        ${row("Uninstall", "Removes the app, its shortcuts and its entry in Windows Settings.",
          `<button class="btn danger" id="se-uninstall" ${canChange ? "" : "disabled"}>Uninstall…</button>`)}
      </div>

      ${lic ? `<h2 class="section-title">Trimmel Plus</h2><div class="panel list" id="se-plus">${plusRows(lic)}</div>` : ""}

      ${auth?.configured ? `
      <h2 class="section-title">Account</h2>
      <div class="panel list">
        ${row("Google account", esc(auth.user ? `${auth.user.name} · ${auth.user.email}` : "Not signed in"),
          auth.user ? `<button class="btn" id="se-signout">Sign out</button>` : "")}
      </div>` : ""}

      <h2 class="section-title">Your data</h2>
      <div class="panel list">
        ${row("Saved settings", `${esc(install.settingsDir)}<br><span class="muted">Sign-in configuration only. Nothing about your files is kept.</span>`,
          `<button class="btn" id="se-open-settings">Open folder</button>
           <button class="btn danger" id="se-delete">Delete…</button>`)}
      </div>`

    const on = (id: string, fn: (btn: HTMLButtonElement) => void | Promise<void>) =>
      body.querySelector<HTMLButtonElement>(id)?.addEventListener("click", e => fn(e.currentTarget as HTMLButtonElement))
    const safe = async (fn: () => Promise<unknown>) => { try { await fn() } catch (e) { toast(errMsg(e), "err") } }

    on("#se-update", btn => safe(async () => {
      const msg = body.querySelector<HTMLElement>("#se-update-msg")!
      const r: UpdateInfo = await busy(btn, "Checking…", () => api.CheckUpdate())
      msg.hidden = false
      msg.innerHTML = r.available
        ? `<div class="grow"><strong>Version ${esc(r.latest)} is available</strong><div class="muted small">Released ${esc(r.date)}. You have ${esc(r.current)}.</div></div>
           <button class="btn primary" id="se-get">Download</button>`
        : `<div class="grow muted">You are up to date${r.current === "dev" ? " (development build)" : ""}. Latest release: ${esc(r.latest)}.</div>`
      msg.querySelector("#se-get")?.addEventListener("click", () => api.OpenDownloadPage())
    }))
    const plus = body.querySelector<HTMLElement>("#se-plus")
    const refreshPlus = (next: LicenseStatus) => { if (plus) plus.innerHTML = plusRows(next); wirePlus() }
    const wirePlus = () => {
      const click = (id: string, fn: (btn: HTMLButtonElement) => void | Promise<void>) =>
        plus?.querySelector<HTMLButtonElement>(id)?.addEventListener("click", e => fn(e.currentTarget as HTMLButtonElement))
      click("#se-plus-about", () => api.OpenPlusPage())
      click("#se-plus-upgrade", () => api.OpenAccountPage())
      click("#se-plus-trial", btn => safe(async () => refreshPlus(await busy(btn, "Starting…", () => api.StartTrial()))))
      click("#se-plus-remove", btn => safe(async () => refreshPlus(await busy(btn, "Removing…", () => api.RemoveLicense()))))
      click("#se-plus-activate", btn => safe(async () => {
        const key = plus!.querySelector<HTMLInputElement>("#se-plus-key")!.value
        refreshPlus(await busy(btn, "Checking…", () => api.ActivateLicense(key)))
        toast("Trimmel Plus is active")
      }))
    }
    wirePlus()

    on("#se-open-install", () => safe(() => api.OpenFolder("install")))
    on("#se-open-settings", () => safe(() => api.OpenFolder("settings")))
    on("#se-change", () => safe(async () => {
      if (confirm("Trimmel will close and Setup will open so you can choose a new folder. Continue?")) await api.RunSetup("modify")
    }))
    on("#se-uninstall", () => safe(async () => {
      if (confirm("Trimmel will close and Setup will open to remove it. Continue?")) await api.RunSetup("uninstall")
    }))
    on("#se-signout", () => safe(async () => { await api.Logout(); location.reload() }))
    on("#se-delete", () => safe(async () => {
      if (!confirm("Sign out and delete the saved settings for this Windows user? You can sign in again afterwards.")) return
      await api.DeleteSettings()
      toast("Saved settings deleted")
      location.reload()
    }))
  }

  api.AppInfo().then(render).catch(e => { body.innerHTML = `<p class="error">${esc(errMsg(e))}</p>` })
}
