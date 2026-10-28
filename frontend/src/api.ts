// Typed access to the Go methods on App (app.go). Wails injects these at
// runtime as window.go.main.App.*; each returns a Promise and rejects with
// the Go error message. Field names match the json tags on the Go structs.

interface Hint { level: "info" | "warn"; message: string }

export interface Overview {
  system: {
    hostname: string
    os: string
    uptimeSeconds: number
    cpu: { model: string; cores: number; threads: number; usagePercent: number }
    memory: { total: number; used: number; available: number; usedPercent: number }
    disks: { mount: string; fsType: string; total: number; free: number; usedPercent: number }[] | null
    warnings: string[] | null
  }
  hints: Hint[] | null
}

export interface Process {
  pid: number
  name: string
  cpuPercent: number
  memoryRss: number
  protected: boolean
  relatedTo: string
  vendor: string
  path: string
}

export interface CleanupTarget {
  id: string
  name: string
  description: string
  requiresAdmin: boolean
  available: boolean
  bytes: number
  files: number
  error?: string
}

interface CleanResult {
  id: string
  freedBytes: number
  deleted: number
  skipped: number
  error?: string
}

export interface StartupItem {
  id: string
  name: string
  command: string
  source: string
  scope: "user" | "machine"
  enabled: boolean
  requiresAdmin: boolean
  impact: "high" | "medium" | "low"
  note: string
  recommend: "disable" | "keep" | ""
}

export interface Driver {
  device: string
  class: string
  provider: string
  version: string
  date: string
  signed: boolean
  infName: string
}

export interface DriverUpdate {
  id: string
  title: string
  manufacturer: string
  model: string
  class: string
  date: string
  needsEula: boolean
}

interface InstallResult {
  installed: string[] | null
  failed: string[] | null
  skipped: string[] | null
  rebootRequired: boolean
  restorePoint: boolean
}

interface AuthUser {
  subject: string
  name: string
  email: string
  picture: string
}

export interface AuthStatus {
  configured: boolean
  user: AuthUser | null
  configPath: string
  error?: string
}

interface HealthIssue {
  level: "warn" | "info"
  title: string
  detail: string
  view: string
  action: string
}

export interface HealthReport {
  score: number
  label: string
  issues: HealthIssue[] | null
}

export interface Prefs {
  welcomed: boolean
  reminderDays: number
  daysSinceTidy: number
}

export interface StorageEntry {
  name: string
  path: string
  size: number
  files: number
  isDir: boolean
  hint: string
}

export interface StorageSuggestion {
  id: string
  title: string
  detail: string
  size: number
  action: "cleanup" | "move" | "uninstall" | "review" | "manual"
  view: string
  path: string
  dest: string
  safety: "safe" | "review" | "careful"
}

export interface StorageResult {
  path: string
  total: number
  entries: StorageEntry[] | null
  partial: boolean
  skipped: number
}

export interface InstalledApp {
  id: string
  name: string
  version: string
  publisher: string
  size: number
  installDate: string
  shared: boolean
}

export interface AppInfo {
  version: string
  admin: boolean
  install: { installed: boolean; dir: string; exe: string; hasUninstall: boolean; settingsDir: string }
}

export interface UpdateInfo {
  current: string
  latest: string
  available: boolean
  date: string
  url: string
}

export interface LicenseStatus {
  tier: "free" | "trial" | "plus"
  note: string
  email?: string
  expires?: string
  trialDaysLeft: number
  canTrial: boolean
  canActivate: boolean
}

export interface ScheduleView {
  enabled: boolean
  frequency: "weekly" | "monthly"
  lastRun?: string
  lastFreed: number
  lastNote?: string
}

export interface AlertsView {
  lowDisk: boolean
  threshold: number
  report: boolean
}

interface AppBindings {
  GetHealth(): Promise<HealthReport>
  GetPrefs(): Promise<Prefs>
  SetReminderDays(n: number): Promise<void>
  MarkWelcomed(): Promise<void>
  AnalyzeFolder(path: string): Promise<StorageResult>
  GetStorageSuggestions(drive: string): Promise<StorageSuggestion[] | null>
  LargestFiles(path: string): Promise<{ files: StorageEntry[] | null, partial: boolean }>
  RevealInExplorer(path: string): Promise<void>
  ListApps(): Promise<InstalledApp[] | null>
  UninstallApp(id: string): Promise<void>
  ScanInstalls(): Promise<Install[] | null>
  MoveInstall(path: string, destDrive: string): Promise<MoveResult>
  MoveInstallBack(path: string): Promise<MoveResult>
  ScanOldVersions(): Promise<OldVersion[] | null>
  UninstallOldVersion(id: string): Promise<void>
  ScanLeftovers(): Promise<Leftover[] | null>
  DeleteLeftover(path: string): Promise<void>
  AuthStatus(): Promise<AuthStatus>
  Login(): Promise<AuthUser>
  CancelLogin(): Promise<void>
  Logout(): Promise<void>
  IsAdmin(): Promise<boolean>
  OtherAccount(): Promise<string>
  AppInfo(): Promise<AppInfo>
  CheckUpdate(): Promise<UpdateInfo>
  OpenDownloadPage(): Promise<void>
  LicenseStatus(): Promise<LicenseStatus>
  StartTrial(): Promise<LicenseStatus>
  ActivateLicense(key: string): Promise<LicenseStatus>
  RemoveLicense(): Promise<LicenseStatus>
  OpenPlusPage(): Promise<void>
  OpenAccountPage(): Promise<void>
  GetAlerts(): Promise<AlertsView>
  SetAlerts(lowDisk: boolean, threshold: number, report: boolean): Promise<AlertsView>
  GetSchedule(): Promise<ScheduleView>
  SetSchedule(enabled: boolean, frequency: string): Promise<ScheduleView>
  OpenFolder(kind: "install" | "settings"): Promise<void>
  RunSetup(mode: "modify" | "uninstall"): Promise<void>
  DeleteSettings(): Promise<void>
  GetOverview(): Promise<Overview>
  ListProcesses(): Promise<Process[] | null>
  KillProcess(pid: number, name: string): Promise<void>
  ScanCleanup(): Promise<CleanupTarget[] | null>
  RunCleanup(ids: string[]): Promise<CleanResult[] | null>
  ListStartup(): Promise<StartupItem[] | null>
  SetStartupEnabled(id: string, enabled: boolean): Promise<void>
  ListDrivers(): Promise<Driver[] | null>
  DriverScanState(): Promise<DriverScanState>
  StartDriverScan(force: boolean): Promise<DriverScanState>
  InstallDriverUpdates(ids: string[]): Promise<InstallResult>
  NetworkMetered(): Promise<boolean>
}

declare global {
  interface Window {
    go: { main: { App: AppBindings } }
    runtime: { EventsOn(name: string, cb: (...data: any[]) => void): () => void }
  }
}

export const api: AppBindings = new Proxy({} as AppBindings, {
  get: (_t, method: string) => (...args: unknown[]) =>
    (window.go.main.App as unknown as Record<string, (...a: unknown[]) => unknown>)[method](...args),
})

export interface Install {
  name: string
  publisher: string
  path: string
  drive: string
  size: number
  movable: boolean
  reason: string
  movedTo: string
  broken: boolean
}

export interface DriverProgress {
  stage: "restore" | "download" | "install"
  count: number
}

export interface MoveProgress {
  name: string
  stage: string
  percent: number
}

interface MoveResult {
  dest: string
  bytes: number
  warning?: string
}

export interface DriverScanState {
  updates: DriverUpdate[] | null
  checkedAt: number // unix ms, 0 = never
  checking: boolean
  error?: string
}

export interface OldVersion {
  id: string
  name: string
  version: string
  publisher: string
  size: number
  newer: string
}

export interface Leftover {
  path: string
  drive: string
  size: number
}
