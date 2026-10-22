import { api, AppInfo, UpdateInfo } from "../api"
import { busy, errMsg, esc, toast, View } from "../ui"

const row = (title: string, sub: string, action = "") => `
  <div class="list-row">
    <div class="grow"><strong>${title}</strong><div class="muted small mono">${sub}</div></div>
    <div class="row-actions">${action}</div>
  </div>`

export const settingsView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Settings</h1></header>
    <div id="se-body"><p class="muted">Loading…</p></div>`
  const body = root.querySelector<HTMLElement>("#se-body")!

  const render = async (info: AppInfo) => {
    const { install } = info
    const auth = await api.AuthStatus().catch(() => null)
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
