export const esc = (s: unknown): string =>
  String(s ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!)

export function fmtBytes(n: number): string {
  if (!n) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1)
  const v = n / 1024 ** i
  return `${v >= 100 || i === 0 ? v.toFixed(0) : v.toFixed(1)} ${units[i]}`
}

export function fmtUptime(sec: number): string {
  const d = Math.floor(sec / 86400), h = Math.floor((sec % 86400) / 3600), m = Math.floor((sec % 3600) / 60)
  return d ? `${d}d ${h}h` : h ? `${h}h ${m}m` : `${m}m`
}

export const errMsg = (e: unknown): string => (e instanceof Error ? e.message : String(e))

let toastTimer: number | undefined
export function toast(msg: string, kind: "ok" | "err" = "ok") {
  const el = document.getElementById("toast")!
  el.textContent = msg
  el.className = `toast ${kind}`
  el.hidden = false
  clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => (el.hidden = true), 4000)
}

export function bar(pct: number): string {
  const level = pct >= 90 ? "crit" : pct >= 75 ? "warn" : ""
  return `<div class="bar"><div class="bar-fill ${level}" style="width:${Math.min(100, Math.max(0, pct)).toFixed(1)}%"></div></div>`
}

/** Disables a button and shows a busy label while the promise runs. */
export async function busy<T>(btn: HTMLButtonElement, label: string, fn: () => Promise<T>): Promise<T> {
  const original = btn.textContent
  btn.disabled = true
  btn.innerHTML = `<span class="spinner"></span>${esc(label)}`
  try {
    return await fn()
  } finally {
    btn.disabled = false
    btn.textContent = original
  }
}

/** A view renders into its container and may return a cleanup function. */
export type ViewParams = Record<string, string>
export type View = (root: HTMLElement, params?: ViewParams) => void | (() => void)

/** Lets a view jump to another tab; main.ts installs the handler. */
export const nav = { go: (_view: string, _params?: ViewParams) => {} }

/** "Related to ..." caption shown under a process name. */
export function relatedLine(p: { relatedTo: string, vendor: string }): string {
  const parts = [p.relatedTo, p.vendor && !p.relatedTo.includes(p.vendor) ? p.vendor : ""].filter(Boolean)
  return `<div class="muted small related">${parts.length ? "Related to: " + esc(parts.join(" · ")) : "Unknown program. Hover for its location."}</div>`
}

export function fmtAgo(ms: number): string {
  const s = Math.max(0, (Date.now() - ms) / 1000)
  if (s < 60) return "just now"
  if (s < 3600) return `${Math.floor(s / 60)} min ago`
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`
  return `${Math.floor(s / 86400)} d ago`
}
