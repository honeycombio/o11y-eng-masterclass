output "llm_sli_query_id" {
  value       = honeycombio_query.llm_sli.id
  description = "Saved query: 'LLM: quality SLI (MC4's formula, reused)'."
}
