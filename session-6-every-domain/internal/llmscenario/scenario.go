// Package llmscenario generates the Masterclass 6 LLM-eval supplement.
//
// # The scenario
//
// This is deliberately small. The live LLM demo runs on real Claude Code
// OTel telemetry (see ../../README.md) — real cost, real tokens, real
// multi-step spans (claude_code.interaction / .llm_request / .tool), all
// pointed at session-1's shared Collector. That data has no quality signal
// anywhere in it: Claude Code's telemetry has success/failure on tool
// execution, but nothing resembling a continuous eval score. So this
// package generates only the one thing real data can't provide: a batch of
// scored conversations, standing in for the output of an eval pipeline
// (Chapter 21's "the score comes from an eval pipeline" line), each
// carrying llm.response.quality_score — the exact attribute MC4's SLI
// formula (count(llm.response.quality_score > 0.7) / count(*)) names, so
// that formula runs unmodified against this dataset.
//
// # Why a regressed population, not just noise around one mean
//
// QualityRegressionShare of conversations score low
// (QualityRegressedRange) instead of high (QualityGoodRange), and nothing
// else about them differs — no error status, no latency change. Chapter
// 21's central claim is that success/failure can't capture quality; a
// dataset where quality was just Gaussian noise around a single mean
// wouldn't demonstrate a real regression for the SLI to catch, just
// measurement jitter.
//
// # Delivery test sizing
//
// One span per conversation — ConversationCount alone must clear the SDK's
// default 2048-span queue with margin; see the seeder's maxQueueSize and
// TestGenerate_ConversationCountExceedsQueueMargin.
package llmscenario

import (
	"math/rand"
	"time"
)

// Config describes one generated dataset.
type Config struct {
	ConversationCount int
	Window            time.Duration
	Now               time.Time

	// QualityRegressionShare is the fraction of conversations whose score
	// falls in QualityRegressedRange instead of QualityGoodRange.
	QualityRegressionShare float64
	QualityGoodRange       [2]float64
	QualityRegressedRange  [2]float64
}

// DefaultConfig returns the tuned demo configuration described in the
// package comment. Now must still be set by the caller.
func DefaultConfig() Config {
	return Config{
		ConversationCount:      5000,
		Window:                 24 * time.Hour,
		QualityRegressionShare: 0.12,
		QualityGoodRange:       [2]float64{0.75, 0.98},
		QualityRegressedRange:  [2]float64{0.30, 0.65},
	}
}

// Conversation is one generated, evaluated conversation.
type Conversation struct {
	Start time.Time
	// QualityScore is llm.response.quality_score.
	QualityScore float64
	// Regressed is exposed so tests can assert on the population rather
	// than re-deriving the threshold.
	Regressed bool
}

// Generate produces the dataset, in emission order.
func Generate(cfg Config, rng *rand.Rand) []Conversation {
	out := make([]Conversation, 0, cfg.ConversationCount)

	for i := 0; i < cfg.ConversationCount; i++ {
		start := cfg.Now.Add(-time.Duration(rng.Int63n(int64(cfg.Window))))

		regressed := rng.Float64() < cfg.QualityRegressionShare
		quality := uniform(rng, cfg.QualityGoodRange)
		if regressed {
			quality = uniform(rng, cfg.QualityRegressedRange)
		}

		out = append(out, Conversation{
			Start:        start,
			QualityScore: quality,
			Regressed:    regressed,
		})
	}

	return out
}

func uniform(rng *rand.Rand, r [2]float64) float64 {
	return r[0] + rng.Float64()*(r[1]-r[0])
}
