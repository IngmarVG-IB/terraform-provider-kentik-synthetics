terraform {
  required_providers {
    kentik-synthetics = {
      source  = "kentik/kentik-synthetics"
      version = ">= 0.3.0"
    }
  }
}

provider "kentik-synthetics" {
  # Required. Can also be set with KTAPI_AUTH_EMAIL / KTAPI_AUTH_TOKEN; prefer the environment for the token.
  email = "john@example.com"
  token = var.kentik_token

  # Optional. US cluster (grpc.api.kentik.com:443) by default; "EU" selects grpc.api.kentik.eu:443.
  region = "US"

  # Optional retry of transient API errors (UNAVAILABLE, RESOURCE_EXHAUSTED).
  retry {
    max_attempts = 5
    min_delay    = "1s"
    max_delay    = "30s"
  }
}

variable "kentik_token" {
  type      = string
  sensitive = true
}
