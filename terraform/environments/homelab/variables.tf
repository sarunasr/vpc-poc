variable "proxmox_endpoint" {
  description = "Proxmox API endpoint, including trailing slash."
  type        = string
  default     = "https://185.170.196.225:8006/"
}

variable "proxmox_api_token" {
  description = "Dedicated, least-privilege Proxmox API token."
  type        = string
  sensitive   = true
}

variable "proxmox_insecure" {
  description = "Skip Proxmox TLS verification only for the self-signed lab endpoint."
  type        = bool
  default     = true
}

variable "name" {
  type    = string
  default = "gitops-poc-201"
}

variable "vm_id" {
  type    = number
  default = 201
}

variable "node_name" {
  type    = string
  default = "srv2050453"
}

variable "template_id" {
  type    = number
  default = 9000
}

variable "cores" {
  type    = number
  default = 1
}

variable "memory_mb" {
  type    = number
  default = 1024
}

variable "disk_size_gb" {
  type    = number
  default = 8
}

variable "bridge" {
  type    = string
  default = "vmbr0"
}

variable "ipv4_cidr" {
  type    = string
  default = "10.10.10.21/24"
}

variable "gateway" {
  type    = string
  default = "10.10.10.1"
}

variable "dns_servers" {
  type    = list(string)
  default = ["1.1.1.1"]
}

variable "cloud_init_user" {
  type    = string
  default = "debian"
}

variable "ssh_public_keys" {
  type      = list(string)
  sensitive = true
}
