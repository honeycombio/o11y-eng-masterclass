package perfscenario

import (
	"math/rand"
	"testing"
	"time"
)

func testConfig() Config {
	cfg := DefaultConfig()
	cfg.Now = time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	return cfg
}

func generate(t *testing.T) (Config, []Request) {
	t.Helper()
	cfg := testConfig()
	return cfg, Generate(cfg, rand.New(rand.NewSource(7)))
}

func requestsFor(reqs []Request, queryText string) []Request {
	var out []Request
	for _, r := range reqs {
		if r.QueryText == queryText {
			out = append(out, r)
		}
	}
	return out
}

// TestDefaultConfig_UserTypeSharesSumToOne guards pick's cumulative-boundary
// selection: if the shares don't sum to 1, the last bucket silently absorbs
// the shortfall or overflow.
func TestDefaultConfig_UserTypeSharesSumToOne(t *testing.T) {
	cfg := DefaultConfig()
	var sum float64
	for _, v := range cfg.UserTypeShares {
		sum += v
	}
	if diff := sum - 1.0; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("user type shares sum to %v, want 1.0", sum)
	}
}

// TestGenerate_BimodalHasTwoClustersLongTailDoesNot is the numeric version
// of the package comment's central claim: the bimodal template's durations
// split into two tight bands with a gap between them, while the long-tail
// template's durations spread continuously with no such gap. If this
// distinction doesn't hold, "a bimodal heatmap is a different problem than a
// long tail" isn't actually true of the seeded data.
func TestGenerate_BimodalHasTwoClustersLongTailDoesNot(t *testing.T) {
	cfg, reqs := generate(t)

	var bimodalTmpl, longTailTmpl QueryTemplate
	for _, tmpl := range cfg.Templates {
		switch tmpl.Kind {
		case KindBimodal:
			bimodalTmpl = tmpl
		case KindLongTail:
			longTailTmpl = tmpl
		}
	}

	bimodal := requestsFor(reqs, bimodalTmpl.Text)
	var fastCount, slowCount, midBandCount int
	// The gap: anything strictly between 2x the fast baseline and half the
	// slow duration is "in between" and should be rare to empty.
	midLow := bimodalTmpl.Baseline * 2
	midHigh := bimodalTmpl.SlowDuration / 2
	for _, r := range bimodal {
		switch {
		case r.Duration < midLow:
			fastCount++
		case r.Duration > midHigh:
			slowCount++
		default:
			midBandCount++
		}
	}
	if fastCount == 0 || slowCount == 0 {
		t.Fatalf("bimodal template: fast=%d slow=%d, want both clusters populated", fastCount, slowCount)
	}
	if midBandCount > len(bimodal)/20 {
		t.Errorf("bimodal template: %d of %d requests fall in the gap between clusters, want the gap mostly empty", midBandCount, len(bimodal))
	}

	longTail := requestsFor(reqs, longTailTmpl.Text)
	inGap := 0
	longTailMidLow := longTailTmpl.Baseline * 2
	for _, r := range longTail {
		if r.Duration >= longTailMidLow {
			inGap++
		}
	}
	if inGap < len(longTail)/10 {
		t.Errorf("long-tail template: only %d of %d requests exceed 2x baseline, want a real continuous tail, not a near-empty gap like the bimodal template has", inGap, len(longTail))
	}
}

// TestGenerate_EnterpriseRequestsAreSlower guards Chapter 20's third
// workflow step (AVG(duration_ms) GROUP BY user.type) having something real
// to find: enterprise requests should average meaningfully slower than free
// requests, across the whole dataset.
func TestGenerate_EnterpriseRequestsAreSlower(t *testing.T) {
	_, reqs := generate(t)

	var freeTotal, entTotal time.Duration
	var freeCount, entCount int
	for _, r := range reqs {
		switch r.UserType {
		case "free":
			freeTotal += r.Duration
			freeCount++
		case "enterprise":
			entTotal += r.Duration
			entCount++
		}
	}
	freeAvg := freeTotal / time.Duration(freeCount)
	entAvg := entTotal / time.Duration(entCount)

	if entAvg < freeAvg*3/2 {
		t.Errorf("enterprise avg duration %s is not meaningfully slower than free avg %s", entAvg, freeAvg)
	}
}

// TestGenerate_NoLatencyRegressionAcrossMigration guards the package
// comment's other central claim: P50 and P95 duration, computed separately
// for amd64 (pre-migration) and arm64 (post-migration) requests, are close
// to each other. The Graviton story is that production data confirms no
// regression — if the seeded data disagreed, the story would be a lie.
func TestGenerate_NoLatencyRegressionAcrossMigration(t *testing.T) {
	_, reqs := generate(t)

	var amd64Reqs, arm64Reqs []Request
	for _, r := range reqs {
		if r.Arch == "amd64" {
			amd64Reqs = append(amd64Reqs, r)
		} else {
			arm64Reqs = append(arm64Reqs, r)
		}
	}
	if len(amd64Reqs) == 0 || len(arm64Reqs) == 0 {
		t.Fatalf("got %d amd64 and %d arm64 requests, want both populations non-empty", len(amd64Reqs), len(arm64Reqs))
	}

	// P50 sits deep in the "fast" region for every template (bimodal's slow
	// cluster and long-tail's minority are both well above the median), so
	// it's a statistically stable estimator and gets a tight tolerance. P95
	// sits right at the bimodal template's cluster boundary — a small shift
	// in how many "slow" draws land in one half versus the other moves the
	// reported value by a full cluster-to-cluster jump, not a smooth
	// percentage — so it's inherently noisier for any finite sample and
	// gets a looser one. Both still catch a real regression (e.g. a
	// duration multiplier accidentally applied only post-migration), just
	// not phantom noise from where the P95 boundary happens to fall.
	tolerances := map[float64]float64{50: 0.15, 95: 0.20}
	for _, p := range []float64{50, 95} {
		before := Percentile(amd64Reqs, p)
		after := Percentile(arm64Reqs, p)
		ratio := float64(after) / float64(before)
		tol := tolerances[p]
		if ratio < 1-tol || ratio > 1+tol {
			t.Errorf("P%.0f duration: amd64=%s arm64=%s (ratio %.2f), want within %.0f%% of each other", p, before, after, ratio, tol*100)
		}
	}
}

// TestGenerate_RequestCountExceedsQueueMargin guards the delivery test's
// precondition, same rationale as every other seeder in this repo.
func TestGenerate_RequestCountExceedsQueueMargin(t *testing.T) {
	cfg := DefaultConfig()
	const defaultQueueSize = 2048
	if cfg.RequestCount <= defaultQueueSize*2 {
		t.Fatalf("default config emits %d requests (one span each), not comfortably past the %d-span default queue; raise RequestCount", cfg.RequestCount, defaultQueueSize)
	}
}
