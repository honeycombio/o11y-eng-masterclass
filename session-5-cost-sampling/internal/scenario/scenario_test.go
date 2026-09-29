package scenario

import (
	"math/rand"
	"os"
	"regexp"
	"testing"
	"time"
)

func generate(t *testing.T) (Config, []Request) {
	t.Helper()
	cfg := DefaultConfig()
	return cfg, Generate(cfg, rand.New(rand.NewSource(7)))
}

// TestDefaultConfig_RouteSharesSumToOne guards the input to routeFor's
// cumulative-sum selection and to the package comment's retained-share
// arithmetic: if the shares don't sum to 1, "85/10/4/1" stops meaning what
// the docs say it means.
func TestDefaultConfig_RouteSharesSumToOne(t *testing.T) {
	cfg := DefaultConfig()

	var sum float64
	for _, r := range cfg.Routes {
		sum += r.Share
	}
	if diff := sum - 1.0; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("route shares sum to %v, want 1.0", sum)
	}
}

// TestGenerate_RouteDistributionMatchesShares checks each route's observed
// share against its configured share. The Collector's per-key adaptive rate
// only visibly differs from a flat rate if /api/admin/report's 1% is both
// present and clearly rarer than /api/search's 85% — this test is what
// guards that the skew survives into the generated data.
func TestGenerate_RouteDistributionMatchesShares(t *testing.T) {
	cfg, reqs := generate(t)

	counts := make(map[string]int, len(cfg.Routes))
	for _, r := range reqs {
		counts[r.Route]++
	}

	for _, rt := range cfg.Routes {
		got := float64(counts[rt.Path]) / float64(len(reqs))
		if diff := got - rt.Share; diff > 0.03 || diff < -0.03 {
			t.Errorf("route %s: observed share %.3f, want close to configured %.3f", rt.Path, got, rt.Share)
		}
	}
}

// TestGenerate_LowVolumeRouteStaysVisible is the numeric version of "the skew
// is the point" from the package comment: at the default RequestCount, the
// rarest route still needs enough requests to be a real, rankable population
// rather than a handful of outliers noise would swallow.
func TestGenerate_LowVolumeRouteStaysVisible(t *testing.T) {
	cfg, reqs := generate(t)

	var rarest string
	minShare := 1.0
	for _, rt := range cfg.Routes {
		if rt.Share < minShare {
			minShare = rt.Share
			rarest = rt.Path
		}
	}

	var n int
	for _, r := range reqs {
		if r.Route == rarest {
			n++
		}
	}
	if n < 25 {
		t.Fatalf("rarest route %s has only %d requests; too few for the per-key demo to show it as a real population", rarest, n)
	}
}

// TestGenerate_ErrorRateMatchesConfig guards one half of the package
// comment's retained-share arithmetic (errorRate*1.0 term): if the actual
// generated error rate drifts from cfg.ErrorRate, the live demo's ~81%
// reduction claim stops matching what the Collector actually receives.
func TestGenerate_ErrorRateMatchesConfig(t *testing.T) {
	cfg, reqs := generate(t)

	var errored int
	for _, r := range reqs {
		if r.Errored {
			errored++
		}
	}
	got := float64(errored) / float64(len(reqs))
	if diff := got - cfg.ErrorRate; diff > 0.02 || diff < -0.02 {
		t.Errorf("observed error rate %.3f, want close to configured %.3f", got, cfg.ErrorRate)
	}
}

// TestGenerate_ErroredRequestsCarryErrorType checks the generator's own
// invariant: an errored request always has a non-empty ErrorType, and a
// non-errored one never does, matching the field's doc comment.
func TestGenerate_ErroredRequestsCarryErrorType(t *testing.T) {
	_, reqs := generate(t)

	for _, r := range reqs {
		if r.Errored && r.ErrorType == "" {
			t.Fatalf("request marked errored but ErrorType is empty")
		}
		if !r.Errored && r.ErrorType != "" {
			t.Fatalf("request not errored but ErrorType is %q", r.ErrorType)
		}
	}
}

// TestGenerate_RequestCountExceedsQueueMargin guards the delivery test's
// precondition rather than re-deriving it: two spans per request at the
// default RequestCount must clear the SDK's default 2048-span batch queue
// with real margin, or a dropped-span regression would go unnoticed the same
// way session 4's package comment describes.
func TestGenerate_RequestCountExceedsQueueMargin(t *testing.T) {
	cfg := DefaultConfig()
	const defaultQueueSize = 2048

	spans := cfg.RequestCount * 2
	if spans <= defaultQueueSize*2 {
		t.Fatalf("default config emits %d spans, not comfortably past the %d-span default queue; raise RequestCount", spans, defaultQueueSize)
	}
}

// collectorConfig is the shared Collector config whose adaptive_tail_sampling
// rules this dataset exercises.
const collectorConfig = "../../../session-1-fundamentals/collector/otel-collector-config.yaml"

// minAdjustmentIntervals is how many adjustment intervals Config.Duration must
// span. The first interval is sampled flat (see the package comment), so four
// keeps at least three quarters of the traffic on per-route rates.
const minAdjustmentIntervals = 4

// TestDefaultConfig_DurationSpansAdjustmentIntervals guards the pacing the
// per-route demo depends on. It reads adjustment_interval from the real
// Collector config rather than a copied constant, so lengthening the interval
// there without lengthening Duration here fails CI instead of silently
// reverting the live demo to a flat sample.
func TestDefaultConfig_DurationSpansAdjustmentIntervals(t *testing.T) {
	raw, err := os.ReadFile(collectorConfig)
	if err != nil {
		t.Fatalf("reading Collector config: %v", err)
	}
	match := regexp.MustCompile(`(?m)^\s*adjustment_interval:\s*(\S+)\s*$`).FindSubmatch(raw)
	if match == nil {
		t.Fatalf("no adjustment_interval in %s", collectorConfig)
	}
	interval, err := time.ParseDuration(string(match[1]))
	if err != nil {
		t.Fatalf("parsing adjustment_interval %q: %v", match[1], err)
	}

	cfg := DefaultConfig()
	if want := minAdjustmentIntervals * interval; cfg.Duration < want {
		t.Fatalf("Duration %s spans fewer than %d adjustment intervals of %s (want >= %s); most traffic would be sampled at the flat cold-start rate", cfg.Duration, minAdjustmentIntervals, interval, want)
	}
}
