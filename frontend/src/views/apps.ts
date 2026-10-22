import { api, InstalledApp } from "../api"
import { errMsg, esc, fmtBytes, toast, View } from "../ui"

type SortKey = "size" | "name" | "date"

export const appsView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Apps</h1><span class="muted" id="ap-count"></span>
      <button class="btn" id="ap-rescan">Rescan</button></header>
    <p class="muted small">Every program Windows lists as installed. Uninstall opens the program's own uninstaller, so you see and
      confirm what is removed. Items marked <span class="tag warn">shared</span> may be needed by other programs.</p>
    <div class="toolbar">
      <input id="ap-filter" type="search" placeholder="Filter by name or publisher" aria-label="Filter apps" />
      <label class="muted">Sort by
        <select id="ap-sort" aria-label="Sort apps">
          <option value="size">Size</option><option value="name">Name</option><option value="date">Install date (newest)</option>
        </select></label>
    </div>
    <div id="ap-body"><p class="muted"><span class="spinner"></span>Reading installed programs…</p></div>`

  let apps: InstalledApp[] = []
  let stopped = false
  let working = false
  const body = root.querySelector<HTMLElement>("#ap-body")!
  const filterEl = root.querySelector<HTMLInputElement>("#ap-filter")!
  const sortEl = root.querySelector<HTMLSelectElement>("#ap-sort")!

  const render = () => {
    const f = filterEl.value.trim().toLowerCase()
    const sort = sortEl.value as SortKey
    const list = apps.filter(a => !f || `${a.name} ${a.publisher}`.toLowerCase().includes(f))
    list.sort((a, b) =>
      sort === "name" ? a.name.localeCompare(b.name)
      : sort === "date" ? b.installDate.localeCompare(a.installDate) || a.name.localeCompare(b.name)
      : b.size - a.size || a.name.localeCompare(b.name))
    const total = apps.reduce((n, a) => n + a.size, 0)
    root.querySelector("#ap-count")!.textContent = `${apps.length} programs · ${fmtBytes(total)} counted`
    body.innerHTML = list.length ? `<div class="table-wrap"><table>
      <thead><tr><th>Program</th><th>Version</th><th>Installed</th><th class="num">Size</th><th></th></tr></thead>
      <tbody>${list.map(a => `
        <tr>
          <td class="name">${esc(a.name)}${a.shared ? ' <span class="tag warn" title="Windows or other programs may depend on this">shared</span>' : ""}
            <div class="muted small">${esc(a.publisher)}</div></td>
          <td class="muted">${esc(a.version)}</td>
          <td class="muted">${esc(a.installDate || "—")}</td>
          <td class="num">${a.size ? fmtBytes(a.size) : '<span class="muted">unknown</span>'}</td>
          <td class="num"><button class="btn small" data-un="${esc(a.id)}" ${working ? "disabled" : ""}
            aria-label="Uninstall ${esc(a.name)}">Uninstall</button></td>
        </tr>`).join("")}</tbody></table></div>`
      : '<p class="muted">No matching programs.</p>'
  }

  const load = async () => {
    body.innerHTML = '<p class="muted"><span class="spinner"></span>Reading installed programs…</p>'
    try {
      apps = (await api.ListApps()) ?? []
      if (!stopped) render()
    } catch (e) {
      if (!stopped) body.innerHTML = `<p class="error">${esc(errMsg(e))}</p>`
    }
  }

  body.addEventListener("click", async e => {
    const btn = (e.target as HTMLElement).closest<HTMLButtonElement>("[data-un]")
    if (!btn || working) return
    const app = apps.find(a => a.id === btn.dataset.un)
    if (!app) return
    const warn = app.shared ? "\n\nThis looks like a shared component. Other programs may stop working if you remove it." : ""
    if (!confirm(`Uninstall ${app.name}? Its own uninstaller will open and ask you to confirm.${warn}`)) return
    working = true; render()
    try {
      await api.UninstallApp(app.id)
      toast(`${app.name}: uninstaller finished`)
    } catch (err) {
      toast(errMsg(err), "err")
    } finally {
      working = false
      if (!stopped) await load()
    }
  })

  filterEl.addEventListener("input", render)
  sortEl.addEventListener("change", render)
  root.querySelector("#ap-rescan")!.addEventListener("click", () => { if (!working) load() })

  load()
  return () => { stopped = true }
}
