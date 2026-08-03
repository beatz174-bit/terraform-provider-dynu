# dynu_domain Resource

Manages a Dynu DNS root domain.

## Example Usage

```terraform
resource "dynu_domain" "example" {
  name         = "my-test-domain.example"
  ipv4_address = "203.0.113.10"
  ttl          = 300
}

resource "dynu_dns_record" "www" {
  hostname    = "www.${dynu_domain.example.name}"
  record_type = "A"
  content     = "203.0.113.20"
}
```

## Schema

### Required

- `name` (String) Root domain name. Changing this forces replacement.

### Optional + Computed

- `ipv4_address` (String)
- `ipv6_address` (String)
- `ttl` (Number)
- `group` (String)

These fields can be configured, and also reflect values returned by Dynu.

### Computed

- `id` (Number) Dynu numeric domain ID.
- `state` (String) Dynu state.
- `token` (String, Sensitive) Dynu domain token.
- `unicode_name` (String) Unicode representation of the domain name.
- `ipv4` (Boolean) Whether IPv4 support is enabled.
- `ipv6` (Boolean) Whether IPv6 support is enabled.
- `ipv4_wildcard_alias` (Boolean) Whether IPv4 wildcard alias is enabled.
- `ipv6_wildcard_alias` (Boolean) Whether IPv6 wildcard alias is enabled.
- `allow_zone_transfer` (Boolean) Whether zone transfer is allowed.
- `dnssec` (Boolean) Whether DNSSEC is enabled.
- `created_on` (String) Creation timestamp as returned by Dynu.
- `updated_on` (String) Last update timestamp as returned by Dynu.

## Import

Import using the numeric Dynu domain ID:

```bash
terraform import dynu_domain.example 1234
```

## Warning

Deleting this resource deletes the entire Dynu DNS zone for that domain.
