terraform {
  required_providers {
    dynu = {
      source  = "beatz174-bit/dynu"
      version = "~> 0.2.0"
    }
  }
}

provider "dynu" {}

data "dynu_domains" "all" {}

output "domains" {
  value = data.dynu_domains.all.domains
}
