# Four saved queries against the perf-service dataset, one per step of
# Chapter 20's workflow. host.arch is queryable directly, so the before/after
# migration comparison filters on that attribute rather than needing to know
# the migration's exact timestamp — only the marker (marker_perf.tf) needs
# perf_migration_start.

# Step 1: find expensive operations.
data "honeycombio_query_specification" "perf_slow_queries" {
  time_range = var.perf_query_window_seconds

  breakdowns = ["db.query.text"]

  calculation {
    op     = "P99"
    column = "duration_ms"
  }

  order {
    op     = "P99"
    column = "duration_ms"
    order  = "descending"
  }
}

resource "honeycombio_query" "perf_slow_queries" {
  dataset    = var.perf_dataset
  query_json = data.honeycombio_query_specification.perf_slow_queries.json
}

resource "honeycombio_query_annotation" "perf_slow_queries" {
  dataset     = var.perf_dataset
  query_id    = honeycombio_query.perf_slow_queries.id
  name        = "Perf: P99 duration by query"
  description = "Chapter 20 step 1. The bimodal query (orders WHERE status) and the long-tail query (audit_log) both rank above the three fast queries — but a single P99 number can't tell you which kind of problem each one is. That's what the next query is for."
}

# Step 2: characterise the distribution. Filtered to the bimodal query found
# in step 1 — a heatmap doesn't mean much broken down by query text at the
# same time, so this looks at one expensive query's own shape.
data "honeycombio_query_specification" "perf_query_heatmap" {
  time_range = var.perf_query_window_seconds

  calculation {
    op     = "HEATMAP"
    column = "duration_ms"
  }

  filter {
    column = "db.query.text"
    op     = "="
    value  = "SELECT * FROM orders WHERE status = $1"
  }
}

resource "honeycombio_query" "perf_query_heatmap" {
  dataset    = var.perf_dataset
  query_json = data.honeycombio_query_specification.perf_query_heatmap.json
}

resource "honeycombio_query_annotation" "perf_query_heatmap" {
  dataset     = var.perf_dataset
  query_id    = honeycombio_query.perf_query_heatmap.id
  name        = "Perf: duration heatmap, orders-by-status query"
  description = "Chapter 20 step 2. Two clean bands, not a smear — a missing index on one status value, not a generally slow query. Compare against the audit_log query's heatmap, which spreads continuously instead."
}

# Step 3: correlate with attributes.
data "honeycombio_query_specification" "perf_duration_by_user_type" {
  time_range = var.perf_query_window_seconds
  breakdowns = ["user.type"]

  calculation {
    op     = "AVG"
    column = "duration_ms"
  }

  order {
    op     = "AVG"
    column = "duration_ms"
    order  = "descending"
  }
}

resource "honeycombio_query" "perf_duration_by_user_type" {
  dataset    = var.perf_dataset
  query_json = data.honeycombio_query_specification.perf_duration_by_user_type.json
}

resource "honeycombio_query_annotation" "perf_duration_by_user_type" {
  dataset     = var.perf_dataset
  query_id    = honeycombio_query.perf_duration_by_user_type.id
  name        = "Perf: AVG duration by user.type"
  description = "Chapter 20 step 3. Enterprise accounts run measurably slower — larger accounts, more data per request."
}

# Step 4: track optimisation impact across the Graviton migration. Two
# queries, one per arch, both P50 and P95 so the comparison in the seeder's
# own printed summary can be reproduced in Honeycomb directly.
data "honeycombio_query_specification" "perf_duration_amd64" {
  time_range = var.perf_query_window_seconds

  calculation {
    op     = "P50"
    column = "duration_ms"
  }
  calculation {
    op     = "P95"
    column = "duration_ms"
  }

  filter {
    column = "host.arch"
    op     = "="
    value  = "amd64"
  }
}

resource "honeycombio_query" "perf_duration_amd64" {
  dataset    = var.perf_dataset
  query_json = data.honeycombio_query_specification.perf_duration_amd64.json
}

resource "honeycombio_query_annotation" "perf_duration_amd64" {
  dataset     = var.perf_dataset
  query_id    = honeycombio_query.perf_duration_amd64.id
  name        = "Perf: P50/P95 duration, amd64 (pre-migration)"
  description = "Chapter 20 step 4, before. Compare against the arm64 query below — the whole point is that these two should look the same."
}

data "honeycombio_query_specification" "perf_duration_arm64" {
  time_range = var.perf_query_window_seconds

  calculation {
    op     = "P50"
    column = "duration_ms"
  }
  calculation {
    op     = "P95"
    column = "duration_ms"
  }

  filter {
    column = "host.arch"
    op     = "="
    value  = "arm64"
  }
}

resource "honeycombio_query" "perf_duration_arm64" {
  dataset    = var.perf_dataset
  query_json = data.honeycombio_query_specification.perf_duration_arm64.json
}

resource "honeycombio_query_annotation" "perf_duration_arm64" {
  dataset     = var.perf_dataset
  query_id    = honeycombio_query.perf_duration_arm64.id
  name        = "Perf: P50/P95 duration, arm64 (post-migration)"
  description = "Chapter 20 step 4, after. No regression is the finding, not the assumption — see internal/perfscenario's package comment for how the seeded data backs that up honestly."
}
