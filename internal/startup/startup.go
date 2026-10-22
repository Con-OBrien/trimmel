// Package startup lists programs that launch at sign-in and enables or
// disables them the same way Task Manager does: by writing a flag under
// Explorer\StartupApproved rather than deleting the entry. Disabling is
// therefore fully reversible, and the state shows up in Task Manager too.
package startup

type Item struct {
	ID            string `json:"id"` // "<source>|<name>", stable across refreshes
	Name          string `json:"name"`
	Command       string `json:"command"`
	Source        string `json:"source"` // human-readable location
	Scope         string `json:"scope"`  // "user" or "machine"
	Enabled       bool   `json:"enabled"`
	RequiresAdmin bool   `json:"requiresAdmin"`
	Impact        string `json:"impact"`    // "high" | "medium" | "low": rough effect on sign-in time
	Note          string `json:"note"`      // plain-English explanation, may be empty
	Recommend     string `json:"recommend"` // "disable" | "keep" | ""
}
