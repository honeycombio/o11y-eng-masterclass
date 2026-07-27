// Package scenario generates the Masterclass 2 canary-regression dataset.
//
// # The scenario
//
// This reproduces the worked example from Chapter 2, Practice 5 (p.33):
//
//	"You can see that the new build is 40% slower for users on the enterprise
//	 plan hitting the billing endpoint. That signal is available at 1%. By the
//	 time it's moving aggregate metrics, it's already affecting a lot of people."
//
// A deploy happens partway through the window. After it, a small canary slice
// of traffic runs service.version 1.5.0 instead of 1.4.2. On the canary, and
// only for enterprise users, and only on /api/billing, requests are ~40%
// slower. Nothing else changes.
//
// The point of the demo is that this is nearly invisible in aggregate — P50
// does not move at all — but the core analysis loop (Chapter 8), automated as
// BubbleUp, isolates it in seconds because every event carries the dimensions
// needed to explain it.
//
// # Why these proportions
//
// The affected population is the product of three independent filters, so the
// numbers have to be chosen deliberately or the regression rounds away to
// nothing. With the defaults below and 10,000 traces over a 4h window with the
// deploy 90m ago:
//
//	post-deploy traces      10000 * (90/240)      ~= 3750
//	... on /api/billing     * 0.30                ~= 1125
//	... enterprise          * 0.45 (billing-only) ~=  506
//	... on the canary       * 0.10                ~=   50
//
// ~50 slow requests against 10,000 total. That is enough to draw a BubbleUp
// box around on a heatmap, while still being only ~0.5% of all traffic — so
// P50 is flat and P99 is barely perturbed. That gap between "invisible in
// aggregate" and "obvious in BubbleUp" IS the lesson; if you tune these
// numbers, keep it.
//
// Enterprise users are deliberately overrepresented on /api/billing (45% there
// versus 8% on checkout). That is both realistic — billing and invoicing skew
// enterprise — and necessary, since three multiplied filters at uniform rates
// would leave single-digit affected requests.
package scenario

import (
	"math/rand"
	"time"
)

// Config describes one generated dataset.
type Config struct {
	// TraceCount is the total number of root requests across the window.
	TraceCount int
	// Window is how far back from Now the dataset extends.
	Window time.Duration
	// DeployAgo is how long before Now the canary deploy happened. Requests
	// older than this run BaselineVersion only.
	DeployAgo time.Duration
	// Now anchors the dataset. Passed in rather than read from the clock so
	// generation is deterministic and testable.
	Now time.Time

	BaselineVersion string
	CanaryVersion   string
	// CanaryShare is the fraction of post-deploy traffic on CanaryVersion.
	CanaryShare float64
	// RegressionFactor multiplies duration for affected requests. 1.4 = 40%
	// slower, per the book.
	RegressionFactor float64
}

// DefaultConfig returns the tuned demo configuration described in the package
// comment. Now must still be set by the caller.
func DefaultConfig() Config {
	return Config{
		TraceCount:       10000,
		Window:           4 * time.Hour,
		DeployAgo:        90 * time.Minute,
		BaselineVersion:  "1.4.2",
		CanaryVersion:    "1.5.0",
		CanaryShare:      0.10,
		RegressionFactor: 1.4,
	}
}

// DeployTime is the instant the canary deploy went out. The Honeycomb deploy
// marker must be created at exactly this time or the before/after boundary in
// the demo will not line up with the data.
func (c Config) DeployTime() time.Time {
	return c.Now.Add(-c.DeployAgo)
}

// Route is one of the two endpoints in the dataset.
type Route string

const (
	RouteCheckout Route = "/api/checkout"
	RouteBilling  Route = "/api/billing"
)

// Request is one generated root request, fully resolved. The generator decides
// everything about a request up front so the emitter stays dumb and the
// scenario logic stays testable without an OTel SDK.
type Request struct {
	Start    time.Time
	Route    Route
	UserType string
	UserID   int
	Version  string
	// Regressed is true when this request hit the canary regression. Exposed
	// so tests can assert on the population rather than re-deriving the rule.
	Regressed bool
	// Errored and ErrorType carry the ordinary background failure rate. These
	// are deliberately independent of the canary: the book's example is a pure
	// latency regression, and keeping errors uncorrelated means BubbleUp has to
	// surface the latency dimensions on their own merits rather than riding on
	// an error signal that would have tripped a conventional alert anyway.
	Errored   bool
	ErrorType string
	// Steps are the child spans, in order.
	Steps []Step
}

// StatusCode is the HTTP status this request returned.
func (r Request) StatusCode() int {
	if !r.Errored {
		if r.Route == RouteBilling {
			return 200
		}
		return 201
	}
	return 402
}

// Step is one child span within a request.
type Step struct {
	Name     string
	Duration time.Duration
}

// Duration is the total wall time of the request.
func (r Request) Duration() time.Duration {
	var total time.Duration
	for _, s := range r.Steps {
		total += s.Duration
	}
	return total
}

// Generate produces the dataset. Requests come back in arbitrary time order,
// which is fine: each carries its own explicit start timestamp.
func Generate(cfg Config, rng *rand.Rand) []Request {
	deploy := cfg.DeployTime()
	out := make([]Request, 0, cfg.TraceCount)

	for i := 0; i < cfg.TraceCount; i++ {
		start := cfg.Now.Add(-time.Duration(rng.Int63n(int64(cfg.Window))))

		route := RouteCheckout
		if rng.Float64() < 0.30 {
			route = RouteBilling
		}

		userType := userTypeFor(route, rng)

		version := cfg.BaselineVersion
		onCanary := false
		if start.After(deploy) && rng.Float64() < cfg.CanaryShare {
			version = cfg.CanaryVersion
			onCanary = true
		}

		// The regression rule, stated once: canary build AND enterprise plan
		// AND the billing endpoint. Any two of the three is not enough, which
		// is exactly why a single-dimension dashboard misses it.
		regressed := onCanary && route == RouteBilling && userType == "enterprise"

		factor := 1.0
		if regressed {
			factor = cfg.RegressionFactor
		}

		errored, errorType := failureFor(route, rng)

		out = append(out, Request{
			Start:     start,
			Route:     route,
			UserType:  userType,
			UserID:    rng.Intn(5000),
			Version:   version,
			Regressed: regressed,
			Errored:   errored,
			ErrorType: errorType,
			Steps:     stepsFor(route, factor, rng),
		})
	}

	return out
}

// failureFor is the ordinary background error rate, matching the failure mode
// Masterclass 1's checkout dataset already uses so the two sessions' data reads
// consistently.
func failureFor(route Route, rng *rand.Rand) (bool, string) {
	if route == RouteBilling {
		if rng.Float64() < 0.02 {
			return true, "payment_method_expired"
		}
		return false, ""
	}
	if rng.Float64() < 0.07 {
		return true, "payment_declined"
	}
	return false, ""
}

// userTypeFor picks a plan, conditioned on route. Billing skews enterprise;
// see the package comment for why that matters.
func userTypeFor(route Route, rng *rand.Rand) string {
	r := rng.Float64()
	if route == RouteBilling {
		switch {
		case r < 0.20:
			return "free"
		case r < 0.55:
			return "premium"
		default:
			return "enterprise"
		}
	}
	switch {
	case r < 0.65:
		return "free"
	case r < 0.92:
		return "premium"
	default:
		return "enterprise"
	}
}

// stepsFor builds the child spans for a route.
//
// When factor > 1, the whole request must end up factor-times slower, because
// the book's claim is about the request ("the new build is 40% slower"). But
// all of that added time is loaded onto a single step rather than smeared
// across every step, so the trace waterfall still shows *where* it went once
// you drill in from BubbleUp. Those two requirements together mean the slow
// step absorbs more than 40% — it carries the whole delta.
func stepsFor(route Route, factor float64, rng *rand.Rand) []Step {
	var steps []Step
	slowStep := -1

	switch route {
	case RouteBilling:
		// These bands are deliberately tight — a ~77-114ms total. A 40% shift
		// has to land clearly outside the healthy distribution, or the slowest
		// normal requests crowd into the BubbleUp box alongside the regressed
		// ones and dilute the very dimensions the demo needs to surface. An
		// earlier, looser version of this (65-157ms) left the box only ~56%
		// canary, which is not a story you can tell on stage.
		steps = []Step{
			{"auth", jitter(rng, 5*time.Millisecond, 10*time.Millisecond)},
			{"load_account", jitter(rng, 18*time.Millisecond, 28*time.Millisecond)},
			// The regression lives in invoice rendering: an N+1 query pattern
			// that only bites on accounts with many line items, which in
			// practice means enterprise accounts.
			{"render_invoice", jitter(rng, 50*time.Millisecond, 68*time.Millisecond)},
			{"emit_receipt", jitter(rng, 4*time.Millisecond, 8*time.Millisecond)},
		}
		slowStep = 2
	default:
		steps = []Step{
			{"auth", jitter(rng, 5*time.Millisecond, 15*time.Millisecond)},
			{"inventory_check", jitter(rng, 10*time.Millisecond, 30*time.Millisecond)},
			{"payment", jitter(rng, 20*time.Millisecond, 60*time.Millisecond)},
			{"confirmation", jitter(rng, 5*time.Millisecond, 10*time.Millisecond)},
		}
	}

	if factor > 1 && slowStep >= 0 {
		var base time.Duration
		for _, s := range steps {
			base += s.Duration
		}
		steps[slowStep].Duration += time.Duration(float64(base) * (factor - 1))
	}

	return steps
}

func jitter(rng *rand.Rand, min, max time.Duration) time.Duration {
	return min + time.Duration(rng.Int63n(int64(max-min)))
}
