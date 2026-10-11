output "vms" {
  description = "Managed VMs keyed by their stable VirtualMachine names."
  value = {
    for name, vm in module.vms : name => {
      vm_id          = vm.vm_id
      name           = vm.name
      ipv4_addresses = vm.ipv4_addresses
    }
  }
}
