package scenario

import (
	"math/rand"
	"testing"
	"time"
)

func testConfig() Config {
	cfg := DefaultConfig()
	// Fixed anchor so the dataset is fully deterministic.
	cfg.Now = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	return cfg
}

func generate(t *testing.T) (Config, []Request) {
	t.Helper()
	cfg := testConfig()
	return cfg, Generate(cfg, rand.New(rand.NewSource(7)))
}

// TestGenerate_IncidentRule pins the rule down: all three conditions are
// required. If a refactor lets any two of the three trigger it, the demo
// stops teaching that BubbleUp is ranking a real multi-dimension population
// rather than one dimension that happens to correlate.
func TestGenerate_IncidentRule(t *testing.T) {
	cfg, reqs := generate(t)
	incidentStart := cfg.IncidentStart()

	for _, r := range reqs {
		if r.Route == RouteHealthz {
			if r.Incident {
				t.Fatalf("healthz request marked incident; health checks are never affected")
			}
			continue
		}
		want := r.Start.After(incidentStart) && r.Version == cfg.BuggyVersion && r.UserType == "enterprise"
		if r.Incident != want {
			t.Fatalf("incident=%t for start=%s version=%s type=%s; want %t",
				r.Incident, r.Start, r.Version, r.UserType, want)
		}
	}
}

// TestGenerate_IncidentIsOngoing guards the assumption the burn-rate demo and
// its trigger both depend on: the incident is still happening at Now, not a
// past event with a clean end boundary.
func TestGenerate_IncidentIsOngoing(t *testing.T) {
	cfg, reqs := generate(t)

	var sawIncidentNearNow bool
	for _, r := range reqs {
		if !r.Incident {
			continue
		}
		if r.Start.Before(cfg.Now.Add(-cfg.IncidentAgo)) {
			t.Fatalf("incident request at %s predates the incident window start", r.Start)
		}
		if cfg.Now.Sub(r.Start) < 5*time.Minute {
			sawIncidentNearNow = true
		}
	}
	if !sawIncidentNearNow {
		t.Error("no incident request within 5 minutes of Now; the incident should still be live, not just historical")
	}
}

// TestGenerate_ErrorSpikeIsDemoSized checks the affected population is large
// enough to move a trailing-1-hour AVG() query clearly, matching the
// package comment's arithmetic.
func TestGenerate_ErrorSpikeIsDemoSized(t *testing.T) {
	cfg, reqs := generate(t)

	var incidentPop, incidentErrors int
	for _, r := range reqs {
		if !r.Incident {
			continue
		}
		incidentPop++
		if r.Errored {
			incidentErrors++
		}
	}

	if incidentPop < 25 {
		t.Fatalf("only %d requests in the incident population; too few to move a 1-hour AVG() query", incidentPop)
	}
	got := float64(incidentErrors) / float64(incidentPop)
	if got < cfg.IncidentErrorRate*0.6 {
		t.Errorf("incident population error rate %.2f, want close to configured %.2f", got, cfg.IncidentErrorRate)
	}
}

// TestGenerate_OverallSLIStaysHealthy asserts the flip side: blended across
// the whole 24h window and every user type, the SLI barely moves. That's what
// lets the session say a 28-day-baseline target wouldn't have caught this on
// its own — only the short trailing window does.
func TestGenerate_OverallSLIStaysHealthy(t *testing.T) {
	_, reqs := generate(t)

	var total, good int
	for _, r := range reqs {
		if r.Route == RouteHealthz {
			continue
		}
		total++
		if r.StatusCode() < 500 {
			good++
		}
	}

	ratio := float64(good) / float64(total)
	if ratio < 0.95 {
		t.Errorf("overall SLI (route != healthz) is %.3f; incident is dominating the whole window instead of hiding inside it", ratio)
	}
}

// TestGenerate_HealthzExclusionMasksTheDrop is the numeric version of the
// package comment's pedagogical claim. It computes the SLI two ways — with
// and without the "route != healthz" filter, restricted to the incident
// window — and asserts that skipping the filter meaningfully hides the drop.
// If this test can't tell the difference, the demo's whole reason for
// excluding health checks stops being true of the data.
func TestGenerate_HealthzExclusionMasksTheDrop(t *testing.T) {
	cfg, reqs := generate(t)
	incidentStart := cfg.IncidentStart()

	var withFilterTotal, withFilterGood int
	var noFilterTotal, noFilterGood int

	for _, r := range reqs {
		if !r.Start.After(incidentStart) {
			continue
		}
		noFilterTotal++
		if r.StatusCode() < 500 {
			noFilterGood++
		}
		if r.Route == RouteHealthz {
			continue
		}
		withFilterTotal++
		if r.StatusCode() < 500 {
			withFilterGood++
		}
	}

	filtered := float64(withFilterGood) / float64(withFilterTotal)
	unfiltered := float64(noFilterGood) / float64(noFilterTotal)

	if unfiltered <= filtered {
		t.Fatalf("unfiltered SLI (%.3f) is not higher than the route-filtered SLI (%.3f); healthz traffic should be diluting the incident away",
			unfiltered, filtered)
	}
	if spread := unfiltered - filtered; spread < 0.03 {
		t.Errorf("healthz dilution only moves the ratio by %.3f; too small to make the exclusion clause visibly matter in a live query", spread)
	}
}

// TestGenerate_BubbleUpDimensionsAreSkewed approximates what BubbleUp
// computes: for each dimension value, its prevalence inside the errored
// population during the incident window versus the healthy baseline
// population. Both dimensions the slides claim BubbleUp surfaces need a wide
// enough spread to actually rank near the top on screen.
func TestGenerate_BubbleUpDimensionsAreSkewed(t *testing.T) {
	cfg, reqs := generate(t)
	incidentStart := cfg.IncidentStart()

	var errored, healthy []Request
	for _, r := range reqs {
		if r.Route == RouteHealthz || !r.Start.After(incidentStart) {
			continue
		}
		if r.Errored {
			errored = append(errored, r)
		} else {
			healthy = append(healthy, r)
		}
	}

	tests := []struct {
		dimension string
		value     func(Request) string
		want      string
		minSpread float64 // percentage points
	}{
		{"service.version", func(r Request) string { return r.Version }, cfg.BuggyVersion, 40},
		{"user.type", func(r Request) string { return r.UserType }, "enterprise", 40},
	}

	for _, tc := range tests {
		in := prevalence(errored, tc.value, tc.want)
		base := prevalence(healthy, tc.value, tc.want)
		if spread := (in - base) * 100; spread < tc.minSpread {
			t.Errorf("%s=%s is %.0f%% inside errors vs %.0f%% healthy baseline (%.0f point spread); want at least %.0f points to rank in BubbleUp",
				tc.dimension, tc.want, in*100, base*100, spread, tc.minSpread)
		}
	}
}

func prevalence(reqs []Request, value func(Request) string, want string) float64 {
	if len(reqs) == 0 {
		return 0
	}
	var n int
	for _, r := range reqs {
		if value(r) == want {
			n++
		}
	}
	return float64(n) / float64(len(reqs))
}
