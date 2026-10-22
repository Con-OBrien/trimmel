import { api, Driver, DriverProgress, DriverScanState, DriverUpdate } from "../api"
import { busy, errMsg, esc, fmtAgo, toast, View } from "../ui"

interface Category { name: string, desc: string, classes: string[] }

const CATEGORIES: Category[] = [
  { name: "Graphics", desc: "Video cards and displays", classes: ["DISPLAY", "MONITOR"] },
  { name: "Audio", desc: "Sound devices, speakers, microphones", classes: ["MEDIA", "AUDIOENDPOINT"] },
  { name: "Network", desc: "Ethernet, Wi-Fi and virtual adapters", classes: ["NET", "NETCLIENT", "NETSERVICE", "NETTRANS"] },
  { name: "Bluetooth", desc: "Bluetooth radios and paired devices", classes: ["BLUETOOTH"] },
  { name: "Storage", desc: "Disks, drive controllers, volumes", classes: ["DISKDRIVE", "HDC", "SCSIADAPTER", "CDROM", "VOLUME", "VOLUMESNAPSHOT", "STORAGEVOLUME", "NVME"] },
  { name: "Processor & Chipset", desc: "CPU, motherboard and platform drivers", classes: ["PROCESSOR", "SYSTEM", "FIRMWARE", "COMPUTER", "MTD"] },
  { name: "Input", desc: "Keyboards, mice, touchpads, game controllers", classes: ["KEYBOARD", "MOUSE", "HIDCLASS", "XBOXCOMPOSITE"] },
  { name: "USB & Ports", desc: "USB controllers, hubs, serial and parallel ports", classes: ["USB", "PORTS", "1394", "USBDEVICE"] },
  { name: "Cameras & Imaging", desc: "Webcams and scanners", classes: ["CAMERA", "IMAGE"] },
  { name: "Printers", desc: "Printers and print queues", classes: ["PRINTQUEUE", "PRINTER"] },
  { name: "Power & Battery", desc: "Batteries and power management", classes: ["BATTERY"] },
  { name: "Security & Sensors", desc: "Smart cards, fingerprint, TPM, sensors", classes: ["SMARTCARDREADER", "BIOMETRIC", "SENSOR", "SECURITYDEVICES"] },
  { name: "Software & Virtual", desc: "Software-defined components and extensions", classes: ["SOFTWAREDEVICE", "SOFTWARECOMPONENT", "EXTENSION", "WPD", "HOSTSYSTEM"] },
]
const OTHER: Category = { name: "Other", desc: "Everything not covered above", classes: [] }

const categoryOf = (cls: string): Category =>
  CATEGORIES.find(c => c.classes.includes(cls.toUpperCase())) ?? OTHER
const categoryRank = (name: string) => {
  const i = CATEGORIES.findIndex(c => c.name === name)
  return i < 0 ? CATEGORIES.length : i
}

export const driversView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Drivers</h1>
      <span class="muted small" id="dr-status"></span>
      <button class="btn primary" id="dr-check">Check now</button></header>
    <p class="muted small">Updates come from Windows Update, so they're signed by Microsoft and matched to your hardware.
      Checked automatically when you open the app.</p>
    <div id="dr-progress"></div>
    <div id="dr-updates"></div>
    <h2>Installed <span class="muted small" id="dr-count"></span></h2>
    <div class="toolbar"><input id="dr-filter" type="search" placeholder="Filter by device, class or provider" /></div>
    <p class="muted small">Grouped by what the hardware does. Identical entries are merged (×N), e.g. several USB hubs sharing one driver.</p>
    <div id="dr-groups"><p class="muted">Loading…</p></div>`

  let drivers: Driver[] = []
  let updates: DriverUpdate[] = []
  let scan: DriverScanState = { updates: [], checkedAt: 0, checking: false }
  const seen = new Set<string>()
  const chosen = new Set<string>()
  const filterEl = root.querySelector<HTMLInputElement>("#dr-filter")!
  const updatesEl = root.querySelector<HTMLElement>("#dr-updates")!
  const progEl = root.querySelector<HTMLElement>("#dr-progress")!

  // Windows Update gives no byte counts, so show which of the three steps is running.
  const STEPS: Record<DriverProgress["stage"], string> = {
    restore: "Making a System Restore point…",
    download: "Downloading from Windows Update…",
    install: "Installing…",
  }
  const showProgress = (stage: DriverProgress["stage"], count: number) => {
    const n = ["restore", "download", "install"].indexOf(stage) + 1
    progEl.innerHTML = `<div class="panel"><div class="row-between"><strong>${count} driver update${count === 1 ? "" : "s"}</strong>
      <span class="muted">Step ${n} of 3: ${STEPS[stage]}</span></div>
      <div class="bar"><div class="bar-fill" style="width:${(n * 100) / 3}%"></div></div>
      <div class="muted small">Keep the app open. This can take several minutes.</div></div>`
  }
  const off = window.runtime?.EventsOn("drivers:progress", (p: DriverProgress) => showProgress(p.stage, p.count))
  const statusEl = root.querySelector<HTMLElement>("#dr-status")!
  const checkBtn = root.querySelector<HTMLButtonElement>("#dr-check")!

  const renderInstalled = () => {
    const f = filterEl.value.trim().toLowerCase()
    const list = drivers.filter(d => !f || `${d.device} ${d.class} ${d.provider} ${categoryOf(d.class).name}`.toLowerCase().includes(f))

    // Collapse identical entries (same device, provider, version) into one row with a count.
    const groups = new Map<string, Map<string, { d: Driver, n: number }>>()
    for (const d of list) {
      const cat = categoryOf(d.class).name
      const rows = groups.get(cat) ?? new Map<string, { d: Driver, n: number }>()
      groups.set(cat, rows)
      const key = `${d.device}|${d.provider}|${d.version}|${d.date}`
      const hit = rows.get(key)
      if (hit) hit.n++
      else rows.set(key, { d, n: 1 })
    }
    const cats = [...groups.keys()].sort((a, b) => categoryRank(a) - categoryRank(b))
    const unique = cats.reduce((n, c) => n + groups.get(c)!.size, 0)
    root.querySelector("#dr-count")!.textContent = `(${drivers.length} drivers, ${unique} unique)`

    root.querySelector("#dr-groups")!.innerHTML = cats.map(cat => {
      const rows = [...groups.get(cat)!.values()]
      const info = CATEGORIES.find(c => c.name === cat) ?? OTHER
      return `
      <details class="drv-group" ${f ? "open" : ""}>
        <summary><strong>${esc(cat)}</strong> <span class="muted small">${rows.length} · ${esc(info.desc)}</span></summary>
        <div class="table-wrap"><table>
          <thead><tr><th>Device</th><th>Provider</th><th>Version</th><th>Date</th></tr></thead>
          <tbody>${rows.map(({ d, n }) => `
            <tr>
              <td>${esc(d.device)}${n > 1 ? ` <span class="tag">×${n}</span>` : ""}${d.signed ? "" : ' <span class="tag warn">unsigned</span>'}
                <div class="muted small">${esc(d.class)}</div></td>
              <td class="muted">${esc(d.provider)}</td>
              <td class="mono small">${esc(d.version)}</td>
              <td class="muted">${esc(d.date)}</td>
            </tr>`).join("")}</tbody>
        </table></div>
      </details>`
    }).join("") || '<p class="muted">No matching drivers.</p>'
  }

  const renderUpdates = () => {
    const checked = scan.checkedAt ? `Last checked ${fmtAgo(scan.checkedAt)}` : "Not checked yet"
    statusEl.textContent = scan.checking ? "Checking Windows Update…" : checked
    checkBtn.disabled = scan.checking
    if (scan.checking && !scan.checkedAt) {
      updatesEl.innerHTML = '<div class="panel"><p class="muted"><span class="spinner"></span>Asking Windows Update. This can take a minute or two…</p></div>'
      return
    }
    if (scan.error && !scan.checkedAt) {
      updatesEl.innerHTML = `<div class="panel"><p class="error">${esc(scan.error)}</p></div>`
      return
    }
    if (!updates.length) {
      updatesEl.innerHTML = '<div class="panel"><p class="ok">Your drivers are up to date with Windows Update.</p></div>'
      return
    }
    updatesEl.innerHTML = `
      <div class="panel list">
        ${updates.map(u => `
          <label class="list-row ${u.needsEula ? "disabled" : ""}">
            <input type="checkbox" data-id="${esc(u.id)}" ${chosen.has(u.id) ? "checked" : ""} ${u.needsEula ? "disabled" : ""} />
            <div class="grow">
              <div><strong>${esc(u.title)}</strong></div>
              <div class="muted small">${esc([u.manufacturer, u.class, u.date].filter(Boolean).join(" · "))}</div>
              ${u.needsEula ? '<div class="muted small">Has licence terms to accept: install it from Windows Update.</div>' : ""}
            </div>
          </label>`).join("")}
      </div>
      <div class="action-bar">
        <span>${updates.length} update${updates.length === 1 ? "" : "s"} available</span>
        <button class="btn primary" id="dr-install" ${chosen.size ? "" : "disabled"}>Install selected</button>
      </div>`
  }

  updatesEl.addEventListener("change", e => {
    const cb = e.target as HTMLInputElement
    if (!cb.dataset.id) return
    cb.checked ? chosen.add(cb.dataset.id) : chosen.delete(cb.dataset.id)
    root.querySelector<HTMLButtonElement>("#dr-install")!.disabled = chosen.size === 0
  })

  updatesEl.addEventListener("click", async e => {
    const btn = (e.target as HTMLElement).closest<HTMLButtonElement>("#dr-install")
    if (!btn) return
    const metered = await api.NetworkMetered().catch(() => false)
    if (!confirm(`${metered ? "This connection is metered, so the download may use part of your data allowance.\n\n" : ""}Install ${chosen.size} driver update(s)? Devices may briefly disconnect while they install. A System Restore point is made first if Windows allows it, and this can take several minutes.`)) return
    showProgress("restore", chosen.size)
    try {
      const r = await busy(btn, "Downloading and installing…", () => api.InstallDriverUpdates([...chosen]))
      const ok = r.installed?.length ?? 0, bad = r.failed?.length ?? 0
      toast(`${ok} installed${bad ? `, ${bad} failed` : ""}${r.rebootRequired ? ". Restart to finish." : ""}${r.restorePoint ? "" : ". No restore point was made (System Protection may be off)"}`, bad ? "err" : "ok")
      progEl.innerHTML = ""
      chosen.clear()
      lastKey = ""
      await poll()
      loadInstalled()
    } catch (err) {
      progEl.innerHTML = ""
      toast(errMsg(err), "err")
    }
  })

  checkBtn.addEventListener("click", () => poll(true))

  // Fetch scan state (optionally starting a scan) and re-render when it changed.
  let lastKey = ""
  const poll = async (force = false) => {
    try {
      scan = await (force || !lastKey ? api.StartDriverScan(force) : api.DriverScanState())
    } catch (err) {
      updatesEl.innerHTML = `<div class="panel"><p class="error">${esc(errMsg(err))}</p></div>`
      return
    }
    updates = scan.updates ?? []
    const key = `${scan.checkedAt}|${scan.checking}|${scan.error}|${updates.length}`
    if (key === lastKey) return
    lastKey = key
    const ids = new Set(updates.map(u => u.id))
    for (const id of [...chosen]) if (!ids.has(id)) chosen.delete(id)
    if (!scan.checking) updates.forEach(u => { if (!u.needsEula && !seen.has(u.id)) { chosen.add(u.id); seen.add(u.id) } })
    renderUpdates()
  }

  const loadInstalled = async () => {
    try {
      drivers = (await api.ListDrivers()) ?? []
      renderInstalled()
    } catch (e) {
      root.querySelector("#dr-groups")!.innerHTML = `<p class="error">${esc(errMsg(e))}</p>`
    }
  }

  filterEl.addEventListener("input", renderInstalled)
  loadInstalled()
  poll()
  const timer = window.setInterval(() => poll(), 2000)
  return () => { clearInterval(timer); off?.() }
}
