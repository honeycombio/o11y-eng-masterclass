// Package cicdscenario generates the Masterclass 6 CI/CD pipeline dataset.
//
// # The scenario
//
// One pipeline, masterclass-app-ci, run many times over a two-week window.
// Each run is a trace: a root pipeline span, four child task spans (checkout,
// build, test, deploy, in that order), and the test task fans out one child
// span per test case — Chapter 18's "one span per test case to query
// flakiness and duration." Attributes follow OTel's own CI/CD semantic
// conventions (cicd.pipeline.name, cicd.pipeline.task.name,
// cicd.pipeline.task.run.result, vcs.change.id for the PR number) rather than
// inventing names, per Chapter 6's own instrumentation checklist. Test-case
// spans divide the test task's own duration evenly across however many test
// cases ran; their per-span duration is a placeholder to make the span tile
// its parent, not an independent signal — only FlakyTestName's pass/fail
// result carries a real story, so don't query test-case durations for
// outliers.
//
// # The regression
//
// The build task's duration is flat at a baseline (~12 minutes, with jitter)
// for the whole window, except for runs after RegressionAgo, which jump to a
// regressed duration (~22 minutes) — the curriculum's "a build-time
// regression from 12 to 22 minutes caught on day one." That step change,
// not a gradual drift, is what a P95-duration trigger on a trailing window
// catches quickly; see TestGenerate_RegressionIsAStepChange.
//
// # The flaky test
//
// Exactly one test case (FlakyTestName) fails at FlakyTestFailureRate on every
// run, independent of the build regression — a separate story from the
// duration regression, on purpose. Chapter 18 treats flakiness and duration
// as two different things build observability answers, and conflating them
// into one seeded failure would only teach one lesson instead of two. Every
// other test case always passes.
//
// # Delivery test sizing
//
// Spans per run: 1 (pipeline root) + 4 (tasks) + TestCount (one per test
// case). At the defaults (20 tests), that's 25 spans/run — RunCount*25 must
// clear the SDK's default 2048-span queue with margin; see the seeder's
// maxQueueSize and TestGenerate_RunCountExceedsQueueMargin.
package cicdscenario

import (
	"fmt"
	"math/rand"
	"time"
)

// FlakyTestName is the one test case that fails intermittently. Named so the
// seeder and any query built against this dataset can refer to it by a
// literal rather than an index.
const FlakyTestName = "TestPaymentRetryIsIdempotent"

// Config describes one generated dataset.
type Config struct {
	// RunCount is the number of pipeline runs (traces) to generate.
	RunCount int
	// Window is how far back from Now the runs are spread.
	Window time.Duration
	// RegressionAgo is how long before Now the build regression started. It
	// is still ongoing at Now, same reasoning as session 4's IncidentAgo:
	// a trailing-window trigger needs something current to find.
	RegressionAgo time.Duration
	// Now anchors the dataset.
	Now time.Time

	TestCount              int
	FlakyTestFailureRate   float64
	BaselineBuildDuration  time.Duration
	RegressedBuildDuration time.Duration
}

// DefaultConfig returns the tuned demo configuration described in the
// package comment. Now must still be set by the caller.
func DefaultConfig() Config {
	return Config{
		RunCount:               500,
		Window:                 14 * 24 * time.Hour,
		RegressionAgo:          20 * time.Hour,
		TestCount:              20,
		FlakyTestFailureRate:   0.15,
		BaselineBuildDuration:  12 * time.Minute,
		RegressedBuildDuration: 22 * time.Minute,
	}
}

// RegressionStart is the instant the build regression began.
func (c Config) RegressionStart() time.Time {
	return c.Now.Add(-c.RegressionAgo)
}

// TaskResult is the outcome of one task or test-case span.
type TaskResult string

const (
	ResultSuccess TaskResult = "success"
	ResultFailure TaskResult = "failure"
)

// TestCase is one test-case span within a run's test task.
type TestCase struct {
	Name   string
	Result TaskResult
}

// Task is one task span within a run.
type Task struct {
	Name     string
	Duration time.Duration
	Result   TaskResult
	// Tests is non-empty only for the "test" task.
	Tests []TestCase
}

// Run is one pipeline run (one trace), fully resolved.
type Run struct {
	Start time.Time
	RunID string
	// PRNumber is the vcs.change.id this run was triggered by.
	PRNumber int
	// Regressed is true if this run falls after RegressionStart, so its
	// build task uses RegressedBuildDuration. Exposed so tests can assert on
	// the population rather than re-deriving the rule.
	Regressed bool
	Tasks     []Task
	Result    TaskResult
}

// Generate produces the dataset, in emission order.
func Generate(cfg Config, rng *rand.Rand) []Run {
	regressionStart := cfg.RegressionStart()
	out := make([]Run, 0, cfg.RunCount)

	for i := 0; i < cfg.RunCount; i++ {
		start := cfg.Now.Add(-time.Duration(rng.Int63n(int64(cfg.Window))))
		regressed := start.After(regressionStart)

		buildDuration := jitter(rng, cfg.BaselineBuildDuration, 0.15)
		if regressed {
			buildDuration = jitter(rng, cfg.RegressedBuildDuration, 0.15)
		}

		tests := make([]TestCase, 0, cfg.TestCount)
		testTaskFailed := false
		for t := 0; t < cfg.TestCount; t++ {
			name := fmt.Sprintf("TestCase%02d", t)
			result := ResultSuccess
			if t == 0 {
				name = FlakyTestName
				if rng.Float64() < cfg.FlakyTestFailureRate {
					result = ResultFailure
					testTaskFailed = true
				}
			}
			tests = append(tests, TestCase{Name: name, Result: result})
		}
		testResult := ResultSuccess
		if testTaskFailed {
			testResult = ResultFailure
		}

		tasks := []Task{
			{Name: "checkout", Duration: jitter(rng, 8*time.Second, 0.2), Result: ResultSuccess},
			{Name: "build", Duration: buildDuration, Result: ResultSuccess},
			{Name: "test", Duration: jitter(rng, 2*time.Minute, 0.2), Result: testResult, Tests: tests},
			{Name: "deploy", Duration: jitter(rng, 30*time.Second, 0.2), Result: ResultSuccess},
		}

		runResult := ResultSuccess
		if testTaskFailed {
			runResult = ResultFailure
		}

		out = append(out, Run{
			Start:     start,
			RunID:     fmt.Sprintf("run-%06d", i),
			PRNumber:  1000 + rng.Intn(4000),
			Regressed: regressed,
			Tasks:     tasks,
			Result:    runResult,
		})
	}

	return out
}

func jitter(rng *rand.Rand, base time.Duration, spread float64) time.Duration {
	factor := 1 + spread*(2*rng.Float64()-1)
	return time.Duration(float64(base) * factor)
}
