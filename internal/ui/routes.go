package ui

import "elom/locales"

// Screen is one screen of the app, as the sidebar lists it. The label and
// summary are functions because the app's language can change while it
// runs: a view is rebuilt for every frame, so it asks again each time.
type Screen struct {
	ID      string
	Label   func() string
	Summary func() string
	Phase   string
}

// Section groups screens in the sidebar.
type Section struct {
	Title   func() string
	Screens []Screen
}

// Sections is the app's navigation. The order is the order of the tabs.
var Sections = []Section{
	{locales.SectionCleanUp, []Screen{
		{"clean", locales.NavClean, locales.CleanSummary, "Phase 2"},
		// Apps holds Uninstall, Updates and Startup as its own tabs.
		{"software", locales.NavApps, locales.SoftwareSummary, "Phase 5"},
		{"optimize", locales.NavOptimize, locales.OptimizeSummary, "Phase 6"},
	}},
	{locales.SectionExplore, []Screen{
		{"analyze", locales.NavAnalyze, locales.AnalyzeSummary, "Phase 3"},
		{"status", locales.NavStatus, locales.StatusSummary, "Phase 4"},
	}},
	{locales.SectionElom, []Screen{
		{"settings", locales.NavSettings, locales.SettingsSummary, "Phase 1"},
	}},
}

// allScreens flattens the sections into the tab order.
func allScreens() []Screen {
	var out []Screen
	for _, s := range Sections {
		out = append(out, s.Screens...)
	}
	return out
}

// screenIndex is the tab index of a screen, 0 for an unknown one.
func screenIndex(id string) int {
	for i, s := range allScreens() {
		if s.ID == id {
			return i
		}
	}
	return 0
}

// screenFor returns the screen with the given ID, or the first screen.
func screenFor(id string) Screen {
	for _, s := range Sections {
		for _, r := range s.Screens {
			if r.ID == id {
				return r
			}
		}
	}
	return Sections[0].Screens[0]
}

// firstScreen is where the app opens.
func firstScreen() string { return Sections[0].Screens[0].ID }
