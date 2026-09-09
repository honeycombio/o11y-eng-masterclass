package cicdscenario

import (
	"fmt"
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

// TestGenerate_BranchSplitMatchesConfig checks the trunk-versus-PR split
// lands near MainBranchShare and that both cohorts are populated. The
// breakdown query in honeycomb-setup needs two rows to make Chapter 18's
// blast-radius point; one empty cohort would quietly reduce it to a single
// number.
func TestGenerate_BranchSplitMatchesConfig(t *testing.T) {
	cfg, runs := generate(t)

	var onTrunk int
	for _, r := range runs {
		if r.Branch == MainBranchName {
			onTrunk++
		}
	}
	prRuns := len(runs) - onTrunk

	if onTrunk == 0 || prRuns == 0 {
		t.Fatalf("branch cohorts are %d on trunk and %d on PR branches; the breakdown demo needs both", onTrunk, prRuns)
	}

	got := float64(onTrunk) / float64(len(runs))
	const tolerance = 0.05
	if diff := got - cfg.MainBranchShare; diff < -tolerance || diff > tolerance {
		t.Errorf("trunk share = %.3f, want %.3f ±%.2f", got, cfg.MainBranchShare, tolerance)
	}
}

// TestGenerate_BranchNameTracksPRNumber asserts vcs.ref.head.name and
// vcs.change.id stay consistent: a PR run's branch is derived from its own PR
// number, and a trunk run still carries the PR number that landed, so the two
// attributes never contradict each other within a trace.
func TestGenerate_BranchNameTracksPRNumber(t *testing.T) {
	_, runs := generate(t)

	for _, r := range runs {
		if r.PRNumber == 0 {
			t.Errorf("run %s: no PR number; every run traces back to one", r.RunID)
		}
		if r.Branch == MainBranchName {
			continue
		}
		if want := fmt.Sprintf("pr-%d", r.PRNumber); r.Branch != want {
			t.Errorf("run %s: branch %q, want %q", r.RunID, r.Branch, want)
		}
	}
}

// TestGenerate_FlakyTestFailsOnBothBranches guards the reason the split
// exists at all. The flaky test is branch-independent, so it has to fail on
// trunk as well as on PR branches — that is what makes "same test, different
// blast radius" a claim about ownership rather than about one unlucky branch.
func TestGenerate_FlakyTestFailsOnBothBranches(t *testing.T) {
	_, runs := generate(t)

	failures := map[bool]int{}
	for _, r := range runs {
		for _, task := range r.Tasks {
			for _, tc := range task.Tests {
				if tc.Name == FlakyTestName && tc.Result == ResultFailure {
					failures[r.Branch == MainBranchName]++
				}
			}
		}
	}

	if failures[true] == 0 || failures[false] == 0 {
		t.Fatalf("flaky-test failures: %d on trunk, %d on PR branches; both must be non-zero", failures[true], failures[false])
	}
}
