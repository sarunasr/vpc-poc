module "vm" {
  source = "../../modules/proxmox-vm"

  name            = var.name
  vm_id           = var.vm_id
  node_name       = var.node_name
  template_id     = var.template_id
  cores           = var.cores
  memory_mb       = var.memory_mb
  disk_size_gb    = var.disk_size_gb
  bridge          = var.bridge
  ipv4_cidr       = var.ipv4_cidr
  gateway         = var.gateway
  dns_servers     = var.dns_servers
  cloud_init_user = var.cloud_init_user
  ssh_public_keys = var.ssh_public_keys
}
