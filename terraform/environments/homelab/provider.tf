terraform {
  backend "kubernetes" {
    secret_suffix     = "homelab-proxmox"
    namespace         = "infrastructure"
    in_cluster_config = true
  }
}

provider "proxmox" {
  endpoint  = var.proxmox_endpoint
  api_token = var.proxmox_api_token
  insecure  = var.proxmox_insecure
}
