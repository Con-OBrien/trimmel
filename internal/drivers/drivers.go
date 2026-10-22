// Package drivers lists installed device drivers and finds driver updates
// through the Windows Update Agent, the same channel Windows Update uses.
// Updates come from Microsoft's catalogue, are signed, and are matched to
// the hardware IDs of devices actually present.
package drivers

type Driver struct {
	Device   string `json:"device"`
	Class    string `json:"class"`
	Provider string `json:"provider"`
	Version  string `json:"version"`
	Date     string `json:"date"` // YYYY-MM-DD, empty if unknown
	Signed   bool   `json:"signed"`
	InfName  string `json:"infName"`
}

type Update struct {
	ID           string `json:"id"` // Windows Update UpdateID
	Title        string `json:"title"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Class        string `json:"class"`
	Date         string `json:"date"`
	NeedsEula    bool   `json:"needsEula"`
}

type InstallResult struct {
	Installed      []string `json:"installed"` // titles
	Failed         []string `json:"failed"`
	Skipped        []string `json:"skipped"` // e.g. licence terms not yet accepted
	RebootRequired bool     `json:"rebootRequired"`
	RestorePoint   bool     `json:"restorePoint"` // a System Restore point was made first
}
