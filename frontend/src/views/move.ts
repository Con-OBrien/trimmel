import { api, Install, Leftover, MoveProgress } from "../api"
import { errMsg, esc, fmtBytes, toast, View } from "../ui"

const STAGES: Record<string, string> = {
  checking: "Checking…",
  copying: "Copying",
  verifying: "Verifying the copy…",
  switching: "Linking the old location…",
  cleaning: "Freeing space on the old drive…",
  done: "Done",
}

export const moveView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Move apps</h1>
      <button class="btn" id="mv-rescan">Rescan</button></header>
    <p class="muted small">Move big programs and games to another drive to free up space. The original location becomes a link,
      so shortcuts and updates keep working. Close the program first. Moves can be undone with "Move back".</p>
    <div class="toolbar">
      <label class="muted">Move to <select id="mv-dest"></select></label>
      <input id="mv-filter" type="search" placeholder="Filter by name or publisher" />
      <span class="muted small" id="mv-admin"></span>
    </div>
    <div id="mv-left"></div>
    <div id="mv-progress"></div>
    <div id="mv-body"><p class="muted"><span class="spinner"></span>Looking for installed programs…</p></div>`

  let installs: Install[] = []
  let drives: { mount: string; free: number; total: number }[] = []
  let leftovers: Leftover[] = []
  let working = false
  let stopped = false
  const destEl = root.querySelector<HTMLSelectElement>("#mv-dest")!
  const filterEl = root.querySelector<HTMLInputElement>("#mv-filter")!
  const progEl = root.querySelector<HTMLElement>("#mv-progress")!
  const leftEl = root.querySelector<HTMLElement>("#mv-left")!
  const bodyEl = root.querySelector<HTMLElement>("#mv-body")!

  const off = window.runtime?.EventsOn("move:progress", (p: MoveProgress) => {
    if (stopped) return
    progEl.innerHTML = `
      <div class="panel"><div class="row-between"><strong>${esc(p.name)}</strong>
        <span class="muted">${esc(STAGES[p.stage] ?? p.stage)}${p.stage === "copying" ? ` ${p.percent.toFixed(0)}%` : ""}</span></div>
        <div class="bar"><div class="bar-fill" style="width:${p.percent.toFixed(1)}%"></div></div></div>`
  })

  const renderDest = () => {
    const prev = destEl.value
    destEl.innerHTML = drives.map(d =>
      `<option value="${esc(d.mount)}">${esc(d.mount)} · ${fmtBytes(d.free)} free</option>`).join("")
    if (prev && drives.some(d => d.mount === prev)) destEl.value = prev
    else {
      // Default to the non-system drive with the most free space.
      const best = [...drives].filter(d => d.mount !== "C:").sort((a, b) => b.free - a.free)[0] ?? drives[0]
      if (best) destEl.value = best.mount
    }
  }

  const renderLeftovers = () => {
    if (!leftovers.length) { leftEl.innerHTML = ""; return }
    const total = leftovers.reduce((n, l) => n + l.size, 0)
    leftEl.innerHTML = `
      <div class="panel">
        <div class="hint warn"><strong>Leftover copies (${fmtBytes(total)})</strong><br>
          These folders were left on another drive by a move that didn't finish or was undone. No installed program uses them,
          so they are safe to delete. Deleting one before retrying the move avoids having the program in two places.</div>
        <table><tbody>${leftovers.map((l, i) => `
          <tr><td>${esc(l.path)}</td><td class="num">${fmtBytes(l.size)}</td>
            <td class="num"><button class="btn small" data-del="${i}" ${working ? "disabled" : ""}>Delete</button></td></tr>`).join("")}
        </tbody></table>
      </div>`
  }

  const render = () => {
    renderLeftovers()
    const f = filterEl.value.trim().toLowerCase()
    const dest = destEl.value
    const list = installs.filter(i => !f || `${i.name} ${i.publisher}`.toLowerCase().includes(f))
    if (!list.length) { bodyEl.innerHTML = '<p class="muted">No large programs found.</p>'; return }
    bodyEl.innerHTML = `<div class="table-wrap"><table class="move-table">
      <thead><tr><th>Program</th><th>Drive</th><th class="num">Size</th><th></th></tr></thead>
      <tbody>${list.map(i => {
        const idx = installs.indexOf(i)
        let action: string
        if (i.movedTo) {
          action = `<button class="btn small" data-back="${idx}" ${working ? "disabled" : ""}>Move back</button>`
        } else if (!i.movable) {
          action = `<span class="muted small">${esc(i.reason)}</span>`
        } else if (!dest || i.drive.toLowerCase() === dest.toLowerCase()) {
          action = '<span class="muted small">Already on this drive</span>'
        } else {
          action = `<button class="btn primary small" data-move="${idx}" ${working ? "disabled" : ""}>Move to ${esc(dest)}</button>`
        }
        return `
        <tr>
          <td class="name">${esc(i.name)}${i.movedTo ? ` <span class="tag">moved to ${esc(i.movedTo.slice(0, 2))}</span>` : ""}
            <div class="muted small related" title="${esc(i.path)}">${esc(i.publisher ? i.publisher + " · " : "")}${esc(i.path)}</div></td>
          <td class="muted">${esc(i.drive)}</td>
          <td class="num">${fmtBytes(i.size)}</td>
          <td class="num">${action}</td>
        </tr>`
      }).join("")}</tbody></table></div>`
  }

  const load = async () => {
    bodyEl.innerHTML = '<p class="muted"><span class="spinner"></span>Looking for installed programs…</p>'
    try {
      const [o, list, left] = await Promise.all([api.GetOverview(), api.ScanInstalls(), api.ScanLeftovers()])
      if (stopped) return
      drives = (o.system.disks ?? []).map(d => ({ mount: d.mount, free: d.free, total: d.total }))
      installs = list ?? []
      leftovers = left ?? []
      renderDest()
      if (drives.length < 2) {
        bodyEl.innerHTML = '<div class="panel"><p class="muted">Only one drive was found. Connect or add a second drive to move programs.</p></div>'
        return
      }
      render()
    } catch (e) {
      if (!stopped) bodyEl.innerHTML = `<p class="error">${esc(errMsg(e))}</p>`
    }
  }

  const run = async (label: string, fn: () => Promise<{ dest: string; bytes: number; warning?: string }>) => {
    working = true
    render()
    progEl.innerHTML = ""
    try {
      const r = await fn()
      toast(`${label}: freed ${fmtBytes(r.bytes)}${r.warning ? ". " + r.warning : ""}`, r.warning ? "err" : "ok")
    } catch (e) {
      toast(errMsg(e), "err")
    } finally {
      working = false
      progEl.innerHTML = ""
      if (!stopped) await load()
    }
  }

  root.addEventListener("click", e => {
    if (working) return
    const t = e.target as HTMLElement
    const del = t.closest<HTMLElement>("[data-del]")
    if (del) {
      const l = leftovers[Number(del.dataset.del)]
      if (!confirm(`Permanently delete ${l.path} (${fmtBytes(l.size)})? This can't be undone.`)) return
      working = true; render()
      api.DeleteLeftover(l.path)
        .then(() => toast(`Deleted ${l.path}`))
        .catch(err => toast(errMsg(err), "err"))
        .finally(() => { working = false; if (!stopped) load() })
      return
    }
    const mv = t.closest<HTMLElement>("[data-move]")
    const back = t.closest<HTMLElement>("[data-back]")
    if (mv) {
      const i = installs[Number(mv.dataset.move)]
      const dest = destEl.value
      if (!confirm(`Move ${i.name} (${fmtBytes(i.size)}) to ${dest}?\n\nMake sure it is closed. This can take several minutes, and you can move it back later.`)) return
      run(`Moved ${i.name}`, () => api.MoveInstall(i.path, dest))
    } else if (back) {
      const i = installs[Number(back.dataset.back)]
      if (!confirm(`Move ${i.name} back to ${i.drive}? Make sure it is closed.`)) return
      run(`Moved back ${i.name}`, () => api.MoveInstallBack(i.path))
    }
  })
  root.querySelector("#mv-rescan")!.addEventListener("click", () => { if (!working) load() })
  destEl.addEventListener("change", render)
  filterEl.addEventListener("input", render)

  api.IsAdmin().then(a => {
    if (!a && !stopped) root.querySelector("#mv-admin")!.textContent = "Programs in Program Files need the app to run as administrator."
  })

  load()
  return () => { stopped = true; off?.() }
}
