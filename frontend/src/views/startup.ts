import { api, StartupItem } from "../api"
import { accountWarning, errMsg, esc, toast, View } from "../ui"

export const startupView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Startup</h1><span class="muted" id="st-count"></span></header>
    <p class="muted small">Disabling works like Task Manager: the entry stays, Windows just skips it at sign-in.
      You can turn it back on any time. Impact is a rough guide from what each program usually does. Microsoft Store apps are listed too.</p>
    <div id="st-account"></div>
    <div id="st-suggest"></div>
    <div id="st-body" class="panel list"><p class="muted">Loading…</p></div>`

  const body = root.querySelector<HTMLElement>("#st-body")!
  const suggestEl = root.querySelector<HTMLElement>("#st-suggest")!
  let items: StartupItem[] = []

  const IMPACT_ORDER = { high: 0, medium: 1, low: 2 }
  const sorted = () => [...items].sort((a, b) =>
    Number(b.enabled) - Number(a.enabled) ||
    IMPACT_ORDER[a.impact] - IMPACT_ORDER[b.impact] ||
    a.name.localeCompare(b.name))

  const render = () => {
    const on = items.filter(i => i.enabled).length
    root.querySelector("#st-count")!.textContent = `${on} of ${items.length} enabled`
    const suggest = items.filter(i => i.enabled && i.recommend === "disable")
    suggestEl.innerHTML = suggest.length
      ? `<div class="hint">${suggest.length} enabled ${suggest.length === 1 ? "program is" : "programs are"} usually safe to turn off
          (${suggest.slice(0, 4).map(i => esc(i.name)).join(", ")}${suggest.length > 4 ? ", …" : ""}). Look for the
          <span class="tag good">suggested</span> tag below.</div>`
      : ""
    body.innerHTML = items.length ? sorted().map(i => `
      <div class="list-row">
        <div class="grow">
          <div><strong>${esc(i.name)}</strong>
            <span class="tag">${i.scope === "machine" ? "all users" : "you"}</span>
            <span class="tag impact-${i.impact}" title="Rough effect on sign-in time, based on what this program usually does">${i.impact} impact</span>
            ${i.enabled && i.recommend === "disable" ? '<span class="tag good">suggested: turn off</span>' : ""}
            ${i.recommend === "keep" ? '<span class="tag">best to keep</span>' : ""}</div>
          ${i.note ? `<div class="small">${esc(i.note)}</div>` : ""}
          <div class="muted small mono" title="${esc(i.command)}">${esc(i.command)}</div>
          <div class="muted small">${esc(i.source)}</div>
        </div>
        <label class="switch" title="${i.enabled ? "Disable" : "Enable"}">
          <input type="checkbox" data-id="${esc(i.id)}" aria-label="${esc(i.name)} starts with Windows" ${i.enabled ? "checked" : ""} />
          <span class="slider"></span>
        </label>
      </div>`).join("") : '<p class="muted">No startup programs found.</p>'
  }

  const load = async () => {
    try {
      items = (await api.ListStartup()) ?? []
      render()
    } catch (e) {
      body.innerHTML = `<p class="error">${esc(errMsg(e))}</p>`
    }
  }

  body.addEventListener("change", async e => {
    const cb = e.target as HTMLInputElement
    const item = items.find(i => i.id === cb.dataset.id)
    if (!item) return
    cb.disabled = true
    try {
      await api.SetStartupEnabled(item.id, cb.checked)
      item.enabled = cb.checked
      toast(`${item.name} ${cb.checked ? "enabled" : "disabled"}`)
      render()
    } catch (err) {
      cb.checked = !cb.checked
      cb.disabled = false
      toast(errMsg(err), "err")
    }
  })

  accountWarning().then(h => { root.querySelector("#st-account")!.innerHTML = h })
  load()
}
