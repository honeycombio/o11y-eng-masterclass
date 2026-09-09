# The free-tier stand-in for a burn-alert: a plain threshold Trigger over the
# trailing-window SLI query, rather than a honeycombio_burn_alert against a
# real honeycombio_slo (neither resource is usable on Honeycomb Free — see
# ../../README.md). It approximates the SRE Workbook's fast-burn window (2% of
# budget in 1 hour), the shape Chapter 12 generalises as a relative burn
# alert, as "the trailing hour's success ratio dropped
# below burn_threshold" — real error-budget burn math, not a raw threshold,
# but close enough to demo the shape of the alert: it fires on the recent
# window, not on an average since the dawn of the dataset.
resource "honeycombio_trigger" "sli_burn" {
  name        = "SLI burn: sli-demo-service trailing window"
  description = "Fires when the trailing-window SLI (see burn_trigger_sli query) drops below the configured threshold. Free-tier stand-in for an SLO burn alert."
  dataset     = var.dataset
  # query_json rather than query_id pointing at honeycombio_query.burn_trigger_sli:
  # that resource still exists as a convenient saved-query bookmark for the demo,
  # but the trigger gets its own copy of the same specification directly, which
  # is the provider's documented recommendation over wiring triggers to a shared
  # query_id.
  query_json = data.honeycombio_query_specification.burn_trigger_sli.json

  # Below var.burn_threshold, checked every 5 minutes — frequent enough that a
  # live demo doesn't need to wait long after the incident starts to see it
  # fire, without being so frequent it evaluates on noise.
  threshold {
    op    = "<"
    value = var.burn_threshold
  }
  frequency = 300

  recipient {
    type   = var.trigger_recipient_type
    target = var.trigger_recipient_target
  }
}
