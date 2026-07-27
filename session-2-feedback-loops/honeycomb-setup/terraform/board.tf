# The weekly observability review board from Chapter 24: a standing set of
# queries a team walks in 30 minutes. The point is that the ritual is
# low-friction because the queries are already saved — nobody has to remember
# how to build them while the meeting waits.
#
# These four also happen to be the demo's path: spot the shift, scope it to the
# build, scope it to the plan, then look at what broke.

# Every panel needs both a query and an annotation (the annotation is what gives
# the panel its title), so they come in pairs throughout.

# 1. Latency by build. The deploy marker plus this graph is the whole
#    "did the deploy do it?" question in one view.
data "honeycombio_query_specification" "latency_by_build" {
  time_range = var.review_window_seconds
  breakdowns = ["service.version"]

  calculation {
    op     = "P95"
    column = "duration_ms"
  }

  # Root spans only. Without this the percentile is computed across child spans
  # too, which drags it toward whatever the fastest step happens to be.
  filter {
    column = "trace.parent_id"
    op     = "does-not-exist"
  }
}

resource "honeycombio_query" "latency_by_build" {
  dataset    = var.dataset
  query_json = data.honeycombio_query_specification.latency_by_build.json
}

resource "honeycombio_query_annotation" "latency_by_build" {
  dataset     = var.dataset
  query_id    = honeycombio_query.latency_by_build.id
  name        = "P95 latency by build"
  description = "Did the deploy move latency? Compare ${var.baseline_version} against ${var.canary_version}."
}

# 2. Billing latency as a heatmap. This is the BubbleUp entry point: a heatmap
#    shows the distribution, so a second band of slow requests is visible even
#    when the percentile line barely moves.
data "honeycombio_query_specification" "billing_latency_heatmap" {
  time_range = var.review_window_seconds

  calculation {
    op     = "HEATMAP"
    column = "duration_ms"
  }

  filter {
    column = "http.route"
    op     = "="
    value  = "/api/billing"
  }

  filter {
    column = "trace.parent_id"
    op     = "does-not-exist"
  }
}

resource "honeycombio_query" "billing_latency_heatmap" {
  dataset    = var.dataset
  query_json = data.honeycombio_query_specification.billing_latency_heatmap.json
}

resource "honeycombio_query_annotation" "billing_latency_heatmap" {
  dataset     = var.dataset
  query_id    = honeycombio_query.billing_latency_heatmap.id
  name        = "Billing latency distribution"
  description = "Draw a BubbleUp box around the slow band. Start the core analysis loop here."
}

# 3. Latency by plan. Answers "for whom?" — the question incident responders
#    get asked first and can usually least easily answer.
data "honeycombio_query_specification" "latency_by_plan" {
  time_range = var.review_window_seconds
  breakdowns = ["user.type"]

  calculation {
    op     = "P95"
    column = "duration_ms"
  }

  filter {
    column = "http.route"
    op     = "="
    value  = "/api/billing"
  }

  filter {
    column = "trace.parent_id"
    op     = "does-not-exist"
  }
}

resource "honeycombio_query" "latency_by_plan" {
  dataset    = var.dataset
  query_json = data.honeycombio_query_specification.latency_by_plan.json
}

resource "honeycombio_query_annotation" "latency_by_plan" {
  dataset     = var.dataset
  query_id    = honeycombio_query.latency_by_plan.id
  name        = "Billing P95 by plan"
  description = "Which customers feel it. Enterprise diverging from free and premium is the signal."
}

# 4. Top error patterns, the third standing item in the weekly review.
data "honeycombio_query_specification" "errors_by_type" {
  time_range = var.review_window_seconds
  breakdowns = ["error.type"]

  calculation {
    op = "COUNT"
  }

  filter {
    column = "error"
    op     = "="
    value  = "true"
  }

  order {
    op    = "COUNT"
    order = "descending"
  }
}

resource "honeycombio_query" "errors_by_type" {
  dataset    = var.dataset
  query_json = data.honeycombio_query_specification.errors_by_type.json
}

resource "honeycombio_query_annotation" "errors_by_type" {
  dataset     = var.dataset
  query_id    = honeycombio_query.errors_by_type.id
  name        = "Top error patterns"
  description = "Standing weekly-review item: what is failing, and is the mix changing?"
}

resource "honeycombio_flexible_board" "weekly_review" {
  name        = "Weekly Observability Review"
  description = <<-EOT
    The 30-minute standing review from Chapter 24. Walk it top to bottom:
    did latency move, where is the distribution splitting, who feels it, and
    what is erroring.

    Managed by Terraform in session-2-feedback-loops/honeycomb-setup/terraform.
  EOT

  panel {
    type = "query"

    position {
      x_coordinate = 0
      y_coordinate = 0
      width        = 6
      height       = 4
    }

    query_panel {
      query_id            = honeycombio_query.latency_by_build.id
      query_annotation_id = honeycombio_query_annotation.latency_by_build.id
      query_style         = "combo"
    }
  }

  panel {
    type = "query"

    position {
      x_coordinate = 6
      y_coordinate = 0
      width        = 6
      height       = 4
    }

    query_panel {
      query_id            = honeycombio_query.billing_latency_heatmap.id
      query_annotation_id = honeycombio_query_annotation.billing_latency_heatmap.id
      query_style         = "graph"
    }
  }

  panel {
    type = "query"

    position {
      x_coordinate = 0
      y_coordinate = 4
      width        = 6
      height       = 4
    }

    query_panel {
      query_id            = honeycombio_query.latency_by_plan.id
      query_annotation_id = honeycombio_query_annotation.latency_by_plan.id
      query_style         = "combo"
    }
  }

  panel {
    type = "query"

    position {
      x_coordinate = 6
      y_coordinate = 4
      width        = 6
      height       = 4
    }

    query_panel {
      query_id            = honeycombio_query.errors_by_type.id
      query_annotation_id = honeycombio_query_annotation.errors_by_type.id
      query_style         = "graph"
    }
  }
}
