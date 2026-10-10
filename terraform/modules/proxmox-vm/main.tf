resource "proxmox_virtual_environment_vm" "this" {
  name        = var.name
  vm_id       = var.vm_id
  node_name   = var.node_name
  description = "Managed by infrastructure-gitops"
  on_boot     = true
  started     = true

  clone {
    vm_id = var.template_id
    full  = true
  }

  cpu {
    cores = var.cores
    type  = "qemu64"
  }

  memory {
    dedicated = var.memory_mb
  }

  disk {
    interface    = "scsi0"
    datastore_id = "local"
    size         = var.disk_size_gb
    iothread     = true
  }

  network_device {
    bridge = var.bridge
    model  = "virtio"
  }

  initialization {
    datastore_id = "local"

    user_account {
      username = var.cloud_init_user
      keys     = var.ssh_public_keys
    }

    ip_config {
      ipv4 {
        address = var.ipv4_cidr
        gateway = var.gateway
      }
    }

    dns {
      servers = var.dns_servers
    }
  }

  agent {
    enabled = true
  }

  lifecycle {
    prevent_destroy = true
  }
}
