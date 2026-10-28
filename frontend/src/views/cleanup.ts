import { AlertsView, api, CleanupTarget, ScheduleView } from "../api"
import { accountWarning, busy, errMsg, esc, fmtBytes, toast, View } from "../ui"

export const cleanupView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Cleanup</h1>
      <button class="btn" id="cl-scan">Scan again</button></header>
    <div id="cl-account"></div>
    <div id="cl-sched"></div>
    <div id="cl-alerts"></div>
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

  const schedEl = root.querySelector<HTMLElement>("#cl-sched")!
  const renderSched = (v: ScheduleView, plus: boolean) => {
    const last = v.lastRun
      ? `Last run ${esc(v.lastRun)}${v.lastFreed ? `: freed ${fmtBytes(v.lastFreed)}` : ""}. ${esc(v.lastNote ?? "")}`
      : "It has not run yet."
    schedEl.innerHTML = `
      <div class="panel"><div class="row-between">
        <div><strong>Scheduled cleanup</strong> <span class="tag">Plus</span>
          <div class="muted small">Clears temp files and browser caches by itself, weekly or monthly. It never empties the Recycle Bin. ${plus ? last : "Part of Trimmel Plus. Everything on this page still works by hand."}</div></div>
        <div class="row-actions">
          <select id="cl-freq" aria-label="How often" ${plus ? "" : "disabled"}>
            <option value="weekly" ${v.frequency === "weekly" ? "selected" : ""}>Weekly</option>
            <option value="monthly" ${v.frequency === "monthly" ? "selected" : ""}>Monthly</option></select>
          <button class="btn ${v.enabled ? "" : "primary"}" id="cl-sched-toggle" ${plus ? "" : "disabled"}>${v.enabled ? "Switch off" : "Switch on"}</button>
        </div></div></div>`
  }
  const loadSched = async () => {
    try {
      const [v, lic] = await Promise.all([api.GetSchedule(), api.LicenseStatus()])
      let cur = v
      const plus = lic.tier !== "free"
      renderSched(cur, plus)
      schedEl.onclick = async e => {
        if (!(e.target as HTMLElement).closest("#cl-sched-toggle")) return
        const freq = schedEl.querySelector<HTMLSelectElement>("#cl-freq")!.value
        try {
          cur = await api.SetSchedule(!cur.enabled, freq)
          toast(cur.enabled ? "Scheduled cleanup is on." : "Scheduled cleanup is off.")
        } catch (err) { toast(errMsg(err), "err") }
        renderSched(cur, plus)
      }
    } catch { schedEl.innerHTML = "" }
  }

  const alertsEl = root.querySelector<HTMLElement>("#cl-alerts")!
  const renderAlerts = (v: AlertsView, plus: boolean) => {
    const dis = plus ? "" : "disabled"
    alertsEl.innerHTML = `
      <div class="panel"><strong>Alerts</strong> <span class="tag">Plus</span>
        <div class="muted small">Quiet Windows notifications, checked once a day. They never change anything; the button only opens Trimmel.
          ${plus ? "" : "Part of Trimmel Plus."}</div>
        <label class="list-row"><input type="checkbox" id="al-low" ${v.lowDisk ? "checked" : ""} ${dis}>
          <div class="grow">Tell me when a drive is low on space (at most once a week per drive)</div>
          <select id="al-pct" aria-label="Low space threshold" ${dis}>
            ${[5, 10, 15, 20].map(p => `<option value="${p}" ${v.threshold === p ? "selected" : ""}>below ${p}% free</option>`).join("")}</select></label>
        <label class="list-row"><input type="checkbox" id="al-rep" ${v.report ? "checked" : ""} ${dis}>
          <div class="grow">Send me a weekly summary of free space</div></label>
      </div>`
  }
  const loadAlerts = async () => {
    try {
      const [v, lic] = await Promise.all([api.GetAlerts(), api.LicenseStatus()])
      const plus = lic.tier !== "free"
      let cur = v
      renderAlerts(cur, plus)
      alertsEl.onchange = async () => {
        const low = alertsEl.querySelector<HTMLInputElement>("#al-low")!.checked
        const rep = alertsEl.querySelector<HTMLInputElement>("#al-rep")!.checked
        const pct = Number(alertsEl.querySelector<HTMLSelectElement>("#al-pct")!.value)
        try {
          cur = await api.SetAlerts(low, pct, rep)
          toast("Alert settings saved.")
        } catch (err) { toast(errMsg(err), "err") }
        renderAlerts(cur, plus)
      }
    } catch { alertsEl.innerHTML = "" }
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
  accountWarning().then(h => { root.querySelector("#cl-account")!.innerHTML = h })
  scan()
  loadSched()
  loadAlerts()
}
