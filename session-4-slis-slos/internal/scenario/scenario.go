// Package scenario generates the Masterclass 4 SLI/SLO dataset.
//
// # The scenario
//
// A single business endpoint, /api/orders, plus a /healthz check that gets
// polled far more often than anyone calls the real endpoint. Health checks
// always succeed. That imbalance is the point: Chapter 11's HTTP-API SLI is
//
//	count(status_code < 500) / count(*) where route != '/healthz'
//
// and this dataset exists to make the "where route != '/healthz'" clause
// matter. Leave it out and the healthy healthz traffic dilutes a real
// incident into invisibility — see TestGenerate_HealthzExclusionMasksTheDrop.
//
// Most traffic runs a healthy build. A minority is still on an older build
// that carries a latent defect: for a recent, ongoing window, requests from
// that build for enterprise customers fail at an elevated rate. Everywhere
// else — other users, other times, /healthz — the error rate is the ordinary
// background rate. That gives BubbleUp two real dimensions to rank
// (service.version and user.type), and gives the burn-rate demo something
// genuinely happening in the trailing hour rather than a flat line.
//
// # Why these proportions
//
// The incident has to survive two competing pressures: visible enough that a
// 1-hour AVG() query and a BubbleUp box both find it cleanly, subtle enough
// that it doesn't dominate the whole dataset (a burn-rate alert on a SLI
// that's obviously broken all the time isn't a burn-rate demo, it's just an
// outage). The incident population is the product of four independent
// filters (in the window, not a health check, enterprise, buggy build), and
// four compounding fractions shrink fast — a naive 24-hour dataset with a
// small enterprise share leaves single-digit affected requests. So the window
// is deliberately short (6h, "this seed covers a recent stretch of traffic",
// not a full day) and the enterprise and buggy-build shares are deliberately
// generous. With the defaults below and 15,000 requests:
//
//	requests in the incident window     15000 * (60/360)          = 2500
//	... on /api/orders (not healthz)    * 0.40                    = 1000
//	... enterprise                      * 0.20                    =  200
//	... on the buggy build              * 0.30                    =   60
//
// ~60 affected requests, most erroring at IncidentErrorRate, is enough for
// TestGenerate_ErrorSpikeIsDemoSized to hold and for the burn query to show a
// clean drop without the incident swallowing the whole dataset.
package scenario

import (
	"math/rand"
	"time"
)

// Config describes one generated dataset.
type Config struct {
	// RequestCount is the total number of requests across the window,
	// /healthz included.
	RequestCount int
	// Window is how far back from Now the dataset extends.
	Window time.Duration
	// IncidentAgo is how long before Now the incident started. It is still
	// ongoing at Now — deliberately, so a trailing-1-hour burn query and a
	// live trigger both see it as current rather than historical.
	IncidentAgo time.Duration
	// Now anchors the dataset. Passed in rather than read from the clock so
	// generation is deterministic and testable.
	Now time.Time

	HealthyVersion string
	BuggyVersion   string
	// BuggyShare is the fraction of /api/orders traffic running BuggyVersion,
	// throughout the whole window (an incomplete rollout, not a deploy that
	// happens mid-window — there is no single deploy time to mark here).
	BuggyShare float64
	// HealthzShare is the fraction of all requests that are health checks
	// rather than /api/orders. Health checks always succeed.
	HealthzShare float64
	// BaselineErrorRate is the ordinary background 5xx rate on /api/orders,
	// outside the incident population.
	BaselineErrorRate float64
	// IncidentErrorRate is the 5xx rate for the affected population: buggy
	// build, enterprise plan, /api/orders, inside the incident window.
	IncidentErrorRate float64
}

// DefaultConfig returns the tuned demo configuration described in the package
// comment. Now must still be set by the caller.
func DefaultConfig() Config {
	return Config{
		RequestCount:      15000,
		Window:            6 * time.Hour,
		IncidentAgo:       60 * time.Minute,
		HealthyVersion:    "1.5.0",
		BuggyVersion:      "1.4.2",
		BuggyShare:        0.30,
		HealthzShare:      0.68,
		BaselineErrorRate: 0.01,
		IncidentErrorRate: 0.65,
	}
}

// IncidentStart is the instant the incident began. It runs from here through
// Now.
func (c Config) IncidentStart() time.Time {
	return c.Now.Add(-c.IncidentAgo)
}

// Route is one of the two endpoints in the dataset.
type Route string

const (
	RouteOrders  Route = "/api/orders"
	RouteHealthz Route = "/healthz"
)

// Request is one generated request, fully resolved. The generator decides
// everything up front so the emitter stays dumb and the scenario logic stays
// testable without an OTel SDK.
type Request struct {
	Start    time.Time
	Route    Route
	Method   string
	UserType string
	UserID   int
	Version  string
	// Incident is true when this request falls inside the affected
	// population: buggy build, enterprise, /api/orders, incident window.
	// Exposed so tests can assert on the population rather than re-deriving
	// the rule.
	Incident  bool
	Errored   bool
	ErrorType string
	Duration  time.Duration
}

// StatusCode is the HTTP status this request returned.
func (r Request) StatusCode() int {
	if !r.Errored {
		return 200
	}
	return 503
}

// Generate produces the dataset. Requests come back in arbitrary time order,
// which is fine: each carries its own explicit start timestamp.
func Generate(cfg Config, rng *rand.Rand) []Request {
	incidentStart := cfg.IncidentStart()
	out := make([]Request, 0, cfg.RequestCount)

	for i := 0; i < cfg.RequestCount; i++ {
		start := cfg.Now.Add(-time.Duration(rng.Int63n(int64(cfg.Window))))

		if rng.Float64() < cfg.HealthzShare {
			out = append(out, Request{
				Start:    start,
				Route:    RouteHealthz,
				Method:   "GET",
				Version:  versionFor(cfg, rng),
				UserType: "system",
				UserID:   0,
				Duration: jitter(rng, 2*time.Millisecond, 6*time.Millisecond),
			})
			continue
		}

		version := versionFor(cfg, rng)
		userType := userTypeFor(rng)
		inWindow := start.After(incidentStart)
		incident := inWindow && version == cfg.BuggyVersion && userType == "enterprise"

		errRate := cfg.BaselineErrorRate
		if incident {
			errRate = cfg.IncidentErrorRate
		}
		errored := rng.Float64() < errRate
		errorType := ""
		if errored {
			errorType = "dependency_unavailable"
		}

		out = append(out, Request{
			Start:     start,
			Route:     RouteOrders,
			Method:    "POST",
			UserType:  userType,
			UserID:    rng.Intn(5000),
			Version:   version,
			Incident:  incident,
			Errored:   errored,
			ErrorType: errorType,
			Duration:  jitter(rng, 20*time.Millisecond, 90*time.Millisecond),
		})
	}

	return out
}

func versionFor(cfg Config, rng *rand.Rand) string {
	if rng.Float64() < cfg.BuggyShare {
		return cfg.BuggyVersion
	}
	return cfg.HealthyVersion
}

func userTypeFor(rng *rand.Rand) string {
	r := rng.Float64()
	switch {
	case r < 0.50:
		return "free"
	case r < 0.80:
		return "premium"
	default:
		return "enterprise"
	}
}

func jitter(rng *rand.Rand, minD, maxD time.Duration) time.Duration {
	return minD + time.Duration(rng.Int63n(int64(maxD-minD)))
}
