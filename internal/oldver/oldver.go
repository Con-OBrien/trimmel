// Package oldver finds installed programs that also have a newer version of
// themselves installed, so the older copy can be removed.
//
// Detection is deliberately conservative. Programs are grouped by publisher,
// name (with version numbers stripped) and architecture, and only entries
// with a strictly lower DisplayVersion than another in the group are
// reported. Runtimes, SDKs and Microsoft software are never reported because
// other programs often depend on a specific old version.
package oldver

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Old struct {
	ID        string `json:"id"` // registry location, opaque to the UI
	Name      string `json:"name"`
	Version   string `json:"version"`
	Publisher string `json:"publisher"`
	Size      uint64 `json:"size"`  // installer estimate, else measured from the install folder
	Newer     string `json:"newer"` // e.g. "IntelliJ IDEA 2026.2.2 (262.1234)"
}

type App struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Publisher   string `json:"publisher"`
	Size        uint64 `json:"size"`        // installer estimate, else measured; 0 = unknown
	InstallDate string `json:"installDate"` // YYYY-MM-DD, "" if unknown
	Drive       string `json:"drive"`       // e.g. "C:", "" if the install folder is unknown
	Shared      bool   `json:"shared"`      // other programs may depend on it (runtimes, Microsoft components)
}

type entry struct {
	id, name, version, publisher, uninstall, location, date string
	size                                                    uint64
	hasParent                                               bool // update or hotfix of another product
}

var excluded = []string{
	"python", "java", "jdk", "jre", "openjdk", "node.js", "nodejs", ".net", "dotnet",
	"visual c++", "redistributable", "runtime", "sdk", "cuda", "directx", "vulkan",
	"driver", "go programming", "rust", "ruby", "perl", "hotfix", "security update",
}

var (
	reParens  = regexp.MustCompile(`\([^)]*\)`)
	reVersion = regexp.MustCompile(`\bv?\d+(\.\d+)*[a-z]?\b`)
	reSpace   = regexp.MustCompile(`[^a-z0-9+#]+`)
	reDigits  = regexp.MustCompile(`\d+`)
)

func arch(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "x86"), strings.Contains(n, "32-bit"), strings.Contains(n, "32 bit"):
		return "x86"
	case strings.Contains(n, "x64"), strings.Contains(n, "64-bit"), strings.Contains(n, "64 bit"):
		return "x64"
	}
	return ""
}

// groupKey is empty when the entry must not take part.
func groupKey(e entry) string {
	ln := strings.ToLower(e.name)
	if e.hasParent || e.uninstall == "" || strings.Contains(strings.ToLower(e.publisher), "microsoft") {
		return ""
	}
	for _, x := range excluded {
		if strings.Contains(ln, x) {
			return ""
		}
	}
	base := reParens.ReplaceAllString(ln, " ")
	base = reVersion.ReplaceAllString(base, " ")
	base = strings.TrimSpace(reSpace.ReplaceAllString(base, " "))
	if len(base) < 3 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(e.publisher)) + "|" + base + "|" + arch(e.name)
}

func parseVersion(v string) []int {
	var out []int
	for _, p := range reDigits.FindAllString(v, -1) {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out = append(out, n)
	}
	return out
}

func cmpVersion(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func find(entries []entry) []Old {
	groups := map[string][]entry{}
	for _, e := range entries {
		if k := groupKey(e); k != "" && len(parseVersion(e.version)) > 0 {
			groups[k] = append(groups[k], e)
		}
	}

	var out []Old
	for _, g := range groups {
		if len(g) < 2 {
			continue
		}
		newest := g[0]
		for _, e := range g[1:] {
			if cmpVersion(parseVersion(e.version), parseVersion(newest.version)) > 0 {
				newest = e
			}
		}
		for _, e := range g {
			if cmpVersion(parseVersion(e.version), parseVersion(newest.version)) < 0 {
				out = append(out, Old{
					ID: e.id, Name: e.name, Version: e.version, Publisher: e.publisher, Size: e.size,
					Newer: strings.TrimSpace(newest.name + " (" + newest.version + ")"),
				})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Size != out[j].Size {
			return out[i].Size > out[j].Size
		}
		return out[i].Name < out[j].Name
	})
	return out
}
