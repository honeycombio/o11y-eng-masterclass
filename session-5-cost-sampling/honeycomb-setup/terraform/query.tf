# Two saved queries, both plain COUNT() with no derived column — unlike
# session 4, there's no boolean SLI to compute here. The whole demo is that
# Honeycomb's ingest already weights COUNT() by the sample rate the
# Collector's adaptive_tail_sampling processor recorded (as W3C tracestate
# ot=th:<hex>, per https://github.com/honeycombio/husky), so a query against
# the physically-smaller stored dataset reconstructs the real traffic volume
# without the seeder or this Terraform doing any weighting math themselves.
#
#  - cost_sampling_population: total reconstructed request count, no
#    breakdown. Compare this to the seeder's own printed "seeded N requests"
#    — they should be close, even though only a fraction of that N was
#    physically stored.
#  - cost_sampling_by_route: the same COUNT(), broken down by http.route.
#    This is the per-key half of the demo: the reconstructed shares across
#    /api/search, /api/checkout, /api/refund, /api/admin/report should still
#    look like the seeded 85/10/4/1 split, including the 1% route, because
#    the Collector's adaptive_percentage sampler fingerprints on route rather
#    than applying one global rate.

data "honeycombio_query_specification" "cost_sampling_population" {
  time_range = var.query_window_seconds

  calculation {
    op = "COUNT"
  }
}

resource "honeycombio_query" "cost_sampling_population" {
  dataset    = var.dataset
  query_json = data.honeycombio_query_specification.cost_sampling_population.json
}

resource "honeycombio_query_annotation" "cost_sampling_population" {
  dataset     = var.dataset
  query_id    = honeycombio_query.cost_sampling_population.id
  name        = "Cost/sampling: reconstructed request count"
  description = "Weighted COUNT() over the sampled dataset. Compare to the seeder's own printed request count."
}

data "honeycombio_query_specification" "cost_sampling_by_route" {
  time_range = var.query_window_seconds
  breakdowns = ["http.route"]

  calculation {
    op = "COUNT"
  }

  order {
    op = "COUNT"
  }
}

resource "honeycombio_query" "cost_sampling_by_route" {
  dataset    = var.dataset
  query_json = data.honeycombio_query_specification.cost_sampling_by_route.json
}

resource "honeycombio_query_annotation" "cost_sampling_by_route" {
  dataset     = var.dataset
  query_id    = honeycombio_query.cost_sampling_by_route.id
  name        = "Cost/sampling: reconstructed count by route"
  description = "The per-key half of the demo: even /api/admin/report's 1% share should still show up close to its seeded proportion."
}
