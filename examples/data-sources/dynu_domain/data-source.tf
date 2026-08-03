terraform {
  required_providers {
    dynu = {
      source  = "beatz174-bit/dynu"
      version = "~> 0.2.0"
    }
  }
}

provider "dynu" {}

data "dynu_domain" "selected" {
  hostname = "www.example.com"
}

output "domain" {
  value = data.dynu_domain.selected.domain
}
