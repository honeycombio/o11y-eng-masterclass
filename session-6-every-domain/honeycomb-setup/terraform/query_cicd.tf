# Two saved queries plus a trigger, all against the cicd-pipeline dataset.
#
#  - cicd_build_p95: P95(duration_ms) of the build task, trailing window,
#    no breakdown. What the trigger below watches — same "one unambiguous
#    number" reasoning as session 4's burn_trigger_sli.
#  - cicd_flaky_tests: COUNT of test-case spans with error = true, broken
#    down by test.name. Chapter 18's "one span per test case to query
#    flakiness" made concrete: the flaky test case (see
#    internal/cicdscenario.FlakyTestName) is the only row that ever appears.

data "honeycombio_query_specification" "cicd_build_p95" {
  time_range = var.cicd_burn_window_seconds

  calculation {
    op     = "P95"
    column = "duration_ms"
  }

  filter {
    column = "cicd.pipeline.task.name"
    op     = "="
    value  = "build"
  }
}

resource "honeycombio_query" "cicd_build_p95" {
  dataset    = var.cicd_dataset
  query_json = data.honeycombio_query_specification.cicd_build_p95.json
}

resource "honeycombio_query_annotation" "cicd_build_p95" {
  dataset     = var.cicd_dataset
  query_id    = honeycombio_query.cicd_build_p95.id
  name        = "CI/CD: build task P95 duration, trailing window"
  description = "What the build-duration trigger watches. Compare against the ~12m baseline / ~22m regressed durations in internal/cicdscenario."
}

data "honeycombio_query_specification" "cicd_flaky_tests" {
  time_range = 14 * 24 * 3600 # 14d, matching the seeder's Window
  breakdowns = ["test.name"]

  calculation {
    op = "COUNT"
  }

  filter {
    column = "error"
    op     = "="
    value  = true
  }

  order {
    op = "COUNT"
  }
}

resource "honeycombio_query" "cicd_flaky_tests" {
  dataset    = var.cicd_dataset
  query_json = data.honeycombio_query_specification.cicd_flaky_tests.json
}

resource "honeycombio_query_annotation" "cicd_flaky_tests" {
  dataset     = var.cicd_dataset
  query_id    = honeycombio_query.cicd_flaky_tests.id
  name        = "CI/CD: failed test cases by name"
  description = "One span per test case, filtered to failures. The flaky test case is the only row — everything else always passes."
}

# The free-tier stand-in for a build-time alert: a plain threshold Trigger
# over the trailing-window P95 query, same shape as session 4's burn trigger.
resource "honeycombio_trigger" "cicd_build_p95" {
  name        = "CI/CD build regression: cicd-pipeline trailing window"
  description = "Fires when the trailing-window build-task P95 duration (see cicd_build_p95 query) exceeds the configured threshold."
  dataset     = var.cicd_dataset
  # query_json rather than query_id pointing at the saved query above, same
  # reasoning as session 4: the provider's documented recommendation is a
  # trigger's own copy of the specification, not a shared query_id.
  query_json = data.honeycombio_query_specification.cicd_build_p95.json

  threshold {
    op    = ">"
    value = var.cicd_build_p95_threshold_ms
  }
  frequency = 300 # 5m, same cadence as session 4's trigger

  recipient {
    type   = var.cicd_trigger_recipient_type
    target = var.cicd_trigger_recipient_target
  }
}
