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
  # The key needs permission to manage queries. The send-events key the
  # Collector uses is a different key and will not work.
  #
  # Honeycomb EU: set api_url here, or export
  #   HONEYCOMB_API_ENDPOINT=https://api.eu1.honeycomb.io
  #
  # Every resource in this directory works on Honeycomb Free — there's no
  # SLO/burn-alert-style gate here at all, unlike session 4. This demo is
  # just COUNT() queries against however Honeycomb's ingest weighted the
  # sampled data, which is Free-plan-native.
  features {}
}
