data "honeycombio_query_specification" "llm_sli" {
  time_range = var.llm_eval_query_window_seconds

  calculation {
    op     = "AVG"
    column = honeycombio_derived_column.llm_sli_good_response.alias
  }
}

resource "honeycombio_query" "llm_sli" {
  dataset    = var.llm_eval_dataset
  query_json = data.honeycombio_query_specification.llm_sli.json
}

resource "honeycombio_query_annotation" "llm_sli" {
  dataset     = var.llm_eval_dataset
  query_id    = honeycombio_query.llm_sli.id
  name        = "LLM: quality SLI (MC4's formula, reused)"
  description = "AVG(sli_good_response) over the eval-pipeline output. Below 1.0 means some share of conversations regressed on quality without ever looking broken from an infra perspective."
}
