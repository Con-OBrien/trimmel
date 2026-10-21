import { api, CleanupTarget } from "../api"
import { busy, errMsg, esc, fmtBytes, toast, View } from "../ui"

export const cleanupView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Cleanup</h1>
      <button class="btn" id="cl-scan">Scan again</button></header>
    <div id="cl-body"><p class="muted">Scanning…</p></div>`

  let targets: CleanupTarget[] = []
  const selected = new Set<string>()
  const body = root.querySelector<HTMLElement>("#cl-body")!

  const total = () => targets.filter(t => selected.has(t.id)).reduce((s, t) => s + t.bytes, 0)

  const render = () => {
    body.innerHTML = `
      <div class="panel list">
        ${targets.map(t => `
          <label class="list-row ${t.available ? "" : "disabled"}">
            <input type="checkbox" data-id="${esc(t.id)}"
              ${selected.has(t.id) ? "checked" : ""} ${t.available ? "" : "disabled"} />
            <div class="grow">
              <div><strong>${esc(t.name)}</strong>
                ${t.requiresAdmin ? '<span class="tag">admin</span>' : ""}</div>
              <div class="muted small">${esc(t.description)}</div>
              ${t.error ? `<div class="small ${t.available ? "error" : "muted"}">${esc(t.error)}</div>` : ""}
            </div>
            <div class="num">
              <strong>${t.available ? fmtBytes(t.bytes) : "—"}</strong>
              <div class="muted small">${t.available ? `${t.files.toLocaleString()} items` : ""}</div>
            </div>
          </label>`).join("")}
      </div>
      <div class="action-bar">
        <span>Selected: <strong id="cl-total">${fmtBytes(total())}</strong></span>
        <button class="btn primary" id="cl-run" ${selected.size ? "" : "disabled"}>Clean selected</button>
      </div>`
  }

  const scan = async () => {
    body.innerHTML = '<p class="muted"><span class="spinner"></span>Scanning…</p>'
    try {
      targets = (await api.ScanCleanup()) ?? []
      selected.clear()
      // Pre-select safe, non-empty targets. The Recycle Bin is opt-in because emptying it is permanent.
      targets.forEach(t => { if (t.available && t.bytes > 0 && t.id !== "recycle-bin") selected.add(t.id) })
      render()
    } catch (e) {
      body.innerHTML = `<p class="error">${esc(errMsg(e))}</p>`
    }
  }

  body.addEventListener("change", e => {
    const cb = e.target as HTMLInputElement
    if (!cb.dataset.id) return
    cb.checked ? selected.add(cb.dataset.id) : selected.delete(cb.dataset.id)
    root.querySelector("#cl-total")!.textContent = fmtBytes(total())
    root.querySelector<HTMLButtonElement>("#cl-run")!.disabled = selected.size === 0
  })

  body.addEventListener("click", async e => {
    const btn = (e.target as HTMLElement).closest<HTMLButtonElement>("#cl-run")
    if (!btn) return
    const ids = [...selected]
    if (ids.includes("recycle-bin") && !confirm("Empty the Recycle Bin? Those files can't be recovered.")) return
    try {
      const results = (await busy(btn, "Cleaning…", () => api.RunCleanup(ids))) ?? []
      const freed = results.reduce((s, r) => s + r.freedBytes, 0)
      const skipped = results.reduce((s, r) => s + r.skipped, 0)
      const errors = results.filter(r => r.error)
      toast(`Freed ${fmtBytes(freed)}` + (skipped ? ` · ${skipped} files in use were skipped` : "") +
        (errors.length ? ` · ${errors.length} failed` : ""), errors.length ? "err" : "ok")
      await scan()
    } catch (err) {
      toast(errMsg(err), "err")
    }
  })

  root.querySelector("#cl-scan")!.addEventListener("click", scan)
  scan()
}
