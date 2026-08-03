terraform {
  required_providers {
    dynu = {
      source  = "beatz174-bit/dynu"
      version = "~> 0.2.0"
    }
  }
}

provider "dynu" {
  api_key = var.dynu_api_key
}

resource "dynu_domain" "test" {
  name = var.test_domain
}

resource "dynu_dns_record" "a" {
  hostname    = "a.${var.test_domain}"
  record_type = "A"
  content     = "192.0.2.10"
  ttl         = var.test_ttl
  enabled     = true

  depends_on = [dynu_domain.test]
}

resource "dynu_dns_record" "aaaa" {
  hostname    = "aaaa.${var.test_domain}"
  record_type = "AAAA"
  content     = "2001:db8::10"
  ttl         = var.test_ttl
  enabled     = true

  depends_on = [dynu_domain.test]
}

resource "dynu_dns_record" "cname" {
  hostname    = "www.${var.test_domain}"
  record_type = "CNAME"
  content     = "target.${var.test_domain}"
  ttl         = var.test_ttl
  enabled     = true

  depends_on = [dynu_domain.test]
}

resource "dynu_dns_record" "mx" {
  hostname    = var.test_domain
  record_type = "MX"
  content     = "mail.${var.test_domain}"
  priority    = 10
  ttl         = var.test_ttl
  enabled     = true

  depends_on = [dynu_domain.test]
}

resource "dynu_dns_record" "txt" {
  hostname    = "txt.${var.test_domain}"
  record_type = "TXT"
  content     = "v=spf1 include:example.com -all"
  ttl         = var.test_ttl
  enabled     = true

  depends_on = [dynu_domain.test]
}

resource "dynu_dns_record" "srv" {
  hostname    = "_sip._tcp.${var.test_domain}"
  record_type = "SRV"
  content     = "target.${var.test_domain}"
  priority    = 10
  weight      = 5
  port        = 5060
  ttl         = var.test_ttl
  enabled     = true

  depends_on = [dynu_domain.test]
}

resource "dynu_dns_record" "caa" {
  hostname    = var.test_domain
  record_type = "CAA"
  content     = "letsencrypt.org"
  flags       = 0
  tag         = "issue"
  value       = "letsencrypt.org"
  ttl         = var.test_ttl
  enabled     = true

  depends_on = [dynu_domain.test]
}
