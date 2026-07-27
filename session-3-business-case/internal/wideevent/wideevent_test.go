package wideevent

import (
	"math/rand"
	"testing"
	"time"
)

func generate(t *testing.T) (Config, []Event) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Now = time.Date(2026, 9, 2, 15, 0, 0, 0, time.UTC)
	return cfg, Generate(cfg, rand.New(rand.NewSource(11)))
}

// TestArbitraryQuestionHasAnAnswer is the load-bearing test: the demo is Liz
// running the book's question live, so the dataset has to contain enough
// matching events to return a non-trivial answer. Too few and the screen shows
// an empty result; too many and the four filters are obviously not narrowing
// anything, which undercuts the point.
func TestArbitraryQuestionHasAnAnswer(t *testing.T) {
	_, events := generate(t)

	var matches int
	for _, e := range events {
		if e.MatchesArbitraryQuestion() {
			matches++
		}
	}

	if matches < 5 {
		t.Errorf("only %d events match the book's question; the live demo would show a near-empty result", matches)
	}
	if share := float64(matches) / float64(len(events)); share > 0.01 {
		t.Errorf("%.2f%% of events match; the filters are not narrowing enough to be interesting", share*100)
	}
	t.Logf("%d of %d events match the arbitrary question", matches, len(events))
}

// TestEventsAreActuallyWide guards the premise. If the event narrows to a
// handful of fields, the demo stops demonstrating that the question was
// answerable without being anticipated, and starts looking staged.
func TestEventsAreActuallyWide(t *testing.T) {
	_, events := generate(t)

	// Counted off the Event struct via the emitter's attribute list, which is
	// the number that actually reaches Honeycomb.
	const wantAtLeast = 25
	if got := len(Attributes(events[0])); got < wantAtLeast {
		t.Errorf("event carries %d attributes, want at least %d for the wide-event claim to hold", got, wantAtLeast)
	}
}

// TestClientFieldsAreSelfConsistent catches the sloppiness that makes a
// synthetic dataset obvious: desktop browsers reporting a mobile app version,
// or iOS builds running on Android.
func TestClientFieldsAreSelfConsistent(t *testing.T) {
	_, events := generate(t)

	for _, e := range events {
		switch e.Device {
		case "computer":
			if e.App != "" || e.AppVersion != "" {
				t.Fatalf("desktop event reports app=%q version=%q", e.App, e.AppVersion)
			}
		case "phone", "tablet":
			if e.App == "" || e.AppVersion == "" {
				t.Fatalf("%s event has no app or version", e.Device)
			}
			if e.App == "iOS" && e.OSName != "iOS" {
				t.Fatalf("iOS app reporting os=%q", e.OSName)
			}
			if e.App == "android" && e.OSName != "Android" {
				t.Fatalf("android app reporting os=%q", e.OSName)
			}
		default:
			t.Fatalf("unexpected device %q", e.Device)
		}

		if e.Errored != (e.StatusCode == 402) {
			t.Fatalf("errored=%t with status %d", e.Errored, e.StatusCode)
		}
		if e.Errored && (e.ErrorType == "" || e.ErrorSlug == "") {
			t.Fatalf("errored event missing type or slug")
		}
		if !e.Errored && (e.ErrorType != "" || e.ErrorSlug != "") {
			t.Fatalf("successful event carries error fields")
		}
	}
}

// TestEveryFilterActuallyNarrows checks that each of the four dimensions in the
// question removes a meaningful share of events. If any one of them matched
// nearly everything, the demo would imply a precision the data does not have.
func TestEveryFilterActuallyNarrows(t *testing.T) {
	_, events := generate(t)
	total := float64(len(events))

	tests := []struct {
		name        string
		match       func(Event) bool
		wantBelow   float64
		wantAtLeast float64
	}{
		{"errored", func(e Event) bool { return e.Errored }, 0.20, 0.02},
		{"device=phone", func(e Event) bool { return e.Device == AnswerDevice }, 0.70, 0.30},
		{"region=US-CA", func(e Event) bool { return e.RegionISO == AnswerRegion }, 0.45, 0.15},
		{"app_version=2.3.1", func(e Event) bool { return e.AppVersion == AnswerVersion }, 0.45, 0.10},
		{"lunch hour", func(e Event) bool {
			return e.LocalHour >= LunchHourStart && e.LocalHour < LunchHourEnd
		}, 0.45, 0.10},
	}

	for _, tc := range tests {
		var n int
		for _, e := range events {
			if tc.match(e) {
				n++
			}
		}
		share := float64(n) / total
		if share > tc.wantBelow {
			t.Errorf("%s matches %.0f%% of events; too broad to be a meaningful filter", tc.name, share*100)
		}
		if share < tc.wantAtLeast {
			t.Errorf("%s matches only %.1f%% of events; too rare to combine with the others", tc.name, share*100)
		}
	}
}

// TestGenerate_WithinWindow guards the "past week" part of the question.
func TestGenerate_WithinWindow(t *testing.T) {
	cfg, events := generate(t)
	oldest := cfg.Now.Add(-cfg.Window)

	for _, e := range events {
		if e.Start.Before(oldest) || e.Start.After(cfg.Now) {
			t.Fatalf("event at %s falls outside the %s window ending %s", e.Start, cfg.Window, cfg.Now)
		}
	}
}
