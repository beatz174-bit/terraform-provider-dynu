terraform {
  required_providers {
    dynu = {
      source  = "beatz174-bit/dynu"
#      version = "~> 0.4.0"
    }
  }
}

provider "dynu" {
  # Set var.dynu_api_key in terraform.tfvars.
  api_key = var.dynu_api_key
}

variable "dynu_api_key" {
  type      = string
  sensitive = true
}
