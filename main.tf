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

provider "kubernetes" {
  host                   = digitalocean_kubernetes_cluster.crochet_doks.kube_config[0].host
  cluster_ca_certificate = base64decode(digitalocean_kubernetes_cluster.crochet_doks.kube_config[0].cluster_ca_certificate)
  token                  = digitalocean_kubernetes_cluster.crochet_doks.kube_config[0].token
}

variable "rabbitmq_password" {
  type        = string
  description = "The password for RabbitMQ"
  sensitive   = true
}

variable "digitalocean_registry_token" {
  type        = string
  description = "The token for the DigitalOcean Container Registry"
  sensitive   = true
}

resource "digitalocean_droplet" "crochet_rabbitmq" {
  count    = 1
  name     = "crochet-rabbitmq-${count.index}"
  image    = "ubuntu-22-04-x64"
  region   = "nyc1"
  size     = "s-1vcpu-1gb"
  tags     = ["rabbitmq"]

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

resource "digitalocean_kubernetes_cluster" "crochet_doks" {
  name    = "crochet-doks-cluster"
  region  = "nyc1"
  version = "1.32.2-do.0" 

  node_pool {
    name       = "default-pool"
    size       = "s-1vcpu-2gb"
    node_count = 1
  }
}

resource "digitalocean_container_registry" "crochet_registry" {
  name = "crochet-registry"
  subscription_tier_slug = "basic"
}

resource "kubernetes_secret" "regcred" {
  metadata {
    name = "regcred"
  }
  type = "kubernetes.io/dockerconfigjson"
  data = {
    ".dockerconfigjson" = base64encode(jsonencode({
      auths = {
        "registry.digitalocean.com" = {
          username = "do"
          password = var.digitalocean_registry_token
          email    = "unused@example.com"
          auth     = base64encode("do:${var.digitalocean_registry_token}")
        }
      }
    }))
  }
}

output "kubeconfig" {
  value     = digitalocean_kubernetes_cluster.crochet_doks.kube_config[0].raw_config
  sensitive = true
}

output "registry_endpoint" {
  value = digitalocean_container_registry.crochet_registry.endpoint
}