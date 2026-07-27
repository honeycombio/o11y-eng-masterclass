package scenario

import (
	"math/rand"
	"slices"
	"sort"
	"testing"
	"time"
)

func testConfig() Config {
	cfg := DefaultConfig()
	// Fixed anchor so the dataset is fully deterministic.
	cfg.Now = time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	return cfg
}

func generate(t *testing.T) (Config, []Request) {
	t.Helper()
	cfg := testConfig()
	return cfg, Generate(cfg, rand.New(rand.NewSource(7)))
}

// TestGenerate_RegressionRule pins the rule down: all three conditions are
// required, and every request meeting them is regressed. If a refactor lets any
// two of the three trigger it, the demo stops teaching that single-dimension
// views miss multicausal problems.
func TestGenerate_RegressionRule(t *testing.T) {
	cfg, reqs := generate(t)

	for _, r := range reqs {
		want := r.Version == cfg.CanaryVersion && r.Route == RouteBilling && r.UserType == "enterprise"
		if r.Regressed != want {
			t.Fatalf("regressed=%t for version=%s route=%s type=%s; want %t",
				r.Regressed, r.Version, r.Route, r.UserType, want)
		}
	}
}

// TestGenerate_CanaryOnlyAfterDeploy guards the before/after boundary the
// deploy marker relies on.
func TestGenerate_CanaryOnlyAfterDeploy(t *testing.T) {
	cfg, reqs := generate(t)
	deploy := cfg.DeployTime()

	for _, r := range reqs {
		if r.Version == cfg.CanaryVersion && !r.Start.After(deploy) {
			t.Fatalf("canary request at %s predates deploy at %s", r.Start, deploy)
		}
		if r.Start.Before(cfg.Now.Add(-cfg.Window)) {
			t.Fatalf("request at %s falls outside the %s window", r.Start, cfg.Window)
		}
	}
}

// TestGenerate_AffectedPopulationIsDemoSized is the load-bearing test. The demo
// only works inside a narrow band: enough slow requests to draw a BubbleUp box
// around, few enough that aggregate percentiles stay quiet. Both bounds matter,
// so both are asserted.
func TestGenerate_AffectedPopulationIsDemoSized(t *testing.T) {
	_, reqs := generate(t)

	var regressed int
	for _, r := range reqs {
		if r.Regressed {
			regressed++
		}
	}

	if regressed < 25 {
		t.Errorf("only %d regressed requests; too few to isolate on a heatmap", regressed)
	}
	if share := float64(regressed) / float64(len(reqs)); share > 0.02 {
		t.Errorf("regressed share %.3f exceeds 2%%; the regression stops being an aggregate blind spot", share)
	}
}

// TestGenerate_InvisibleInAggregate asserts the actual pedagogical claim: the
// median across all traffic is unmoved by the regression. This is what lets the
// session say "your dashboard would not have caught this."
func TestGenerate_InvisibleInAggregate(t *testing.T) {
	cfg, reqs := generate(t)
	deploy := cfg.DeployTime()

	var before, after []time.Duration
	for _, r := range reqs {
		if r.Start.After(deploy) {
			after = append(after, r.Duration())
		} else {
			before = append(before, r.Duration())
		}
	}

	p50Before, p50After := percentile(before, 0.50), percentile(after, 0.50)
	drift := relDelta(p50Before, p50After)
	if drift > 0.05 {
		t.Errorf("overall P50 moved %.1f%% across the deploy (%v -> %v); regression is leaking into the aggregate",
			drift*100, p50Before, p50After)
	}

	// ...but the same comparison scoped to the affected cohort must show it
	// clearly, or there is nothing for BubbleUp to find.
	var cohortBefore, cohortAfter []time.Duration
	for _, r := range reqs {
		if r.Route != RouteBilling || r.UserType != "enterprise" {
			continue
		}
		if r.Version == cfg.CanaryVersion {
			cohortAfter = append(cohortAfter, r.Duration())
		} else {
			cohortBefore = append(cohortBefore, r.Duration())
		}
	}

	cohortDrift := relDelta(percentile(cohortBefore, 0.50), percentile(cohortAfter, 0.50))
	if cohortDrift < 0.15 {
		t.Errorf("enterprise-on-billing P50 only moved %.1f%% between builds; too subtle to demo", cohortDrift*100)
	}
}

// TestGenerate_BubbleUpDimensionsAreSkewed approximates what BubbleUp computes:
// for each dimension value, its prevalence inside the slow cohort versus the
// baseline. BubbleUp ranks results by percentage-point difference between the
// two, so that — not their ratio — is what this asserts. For scale, the book's
// worked example surfaces a dimension at 98% inside versus 17% baseline, an
// 81-point spread. All three explanatory dimensions here need a spread wide
// enough to land near the top of that ranked list on screen.
func TestGenerate_BubbleUpDimensionsAreSkewed(t *testing.T) {
	cfg, reqs := generate(t)

	// Isolate the way an operator would during the demo: the slowest requests
	// on the route where the symptom showed up.
	billing := make([]Request, 0, len(reqs))
	for _, r := range reqs {
		if r.Route == RouteBilling {
			billing = append(billing, r)
		}
	}
	sort.Slice(billing, func(i, j int) bool { return billing[i].Duration() > billing[j].Duration() })

	const boxSize = 50
	inside, baseline := billing[:boxSize], billing[boxSize:]

	tests := []struct {
		dimension string
		value     func(Request) string
		want      string
		minSpread float64 // percentage points
	}{
		{"service.version", func(r Request) string { return r.Version }, cfg.CanaryVersion, 60},
		{"user.type", func(r Request) string { return r.UserType }, "enterprise", 30},
	}

	for _, tc := range tests {
		in := prevalence(inside, tc.value, tc.want)
		base := prevalence(baseline, tc.value, tc.want)
		if spread := (in - base) * 100; spread < tc.minSpread {
			t.Errorf("%s=%s is %.0f%% inside vs %.0f%% baseline (%.0f point spread); want at least %.0f points to rank in BubbleUp",
				tc.dimension, tc.want, in*100, base*100, spread, tc.minSpread)
		}
	}
}

// TestGenerate_RequestIsFortyPercentSlower checks the book's actual number: the
// affected cohort's requests, not just one span inside them, are ~40% slower.
func TestGenerate_RequestIsFortyPercentSlower(t *testing.T) {
	cfg, reqs := generate(t)

	var regressed, healthy []time.Duration
	for _, r := range reqs {
		if r.Route != RouteBilling || r.UserType != "enterprise" {
			continue
		}
		if r.Regressed {
			regressed = append(regressed, r.Duration())
		} else {
			healthy = append(healthy, r.Duration())
		}
	}

	got := relDelta(percentile(healthy, 0.50), percentile(regressed, 0.50))
	want := cfg.RegressionFactor - 1
	if got < want*0.8 || got > want*1.2 {
		t.Errorf("affected requests are %.0f%% slower (P50 %v -> %v); want ~%.0f%% per the book",
			got*100, percentile(healthy, 0.50), percentile(regressed, 0.50), want*100)
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

func percentile(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := slices.Clone(ds)
	slices.Sort(s)
	idx := int(p * float64(len(s)-1))
	return s[idx]
}

func relDelta(a, b time.Duration) float64 {
	if a == 0 {
		return 0
	}
	d := float64(b-a) / float64(a)
	if d < 0 {
		return -d
	}
	return d
}
