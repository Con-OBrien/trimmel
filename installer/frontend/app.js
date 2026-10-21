"use strict"

const go = () => window.go.main.App
const screenEl = document.getElementById("screen")
const stepsEl = document.getElementById("steps")

let info = null
const opts = { dir: "", desktopLink: true, removeConfig: false, launch: true }

const esc = s => String(s).replace(/[&<>"]/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c]))
const errMsg = e => (typeof e === "string" ? e : e?.message) || "Something went wrong."

function setSteps(names, active) {
  stepsEl.innerHTML = names.map((n, i) =>
    `<li class="${i < active ? "done" : i === active ? "active" : ""}">
       <span class="num">${i < active ? "✓" : i + 1}</span>${n}</li>`).join("")
}

function show(html, bind) {
  screenEl.style.animation = "none"
  void screenEl.offsetHeight
  screenEl.style.animation = ""
  screenEl.innerHTML = html
  bind?.()
}

const toggleRow = (id, title, sub, checked, disabled) => `
  <label class="row">
    <div class="grow"><b>${title}</b><span class="muted small">${sub}</span></div>
    <span class="switch"><input type="checkbox" id="${id}" ${checked ? "checked" : ""} ${disabled ? "disabled" : ""}><span class="slider"></span></span>
  </label>`

const INSTALL_STEPS = ["Welcome", "Options", "Installing", "Finished"]

/* ---------- Install flow ---------- */

function welcome() {
  setSteps(INSTALL_STEPS, 0)
  const upd = !!info.existing
  show(`
    <h1>${upd ? "Update PC Optimizer" : "Welcome to PC Optimizer"}</h1>
    <p class="lead">${upd
      ? `A copy is already installed. Setup will replace it with version ${esc(info.version)} and keep your settings.`
      : "Free up space, trim slow startups and keep drivers current, with plain-English explanations and nothing deleted behind your back."}</p>
    <div class="grid">
      <div class="feat"><div class="ico">🧹</div><div><b>Cleanup</b><span>See what each item takes up, then tick what goes.</span></div></div>
      <div class="feat"><div class="ico">🚀</div><div><b>Startup</b><span>Switch slow launchers off. Fully reversible.</span></div></div>
      <div class="feat"><div class="ico">📦</div><div><b>Move apps</b><span>Shift big programs to another drive safely.</span></div></div>
      <div class="feat"><div class="ico">🔧</div><div><b>Drivers</b><span>Signed updates straight from Windows Update.</span></div></div>
    </div>
    <div class="foot">
      <span class="muted small">Version ${esc(info.version)} · needs administrator rights</span>
      <div class="right"><button class="btn primary big" id="next">${upd ? "Continue" : "Get started"}</button></div>
    </div>`, () => {
    document.getElementById("next").onclick = options
  })
}

function options() {
  setSteps(INSTALL_STEPS, 1)
  show(`
    <h1>Make it yours</h1>
    <p class="lead">Defaults suit most people. Change them if you like.</p>
    <div class="section">
      <div class="label">Install location</div>
      <div class="panel"><div class="field">
        <input type="text" id="dir" spellcheck="false" value="${esc(opts.dir)}">
        <button class="btn" id="browse">Browse…</button>
      </div></div>
      <div class="muted small" style="margin:8px 0 0 4px">About ${info.sizeMB} MB of space needed.</div>
    </div>
    <div class="section">
      <div class="label">Shortcuts</div>
      <div class="panel">
        ${toggleRow("t-start", "Start Menu", "Always added so you can find the app.", true, true)}
        ${toggleRow("t-desk", "Desktop", "A shortcut on your desktop.", opts.desktopLink)}
        ${toggleRow("t-launch", "Start PC Optimizer when setup finishes", "Windows will ask for admin permission.", opts.launch)}
      </div>
    </div>
    <div class="err" id="err"></div>
    <div class="foot">
      <button class="btn ghost" id="back">Back</button>
      <div class="right"><button class="btn primary big" id="go">${info.existing ? "Update" : "Install"}</button></div>
    </div>`, () => {
    const dir = document.getElementById("dir")
    dir.oninput = () => (opts.dir = dir.value)
    document.getElementById("t-desk").onchange = e => (opts.desktopLink = e.target.checked)
    document.getElementById("t-launch").onchange = e => (opts.launch = e.target.checked)
    document.getElementById("browse").onclick = async () => {
      const p = await go().PickDir(dir.value).catch(() => "")
      if (p) { opts.dir = p.replace(/[\\/]+$/, "") + (/pc optimizer$/i.test(p) ? "" : "\\PC Optimizer"); dir.value = opts.dir }
    }
    document.getElementById("back").onclick = welcome
    document.getElementById("go").onclick = runInstall
  })
}

function progressScreen(title, sub) {
  show(`
    <div class="center">
      <h1>${title}</h1>
      <p class="lead">${sub}</p>
      <div class="stage"><span id="stage"><span class="spinner"></span>Starting…</span><span id="pct" class="muted">0%</span></div>
      <div class="bar"><div class="bar-fill" id="fill"></div></div>
      <div class="err" id="err"></div>
    </div>`)
}

window.runtime.EventsOn("setup:progress", p => {
  const stage = document.getElementById("stage")
  if (!stage) return
  const last = p.percent >= 100
  stage.innerHTML = (last ? "" : '<span class="spinner"></span>') + esc(p.stage)
  document.getElementById("pct").textContent = Math.round(p.percent) + "%"
  document.getElementById("fill").style.width = p.percent + "%"
})

async function runInstall() {
  const err = document.getElementById("err")
  if (!opts.dir.trim()) { err.textContent = "Choose an install folder."; return }
  setSteps(INSTALL_STEPS, 2)
  progressScreen(info.existing ? "Updating…" : "Installing…", "This only takes a moment. Please keep this window open.")
  try {
    await go().Install({ dir: opts.dir.trim(), desktopLink: opts.desktopLink, removeConfig: false })
    await new Promise(r => setTimeout(r, 500))
    finished()
  } catch (e) {
    show(`
      <div class="center">
        <h1>That didn't work</h1>
        <p class="lead">Nothing harmful happened. You can go back and try again.</p>
        <div class="hint warn err" style="color:var(--text)">${esc(errMsg(e))}</div>
      </div>
      <div class="foot"><button class="btn ghost" id="back">Back</button>
        <div class="right"><button class="btn" id="quit">Close</button></div></div>`, () => {
      document.getElementById("back").onclick = options
      document.getElementById("quit").onclick = () => go().Close()
    })
  }
}

const CHECK = `<div class="done-badge"><svg viewBox="0 0 24 24" fill="none" stroke="#178f86" stroke-width="2.8" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12.5l4.5 4.5L19 7.5"/></svg></div>`

function finished() {
  setSteps(INSTALL_STEPS, 4)
  show(`
    <div class="center">
      ${CHECK}
      <h1>PC Optimizer is ready</h1>
      <p class="lead">It's installed${opts.desktopLink ? ", with a shortcut on your desktop and in the Start Menu" : " and in your Start Menu"}.</p>
    </div>
    <div class="foot"><span></span>
      <div class="right">
        <button class="btn" id="quit">Close</button>
        <button class="btn primary big" id="launch">Start PC Optimizer</button>
      </div></div>`, () => {
    document.getElementById("quit").onclick = () => go().Close()
    document.getElementById("launch").onclick = () => go().Launch(opts.dir.trim()).catch(e => alert(errMsg(e)))
    if (opts.launch) setTimeout(() => go().Launch(opts.dir.trim()).catch(() => {}), 900)
  })
}

/* ---------- Uninstall flow ---------- */

const UN_STEPS = ["Confirm", "Removing", "Finished"]

function confirmRemove() {
  setSteps(UN_STEPS, 0)
  show(`
    <h1>Remove PC Optimizer</h1>
    <p class="lead">This takes PC Optimizer off your computer. Files your cleanup already removed stay removed, and apps you moved to another drive keep working.</p>
    <div class="panel">
      ${toggleRow("t-cfg", "Also delete my settings", "Sign-in details and preferences stored for this user.", opts.removeConfig)}
    </div>
    <div class="foot">
      <button class="btn ghost" id="cancel">Cancel</button>
      <div class="right"><button class="btn danger big" id="go">Remove</button></div>
    </div>`, () => {
    document.getElementById("t-cfg").onchange = e => (opts.removeConfig = e.target.checked)
    document.getElementById("cancel").onclick = () => go().Close()
    document.getElementById("go").onclick = runUninstall
  })
}

async function runUninstall() {
  setSteps(UN_STEPS, 1)
  progressScreen("Removing…", "Please keep this window open.")
  try {
    await go().Uninstall({ dir: "", desktopLink: false, removeConfig: opts.removeConfig })
    await new Promise(r => setTimeout(r, 500))
    setSteps(UN_STEPS, 3)
    show(`
      <div class="center">${CHECK}<h1>PC Optimizer was removed</h1>
        <p class="lead">Thanks for trying it. You can reinstall any time.</p></div>
      <div class="foot"><span></span><div class="right"><button class="btn primary big" id="quit">Close</button></div></div>`, () => {
      document.getElementById("quit").onclick = () => go().Close()
    })
  } catch (e) {
    show(`<div class="center"><h1>Couldn't finish removing</h1><p class="lead">Close PC Optimizer and try again.</p>
      <div class="hint warn" style="color:var(--text)">${esc(errMsg(e))}</div></div>
      <div class="foot"><span></span><div class="right"><button class="btn" id="quit">Close</button></div></div>`, () => {
      document.getElementById("quit").onclick = () => go().Close()
    })
  }
}

/* ---------- Boot ---------- */

document.getElementById("min").onclick = () => go().Minimise()
document.getElementById("close").onclick = () => go().Close()

go().Info().then(i => {
  info = i
  opts.dir = i.defaultDir
  document.getElementById("ver").textContent = "Version " + i.version
  i.mode === "uninstall" ? confirmRemove() : welcome()
})
