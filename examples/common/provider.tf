# Shared by every example directory (symlinked as provider.tf).
terraform {
  required_providers {
    kentik-synthetics = {
      source  = "kentik/kentik-synthetics"
      version = ">= 0.3.0"
    }
  }
}

# Credentials are read from KTAPI_AUTH_EMAIL and KTAPI_AUTH_TOKEN.
provider "kentik-synthetics" {}
