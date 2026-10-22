import { api, StorageEntry, StorageResult } from "../api"
import { errMsg, esc, fmtBytes, toast, View } from "../ui"

/** Parent folder of a Windows path ("C:\\" for a top-level folder), or "" at a drive root. */
const parentOf = (p: string): string => {
  const trimmed = p.replace(/[\\/]+$/, "")
  if (trimmed.length <= 2) return ""
  const i = trimmed.lastIndexOf("\\")
  return i <= 2 ? trimmed.slice(0, 3) : trimmed.slice(0, i)
}

export const storageView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Storage</h1></header>
    <p class="muted small">See which folders and files take the most space. This page only looks: nothing is deleted here.
      Open a folder in Explorer to decide what to do with it. Scanning a whole drive can take a few minutes.</p>
    <div class="toolbar">
      <label class="muted">Drive <select id="sg-drive" aria-label="Drive to analyse"></select></label>
      <button class="btn primary" id="sg-scan">Analyse</button>
      <button class="btn" id="sg-up" disabled>Up one level</button>
      <button class="btn" id="sg-big" disabled>Biggest files here</button>
    </div>
    <div class="muted small mono" id="sg-path" aria-live="polite"></div>
    <div id="sg-body"><p class="muted">Pick a drive and choose Analyse.</p></div>`

  let stopped = false
  let current: StorageResult | null = null
  const driveEl = root.querySelector<HTMLSelectElement>("#sg-drive")!
  const body = root.querySelector<HTMLElement>("#sg-body")!
  const pathEl = root.querySelector<HTMLElement>("#sg-path")!
  const upBtn = root.querySelector<HTMLButtonElement>("#sg-up")!
  const bigBtn = root.querySelector<HTMLButtonElement>("#sg-big")!
  const scanBtn = root.querySelector<HTMLButtonElement>("#sg-scan")!

  const setBusy = (on: boolean, text = "") => {
    scanBtn.disabled = on
    bigBtn.disabled = on || !current
    upBtn.disabled = on || !current || !parentOf(current.path)
    if (on) body.innerHTML = `<p class="muted"><span class="spinner"></span>${esc(text)}</p>`
  }

  const note = (r: StorageResult) => [
    r.partial ? "This folder is very large, so the scan stopped early. Sizes shown are a lower bound." : "",
    r.skipped ? `${r.skipped} folder${r.skipped === 1 ? "" : "s"} couldn't be read (permissions), so they aren't counted.` : "",
  ].filter(Boolean).map(t => `<div class="hint warn">${esc(t)}</div>`).join("")

  const render = (r: StorageResult) => {
    current = r
    pathEl.textContent = r.path
    const entries = r.entries ?? []
    const max = entries[0]?.size || 1
    body.innerHTML = `${note(r)}
      <div class="panel list">
        <div class="list-row"><div class="grow"><strong>Total in this folder</strong></div><div class="num"><strong>${fmtBytes(r.total)}</strong></div></div>
        ${entries.length ? entries.slice(0, 200).map((e, i) => row(e, max, r.total, i)).join("") : '<p class="muted" style="padding:14px 18px">This folder is empty.</p>'}
      </div>
      ${entries.length > 200 ? `<p class="muted small">Showing the 200 largest of ${entries.length} items.</p>` : ""}`
    setBusy(false)
  }

  const row = (e: StorageEntry, max: number, total: number, i: number) => {
    const share = total ? (e.size / total) * 100 : 0
    return `
      <div class="list-row">
        <div class="grow">
          <div>${e.isDir
            ? `<button class="link" data-open="${i}" aria-label="Open folder ${esc(e.name)}">📁 ${esc(e.name)}</button>`
            : `📄 ${esc(e.name)}`}
            <span class="muted small">${share >= 0.1 ? share.toFixed(share < 10 ? 1 : 0) + "%" : "<0.1%"}</span></div>
          <div class="bar share-bar"><div class="bar-fill" style="width:${Math.max(1, (e.size / max) * 100).toFixed(1)}%"></div></div>
        </div>
        <div class="num"><strong>${fmtBytes(e.size)}</strong>${e.isDir ? `<div class="muted small">${e.files.toLocaleString()} files</div>` : ""}</div>
        <button class="btn small" data-reveal="${i}" aria-label="Show ${esc(e.name)} in Explorer">Show in Explorer</button>
      </div>`
  }

  const analyse = async (path: string) => {
    setBusy(true, `Measuring ${path} …`)
    try {
      const r = await api.AnalyzeFolder(path)
      if (!stopped) render(r)
    } catch (e) {
      if (stopped) return
      body.innerHTML = `<p class="error">${esc(errMsg(e))}</p>`
      setBusy(false)
    }
  }

  const showBiggest = async () => {
    if (!current) return
    const path = current.path
    setBusy(true, `Looking for the biggest files in ${path} …`)
    try {
      const r = await api.LargestFiles(path)
      if (stopped) return
      const files = r.files ?? []
      pathEl.textContent = `Biggest files in ${path}`
      const max = files[0]?.size || 1
      body.innerHTML = `${r.partial ? '<div class="hint warn">The scan stopped early, so a bigger file may be missing.</div>' : ""}
        <div class="panel list">${files.length ? files.map((f, i) => `
          <div class="list-row">
            <div class="grow"><div>📄 ${esc(f.name)}</div>
              <div class="muted small mono" title="${esc(f.path)}">${esc(f.path)}</div>
              <div class="bar share-bar"><div class="bar-fill" style="width:${Math.max(1, (f.size / max) * 100).toFixed(1)}%"></div></div></div>
            <div class="num"><strong>${fmtBytes(f.size)}</strong></div>
            <button class="btn small" data-revealbig="${i}" aria-label="Show ${esc(f.name)} in Explorer">Show in Explorer</button>
          </div>`).join("") : '<p class="muted" style="padding:14px 18px">No files found.</p>'}</div>
        <p><button class="btn" id="sg-back">Back to folders</button></p>`
      bigFiles = files
      setBusy(false)
      root.querySelector("#sg-back")?.addEventListener("click", () => current && render(current))
    } catch (e) {
      if (!stopped) { body.innerHTML = `<p class="error">${esc(errMsg(e))}</p>`; setBusy(false) }
    }
  }

  let bigFiles: StorageEntry[] = []
  const reveal = (p: string) => api.RevealInExplorer(p).catch(e => toast(errMsg(e), "err"))

  body.addEventListener("click", e => {
    const t = e.target as HTMLElement
    const open = t.closest<HTMLElement>("[data-open]")
    const rev = t.closest<HTMLElement>("[data-reveal]")
    const revBig = t.closest<HTMLElement>("[data-revealbig]")
    if (open && current) analyse(current.entries![Number(open.dataset.open)].path)
    else if (rev && current) reveal(current.entries![Number(rev.dataset.reveal)].path)
    else if (revBig) reveal(bigFiles[Number(revBig.dataset.revealbig)].path)
  })
  scanBtn.addEventListener("click", () => analyse(driveEl.value + "\\"))
  upBtn.addEventListener("click", () => { const p = current && parentOf(current.path); if (p) analyse(p) })
  bigBtn.addEventListener("click", showBiggest)

  api.GetOverview().then(o => {
    if (stopped) return
    driveEl.innerHTML = (o.system.disks ?? []).map(d =>
      `<option value="${esc(d.mount)}">${esc(d.mount)} · ${fmtBytes(d.total - d.free)} used of ${fmtBytes(d.total)}</option>`).join("")
  }).catch(e => { body.innerHTML = `<p class="error">${esc(errMsg(e))}</p>` })

  return () => { stopped = true }
}
