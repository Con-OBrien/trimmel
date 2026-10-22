// Stand-in for the Wails backend, used only to take website screenshots of the
// real UI with sample data. See tools/screenshots/README.md.
(function () {
  const GB = 1024 ** 3, MB = 1024 ** 2
  const q = new URLSearchParams(location.search)
  const ok = v => () => Promise.resolve(v)

  const disks = [
    { mount: "C:", fsType: "NTFS", total: 476 * GB, free: 34 * GB, usedPercent: 92.9 },
    { mount: "D:", fsType: "NTFS", total: 931 * GB, free: 612 * GB, usedPercent: 34.3 },
  ]

  const overview = {
    system: {
      hostname: "HOME-PC", os: "Windows 11 Home", uptimeSeconds: 3 * 86400 + 5 * 3600,
      cpu: { model: "AMD Ryzen 5 5600", cores: 6, threads: 12, usagePercent: 14 },
      memory: { total: 16 * GB, used: 9.7 * GB, available: 6.3 * GB, usedPercent: 60.6 },
      disks, warnings: null,
    },
    hints: [],
  }

  const health = {
    score: 68, label: "Could use a tidy",
    issues: [
      { level: "warn", title: "C: is running low on space", detail: "Less than 10% is free. Cleaning temporary files is the quickest win.", view: "storage", action: "See what uses space" },
      { level: "info", title: "5 programs start with Windows that you can probably turn off", detail: "Turning them off makes sign-in faster. You can switch them back on any time.", view: "startup", action: "Review startup" },
      { level: "info", title: "Time for a tidy", detail: "It has been 41 days since your last Cleanup.", view: "cleanup", action: "Run Cleanup" },
    ],
  }

  const cleanup = [
    { id: "user-temp", name: "Your temp files", description: "Leftovers from installers and apps in %TEMP%. Files from the last 24 hours are kept.", requiresAdmin: false, available: true, bytes: 3.4 * GB, files: 18214 },
    { id: "wu-cache", name: "Windows Update downloads", description: "Update packages that are already installed. Windows re-downloads anything it still needs.", requiresAdmin: true, available: true, bytes: 2.8 * GB, files: 912 },
    { id: "chrome-cache", name: "Chrome cache", description: "Cached web content for all Chrome profiles. Close Chrome first for best results.", requiresAdmin: false, available: true, bytes: 1.2 * GB, files: 7430 },
    { id: "shader-cache", name: "Graphics shader caches", description: "Compiled shaders saved by DirectX, NVIDIA and AMD drivers. They rebuild on demand.", requiresAdmin: false, available: true, bytes: 1.1 * GB, files: 640 },
    { id: "recycle-bin", name: "Recycle Bin", description: "Empties the Recycle Bin on all drives. This cannot be undone.", requiresAdmin: false, available: true, bytes: 880 * MB, files: 214 },
    { id: "crash-dumps", name: "Crash dumps", description: "Memory dumps written when apps crash. Only useful for debugging those crashes.", requiresAdmin: false, available: true, bytes: 640 * MB, files: 12 },
    { id: "edge-cache", name: "Edge cache", description: "Cached web content for all Edge profiles. Close Edge first for best results.", requiresAdmin: false, available: true, bytes: 410 * MB, files: 2310 },
    { id: "error-reports", name: "Windows error reports", description: "Crash and error reports Windows keeps for sending to Microsoft. Safe to remove.", requiresAdmin: true, available: true, bytes: 220 * MB, files: 48 },
  ]

  const st = (name, command, source, scope, enabled, impact, note, recommend) =>
    ({ id: "x|" + name, name, command, source, scope, enabled, requiresAdmin: scope === "machine", impact, note, recommend })
  const startup = [
    st("Microsoft Teams", "C:\\Users\\you\\AppData\\Local\\Microsoft\\Teams\\Update.exe --processStart Teams.exe", "Registry: HKCU\\...\\Run", "user", true, "high", "Microsoft Teams chat. It opens at sign-in and uses a lot of memory. Open it yourself when you need it.", "disable"),
    st("Discord", "C:\\Users\\you\\AppData\\Local\\Discord\\Update.exe --processStart Discord.exe", "Registry: HKCU\\...\\Run", "user", true, "high", "Discord chat. Keep it only if you want to be online as soon as you sign in.", "disable"),
    st("Steam", "\"C:\\Program Files (x86)\\Steam\\steam.exe\" -silent", "Registry: HKCU\\...\\Run", "user", true, "high", "Steam game launcher. Games still start it when needed.", "disable"),
    st("Spotify", "C:\\Users\\you\\AppData\\Roaming\\Spotify\\Spotify.exe /minimized", "Registry: HKCU\\...\\Run", "user", true, "medium", "Spotify music player. Open it yourself when you want music.", "disable"),
    st("AdobeAAMUpdater-1.0", "\"C:\\Program Files (x86)\\Common Files\\Adobe\\OOBE\\PDApp\\UWA\\UpdaterStartupUtility.exe\"", "Registry: HKLM\\...\\Run", "machine", true, "medium", "Adobe update and background helpers. Safe to disable; Adobe apps still work and update when opened.", "disable"),
    st("OneDrive", "\"C:\\Program Files\\Microsoft OneDrive\\OneDrive.exe\" /background", "Registry: HKCU\\...\\Run", "user", true, "medium", "Syncs files to Microsoft OneDrive. Disable it if you don't use OneDrive.", ""),
    st("SecurityHealth", "%windir%\\system32\\SecurityHealthSystray.exe", "Registry: HKLM\\...\\Run", "machine", true, "low", "Windows Security. Keep it on.", "keep"),
    st("RtkAudUService", "\"C:\\WINDOWS\\System32\\RtkAudUService64.exe\" -background", "Registry: HKLM\\...\\Run", "machine", true, "low", "Sound card helper. Disabling it can break audio controls or effects.", "keep"),
    st("Google Update", "C:\\Program Files (x86)\\Google\\Update\\GoogleUpdate.exe /c", "Scheduled task", "machine", false, "low", "Keeps Chrome up to date. Safe to disable; Chrome updates when you open it.", "disable"),
  ]

  const storage = {
    path: "C:\\", total: 442 * GB, partial: false, skipped: 3,
    entries: [
      { name: "Users", path: "C:\\Users", size: 214 * GB, files: 412000, isDir: true, hint: "User accounts: documents, downloads and app data." },
      { name: "Program Files", path: "C:\\Program Files", size: 71 * GB, files: 188000, isDir: true, hint: "Installed programs. Use Apps or Move apps to free space." },
      { name: "Windows", path: "C:\\Windows", size: 38 * GB, files: 301000, isDir: true, hint: "Windows itself. Don't delete anything here." },
      { name: "Program Files (x86)", path: "C:\\Program Files (x86)", size: 33 * GB, files: 102000, isDir: true, hint: "Installed programs. Use Apps or Move apps to free space." },
      { name: "ProgramData", path: "C:\\ProgramData", size: 22 * GB, files: 64000, isDir: true, hint: "Shared app data. Safe to leave." },
      { name: "hiberfil.sys", path: "C:\\hiberfil.sys", size: 6.4 * GB, files: 1, isDir: false, hint: "Hibernation file. Only needed for hibernate and Fast Startup." },
      { name: "pagefile.sys", path: "C:\\pagefile.sys", size: 4 * GB, files: 1, isDir: false, hint: "Windows virtual memory. Needed; leave alone." },
    ],
  }
  const suggestions = [
    { id: "cleanup", title: "Run Cleanup", detail: "Temporary files and caches Windows and apps rebuild when needed: Your temp files 3.4 GB, Windows Update downloads 2.8 GB, Chrome cache 1.2 GB.", size: 10.5 * GB, action: "cleanup", view: "cleanup", path: "", dest: "", safety: "safe" },
    { id: "move:a", title: "Move Elden Ring to D:", detail: "Uses 52.3 GB on C:. A link is left behind so it keeps working, and you can move it back.", size: 52.3 * GB, action: "move", view: "move", path: "C:\\Program Files (x86)\\Steam\\steamapps\\common\\ELDEN RING", dest: "D:", safety: "safe" },
    { id: "videos", title: "Look through your Videos", detail: "Video files are usually the biggest personal files. Delete or move the ones you no longer watch. You could also move it to D:.", size: 38.6 * GB, action: "review", view: "", path: "C:\\Users\\you\\Videos", dest: "", safety: "review" },
    { id: "downloads", title: "Look through your Downloads", detail: "Old installers, ISOs and zip files pile up here. Open the biggest ones and delete what you don't need.", size: 14.2 * GB, action: "review", view: "", path: "C:\\Users\\you\\Downloads", dest: "", safety: "review" },
    { id: "hiberfil", title: "Hibernation file", detail: "Windows reserves this space for hibernate and Fast Startup. If you never hibernate, run `powercfg /h off` in an administrator terminal to free it.", size: 6.4 * GB, action: "manual", view: "", path: "", dest: "", safety: "careful" },
  ]

  const app = (name, version, publisher, size, installDate, shared, drive) => ({ id: name, name, version, publisher, size, installDate, shared, drive: drive || "C:" })
  const apps = [
    app("Visual Studio Community 2022", "17.14.42", "Microsoft Corporation", 5.1 * GB, "2025-06-30", true),
    app("ELDEN RING", "1.0", "FromSoftware", 52.3 * GB, "2025-11-02", false),
    app("Adobe Photoshop 2025", "26.1", "Adobe", 4.7 * GB, "2025-03-14", false),
    app("Docker Desktop", "4.96.0", "Docker Inc.", 3.5 * GB, "2025-09-01", false),
    app("DaVinci Resolve", "19.0", "Blackmagic Design", 3.2 * GB, "2025-05-21", false),
    app("Steam", "2.10", "Valve", 1.9 * GB, "2024-12-11", false),
    app("Discord", "1.0.9", "Discord Inc.", 0.4 * GB, "2025-01-08", false),
    app("VLC media player", "3.0.21", "VideoLAN", 0.2 * GB, "2024-10-19", false),
    app("7-Zip 24.08 (x64)", "24.08", "Igor Pavlov", 0.01 * GB, "2024-09-02", false),
  ]

  const installs = [
    { name: "ELDEN RING", publisher: "FromSoftware", path: "C:\\Program Files (x86)\\Steam\\steamapps\\common\\ELDEN RING", drive: "C:", size: 52.3 * GB, movable: true, reason: "", movedTo: "", broken: false },
    { name: "Adobe Photoshop 2025", publisher: "Adobe", path: "C:\\Program Files\\Adobe\\Adobe Photoshop 2025", drive: "C:", size: 4.7 * GB, movable: true, reason: "", movedTo: "", broken: false },
    { name: "DaVinci Resolve", publisher: "Blackmagic Design", path: "C:\\Program Files\\Blackmagic Design\\DaVinci Resolve", drive: "C:", size: 3.2 * GB, movable: true, reason: "", movedTo: "", broken: false },
    { name: "Cyberpunk 2077", publisher: "CD PROJEKT RED", path: "C:\\Games\\Cyberpunk 2077", drive: "C:", size: 71.8 * GB, movable: true, reason: "", movedTo: "D:\\Moved Programs\\Cyberpunk 2077", broken: false },
    { name: "Visual Studio Community 2022", publisher: "Microsoft Corporation", path: "C:\\Program Files\\Microsoft Visual Studio\\2022", drive: "C:", size: 5.1 * GB, movable: false, reason: "Managed by Microsoft; moving can break updates", movedTo: "", broken: false },
  ]

  const drivers = [
    { device: "NVIDIA GeForce RTX 3060", class: "DISPLAY", provider: "NVIDIA", version: "31.0.15.5222", date: "2025-02-11", signed: true, infName: "oem1.inf" },
    { device: "Realtek High Definition Audio", class: "MEDIA", provider: "Realtek", version: "6.0.9549.1", date: "2024-11-05", signed: true, infName: "oem2.inf" },
    { device: "Intel(R) Wi-Fi 6 AX200", class: "NET", provider: "Intel", version: "22.250.1.2", date: "2024-09-18", signed: true, infName: "oem3.inf" },
  ]
  const driverState = {
    updates: [
      { id: "u1", title: "NVIDIA - Display - 32.0.15.6094", manufacturer: "NVIDIA", model: "GeForce RTX 3060", class: "Display", date: "2025-06-12", needsEula: false },
      { id: "u2", title: "Realtek - MEDIA - 6.0.9600.1", manufacturer: "Realtek", model: "Audio", class: "Media", date: "2025-05-02", needsEula: false },
    ],
    checkedAt: Date.now() - 600000, checking: false,
  }

  window.go = { main: { App: {
    AuthStatus: ok({ configured: false, user: null, configPath: "" }),
    IsAdmin: ok(true),
    OtherAccount: ok(""),
    GetOverview: ok(overview),
    GetHealth: ok(health),
    GetPrefs: ok({ welcomed: true, reminderDays: 14, daysSinceTidy: 41 }),
    SetReminderDays: ok(null),
    MarkWelcomed: ok(null),
    ListProcesses: ok([]),
    ScanCleanup: ok(cleanup),
    ListStartup: ok(startup),
    AnalyzeFolder: ok(storage),
    GetStorageSuggestions: ok(suggestions),
    LargestFiles: ok({ files: [], partial: false }),
    ListApps: ok(apps),
    ScanInstalls: ok(installs),
    ScanLeftovers: ok([]),
    ScanOldVersions: ok([]),
    ListDrivers: ok(drivers),
    DriverScanState: ok(driverState),
    StartDriverScan: ok(driverState),
  } } }
  window.runtime = { EventsOn: () => () => {} }

  // No transitions, so the screenshot never catches one half way.
  const still = document.createElement("style")
  still.textContent = "*,*::before,*::after{transition:none!important;animation:none!important}"
  document.addEventListener("DOMContentLoaded", () => document.head.appendChild(still))

  // Open the requested tab, then press any button named in ?click=
  addEventListener("load", () => {
    const view = q.get("view")
    if (view) setTimeout(() => document.querySelector('[data-view="' + view + '"]')?.click(), 400)
    const click = q.get("click")
    if (click) setTimeout(() => document.querySelector(click)?.click(), 900)
  })
})()
