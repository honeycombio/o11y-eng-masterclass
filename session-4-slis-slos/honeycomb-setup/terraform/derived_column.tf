# The SLI itself. Chapter 11's HTTP-API formula is
#
#   count(status_code < 400) / count(*) where route != '/healthz'
#
# and the two halves of that stay separate on purpose: the derived column is
# just "was this one event good", and the route exclusion lives in each
# query's filter instead of being baked into the expression. That mirrors how
# a real Honeycomb SLO is built (a boolean SLI plus a query scope) and means
# the exact same column backs both the honeycombio_slo this would be if your
# plan included SLOs, and the AVG()-based workshop path below when it doesn't.
resource "honeycombio_derived_column" "sli_good_request" {
  dataset     = var.dataset
  alias       = "sli_good_request"
  expression  = "IF(LT($http.response.status_code, 400), 1, 0)"
  description = "1 for a good event, 0 for a bad one. AVG() of this over a filtered, scoped query is the SLI ratio."
}
