// Package perfscenario generates the Masterclass 6 performance-engineering
// dataset.
//
// # The scenario
//
// One service, five parameterized db.query.text templates, and a Graviton
// (amd64 -> arm64) migration marker halfway through the seeded window.
// Chapter 20's workflow needs three different query duration shapes to
// demonstrate its first three steps, and a stable shape across the migration
// point to demonstrate the fourth:
//
//   - Fast: three templates (Users, Orders, Inventory) with tight jitter
//     around a low baseline — the uninteresting majority of traffic.
//   - Bimodal (OrdersByStatus): most requests are fast, but a fixed share
//     land on a slow path (a missing index on one status value, left
//     unnamed on purpose — the discovery is the heatmap's two clusters, not
//     a groupby). A single P99 number over this template averages the two
//     clusters together and hides both of them; a heatmap doesn't.
//   - Long-tail (AuditLog): a continuous spread from the baseline up to
//     several multiples of it, with no second cluster. Chapter 20's point
//     that "a bimodal heatmap is a different problem than a long tail" only
//     lands if the dataset actually contains one of each — see
//     TestGenerate_BimodalHasTwoClustersLongTailDoesNot.
//
// Every request also carries a user.type (free/premium/enterprise), and
// enterprise requests run measurably slower — larger accounts, more data —
// which is Chapter 20's third step (AVG(duration_ms) GROUP BY user.type).
//
// # The migration
//
// host.arch is amd64 for the first half of the window and arm64 for the
// second, split at MigrationAgo, with no change to any duration
// distribution. That's the point: Chapter 20's worked example trusts
// production data to show the migration was safe, and the seeded data has
// to actually be safe for a P50/P95 before/after comparison to say so
// honestly. See TestGenerate_NoLatencyRegressionAcrossMigration.
//
// # Delivery test sizing
//
// One span per request (no nested trace — this dataset is about query
// duration analysis, not a waterfall), so RequestCount alone must clear the
// SDK's default 2048-span queue with margin; see the seeder's maxQueueSize
// and TestGenerate_RequestCountExceedsQueueMargin.
package perfscenario

import (
	"math"
	"math/rand"
	"sort"
	"time"
)

// QueryKind determines how a template's duration is drawn.
type QueryKind int

const (
	// KindFast is tight jitter around Baseline.
	KindFast QueryKind = iota
	// KindBimodal draws Baseline most of the time and SlowDuration the rest.
	KindBimodal
	// KindLongTail draws Baseline plus an exponential tail, so most requests
	// are fast but a continuous minority run progressively slower.
	KindLongTail
)

// QueryTemplate is one parameterized db.query.text shape.
type QueryTemplate struct {
	Text     string
	Kind     QueryKind
	Baseline time.Duration
	// SlowDuration and SlowShare apply only to KindBimodal.
	SlowDuration time.Duration
	SlowShare    float64
}

// Config describes one generated dataset.
type Config struct {
	RequestCount int
	Window       time.Duration
	MigrationAgo time.Duration
	Now          time.Time

	Templates []QueryTemplate

	// UserTypeShares must sum to 1.0 — see
	// TestDefaultConfig_UserTypeSharesSumToOne.
	UserTypeShares map[string]float64
	// EnterpriseMultiplier scales duration for user.type=enterprise requests
	// on top of whatever the template produced.
	EnterpriseMultiplier float64
}

// DefaultConfig returns the tuned demo configuration described in the
// package comment. Now must still be set by the caller.
func DefaultConfig() Config {
	return Config{
		RequestCount: 24000,
		Window:       48 * time.Hour,
		MigrationAgo: 24 * time.Hour,
		Templates: []QueryTemplate{
			{Text: "SELECT * FROM users WHERE id = $1", Kind: KindFast, Baseline: 5 * time.Millisecond},
			{Text: "SELECT * FROM orders WHERE user_id = $1", Kind: KindFast, Baseline: 8 * time.Millisecond},
			{Text: "UPDATE inventory SET quantity = $1 WHERE sku = $2", Kind: KindFast, Baseline: 12 * time.Millisecond},
			{Text: "SELECT * FROM orders WHERE status = $1", Kind: KindBimodal, Baseline: 10 * time.Millisecond, SlowDuration: 250 * time.Millisecond, SlowShare: 0.20},
			{Text: "SELECT * FROM audit_log WHERE created_at > $1", Kind: KindLongTail, Baseline: 20 * time.Millisecond},
		},
		UserTypeShares: map[string]float64{
			"free":       0.50,
			"premium":    0.30,
			"enterprise": 0.20,
		},
		EnterpriseMultiplier: 1.8,
	}
}

// MigrationStart is the instant the Graviton migration completed. Requests
// before this are amd64; at and after, arm64.
func (c Config) MigrationStart() time.Time {
	return c.Now.Add(-c.MigrationAgo)
}

// Request is one generated request, fully resolved.
type Request struct {
	Start     time.Time
	QueryText string
	UserType  string
	Arch      string
	Duration  time.Duration
}

// Generate produces the dataset, in emission order.
func Generate(cfg Config, rng *rand.Rand) []Request {
	migrationStart := cfg.MigrationStart()
	out := make([]Request, 0, cfg.RequestCount)

	userTypes, userCum := cumulativeShares(cfg.UserTypeShares)

	for i := 0; i < cfg.RequestCount; i++ {
		start := cfg.Now.Add(-time.Duration(rng.Int63n(int64(cfg.Window))))
		tmpl := cfg.Templates[rng.Intn(len(cfg.Templates))]
		userType := pick(userTypes, userCum, rng)

		duration := durationFor(tmpl, rng)
		if userType == "enterprise" {
			duration = time.Duration(float64(duration) * cfg.EnterpriseMultiplier)
		}

		arch := "amd64"
		if start.After(migrationStart) || start.Equal(migrationStart) {
			arch = "arm64"
		}

		out = append(out, Request{
			Start:     start,
			QueryText: tmpl.Text,
			UserType:  userType,
			Arch:      arch,
			Duration:  duration,
		})
	}

	return out
}

func durationFor(tmpl QueryTemplate, rng *rand.Rand) time.Duration {
	switch tmpl.Kind {
	case KindBimodal:
		if rng.Float64() < tmpl.SlowShare {
			return jitter(rng, tmpl.SlowDuration, 0.1)
		}
		return jitter(rng, tmpl.Baseline, 0.1)
	case KindLongTail:
		// Exponential tail: most draws stay near Baseline, a continuous
		// minority stretch out to several multiples of it. rng.ExpFloat64()
		// has mean 1, so scaling it by Baseline keeps the tail's shape tied
		// to the template's own baseline rather than a fixed constant.
		return tmpl.Baseline + time.Duration(rng.ExpFloat64()*float64(tmpl.Baseline)*1.5)
	default:
		return jitter(rng, tmpl.Baseline, 0.15)
	}
}

func jitter(rng *rand.Rand, base time.Duration, spread float64) time.Duration {
	factor := 1 + spread*(2*rng.Float64()-1)
	d := time.Duration(float64(base) * factor)
	if d < 0 {
		return 0
	}
	return d
}

// cumulativeShares turns a share map into a stable-ordered slice of keys and
// their cumulative boundaries, so pick's selection is deterministic for a
// given rng sequence regardless of map iteration order. Sorted rather than
// hard-coded so a caller adding a user type to Config.UserTypeShares gets it
// included instead of silently dropped.
func cumulativeShares(shares map[string]float64) ([]string, []float64) {
	keys := make([]string, 0, len(shares))
	for k := range shares {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	cum := make([]float64, len(keys))
	var running float64
	for i, k := range keys {
		running += shares[k]
		cum[i] = running
	}
	return keys, cum
}

func pick(keys []string, cum []float64, rng *rand.Rand) string {
	r := rng.Float64()
	for i, c := range cum {
		if r < c {
			return keys[i]
		}
	}
	return keys[len(keys)-1]
}

// Percentile returns the p-th percentile (0-100) duration from a slice of
// requests. Exposed so both the package's own tests and the seeder's summary
// output can compute it the same way.
func Percentile(reqs []Request, p float64) time.Duration {
	if len(reqs) == 0 {
		return 0
	}
	durations := make([]float64, len(reqs))
	for i, r := range reqs {
		durations[i] = float64(r.Duration)
	}
	sort.Float64s(durations)
	idx := int(math.Ceil(p/100*float64(len(durations)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(durations) {
		idx = len(durations) - 1
	}
	return time.Duration(durations[idx])
}
