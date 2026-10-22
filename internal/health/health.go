// Package health turns a few measurements into one friendly summary: a score
// out of 100 and a short list of things worth doing, each pointing at the tab
// that fixes it. The score is a guide, not a benchmark.
package health

import "fmt"

type Input struct {
	WorstDiskFreePct float64 // lowest free-space percentage across drives; 100 if none
	WorstDiskMount   string
	MemoryUsedPct    float64
	StartupEnabled   int
	StartupSuggested int // enabled entries we'd suggest turning off
	UptimeDays       float64
	RebootPending    bool
	DaysSinceTidy    int // -1 = never
	TidyReminderOn   bool
}

type Issue struct {
	Level  string `json:"level"` // "warn" | "info"
	Title  string `json:"title"`
	Detail string `json:"detail"`
	View   string `json:"view"`   // tab that helps, "" if none
	Action string `json:"action"` // button label
}

type Report struct {
	Score  int     `json:"score"`
	Label  string  `json:"label"`
	Issues []Issue `json:"issues"`
}

func Evaluate(in Input) Report {
	score := 100
	var issues []Issue
	add := func(cost int, level, title, detail, view, action string) {
		score -= cost
		issues = append(issues, Issue{level, title, detail, view, action})
	}

	switch {
	case in.WorstDiskFreePct < 5:
		add(30, "warn", fmt.Sprintf("%s is almost full", in.WorstDiskMount),
			"Windows slows down and updates can fail when a drive is nearly full. Clean up or move big programs to another drive.", "cleanup", "Free up space")
	case in.WorstDiskFreePct < 10:
		add(25, "warn", fmt.Sprintf("%s is running low on space", in.WorstDiskMount),
			"Less than 10% is free. Cleaning temporary files is the quickest win.", "cleanup", "Free up space")
	case in.WorstDiskFreePct < 20:
		add(10, "info", fmt.Sprintf("%s is getting full", in.WorstDiskMount),
			"Less than 20% is free. See what is using the space.", "storage", "See what uses space")
	}

	switch {
	case in.MemoryUsedPct > 90:
		add(15, "warn", "Memory is nearly full", "Windows is likely using the slower disk as memory. Close the heaviest apps.", "processes", "See heavy apps")
	case in.MemoryUsedPct > 85:
		add(8, "info", "Memory use is high", "Check which apps are using the most.", "processes", "See heavy apps")
	}

	switch {
	case in.StartupSuggested >= 6:
		add(15, "warn", fmt.Sprintf("%d programs start with Windows that you can probably turn off", in.StartupSuggested),
			"They slow down sign-in and use memory in the background.", "startup", "Review startup")
	case in.StartupSuggested >= 3:
		add(10, "info", fmt.Sprintf("%d programs start with Windows that you can probably turn off", in.StartupSuggested),
			"Turning them off makes sign-in faster. You can switch them back on any time.", "startup", "Review startup")
	case in.StartupEnabled > 15:
		add(5, "info", fmt.Sprintf("%d programs start with Windows", in.StartupEnabled), "That is a lot. Review the list.", "startup", "Review startup")
	}

	if in.RebootPending {
		add(8, "info", "A restart is waiting", "Windows has changes that finish only after a restart.", "", "")
	}
	switch {
	case in.UptimeDays > 14:
		add(8, "info", "No restart in over two weeks", "A restart clears leaked memory and finishes updates.", "", "")
	case in.UptimeDays > 7:
		add(4, "info", "No restart in over a week", "A restart now and then keeps things smooth.", "", "")
	}

	if in.TidyReminderOn && (in.DaysSinceTidy < 0 || in.DaysSinceTidy > 30) {
		detail := "You haven't run Cleanup yet."
		if in.DaysSinceTidy > 30 {
			detail = fmt.Sprintf("It has been %d days since your last Cleanup.", in.DaysSinceTidy)
		}
		add(6, "info", "Time for a tidy", detail, "cleanup", "Run Cleanup")
	}

	if score < 0 {
		score = 0
	}
	label := "Healthy"
	switch {
	case score < 65:
		label = "Needs attention"
	case score < 85:
		label = "Could use a tidy"
	}
	return Report{Score: score, Label: label, Issues: issues}
}
