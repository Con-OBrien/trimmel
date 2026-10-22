import { api, DriverScanState, Overview, Process } from "../api"
import { bar, errMsg, esc, fmtBytes, fmtUptime, nav, relatedLine, toast, View } from "../ui"

type Detail = "cpu" | "mem" | "uptime"

export const overviewView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Overview</h1><span class="muted" id="ov-host"></span></header>
    <div id="ov-body"><p class="muted">Collecting system info…</p></div>`

  let stopped = false
  let detail: Detail | null = null
  let last: Overview | null = null
  let procs: Process[] = []
  let drv: DriverScanState | null = null
  const body = root.querySelector<HTMLElement>("#ov-body")!

  const load = async () => {
    try {
      const o = await api.GetOverview()
      if (stopped) return
      last = o
      // Starts a background Windows Update check once per session (then every 6 h).
      drv = await (drv ? api.DriverScanState() : api.StartDriverScan(false)).catch(() => drv)
      if (detail) await loadProcs()
      render()
    } catch (e) {
      if (!stopped) body.innerHTML = `<p class="error">${esc(errMsg(e))}</p>`
    }
  }

  const loadProcs = async () => {
    try { procs = (await api.ListProcesses()) ?? [] } catch { /* keep previous list */ }
  }

  const render = () => {
    if (!last) return
    root.querySelector("#ov-host")!.textContent = `${last.system.hostname} · ${last.system.os}`
    body.innerHTML = detail ? renderDetail(last, detail) : renderMain(last)
  }

  const renderMain = ({ system: s, hints }: Overview) => {
    const disks = (s.disks ?? []).map(d => `
      <div class="disk clickable" role="button" tabindex="0" data-go="cleanup">
        <div class="row-between"><strong>${esc(d.mount)}</strong>
          <span class="muted">${fmtBytes(d.free)} free of ${fmtBytes(d.total)}</span></div>
        ${bar(d.usedPercent)}
      </div>`).join("")

    return `
      <div class="cards">
        <div class="card clickable" role="button" tabindex="0" data-detail="cpu">
          <div class="card-label">CPU</div>
          <div class="card-value">${s.cpu.usagePercent.toFixed(0)}%</div>
          ${bar(s.cpu.usagePercent)}
          <div class="muted small">${esc(s.cpu.model)} · ${s.cpu.cores}C/${s.cpu.threads}T</div>
          <div class="card-more small">Details →</div>
        </div>
        <div class="card clickable" role="button" tabindex="0" data-detail="mem">
          <div class="card-label">Memory</div>
          <div class="card-value">${s.memory.usedPercent.toFixed(0)}%</div>
          ${bar(s.memory.usedPercent)}
          <div class="muted small">${fmtBytes(s.memory.used)} of ${fmtBytes(s.memory.total)}</div>
          <div class="card-more small">Details →</div>
        </div>
        <div class="card clickable" role="button" tabindex="0" data-detail="uptime">
          <div class="card-label">Uptime</div>
          <div class="card-value">${fmtUptime(s.uptimeSeconds)}</div>
          <div class="muted small">since last restart</div>
          <div class="card-more small">Details →</div>
        </div>
        ${driverCard()}
      </div>
      <h2>Drives <span class="muted small">click a drive to free up space</span></h2>
      <div class="panel">${disks || '<p class="muted">No drives found.</p>'}</div>
      <h2>Suggestions</h2>
      <div class="panel">${(hints ?? []).length
        ? (hints ?? []).map(h => `<div class="hint ${h.level}">${esc(h.message)}</div>`).join("")
        : '<p class="muted">Nothing to flag. Your PC looks healthy.</p>'}</div>`
  }

  const driverCard = () => {
    const n = drv?.updates?.length ?? 0
    let value: string, note: string, cls = ""
    if (!drv || (drv.checking && !drv.checkedAt)) { value = "…"; note = "Checking Windows Update" }
    else if (drv.error && !drv.checkedAt) { value = "—"; note = "Couldn't check. Open for details" }
    else { value = String(n); note = n ? `update${n === 1 ? "" : "s"} available` : "All up to date"; cls = n ? "warn" : "" }
    return `
        <div class="card clickable" role="button" tabindex="0" data-go="drivers">
          <div class="card-label">Driver updates</div>
          <div class="card-value ${cls}">${value}</div>
          <div class="muted small">${note}</div>
          <div class="card-more small">Details →</div>
        </div>`
  }

  const procRows = (list: Process[], metric: "cpu" | "mem", totalMem: number) => list.map(p => {
    const pct = metric === "cpu" ? p.cpuPercent : totalMem ? (p.memoryRss / totalMem) * 100 : 0
    const value = metric === "cpu" ? `${p.cpuPercent.toFixed(1)}%` : fmtBytes(p.memoryRss)
    const other = metric === "cpu" ? fmtBytes(p.memoryRss) : `${p.cpuPercent.toFixed(1)}%`
    return `
      <tr>
        <td title="${esc(p.path)}">${esc(p.name)}${relatedLine(p)}<div class="share">${bar(Math.min(100, pct * (metric === "cpu" ? 1 : 4)))}</div></td>
        <td class="num"><strong>${value}</strong></td>
        <td class="num muted">${other}</td>
        <td class="num">${p.protected ? '<span class="muted small">system</span>'
          : `<button class="btn ghost small" data-kill="${p.pid}" data-name="${esc(p.name)}">End task</button>`}</td>
      </tr>`
  }).join("")

  const renderDetail = ({ system: s, hints }: Overview, d: Detail) => {
    const back = '<button class="btn ghost small" data-back>← Overview</button>'

    if (d === "cpu") {
      const top = [...procs].sort((a, b) => b.cpuPercent - a.cpuPercent).slice(0, 10)
      const busy = top.filter(p => p.cpuPercent >= 10)
      return `
        <div class="toolbar">${back}</div>
        <div class="panel detail-head">
          <div><div class="card-label">CPU</div><div class="card-value">${s.cpu.usagePercent.toFixed(0)}%</div></div>
          <div class="grow"><div class="muted small">${esc(s.cpu.model)}</div>
            <div class="muted small">${s.cpu.cores} cores · ${s.cpu.threads} threads</div>${bar(s.cpu.usagePercent)}</div>
        </div>
        <h2>Using the most CPU</h2>
        <div class="table-wrap"><table>
          <thead><tr><th>Process</th><th class="num">CPU</th><th class="num">Memory</th><th></th></tr></thead>
          <tbody>${procRows(top, "cpu", s.memory.total) || '<tr><td colspan="4" class="muted">Measuring…</td></tr>'}</tbody>
        </table></div>
        <div class="panel tips">
          ${busy.length
            ? `<div class="hint warn">${busy.map(p => esc(p.name)).join(", ")} ${busy.length === 1 ? "is" : "are"} using over 10% of your CPU. If you aren't using ${busy.length === 1 ? "it" : "them"}, end the task.</div>`
            : '<div class="hint">No single process is hogging the CPU.</div>'}
          ${s.cpu.usagePercent > 80 ? '<div class="hint warn">Sustained high CPU slows everything down. Check Startup for programs that launch automatically.</div>' : ""}
        </div>
        <div class="toolbar"><button class="btn" data-go="processes" data-sort="cpu">Open full process list</button>
          <button class="btn ghost" data-go="startup">Review startup programs</button></div>`
    }

    if (d === "mem") {
      const top = [...procs].sort((a, b) => b.memoryRss - a.memoryRss).slice(0, 10)
      const m = s.memory
      const heavy = top.filter(p => p.memoryRss >= 500 * 1024 ** 2)
      return `
        <div class="toolbar">${back}</div>
        <div class="panel detail-head">
          <div><div class="card-label">Memory</div><div class="card-value">${m.usedPercent.toFixed(0)}%</div></div>
          <div class="grow">
            <div class="muted small">${fmtBytes(m.used)} used · ${fmtBytes(m.available)} available · ${fmtBytes(m.total)} total</div>
            ${bar(m.usedPercent)}</div>
        </div>
        <h2>Using the most memory</h2>
        <div class="table-wrap"><table>
          <thead><tr><th>Process</th><th class="num">Memory</th><th class="num">CPU</th><th></th></tr></thead>
          <tbody>${procRows(top, "mem", m.total) || '<tr><td colspan="4" class="muted">Loading…</td></tr>'}</tbody>
        </table></div>
        <div class="panel tips">
          ${m.usedPercent > 85 ? '<div class="hint warn">Memory is nearly full, so Windows will start using the slower disk. Close the biggest apps below.</div>' : ""}
          ${heavy.length
            ? `<div class="hint warn">${heavy.map(p => esc(p.name)).join(", ")} ${heavy.length === 1 ? "holds" : "hold"} over 500 MB each.</div>`
            : '<div class="hint">No single process is using a large amount of memory.</div>'}
          <div class="hint">Browsers with many tabs are a common cause. Closing tabs frees memory straight away.</div>
        </div>
        <div class="toolbar"><button class="btn" data-go="processes" data-sort="mem">Open full process list</button></div>`
    }

    const boot = new Date(Date.now() - s.uptimeSeconds * 1000)
    const days = s.uptimeSeconds / 86400
    const startupHint = (hints ?? []).find(h => h.message.includes("launch at startup"))
    return `
      <div class="toolbar">${back}</div>
      <div class="panel detail-head">
        <div><div class="card-label">Uptime</div><div class="card-value">${fmtUptime(s.uptimeSeconds)}</div></div>
        <div class="grow muted">Last restarted ${esc(boot.toLocaleString())}</div>
      </div>
      <div class="panel tips">
        ${days > 7
          ? '<div class="hint warn">No restart in over a week. A restart clears leaked memory and finishes pending updates.</div>'
          : '<div class="hint">Uptime is healthy. Restart when Windows asks for updates.</div>'}
        ${startupHint ? `<div class="hint warn">${esc(startupHint.message)}</div>` : ""}
      </div>
      <div class="toolbar"><button class="btn" data-go="startup">Review startup programs</button></div>`
  }

  body.addEventListener("click", async e => {
    const t = e.target as HTMLElement
    const kill = t.closest<HTMLButtonElement>("[data-kill]")
    if (kill) {
      const name = kill.dataset.name!
      if (!confirm(`End ${name}? Unsaved work in it will be lost.`)) return
      try {
        await api.KillProcess(Number(kill.dataset.kill), name)
        toast(`Ended ${name}`)
        await loadProcs(); render()
      } catch (err) { toast(errMsg(err), "err") }
      return
    }
    if (t.closest("[data-back]")) { detail = null; render(); return }
    const go = t.closest<HTMLElement>("[data-go]")
    if (go) { nav.go(go.dataset.go!, go.dataset.sort ? { sort: go.dataset.sort } : undefined); return }
    const card = t.closest<HTMLElement>("[data-detail]")
    if (card) {
      detail = card.dataset.detail as Detail
      render()
      // The first process sample has no CPU delta; take a second one shortly after.
      await loadProcs(); render()
      window.setTimeout(async () => { if (detail && !stopped) { await loadProcs(); render() } }, 1500)
    }
  })
  body.addEventListener("keydown", e => {
    if ((e.key === "Enter" || e.key === " ") && (e.target as HTMLElement).matches(".clickable")) {
      e.preventDefault();(e.target as HTMLElement).click()
    }
  })

  load()
  const timer = window.setInterval(load, 4000)
  return () => { stopped = true; clearInterval(timer) }
}
