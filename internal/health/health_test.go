package health

import "testing"

func healthy() Input {
	return Input{WorstDiskFreePct: 60, MemoryUsedPct: 40, StartupEnabled: 6, UptimeDays: 1, DaysSinceTidy: 3, TidyReminderOn: true}
}

func TestHealthyPC(t *testing.T) {
	r := Evaluate(healthy())
	if r.Score != 100 || r.Label != "Healthy" || len(r.Issues) != 0 {
		t.Fatalf("got %+v", r)
	}
}

func TestIssuesLowerScoreAndPointToTabs(t *testing.T) {
	in := healthy()
	in.WorstDiskFreePct, in.WorstDiskMount = 4, "C:"
	in.StartupSuggested = 7
	r := Evaluate(in)
	if r.Score != 55 || r.Label != "Needs attention" {
		t.Errorf("score/label: %+v", r)
	}
	views := map[string]bool{}
	for _, i := range r.Issues {
		views[i.View] = true
	}
	if !views["cleanup"] || !views["startup"] {
		t.Errorf("issues should point at cleanup and startup: %+v", r.Issues)
	}
}

func TestScoreNeverBelowZero(t *testing.T) {
	r := Evaluate(Input{WorstDiskFreePct: 1, MemoryUsedPct: 99, StartupSuggested: 20, StartupEnabled: 30, UptimeDays: 90, RebootPending: true, DaysSinceTidy: -1, TidyReminderOn: true})
	if r.Score < 0 {
		t.Fatalf("score %d", r.Score)
	}
}

func TestReminderOffSilencesTidyNudge(t *testing.T) {
	in := healthy()
	in.DaysSinceTidy, in.TidyReminderOn = -1, false
	if r := Evaluate(in); len(r.Issues) != 0 {
		t.Fatalf("got %+v", r.Issues)
	}
}
