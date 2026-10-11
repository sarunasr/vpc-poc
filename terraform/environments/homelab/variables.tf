variable "proxmox_endpoint" {
  description = "Proxmox API endpoint, including trailing slash."
  type        = string
  default     = "https://185.170.196.225:8006/"
}

variable "proxmox_api_token" {
  description = "Dedicated Proxmox API token."
  type        = string
  sensitive   = true
}

variable "proxmox_insecure" {
  description = "Skip Proxmox TLS verification only for the self-signed lab endpoint."
  type        = bool
  default     = true
}

variable "vms" {
  description = "Complete map of VMs managed by this datacenter, keyed by VirtualMachine name."
  type        = map(any)
}

variable "ssh_public_keys" {
  description = "SSH public keys installed by Cloud-Init."
  type        = list(string)
  sensitive   = true
}
