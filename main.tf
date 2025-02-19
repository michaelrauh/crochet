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

variable "rabbitmq_password" {
  type        = string
  description = "The password for RabbitMQ"
  sensitive   = true
}

resource "digitalocean_droplet" "crochet_rabbitmq" {
  count    = 1
  name     = "crochet-rabbitmq-${count.index}"
  image    = "ubuntu-22-04-x64"
  region   = "nyc1"
  size     = "s-1vcpu-1gb"
  tags     = ["rabbitmq"]
  ssh_keys = [var.ssh_fingerprint]

  user_data = <<EOF
#!/bin/bash
apt-get update
apt-get install -y docker.io
systemctl start docker
systemctl enable docker
docker run -d --rm --name rabbitmq \
  -p 5672:5672 -p 15672:15672 \
  -e RABBITMQ_DEFAULT_USER=admin \
  -e RABBITMQ_DEFAULT_PASS=${var.rabbitmq_password} \
  rabbitmq:3-management
EOF
}

output "rabbitmq_connection_string" {
  value     = "amqp://admin:${var.rabbitmq_password}@${digitalocean_droplet.crochet_rabbitmq[0].ipv4_address}:5672"
  sensitive = true
}

 resource "digitalocean_database_cluster" "crochet_postgres" {
   name       = "crochet-postgres-aaaa"
   engine     = "pg"
   version    = "13"
   size       = "db-s-1vcpu-1gb"
   region     = "nyc1"
   node_count = 1
 }

 output "database_url" {
  value     = digitalocean_database_cluster.crochet_postgres.uri
  sensitive = true
}

variable "ssh_fingerprint" {
  description = "The fingerprint of the SSH key to use for the droplet"
  type        = string
}

resource "digitalocean_droplet" "crochet_app" {
  count  = 1
  name   = "crochet-app-${count.index}"
  image  = "ubuntu-22-04-x64"
  region = "nyc1"
  size   = "s-1vcpu-1gb"
  tags   = ["app"]
  ssh_keys = [var.ssh_fingerprint]
}

output "droplet_ip" {
  value = digitalocean_droplet.crochet_app[0].ipv4_address
}