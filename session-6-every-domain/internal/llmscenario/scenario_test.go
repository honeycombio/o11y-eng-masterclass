package llmscenario

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

func generate(t *testing.T) (Config, []Conversation) {
	t.Helper()
	cfg := testConfig()
	return cfg, Generate(cfg, rand.New(rand.NewSource(7)))
}

// TestGenerate_QualityRegressionRateMatchesConfig guards the population
// split the whole package exists to demonstrate.
func TestGenerate_QualityRegressionRateMatchesConfig(t *testing.T) {
	cfg, convos := generate(t)

	var regressed int
	for _, c := range convos {
		if c.Regressed {
			regressed++
		}
	}
	got := float64(regressed) / float64(len(convos))
	if diff := got - cfg.QualityRegressionShare; diff > 0.03 || diff < -0.03 {
		t.Errorf("observed regression rate %.3f, want close to configured %.3f", got, cfg.QualityRegressionShare)
	}
}

// TestGenerate_QualityScoresFallInTheirDeclaredRanges checks there's no
// overlap between the good and regressed populations — MC4's SLI threshold
// (0.7) needs to fall cleanly between them for the formula to mean anything
// against this dataset.
func TestGenerate_QualityScoresFallInTheirDeclaredRanges(t *testing.T) {
	cfg, convos := generate(t)

	if cfg.QualityRegressedRange[1] >= 0.7 {
		t.Fatalf("QualityRegressedRange tops out at %.2f, which is not below MC4's 0.7 SLI threshold", cfg.QualityRegressedRange[1])
	}
	if cfg.QualityGoodRange[0] <= 0.7 {
		t.Fatalf("QualityGoodRange starts at %.2f, which is not above MC4's 0.7 SLI threshold", cfg.QualityGoodRange[0])
	}

	for _, c := range convos {
		if c.Regressed {
			if c.QualityScore < cfg.QualityRegressedRange[0] || c.QualityScore > cfg.QualityRegressedRange[1] {
				t.Fatalf("regressed conversation scored %.2f, outside %v", c.QualityScore, cfg.QualityRegressedRange)
			}
		} else if c.QualityScore < cfg.QualityGoodRange[0] || c.QualityScore > cfg.QualityGoodRange[1] {
			t.Fatalf("good conversation scored %.2f, outside %v", c.QualityScore, cfg.QualityGoodRange)
		}
	}
}

// TestGenerate_SLIFormulaHasSomethingToFind computes MC4's exact formula —
// count(quality_score > 0.7) / count(*) — against this package's output, so
// a drift in the proportions would show up here before it shows up live.
func TestGenerate_SLIFormulaHasSomethingToFind(t *testing.T) {
	_, convos := generate(t)

	var good int
	for _, c := range convos {
		if c.QualityScore > 0.7 {
			good++
		}
	}
	ratio := float64(good) / float64(len(convos))
	if ratio >= 0.99 || ratio <= 0.80 {
		t.Errorf("SLI ratio is %.3f, want comfortably below 1.0 (a real regression) but not so low it reads as a total outage", ratio)
	}
}

// TestGenerate_ConversationCountExceedsQueueMargin guards the delivery
// test's precondition, same rationale as every other seeder in this repo.
func TestGenerate_ConversationCountExceedsQueueMargin(t *testing.T) {
	cfg := DefaultConfig()
	const defaultQueueSize = 2048
	if cfg.ConversationCount <= defaultQueueSize*2 {
		t.Fatalf("default config emits %d conversations (one span each), not comfortably past the %d-span default queue; raise ConversationCount", cfg.ConversationCount, defaultQueueSize)
	}
}
