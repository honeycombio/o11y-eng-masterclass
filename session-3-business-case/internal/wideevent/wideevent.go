// Package wideevent generates the Masterclass 3 dataset: genuinely wide
// checkout events for the Arbitrary Question Test.
//
// # What this is for
//
// Chapter 28's Arbitrary Question Test gives a specific question:
//
//	"Show me all failed checkout attempts from mobile users in California
//	 using version 2.3.1 of the app during lunch hour over the past week."
//
// Legacy observability needs those dimensions indexed in advance and answers
// in hours or days, or not at all. Modern observability answers in under 60
// seconds because nobody had to anticipate the question.
//
// # Why the events are deliberately wide
//
// A dataset carrying exactly the four fields that question needs would prove
// nothing — an attendee would rightly point out it was built for the demo. So
// events here carry ~30 attributes drawn from Chapter 6's tables, of which the
// question happens to use four. The claim being demonstrated is not "we have
// the right fields," it is "we kept everything, so the question is answerable
// without having predicted it."
//
// This is also the wide event Masterclass 1 talks about and, deliberately,
// does not have: its seeded events carry about a dozen fields. If you want a
// dataset that argues the wide-events case on its own, this is the one.
//
// # Attribute naming
//
// Names come from the book's Chapter 6 tables where it has an opinion, and
// from OTel semantic conventions otherwise:
//
//   - user_agent.device with values computer/tablet/phone (Table 6-9). Note
//     that "mobile users" in the question means device == phone; there is no
//     "mobile" value in the convention.
//   - user_agent.app and user_agent.app_version (Table 6-10).
//   - geo.country.iso_code and geo.region.iso_code, from OTel semconv. These
//     are Development-stage rather than stable, and "California" is US-CA in
//     ISO 3166-2. They are written as plain string attributes rather than via
//     a second semconv import, so this module keeps one pinned version.
//   - localization.* (Table 6-18), service.build.* (Table 6-5), ratelimit.*
//     (Table 6-16), stats.* (Table 6-13), user.* (Table 6-15).
package wideevent

import (
	"fmt"
	"math/rand"
	"time"
)

// LunchHourStart and LunchHourEnd bound "lunch hour" in the user's local time.
// The question asks about lunch hour, so the generator has to concentrate
// enough traffic there for the answer to be non-empty.
const (
	LunchHourStart = 12
	LunchHourEnd   = 14
)

// AnswerVersion, AnswerDevice, and AnswerRegion are the values the book's
// question filters on. Exported so the tests and the seeder's closing summary
// can report how many events actually match, rather than asserting a number
// nobody verified.
const (
	AnswerVersion = "2.3.1"
	AnswerDevice  = "phone"
	AnswerRegion  = "US-CA"
)

// Config describes one generated dataset.
type Config struct {
	// EventCount is the number of checkout attempts to generate.
	EventCount int
	// Window is how far back from Now the dataset extends. The question asks
	// about "the past week", so this defaults to 7 days.
	Window time.Duration
	// Now anchors the dataset; passed in rather than read from the clock so
	// generation is deterministic under test.
	Now time.Time
}

func DefaultConfig() Config {
	return Config{
		EventCount: 20000,
		Window:     7 * 24 * time.Hour,
	}
}

// Event is one checkout attempt, fully resolved. Every field here becomes an
// attribute; see the package comment on why there are so many.
type Event struct {
	Start    time.Time
	Duration time.Duration

	// LocalHour is the hour of Start in the user's local time, 0-23. Carried as
	// an explicit attribute rather than left implicit in the timestamp: "during
	// lunch hour" is a local-time concept, and filtering on an integer is both
	// unambiguous and something Honeycomb can do without a derived column.
	LocalHour int

	// Outcome
	StatusCode int
	Errored    bool
	ErrorType  string
	ErrorSlug  string

	// Request and execution
	Route      string
	URLPath    string
	Method     string
	DurationDB int64

	// Identity and business context
	UserID     string
	UserType   string
	UserOrgID  string
	AuthMethod string

	// Client
	Device     string
	App        string
	AppVersion string
	OSName     string

	// Geography
	CountryISO string
	RegionISO  string

	// Localization
	Language string
	Currency string

	// Service and code context
	ServiceVersion string
	ServiceEnv     string
	BuildGitHash   string
	DeployAgeMin   int64
	FeatureFlagNew bool

	// Operational
	RateLimitLimit     int64
	RateLimitRemaining int64
	PostgresQueryCount int64
	RedisQueryCount    int64
	InstanceType       string
	CloudRegion        string
}

// MatchesArbitraryQuestion reports whether this event is one the book's
// question is asking for. The generator, the seeder summary, and the tests all
// go through this so the definition exists in exactly one place.
func (e Event) MatchesArbitraryQuestion() bool {
	if !e.Errored {
		return false
	}
	if e.Device != AnswerDevice || e.RegionISO != AnswerRegion || e.AppVersion != AnswerVersion {
		return false
	}
	return e.LocalHour >= LunchHourStart && e.LocalHour < LunchHourEnd
}

// Generate produces the dataset.
func Generate(cfg Config, rng *rand.Rand) []Event {
	out := make([]Event, 0, cfg.EventCount)

	for range cfg.EventCount {
		out = append(out, generateOne(cfg, rng))
	}
	return out
}

func generateOne(cfg Config, rng *rand.Rand) Event {
	start := startTime(cfg, rng)

	device := weighted(rng, []weightedValue{
		{"phone", 0.55}, {"computer", 0.35}, {"tablet", 0.10},
	})

	app, appVersion, osName := clientFor(device, rng)
	country, region := geoFor(rng)
	userType := weighted(rng, []weightedValue{
		{"free", 0.60}, {"premium", 0.30}, {"enterprise", 0.10},
	})

	errored, errType, errSlug := failureFor(rng)

	dbDuration := int64(20 + rng.Intn(120))
	duration := time.Duration(60+rng.Intn(400)) * time.Millisecond

	return Event{
		Start:      start,
		LocalHour:  start.Hour(),
		Duration:   duration,
		StatusCode: statusFor(errored),
		Errored:    errored,
		ErrorType:  errType,
		ErrorSlug:  errSlug,

		Route:      "/api/checkout",
		URLPath:    "/api/checkout",
		Method:     "POST",
		DurationDB: dbDuration,

		UserID:     fmt.Sprintf("user_%d", rng.Intn(20000)),
		UserType:   userType,
		UserOrgID:  fmt.Sprintf("org_%d", rng.Intn(800)),
		AuthMethod: weighted(rng, []weightedValue{{"token", 0.6}, {"sso-github", 0.25}, {"basic-auth", 0.15}}),

		Device:     device,
		App:        app,
		AppVersion: appVersion,
		OSName:     osName,

		CountryISO: country,
		RegionISO:  region,

		Language: weighted(rng, []weightedValue{{"en-US", 0.7}, {"es-MX", 0.15}, {"fr-CA", 0.1}, {"de-DE", 0.05}}),
		Currency: weighted(rng, []weightedValue{{"USD", 0.8}, {"CAD", 0.1}, {"EUR", 0.1}}),

		ServiceVersion: "1.4.2",
		ServiceEnv:     "production",
		BuildGitHash:   "6f6466b0e693470729b669f3745358df29f97e8d",
		DeployAgeMin:   int64(30 + rng.Intn(4000)),
		FeatureFlagNew: rng.Float64() < 0.25,

		RateLimitLimit:     200000,
		RateLimitRemaining: int64(rng.Intn(200000)),
		PostgresQueryCount: int64(3 + rng.Intn(12)),
		RedisQueryCount:    int64(1 + rng.Intn(8)),
		InstanceType:       weighted(rng, []weightedValue{{"m6i.xlarge", 0.6}, {"m7g.xlarge", 0.4}}),
		CloudRegion:        weighted(rng, []weightedValue{{"us-east-1", 0.5}, {"us-west-2", 0.3}, {"eu-west-1", 0.2}}),
	}
}

// startTime spreads events across the window with a daily shape that has a
// real lunch-hour bulge, because the question filters on it.
func startTime(cfg Config, rng *rand.Rand) time.Time {
	base := cfg.Now.Add(-time.Duration(rng.Int63n(int64(cfg.Window))))

	// Roughly a fifth of traffic is pulled into the 12:00-14:00 window. Two
	// hours out of 24 would otherwise be ~8% of events, which leaves too few
	// matches once the other three filters are applied.
	if rng.Float64() >= 0.20 {
		return base
	}

	hour := LunchHourStart + rng.Intn(LunchHourEnd-LunchHourStart)
	lunch := time.Date(base.Year(), base.Month(), base.Day(), hour,
		rng.Intn(60), rng.Intn(60), 0, base.Location())

	// Rewriting the hour can move a timestamp across either edge of the
	// window: an event drawn near the start of the window and reset to noon
	// can land before the window opens, and one drawn near the end can land
	// after Now. Nudge by a day where that fixes it, and otherwise leave the
	// event at its original time rather than emit something out of range.
	oldest := cfg.Now.Add(-cfg.Window)
	if lunch.Before(oldest) {
		lunch = lunch.AddDate(0, 0, 1)
	} else if lunch.After(cfg.Now) {
		lunch = lunch.AddDate(0, 0, -1)
	}
	if lunch.Before(oldest) || lunch.After(cfg.Now) {
		return base
	}
	return lunch
}

// clientFor keeps the client fields self-consistent: a desktop browser has no
// app version, and iOS builds do not run on Android.
func clientFor(device string, rng *rand.Rand) (app, appVersion, osName string) {
	if device == "computer" {
		return "", "", weighted(rng, []weightedValue{{"macOS", 0.5}, {"Windows", 0.4}, {"Linux", 0.1}})
	}

	app = weighted(rng, []weightedValue{{"iOS", 0.6}, {"android", 0.4}})
	// 2.3.1 is the version the book's question asks about. A realistic install
	// base is spread across several versions, so it is a plurality, not
	// everything — otherwise the filter would not narrow anything.
	appVersion = weighted(rng, []weightedValue{
		{"2.3.1", 0.35}, {"2.3.0", 0.25}, {"2.2.4", 0.20}, {"2.4.0-beta", 0.20},
	})
	if app == "iOS" {
		return app, appVersion, "iOS"
	}
	return app, appVersion, "Android"
}

// geoFor returns country and ISO 3166-2 region. California is deliberately the
// largest single region, as it would be for a US consumer product.
func geoFor(rng *rand.Rand) (country, region string) {
	switch r := rng.Float64(); {
	case r < 0.28:
		return "US", "US-CA"
	case r < 0.44:
		return "US", "US-NY"
	case r < 0.56:
		return "US", "US-TX"
	case r < 0.66:
		return "US", "US-WA"
	case r < 0.74:
		return "US", "US-IL"
	case r < 0.86:
		return "CA", "CA-ON"
	case r < 0.94:
		return "GB", "GB-ENG"
	default:
		return "DE", "DE-BE"
	}
}

func failureFor(rng *rand.Rand) (bool, string, string) {
	switch r := rng.Float64(); {
	case r < 0.055:
		return true, "payment_declined", "err-stripe-card-declined"
	case r < 0.075:
		return true, "payment_method_expired", "err-stripe-card-expired"
	case r < 0.085:
		return true, "inventory_unavailable", "err-inventory-oversold"
	default:
		return false, "", ""
	}
}

func statusFor(errored bool) int {
	if errored {
		return 402
	}
	return 201
}

type weightedValue struct {
	value  string
	weight float64
}

func weighted(rng *rand.Rand, values []weightedValue) string {
	r := rng.Float64()
	var cumulative float64
	for _, v := range values {
		cumulative += v.weight
		if r < cumulative {
			return v.value
		}
	}
	return values[len(values)-1].value
}
