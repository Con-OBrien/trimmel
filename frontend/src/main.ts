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

const views: Record<string, View> = {
  overview: overviewView,
  processes: processesView,
  cleanup: cleanupView,
  startup: startupView,
  drivers: driversView,
  move: moveView,
  oldversions: oldVersionsView,
}

const main = document.getElementById("view")!
let teardown: (() => void) | void

function show(name: string, params?: ViewParams) {
  const swap = () => {
    teardown?.()
    document.querySelectorAll<HTMLElement>(".nav-item").forEach(b =>
      b.classList.toggle("active", b.dataset.view === name))
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

// ---- Sign-in ----

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
  loginMsg.textContent = "Welcome back. Sign in with Google to get started."
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
    loginMsg.textContent = "Welcome back. Sign in with Google to get started."
    loginBtn.disabled = false
    loginCancel.hidden = true
  }
})
loginCancel.addEventListener("click", () => { api.CancelLogin() })

function start(status: AuthStatus) {
  renderUser(status)
  renderAdminStatus()
  show("overview")
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
