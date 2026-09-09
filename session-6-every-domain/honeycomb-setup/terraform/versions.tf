terraform {
  required_version = ">= 1.5"

  required_providers {
    honeycombio = {
      source  = "honeycombio/honeycombio"
      version = "~> 0.52"
    }
  }
}

provider "honeycombio" {
  # Auth comes from the environment, so no secrets live in this repo:
  #   HONEYCOMB_API_KEY                      (v1 configuration key), or
  #   HONEYCOMB_KEY_ID + HONEYCOMB_KEY_SECRET (v2 key pair)
  #
  # The key needs permission to manage derived columns, queries, triggers,
  # and markers. The send-events key the Collector uses is a different key
  # and will not work.
  #
  # Honeycomb EU: set api_url here, or export
  #   HONEYCOMB_API_ENDPOINT=https://api.eu1.honeycomb.io
  #
  # This directory covers all three of session 6's demos (CI/CD, performance,
  # LLM) across three separate datasets — see the *_cicd.tf, *_perf.tf, and
  # *_llm.tf files. Every resource here works on Honeycomb Free.
  features {}
}
