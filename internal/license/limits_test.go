package license

import (
	"strings"
	"testing"
)

func TestMoveAllowed(t *testing.T) {
	const gb = 1 << 30
	cases := []struct {
		name  string
		tier  Tier
		count int
		moved uint64
		add   uint64
		ok    bool
	}{
		{"free first app any size", Free, 0, 0, 80 * gb, true},
		{"free second app within 10 GB total", Free, 1, 4 * gb, 5 * gb, true},
		{"free second app exactly 10 GB", Free, 1, 4 * gb, 6 * gb, true},
		{"free second app over 10 GB", Free, 1, 4 * gb, 7 * gb, false},
		{"free big first app blocks next", Free, 1, 50 * gb, 1 * gb, false},
		{"plus unlimited", Plus, 5, 500 * gb, 100 * gb, true},
		{"trial unlimited", Trial, 5, 500 * gb, 100 * gb, true},
	}
	for _, c := range cases {
		ok, why := MoveAllowed(c.tier, c.count, c.moved, c.add)
		if ok != c.ok {
			t.Errorf("%s: got %v, want %v", c.name, ok, c.ok)
		}
		if ok && why != "" {
			t.Errorf("%s: allowed but gave a reason: %q", c.name, why)
		}
		if !ok && !strings.Contains(why, "Move an app back") {
			t.Errorf("%s: reason should say how to get room: %q", c.name, why)
		}
	}
}
