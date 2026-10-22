package advisor

import "testing"

func TestRankSortsBiggestFirstAndCaps(t *testing.T) {
	var in []Suggestion
	for i := 0; i < 15; i++ {
		in = append(in, Suggestion{ID: string(rune('a' + i)), Size: uint64(i + 1)})
	}
	out := rank(in)
	if len(out) != maxResults {
		t.Fatalf("got %d results, want %d", len(out), maxResults)
	}
	if out[0].Size != 15 || out[len(out)-1].Size != 6 {
		t.Errorf("not sorted biggest first: first=%d last=%d", out[0].Size, out[len(out)-1].Size)
	}
}

func TestOnDrive(t *testing.T) {
	if !onDrive(`C:\Users\x`, "c:") || onDrive(`D:\Games`, "C:") || onDrive(`relative`, "C:") {
		t.Error("onDrive gave a wrong answer")
	}
}

func TestHumanBytes(t *testing.T) {
	for in, want := range map[uint64]string{0: "0 B", 1023: "1023 B", 1536: "1.5 KB", 3 << 30: "3.0 GB"} {
		if got := HumanBytes(in); got != want {
			t.Errorf("%d: got %q want %q", in, got, want)
		}
	}
}

func TestFolderRulesStayInsideProfile(t *testing.T) {
	for _, r := range folderRules {
		p := r.path(`C:\Users\x`, `C:\Users\x\AppData\Local`)
		if len(p) < 12 || p[:11] != `C:\Users\x\` {
			t.Errorf("%s: %q is outside the profile", r.id, p)
		}
		if r.title == "" || r.detail == "" || r.min == 0 {
			t.Errorf("%s: incomplete rule", r.id)
		}
	}
}
