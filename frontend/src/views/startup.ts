import { api, StartupItem } from "../api"
import { errMsg, esc, toast, View } from "../ui"

export const startupView: View = root => {
  root.innerHTML = `
    <header class="view-head"><h1>Startup</h1><span class="muted" id="st-count"></span></header>
    <p class="muted small">Disabling works like Task Manager: the entry stays, Windows just skips it at sign-in.
      You can turn it back on any time.</p>
    <div id="st-body" class="panel list"><p class="muted">Loading…</p></div>`

  const body = root.querySelector<HTMLElement>("#st-body")!
  let items: StartupItem[] = []

  const render = () => {
    const on = items.filter(i => i.enabled).length
    root.querySelector("#st-count")!.textContent = `${on} of ${items.length} enabled`
    body.innerHTML = items.length ? items.map(i => `
      <div class="list-row">
        <div class="grow">
          <div><strong>${esc(i.name)}</strong>
            <span class="tag">${i.scope === "machine" ? "all users" : "you"}</span></div>
          <div class="muted small mono" title="${esc(i.command)}">${esc(i.command)}</div>
          <div class="muted small">${esc(i.source)}</div>
        </div>
        <label class="switch" title="${i.enabled ? "Disable" : "Enable"}">
          <input type="checkbox" data-id="${esc(i.id)}" ${i.enabled ? "checked" : ""} />
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

  load()
}
