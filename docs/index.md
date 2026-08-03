---
page_title: "dynu Provider"
description: |-
  Terraform provider for Dynu DNS domains and DNS records (full CRUD) plus lookup data sources.
---

# dynu Provider

The `dynu` provider manages Dynu root domains and DNS records, and supports lookup data sources for discovery workflows.
This provider repository is standalone and does not require any external companion repository or helper script.

## Example Usage

```terraform
terraform {
  required_providers {
    dynu = {
      source  = "beatz174-bit/dynu"
      version = "~> 0.4.0"
    }
  }
}

provider "dynu" {
  api_key = var.dynu_api_key
}
```

## Schema

### Optional

- `api_key` (String, Sensitive) Dynu API key. Configure via provider argument (for example with `var.dynu_api_key` from `terraform.tfvars`).
- `base_url` (String) Override API base URL. Primarily intended for automated tests.
