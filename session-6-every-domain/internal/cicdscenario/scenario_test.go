package cicdscenario

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

func generate(t *testing.T) (Config, []Run) {
	t.Helper()
	cfg := testConfig()
	return cfg, Generate(cfg, rand.New(rand.NewSource(7)))
}

func buildTask(r Run) Task {
	for _, task := range r.Tasks {
		if task.Name == "build" {
			return task
		}
	}
	panic("run has no build task")
}

// TestGenerate_RegressionIsAStepChange guards the package comment's central
// claim: build duration is flat at baseline, then flat at a higher plateau,
// with nothing in between. A gradual drift wouldn't demonstrate why a
// trailing-window trigger (rather than a slow-moving average) is the right
// tool.
func TestGenerate_RegressionIsAStepChange(t *testing.T) {
	cfg, runs := generate(t)

	const jitterMargin = 0.20 // matches jitter's 0.15 spread plus slack
	baselineHigh := time.Duration(float64(cfg.BaselineBuildDuration) * (1 + jitterMargin))
	regressedLow := time.Duration(float64(cfg.RegressedBuildDuration) * (1 - jitterMargin))

	if baselineHigh >= regressedLow {
		t.Fatalf("baseline jitter ceiling (%s) overlaps regressed jitter floor (%s); the step change would not be visible", baselineHigh, regressedLow)
	}

	for _, r := range runs {
		d := buildTask(r).Duration
		if r.Regressed {
			if d < regressedLow {
				t.Errorf("regressed run %s build duration %s is below the regressed floor %s", r.RunID, d, regressedLow)
			}
		} else if d > baselineHigh {
			t.Errorf("baseline run %s build duration %s is above the baseline ceiling %s", r.RunID, d, baselineHigh)
		}
	}
}

// TestGenerate_RegressionPopulationIsDemoSized guards that enough runs fall
// after the regression start for a trailing-window P95 to be computed from,
// not just a handful of points.
func TestGenerate_RegressionPopulationIsDemoSized(t *testing.T) {
	_, runs := generate(t)

	var regressed int
	for _, r := range runs {
		if r.Regressed {
			regressed++
		}
	}
	if regressed < 10 {
		t.Fatalf("only %d regressed runs; too few for a trailing-window P95 to be meaningful", regressed)
	}
}

// TestGenerate_FlakyTestFailureRateMatchesConfig checks the flaky test's
// observed failure rate independent of the build regression — the package
// comment's claim that these are two separate stories, not one.
func TestGenerate_FlakyTestFailureRateMatchesConfig(t *testing.T) {
	cfg, runs := generate(t)

	var failed int
	for _, r := range runs {
		for _, task := range r.Tasks {
			if task.Name != "test" {
				continue
			}
			for _, tc := range task.Tests {
				if tc.Name == FlakyTestName && tc.Result == ResultFailure {
					failed++
				}
			}
		}
	}

	got := float64(failed) / float64(len(runs))
	if diff := got - cfg.FlakyTestFailureRate; diff > 0.05 || diff < -0.05 {
		t.Errorf("flaky test failure rate %.3f, want close to configured %.3f", got, cfg.FlakyTestFailureRate)
	}
}

// TestGenerate_OnlyTheFlakyTestEverFails guards the package comment's other
// claim: every non-flaky test case always passes, so a flakiness query
// against this dataset has exactly one real answer.
func TestGenerate_OnlyTheFlakyTestEverFails(t *testing.T) {
	_, runs := generate(t)

	for _, r := range runs {
		for _, task := range r.Tasks {
			if task.Name != "test" {
				continue
			}
			for _, tc := range task.Tests {
				if tc.Name != FlakyTestName && tc.Result == ResultFailure {
					t.Fatalf("test case %q failed in run %s; only %s should ever fail", tc.Name, r.RunID, FlakyTestName)
				}
			}
		}
	}
}

// TestGenerate_RunResultReflectsTestFailure checks the roll-up: a run's
// overall Result is failure exactly when its test task's Result is failure,
// which is what makes cicd.pipeline.result a meaningful filter.
func TestGenerate_RunResultReflectsTestFailure(t *testing.T) {
	_, runs := generate(t)

	for _, r := range runs {
		var testResult TaskResult
		for _, task := range r.Tasks {
			if task.Name == "test" {
				testResult = task.Result
			}
		}
		if r.Result != testResult {
			t.Errorf("run %s: overall result %q, want to match test task result %q", r.RunID, r.Result, testResult)
		}
	}
}

// TestGenerate_RunCountExceedsQueueMargin guards the delivery test's
// precondition: 1 root + 4 tasks + TestCount test-case spans per run must
// clear the SDK's default 2048-span queue with margin at the default
// RunCount.
func TestGenerate_RunCountExceedsQueueMargin(t *testing.T) {
	cfg := DefaultConfig()
	const defaultQueueSize = 2048

	spansPerRun := 1 + 4 + cfg.TestCount
	total := cfg.RunCount * spansPerRun
	if total <= defaultQueueSize*2 {
		t.Fatalf("default config emits %d spans, not comfortably past the %d-span default queue; raise RunCount", total, defaultQueueSize)
	}
}
