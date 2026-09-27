resource "cloudflare_zone" "main" {
  zone = var.domain
}
