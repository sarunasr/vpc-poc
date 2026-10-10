output "vm_id" {
  description = "Managed Proxmox VM ID."
  value       = module.vm.vm_id
}

output "vm_name" {
  description = "Managed VM hostname."
  value       = module.vm.name
}

output "vm_ipv4_addresses" {
  description = "VM IPv4 addresses reported by the guest agent."
  value       = module.vm.ipv4_addresses
}
