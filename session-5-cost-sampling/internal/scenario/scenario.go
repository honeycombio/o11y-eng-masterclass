// Package scenario generates the Masterclass 5 cost/sampling dataset.
//
// # The scenario
//
// One service, cost-sampling-service, with four routes at a deliberately
// skewed distribution:
//
//	/api/search          85%
//	/api/checkout        10%
//	/api/refund           4%
//	/api/admin/report     1%
//
// The skew is the point: Chapter 15's per-key adaptive rate only differs
// visibly from a single global rate when low-volume keys exist that a flat
// sample would erase. Every request is a two-span trace — a root HTTP span
// plus one db.query child — not a single span, because the Collector's
// adaptive_tail_sampling processor (see
// ../../../session-1-fundamentals/collector/otel-collector-config.yaml)
// decides per whole trace, and a demo about that needs traces with more than
// one span to make the "whole trace kept or dropped together" behavior real.
//
// A synthetic error rate (default 15%, well above any real service's
// baseline) marks some requests as errored at both the root and the child
// span, consistently. That rate is deliberately generous — this dataset
// exists to make the Collector's keep-errors rule obviously do something
// within a short, live seed, not to model realistic reliability.
//
// Unlike session 4's seeder, this dataset carries no backdated timestamps:
// the emitter sends spans at real wall-clock time, paced over
// Config.Duration, because the sampling decision this scenario demonstrates
// happens downstream in the Collector as spans actually arrive — a
// decision_delay window measured against fabricated history would not mean
// anything.
//
// # Why these numbers
//
// The curriculum's live-demo arithmetic, matching the Collector's
// cost-sampling-service rule (goal_percentage: 5, see the Collector config
// for the full rule):
//
//	retained = errorRate*1.0 + (1-errorRate)*0.05
//	         = 0.15         + 0.85*0.05
//	         = 0.1925                        (~19%, an ~81% reduction)
//
// TestGenerate_ErrorRateMatchesConfig and the package's other tests guard the
// inputs to that formula. Nothing automated checks the output — that would
// mean asserting on the real Collector's actual sampling decision, which
// needs a live Collector and a query, not a unit test. See the top-level
// README's pre-session checklist: comparing the seeder's printed request
// count against Honeycomb's weighted COUNT() is the verification step.
//
// # Delivery test sizing
//
// Two spans per request means RequestCount*2 must exceed the SDK
// BatchSpanProcessor's default queue depth (2048) with margin — see
// cmd/seed-cost-sampling-service's maxQueueSize and TestSeedVolumeExceedsQueue,
// the same guard session 4 uses for the same reason.
package scenario

import "math/rand"

// Route is one endpoint in the dataset, with its share of all requests.
// Error status is an independent draw — see Generate — so Share is not
// conditioned on whether a request ends up errored.
type Route struct {
	Path  string
	Share float64
}

// Config describes one generated dataset.
type Config struct {
	// RequestCount is the total number of traces (two spans each) to
	// generate.
	RequestCount int
	// ErrorRate is the fraction of requests marked errored at both spans.
	ErrorRate float64
	// Routes is the route distribution non-error selection draws from.
	// Shares should sum to 1.0 — see TestDefaultConfig_RouteSharesSumToOne.
	Routes []Route
}

// DefaultConfig returns the tuned demo configuration described in the package
// comment.
func DefaultConfig() Config {
	return Config{
		RequestCount: 5000,
		ErrorRate:    0.15,
		Routes: []Route{
			{Path: "/api/search", Share: 0.85},
			{Path: "/api/checkout", Share: 0.10},
			{Path: "/api/refund", Share: 0.04},
			{Path: "/api/admin/report", Share: 0.01},
		},
	}
}

// Request is one generated request, fully resolved. The generator decides
// everything up front so the emitter stays dumb: pick a route, decide
// errored-or-not, emit two spans, move on.
type Request struct {
	Route     string
	Errored   bool
	ErrorType string
}

// Generate produces the dataset, in emission order.
func Generate(cfg Config, rng *rand.Rand) []Request {
	out := make([]Request, 0, cfg.RequestCount)

	for i := 0; i < cfg.RequestCount; i++ {
		errored := rng.Float64() < cfg.ErrorRate

		req := Request{
			Route:   routeFor(cfg.Routes, rng),
			Errored: errored,
		}
		if errored {
			req.ErrorType = "dependency_unavailable"
		}
		out = append(out, req)
	}

	return out
}

func routeFor(routes []Route, rng *rand.Rand) string {
	r := rng.Float64()
	var cum float64
	for _, rt := range routes {
		cum += rt.Share
		if r < cum {
			return rt.Path
		}
	}
	// Floating-point cumulative sum can fall just short of 1.0; the last
	// route is the correct fallback rather than an empty string.
	return routes[len(routes)-1].Path
}
