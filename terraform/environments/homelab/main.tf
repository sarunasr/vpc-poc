module "vms" {
  for_each = var.vms
  source   = "../../modules/proxmox-vm"

  name            = each.value.name
  vm_id           = each.value.vm_id
  node_name       = each.value.node_name
  template_id     = each.value.template_id
  cores           = each.value.cores
  memory_mb       = each.value.memory_mb
  disk_size_gb    = each.value.disk_size_gb
  bridge          = each.value.bridge
  ipv4_cidr       = each.value.ipv4_cidr
  gateway         = each.value.gateway
  dns_servers     = each.value.dns_servers
  cloud_init_user = each.value.cloud_init_user
  ssh_public_keys = var.ssh_public_keys
}

moved {
  from = module.vm
  to   = module.vms["web01"]
}
