variable "postgres_user" {
  description = "PostgreSQL username for Crochet"
  type        = string
}

variable "postgres_password" {
  description = "PostgreSQL password for Crochet"
  type        = string
}

variable "digitalocean_token" {
  description = "DigitalOcean API token"
  type        = string
}

variable "autoscale_min_nodes" {
  default = 2
}

variable "autoscale_max_nodes" {
  default = 10
}

variable "autoscale_scale_up_cpu" {
  default = 80
}

variable "autoscale_scale_down_cpu" {
  default = 20
}

variable "do_access_key" {
  description = "DigitalOcean Spaces Access Key"
  type        = string
}

variable "do_secret_key" {
  description = "DigitalOcean Spaces Secret Key"
  type        = string
}