# MC4's exact SLI shape, reused: a boolean derived column plus AVG() over
# it. See session-4-slis-slos/honeycomb-setup/terraform/derived_column.tf
# for the original — this is the same pattern against
# llm.response.quality_score instead of http.response.status_code, proving
# the slide's formula (count(llm.response.quality_score > 0.7) / count(*))
# is the same SLI machinery MC4 already taught, not a new mechanism.
resource "honeycombio_derived_column" "llm_sli_good_response" {
  dataset     = var.llm_eval_dataset
  alias       = "sli_good_response"
  expression  = "IF(GT($llm.response.quality_score, 0.7), 1, 0)"
  description = "1 for a good response, 0 for a quality-regressed one. AVG() of this is the SLI ratio."
}
