import { api, OldVersion } from "../api"
import { busy, errMsg, esc, fmtBytes, toast, View } from "../ui"

export const oldVersionsView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Old versions</h1>
      <button class="btn" id="ov-rescan">Rescan</button></header>
    <p class="muted small">Programs that also have a newer version installed. Removing the old one frees space.
      Each uninstall opens the program's own uninstaller so you can confirm it.
      Runtimes, SDKs and Microsoft software are never listed because other programs may need a specific version.</p>
    <div id="ov-body"><p class="muted"><span class="spinner"></span>Checking installed programs…</p></div>`

  let items: OldVersion[] = []
  let stopped = false
  const body = root.querySelector<HTMLElement>("#ov-body")!

  const render = () => {
    if (!items.length) {
      body.innerHTML = '<div class="panel"><p class="ok">No older duplicate versions found.</p></div>'
      return
    }
    const total = items.reduce((n, i) => n + i.size, 0)
    body.innerHTML = `
      <div class="table-wrap"><table>
        <thead><tr><th>Older version</th><th>Replaced by</th><th class="num">Size</th><th></th></tr></thead>
        <tbody>${items.map((i, idx) => `
          <tr>
            <td>${esc(i.name)}<div class="muted small">${esc(i.publisher)} · version ${esc(i.version)}</div></td>
            <td class="muted">${esc(i.newer)}</td>
            <td class="num">${i.size ? fmtBytes(i.size) : "—"}</td>
            <td class="num"><button class="btn small" data-rm="${idx}">Uninstall</button></td>
          </tr>`).join("")}</tbody></table></div>
      <p class="muted small">${items.length} older version${items.length === 1 ? "" : "s"}${total ? ` · about ${fmtBytes(total)} to free` : ""}.
        Sizes come from the installer when it records one, otherwise from the install folder.</p>`
  }

  const load = async () => {
    try {
      items = (await api.ScanOldVersions()) ?? []
      if (!stopped) render()
    } catch (e) {
      if (!stopped) body.innerHTML = `<p class="error">${esc(errMsg(e))}</p>`
    }
  }

  body.addEventListener("click", async e => {
    const btn = (e.target as HTMLElement).closest<HTMLButtonElement>("[data-rm]")
    if (!btn) return
    const item = items[Number(btn.dataset.rm)]
    if (!confirm(`Uninstall ${item.name}?\n\n${item.newer} stays installed. Its uninstaller will open next.`)) return
    try {
      await busy(btn, "Uninstalling…", () => api.UninstallOldVersion(item.id))
      toast(`Finished ${item.name}`)
    } catch (err) {
      toast(errMsg(err), "err")
    }
    if (!stopped) await load() // reflects whether it was really removed
  })
  root.querySelector("#ov-rescan")!.addEventListener("click", load)

  load()
  return () => { stopped = true }
}
