variable "name" {
  description = "VM name and hostname."
  type        = string
}

variable "vm_id" {
  description = "Unique VM ID. POC-managed IDs should be in the 200-299 range."
  type        = number
}

variable "node_name" {
  description = "Proxmox node where the VM will run."
  type        = string
}

variable "template_id" {
  description = "Existing Debian 13 Cloud-Init template VM ID."
  type        = number
  default     = 9000
}

variable "cores" {
  description = "Number of vCPU cores."
  type        = number
  default     = 1
}

variable "memory_mb" {
  description = "Memory in MiB."
  type        = number
  default     = 1024
}

variable "disk_size_gb" {
  description = "Root disk size in GiB."
  type        = number
  default     = 8
}

variable "bridge" {
  description = "Existing Proxmox bridge."
  type        = string
  default     = "vmbr0"
}

variable "ipv4_cidr" {
  description = "Static IPv4 CIDR for Cloud-Init."
  type        = string
}

variable "gateway" {
  description = "IPv4 gateway for Cloud-Init."
  type        = string
  default     = "10.10.10.1"
}

variable "dns_servers" {
  description = "DNS servers for Cloud-Init."
  type        = list(string)
  default     = ["1.1.1.1"]
}

variable "cloud_init_user" {
  description = "Non-root Cloud-Init account."
  type        = string
  default     = "debian"
}

variable "ssh_public_keys" {
  description = "SSH public keys installed by Cloud-Init."
  type        = list(string)
  sensitive   = true
}
