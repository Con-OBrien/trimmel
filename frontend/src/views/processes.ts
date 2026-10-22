import { api, Process } from "../api"
import { errMsg, esc, fmtBytes, relatedLine, toast, View } from "../ui"

type SortKey = "cpu" | "mem" | "name"
type Level = "" | "warn" | "crit"

const REFRESH_MS = 5000
const CPU_WARN = 10, CPU_CRIT = 25
const MEM_WARN = 500 * 1024 ** 2, MEM_CRIT = 1024 ** 3

const level = (v: number, warn: number, crit: number): Level =>
  v >= crit ? "crit" : v >= warn ? "warn" : ""
const worst = (a: Level, b: Level): Level =>
  a === "crit" || b === "crit" ? "crit" : a || b

export const processesView: View = (root, params) => {
  root.innerHTML = `
    <header class="view-head"><h1>Processes</h1><span class="muted" id="pr-count"></span></header>
    <div class="toolbar">
      <input id="pr-filter" type="search" placeholder="Filter by name" />
      <button class="btn ghost small" id="pr-pause">Pause</button>
      <button class="btn ghost small" id="pr-refresh">Refresh</button>
      <span class="muted small" id="pr-state"></span>
      <span class="legend small muted">
        <span><span class="dot"></span>High</span>
        <span><span class="dot crit"></span>Very high</span>
      </span>
    </div>
    <p class="muted small">Updates every 5 seconds, paused while the pointer is over the table. High: CPU ≥ 10% or memory ≥ 500 MB. Very high: CPU ≥ 25% or memory ≥ 1 GB.</p>
    <div class="table-wrap"><table>
      <thead><tr>
        <th data-sort="name">Name</th><th>PID</th>
        <th data-sort="cpu" class="num">CPU</th><th data-sort="mem" class="num">Memory</th><th></th>
      </tr></thead>
      <tbody id="pr-rows"><tr><td colspan="5" class="muted">Loading…</td></tr></tbody>
    </table></div>`

  let sort: SortKey = params?.sort === "mem" || params?.sort === "name" ? params.sort : "cpu"
  let rows: Process[] = []
  let stopped = false
  const filterEl = root.querySelector<HTMLInputElement>("#pr-filter")!

  const render = () => {
    const f = filterEl.value.trim().toLowerCase()
    const list = rows
      .filter(p => !f || `${p.name} ${p.relatedTo} ${p.vendor}`.toLowerCase().includes(f))
      .sort((a, b) => sort === "cpu" ? b.cpuPercent - a.cpuPercent
        : sort === "mem" ? b.memoryRss - a.memoryRss
        : a.name.localeCompare(b.name))
    root.querySelector("#pr-count")!.textContent = `${rows.length} running`
    root.querySelectorAll("th[data-sort]").forEach(th =>
      th.classList.toggle("sorted", (th as HTMLElement).dataset.sort === sort))
    root.querySelector("#pr-rows")!.innerHTML = list.map(p => {
      const cpu = level(p.cpuPercent, CPU_WARN, CPU_CRIT)
      const mem = level(p.memoryRss, MEM_WARN, MEM_CRIT)
      const row = worst(cpu, mem)
      return `
      <tr class="${row ? "row-" + row : ""}">
        <td title="${esc(p.path)}">${esc(p.name)}${relatedLine(p)}</td>
        <td class="muted">${p.pid}</td>
        <td class="num ${cpu}">${p.cpuPercent.toFixed(1)}%</td>
        <td class="num ${mem}">${fmtBytes(p.memoryRss)}</td>
        <td class="num">${p.protected ? '<span class="muted small">system</span>'
          : `<button class="btn ghost small" data-kill="${p.pid}" data-name="${esc(p.name)}">End task</button>`}</td>
      </tr>`
    }).join("")
  }

  let paused = false
  let hovering = false
  const stateEl = root.querySelector<HTMLElement>("#pr-state")!
  const pauseBtn = root.querySelector<HTMLButtonElement>("#pr-pause")!
  const showState = () => {
    stateEl.textContent = paused ? "Paused" : hovering ? "Paused while hovering" : ""
    stateEl.classList.toggle("paused", paused || hovering)
  }

  const load = async () => {
    try {
      const next = (await api.ListProcesses()) ?? []
      if (stopped) return
      rows = next
      render()
    } catch (e) {
      if (!stopped) toast(errMsg(e), "err")
    }
  }

  pauseBtn.addEventListener("click", () => {
    paused = !paused
    pauseBtn.textContent = paused ? "Resume" : "Pause"
    showState()
    if (!paused) load()
  })
  root.querySelector("#pr-refresh")!.addEventListener("click", load)
  const wrap = root.querySelector(".table-wrap")!
  wrap.addEventListener("mouseenter", () => { hovering = true; showState() })
  wrap.addEventListener("mouseleave", () => { hovering = false; showState() })

  filterEl.addEventListener("input", render)
  root.querySelector("thead")!.addEventListener("click", e => {
    const key = (e.target as HTMLElement).dataset.sort as SortKey | undefined
    if (key) { sort = key; render() }
  })
  root.querySelector("#pr-rows")!.addEventListener("click", async e => {
    const btn = (e.target as HTMLElement).closest<HTMLButtonElement>("[data-kill]")
    if (!btn) return
    const name = btn.dataset.name!
    if (!confirm(`End ${name}? Unsaved work in it will be lost.`)) return
    try {
      await api.KillProcess(Number(btn.dataset.kill), name)
      toast(`Ended ${name}`)
      load()
    } catch (err) {
      toast(errMsg(err), "err")
    }
  })

  load()
  const timer = window.setInterval(() => { if (!paused && !hovering) load() }, REFRESH_MS)
  return () => { stopped = true; clearInterval(timer) }
}
