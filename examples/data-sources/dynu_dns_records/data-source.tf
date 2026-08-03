terraform {
  required_providers {
    dynu = {
      source  = "beatz174-bit/dynu"
      version = "~> 0.2.0"
    }
  }
}

provider "dynu" {}

data "dynu_dns_records" "records" {
  hostname = "www.example.com"
}

output "records" {
  value = data.dynu_dns_records.records.records
}
