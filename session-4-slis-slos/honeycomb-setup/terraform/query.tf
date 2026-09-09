# Three saved queries over the same derived column, all excluding /healthz
# the same way — see derived_column.tf for why that filter lives here rather
# than in the expression:
#
#  - baseline_sli: the steady-state view, the workshop's stand-in for a real
#    SLO's 30-day baseline.
#  - burn_trigger_sli: a single trailing-window number, deliberately with no
#    breakdown. This is what the Trigger below watches — a trigger needs one
#    unambiguous value to threshold on, and keeping it separate from the
#    breakdown query means there's no question about which one the alert is
#    actually evaluating.
#  - burn_breakdown_sli: the same trailing window, broken down by
#    service.version and user.type. This is the investigative query for the
#    "drill in" moment — the worst row is already visible without a separate
#    BubbleUp pass, though drawing a BubbleUp box around the errors here still
#    works and is worth demoing too.

data "honeycombio_query_specification" "baseline_sli" {
  time_range = var.baseline_window_seconds

  calculation {
    op     = "AVG"
    column = honeycombio_derived_column.sli_good_request.alias
  }

  filter {
    column = "http.route"
    op     = "!="
    value  = "/healthz"
  }
}

resource "honeycombio_query" "baseline_sli" {
  dataset    = var.dataset
  query_json = data.honeycombio_query_specification.baseline_sli.json
}

resource "honeycombio_query_annotation" "baseline_sli" {
  dataset     = var.dataset
  query_id    = honeycombio_query.baseline_sli.id
  name        = "SLI: AVG success ratio (steady-state window)"
  description = "The free-tier stand-in for an SLO's baseline: AVG(${honeycombio_derived_column.sli_good_request.alias}), route != /healthz."
}

data "honeycombio_query_specification" "burn_trigger_sli" {
  time_range = var.burn_window_seconds

  calculation {
    op     = "AVG"
    column = honeycombio_derived_column.sli_good_request.alias
  }

  filter {
    column = "http.route"
    op     = "!="
    value  = "/healthz"
  }
}

resource "honeycombio_query" "burn_trigger_sli" {
  dataset    = var.dataset
  query_json = data.honeycombio_query_specification.burn_trigger_sli.json
}

resource "honeycombio_query_annotation" "burn_trigger_sli" {
  dataset     = var.dataset
  query_id    = honeycombio_query.burn_trigger_sli.id
  name        = "SLI: AVG success ratio, trailing burn window"
  description = "What the trigger in trigger.tf watches. No breakdown, deliberately: one unambiguous number to threshold on."
}

data "honeycombio_query_specification" "burn_breakdown_sli" {
  time_range = var.burn_window_seconds
  breakdowns = ["service.version", "user.type"]

  calculation {
    op     = "AVG"
    column = honeycombio_derived_column.sli_good_request.alias
  }

  filter {
    column = "http.route"
    op     = "!="
    value  = "/healthz"
  }

  order {
    column = honeycombio_derived_column.sli_good_request.alias
    op     = "AVG"
    order  = "ascending"
  }
}

resource "honeycombio_query" "burn_breakdown_sli" {
  dataset    = var.dataset
  query_json = data.honeycombio_query_specification.burn_breakdown_sli.json
}

resource "honeycombio_query_annotation" "burn_breakdown_sli" {
  dataset     = var.dataset
  query_id    = honeycombio_query.burn_breakdown_sli.id
  name        = "SLI: AVG success ratio, trailing burn window, by version/plan"
  description = "Click into this once the trigger fires. Sorted ascending, so the worst combination of service.version and user.type is the first row."
}
