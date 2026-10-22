package startup

import "testing"

func TestStoreAppName(t *testing.T) {
	for in, want := range map[string]string{
		"SpotifyAB.SpotifyMusic_zpdnekdrzrea0": "SpotifyMusic",
		"MSTeams_8wekyb3d8bbwe":                "MSTeams",
		"Microsoft.YourPhone_8wekyb3d8bbwe":    "YourPhone",
		"Claude_pzs8sxrjxfjjc":                 "Claude",
	} {
		if got := storeAppName(in); got != want {
			t.Errorf("storeAppName(%q) = %q, want %q", in, got, want)
		}
	}
}
