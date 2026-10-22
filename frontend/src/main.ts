import "./style.css"
import { api, AuthStatus } from "./api"
import { errMsg, esc, nav, View, ViewParams } from "./ui"
import { overviewView } from "./views/overview"
import { processesView } from "./views/processes"
import { cleanupView } from "./views/cleanup"
import { startupView } from "./views/startup"
import { driversView } from "./views/drivers"
import { moveView } from "./views/move"
import { oldVersionsView } from "./views/oldversions"
import { storageView } from "./views/storage"
import { appsView } from "./views/apps"
import { settingsView } from "./views/settings"

const views: Record<string, View> = {
  overview: overviewView,
  processes: processesView,
  cleanup: cleanupView,
  startup: startupView,
  drivers: driversView,
  move: moveView,
  oldversions: oldVersionsView,
  storage: storageView,
  apps: appsView,
  settings: settingsView,
}

const main = document.getElementById("view")!
let teardown: (() => void) | void

function show(name: string, params?: ViewParams) {
  const swap = () => {
    teardown?.()
    document.querySelectorAll<HTMLElement>(".nav-item").forEach(b => {
      const on = b.dataset.view === name
      b.classList.toggle("active", on)
      if (on) b.setAttribute("aria-current", "page"); else b.removeAttribute("aria-current")
    })
    main.innerHTML = ""
    main.scrollTop = 0
    teardown = views[name](main, params)
  }
  const reduce = matchMedia("(prefers-reduced-motion: reduce)").matches
  if (!reduce && "startViewTransition" in document) {
    ;(document as any).startViewTransition(swap)
  } else {
    swap()
  }
}

nav.go = show

document.querySelector(".sidebar")!.addEventListener("click", e => {
  const view = (e.target as HTMLElement).closest<HTMLElement>("[data-view]")?.dataset.view
  if (view) show(view)
})

async function renderAdminStatus() {
  const el = document.getElementById("admin-status")!
  const admin = await api.IsAdmin().catch(() => false)
  if (admin) {
    el.innerHTML = '<span><span class="dot ok"></span>Running as admin</span>'
    return
  }
  el.innerHTML = '<span><span class="dot"></span>Limited: not admin</span>'
}


const loginEl = document.getElementById("login")!
const loginMsg = document.getElementById("login-msg")!
const loginErr = document.getElementById("login-err")!
const loginBtn = document.getElementById("login-btn") as HTMLButtonElement
const loginCancel = document.getElementById("login-cancel") as HTMLButtonElement
const userEl = document.getElementById("user-status")!

function renderUser(status: AuthStatus) {
  if (!status.configured || !status.user) { userEl.innerHTML = ""; return }
  userEl.innerHTML = `
    <div class="user-chip" title="${esc(status.user.email)}"><span class="dot ok"></span><span class="name">${esc(status.user.name)}</span></div>
    <button class="btn small" id="logout">Sign out</button>`
  userEl.querySelector("#logout")!.addEventListener("click", async () => {
    await api.Logout()
    teardown?.(); teardown = undefined
    main.innerHTML = ""
    userEl.innerHTML = ""
    showLogin(await api.AuthStatus())
  })
}

function showLogin(status: AuthStatus) {
  loginEl.hidden = false
  loginErr.textContent = status.error ?? ""
  loginMsg.textContent = "Sign in with your Google account to get started."
  loginBtn.disabled = false
  loginCancel.hidden = true
}

loginBtn.addEventListener("click", async () => {
  loginErr.textContent = ""
  loginBtn.disabled = true
  loginCancel.hidden = false
  loginMsg.textContent = "Finish signing in in your browser…"
  try {
    await api.Login()
    loginEl.hidden = true
    start(await api.AuthStatus())
  } catch (e) {
    loginErr.textContent = errMsg(e)
    loginMsg.textContent = "Sign in with your Google account to get started."
    loginBtn.disabled = false
    loginCancel.hidden = true
  }
})
loginCancel.addEventListener("click", () => { api.CancelLogin() })


async function maybeWelcome() {
  const prefs = await api.GetPrefs().catch(() => null)
  if (!prefs || prefs.welcomed) return
  const el = document.getElementById("welcome")!
  const app = document.getElementById("app")!
  const close = (go?: string) => {
    el.hidden = true
    app.inert = false
    api.MarkWelcomed().catch(() => {})
    if (go) show(go)
  }
  el.hidden = false
  app.inert = true // keep keyboard focus inside the dialog
  document.getElementById("welcome-close")!.onclick = () => close()
  document.getElementById("welcome-go")!.onclick = () => close("cleanup")
  el.onkeydown = e => { if (e.key === "Escape") close() }
  document.getElementById("welcome-go")!.focus()
}

function start(status: AuthStatus) {
  renderUser(status)
  renderAdminStatus()
  show("overview")
  maybeWelcome()
}

async function boot() {
  const status = await api.AuthStatus().catch(() => null)
  // A missing config means sign-in is not set up: run open, as before.
  if (!status || !status.configured || status.user) {
    start(status ?? { configured: false, user: null, configPath: "" })
  } else {
    showLogin(status)
  }
}

boot()
