terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.47.0"
    }
  }
}

provider "digitalocean" {
  token = var.digitalocean_token
}

resource "digitalocean_database_cluster" "crochet_postgres" {
  name       = "crochet-postgres"
  engine     = "pg"
  version    = "13"
  size       = "db-s-1vcpu-1gb"
  region     = "nyc1"
  node_count = 1
}

output "database_url" {
  value     = "postgres://${var.postgres_user}:${var.postgres_password}@${digitalocean_database_cluster.crochet_postgres.uri}"
  sensitive = true
}

resource "digitalocean_droplet" "crochet_app" {
  count  = 1
  name   = "crochet-app-${count.index}"
  image  = "ubuntu-20-04-x64"
  region = "nyc1"
  size   = "s-1vcpu-1gb"
  tags   = ["app"]
}