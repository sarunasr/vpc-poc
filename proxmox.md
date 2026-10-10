# Proxmox GitOps Controller POC - Infrastructure Context

## 1. Objective

Build a Proof of Concept (POC) for a Proxmox GitOps Controller.

The controller should manage Proxmox virtual machines declaratively using configuration stored in Git.

The long-term goal is to support:

- Automated VM provisioning and deletion.
- Desired-state reconciliation.
- VM configuration management.
- Ansible role execution and versioning.
- Git-based configuration changes.
- Periodic or scheduled reconciliation.
- Detection of configuration drift.
- Idempotent operations.

The infrastructure described below already exists and is operational.

Do not recreate or modify the existing Proxmox infrastructure unless necessary.

---

## 2. Proxmox Infrastructure

### Hosting

Provider: Hostinger VPS

Operating system: Debian GNU/Linux 13 (Trixie)

Proxmox VE: 9.2

Proxmox Manager: 9.2.21

Proxmox kernel: 7.0.14-23-pve

### Server

Hostname:
srv2050453

FQDN:
srv2050453.hstgr.cloud

Public IPv4:
185.170.196.225

Public IPv6:
2a02:4780:c:534::1

### Hardware Resources

CPU:
AMD EPYC 9354P

Allocated vCPUs:
2

RAM:
8 GB

Disk:
100 GB

Storage:
Single root filesystem on /dev/sda1

### Important Virtualization Limitation

The VPS runs on a KVM hypervisor, but Hostinger does not expose nested virtualization.

Observed:

- /dev/kvm does not exist.
- AMD-V CPU flags (svm) are not exposed.
- Nested KVM acceleration is unavailable.

Proxmox VMs must use QEMU software emulation.

Required VM configuration:

    kvm: 0

CPU type used:

    cpu: qemu64

This configuration has been tested successfully with Alpine Linux and Debian 13.

VM startup and operating system provisioning work, but performance is significantly slower than hardware-accelerated virtualization.

The controller must not assume KVM acceleration is available.

---

## 3. Proxmox Access

### Web Interface

URL:

https://185.170.196.225:8006

Authentication:

Linux PAM

The Proxmox Web UI is installed and working.

### REST API

Base URL:

https://185.170.196.225:8006/api2/json

Proxmox API services:

- pveproxy: active
- pvedaemon: active
- pve-cluster: active

API functionality has been verified using the local Proxmox API client:

    pvesh get /version
    pvesh get /nodes

Example version response:

    release: 9.2
    version: 9.2.21

### API Authentication

A dedicated GitOps controller API user and token have NOT been created yet.

The POC should use a dedicated Proxmox API account with appropriate permissions.

Do not use root credentials for normal controller operations.

API tokens must be stored securely using environment variables or a secret management solution.

Never hardcode API tokens in Git.

For HTTPS, the Proxmox server currently uses a self-signed certificate.

For development, support a configurable TLS verification setting.

Production implementations should verify server certificates.

---

## 4. Proxmox Node

Node name:

    srv2050453

This is a standalone Proxmox node.

There is no multi-node Proxmox cluster.

The Proxmox cluster filesystem is operational:

    /etc/pve

The absence of /etc/pve/corosync.conf is expected because the node is standalone.

Do not attempt to configure Corosync clustering for this POC.

---

## 5. Storage Configuration

Storage name:

    local

Storage type:

    dir

Storage path:

    /var/lib/vz

Available capacity:
Approximately 89 GB at initial configuration.

Storage configuration:

    dir: local
        path /var/lib/vz
        content images,rootdir,iso,vztmpl,backup,snippets
        shared 0

Supported content:

- VM disk images
- LXC root directories
- ISO images
- Container templates
- Backups
- Cloud-Init snippets

Important:

There is no local-lvm storage.

All VM disks are stored on the existing root filesystem.

The controller must use:

    storage: local

Do not assume local-lvm exists.

VM disks use directory-based storage.

Example VM disk:

    local:101/vm-101-disk-0.raw

Monitor disk usage because VM disks and the Proxmox operating system share the same filesystem.

---

## 6. Networking

### Public Network

Interface:

    eth0

IPv4:

    185.170.196.225/24

Gateway:

    185.170.196.254

IPv6:

    2a02:4780:c:534::1/48

Network management:

    Netplan + systemd-networkd

Hostinger manages the public interface through:

    /etc/netplan/50-cloud-init.yaml

Do not modify the public network configuration.

Do not attach eth0 to a Proxmox bridge.

Do not restart systemd-networkd unnecessarily because this is a remote VPS.

### Private VM Network

Bridge:

    vmbr0

Bridge IP:

    10.10.10.1/24

Subnet:

    10.10.10.0/24

Default gateway for VMs:

    10.10.10.1

DNS:

    1.1.1.1

The bridge is managed through systemd-networkd.

Configuration files:

    /etc/systemd/network/20-vmbr0.netdev
    /etc/systemd/network/20-vmbr0.network

Bridge configuration:

    [NetDev]
    Name=vmbr0
    Kind=bridge

    [Bridge]
    STP=false

Network configuration:

    [Match]
    Name=vmbr0

    [Network]
    Address=10.10.10.1/24
    ConfigureWithoutCarrier=yes
    IPMasquerade=ipv4
    IPv4Forwarding=yes

IPv4 forwarding is enabled:

    net.ipv4.ip_forward = 1

### NAT

NAT is handled automatically by systemd-networkd using nftables.

NAT table:

    table ip io.systemd.nat

Masqueraded subnet:

    10.10.10.0/24

VMs can access the internet through the Proxmox host.

Tested successfully:

- VM -> Proxmox gateway
- VM -> Internet
- VM -> DNS

VMs do not have public IPv4 addresses.

The POC should use the existing vmbr0 bridge.

Do not create additional bridges or change NAT rules unless necessary.

---

## 7. Debian 13 VM Template

A working Debian 13 Cloud-Init template already exists.

Template VMID:

    9000

Template name:

    debian13-template

Operating system:

    Debian 13 (Trixie)

Source:

Official Debian GenericCloud image.

Original image:

    debian-13-genericcloud-amd64.qcow2

### Template Configuration

    agent: enabled=1
    bios: seabios
    boot: order=scsi0
    cicustom: vendor=local:snippets/debian-vendor.yaml
    cores: 1
    cpu: qemu64
    kvm: 0
    machine: pc
    memory: 1024
    name: debian13-template
    net0: virtio,bridge=vmbr0
    ostype: l26
    scsihw: virtio-scsi-pci
    sockets: 1
    template: 1

Boot disk:

    scsi0

Disk size:

    8 GB

Cloud-Init drive:

    ide2

Serial console:

    serial0: socket
    vga: serial0

Cloud-Init user:

    debian

SSH authentication:

    SSH public key authentication

Password authentication:

    Disabled

Root SSH login:

    Disabled

### Cloud-Init Vendor Configuration

Location:

    /var/lib/vz/snippets/debian-vendor.yaml

Contents:

    #cloud-config
    package_update: true

    packages:
      - qemu-guest-agent
      - curl
      - wget
      - git
      - python3

    runcmd:
      - systemctl enable --now qemu-guest-agent

This vendor-data configuration is applied automatically to newly provisioned VMs.

### Important Cloud-Init Design Decision

Initially, a custom user-data file was used.

However, this prevented Proxmox from automatically generating unique VM hostnames.

The configuration was changed to:

- Proxmox-generated Cloud-Init user-data.
- Custom vendor-data for shared package installation.
- Per-VM network configuration through Proxmox Cloud-Init parameters.

This approach is confirmed working.

Do not replace Proxmox-generated user-data with a shared custom user-data file unless there is a specific reason.

Cloud-Init is primarily used for first-boot provisioning.

Ongoing configuration management should be handled by the GitOps controller and Ansible.

---

## 8. Existing Virtual Machines

### VM 101

VMID:

    101

Name:

    debian-01

IP:

    10.10.10.11/24

Gateway:

    10.10.10.1

Status at last check:

    Running

Resources:

    CPU: 1
    RAM: 1024 MB
    Disk: 8 GB

Cloud-Init successfully configured networking and SSH.

The QEMU Guest Agent was installed and running.

Note:

VM 101 was created before switching from custom user-data to vendor-data.

Its hostname was initially localhost.

It is not the best reference for validating the final template.

### VM 102

VMID:

    102

Name:

    debian-02

Expected IP:

    10.10.10.12/24

Gateway:

    10.10.10.1

Resources:

    CPU: 1
    RAM: 1024 MB
    Disk: 8 GB

This VM was created using the corrected Cloud-Init template.

Verified inside VM:

    cloud-init status --wait
    status: done

    hostname
    debian-02

    systemctl is-active qemu-guest-agent
    active

Also verified that the following packages were installed:

- qemu-guest-agent
- curl
- wget
- git
- python3

This confirms automated provisioning works.

VM 102 should be treated as the reference example.

Do not delete existing VMs without explicit authorization.

---

## 9. Automatic VM Provisioning

The following commands demonstrate the working provisioning approach.

Example: Create VM 103.

### Clone Template

    qm clone 9000 103 \
      --name debian-03 \
      --full 1

### Configure Resources and Networking

    qm set 103 \
      --cores 1 \
      --memory 1024 \
      --ipconfig0 ip=10.10.10.13/24,gw=10.10.10.1 \
      --nameserver 1.1.1.1

### Update Cloud-Init

    qm cloudinit update 103

### Start VM

    qm start 103

Cloud-Init then configures:

- Hostname
- Static IP
- Default gateway
- DNS
- SSH authorized keys
- User account
- Required packages
- QEMU Guest Agent

No manual guest OS installation is required.

The POC should implement these operations through the Proxmox REST API rather than executing qm commands over SSH.

---

## 10. SSH Access

The Proxmox host is reachable over SSH.

Local workstation:

macOS

SSH authentication:

ED25519 personal SSH key.

Private key location on the Mac:

    ~/.ssh/id_ed25519_personal

Public key:

    ~/.ssh/id_ed25519_personal.pub

Important:

The user's default SSH configuration contains Teleport integration.

For this environment, Teleport must be bypassed.

Working SSH access to Proxmox:

    ssh -F /dev/null \
      -i ~/.ssh/id_ed25519_personal \
      root@185.170.196.225

SSH access to private VMs uses the Proxmox host as a jump host.

Example:

    ssh -F /dev/null \
      -i ~/.ssh/id_ed25519_personal \
      -o IdentitiesOnly=yes \
      -o ProxyCommand="ssh -F /dev/null -i ~/.ssh/id_ed25519_personal -o IdentitiesOnly=yes -W %h:%p root@185.170.196.225" \
      debian@10.10.10.12

This works without Teleport.

The controller itself should communicate with the Proxmox REST API.

It should not depend on SSH access to the Proxmox host for normal operations.

SSH may be used for Ansible configuration management inside VMs.

---

## 11. POC Requirements

### Phase 1: Proxmox API Integration

Implement:

- Proxmox API client.
- API token authentication.
- TLS configuration.
- Node discovery.
- VM listing.
- VM configuration retrieval.
- VM creation from template.
- VM startup and shutdown.
- VM deletion.
- Error handling.

The API client should support asynchronous Proxmox tasks.

Proxmox operations often return a UPID.

The controller must wait for task completion and check the final result instead of assuming that a successful HTTP response means the operation has completed.

### Phase 2: Declarative VM Management

Define VMs using YAML files stored in Git.

Example:

    apiVersion: gitops.proxmox.io/v1alpha1
    kind: VirtualMachine

    metadata:
      name: debian-03

    spec:
      node: srv2050453
      vmid: 103
      template: 9000

      resources:
        cores: 1
        memory: 1024
        disk: 8G

      virtualization:
        kvm: false
        cpuType: qemu64

      network:
        bridge: vmbr0
        ip: 10.10.10.13/24
        gateway: 10.10.10.1
        dns:
          - 1.1.1.1

      cloudInit:
        user: debian

      powerState: running

This manifest is illustrative and is not an existing Proxmox API schema.

The controller should reconcile this desired state against the actual VM configuration.

### Phase 3: Reconciliation

Implement a reconciliation loop that:

1. Reads desired state from Git.
2. Retrieves actual state from Proxmox.
3. Calculates differences.
4. Creates missing VMs.
5. Updates existing VM configurations.
6. Starts or stops VMs as requested.
7. Detects configuration drift.
8. Records operation results.

The reconciliation process must be idempotent.

Re-running reconciliation without configuration changes must not recreate VMs or trigger unnecessary operations.

Handle changes that require VM reboot separately from changes that can be applied online.

VM deletion should require an explicit policy or configuration setting to prevent accidental destruction.

### Phase 4: Ansible Integration

The long-term controller will execute Ansible roles after VM provisioning.

Initial POC can use simple test roles.

Requirements:

- Define roles declaratively.
- Support role version pinning.
- Support different versions for different roles.
- Track applied role versions.
- Reapply roles when their declared versions change.
- Avoid unnecessary role execution when versions have not changed.
- Support initial configuration after VM provisioning.

Example:

    ansible:
      roles:
        - name: common
          version: "1.0.0"

        - name: monitoring
          version: "2.1.0"

For the initial implementation, role changes should be triggered by version changes.

Do not automatically reapply roles simply because their upstream Git repository changed.

Preserve the possibility of implementing that behavior later.

### Phase 5: Reconciliation Scheduling

Support two reconciliation scheduling modes.

1. Interval-based reconciliation.
2. Cron-based reconciliation.

Examples:

    reconciliation:
      interval: 5m

Or:

    reconciliation:
      cron: "0 8 * * *"
      timezone: Europe/Vilnius

Only one scheduling mode should be active for a given configuration.

The controller should support manual reconciliation as well.

### Phase 6: State Tracking

Track:

- VM ID
- VM name
- Desired configuration version
- Last successful reconciliation
- Last applied Ansible role versions
- Last operation result
- Reconciliation errors

For the POC, this can use lightweight local state storage.

The implementation should avoid making the VM lifecycle dependent on a fragile local state file.

Proxmox should remain the source of truth for actual infrastructure state.

Git should remain the source of truth for desired state.

---

## 12. Important Constraints

1. Do not modify the public eth0 networking configuration.

2. Do not disable or replace the existing vmbr0 NAT configuration.

3. Do not assume nested KVM virtualization is available.

4. Always use kvm: 0 for VMs in this environment.

5. Use storage named local, not local-lvm.

6. Use template 9000 for Debian 13 VM creation.

7. Do not recreate the existing Debian template.

8. Do not delete VM 101 or VM 102.

9. Do not use root credentials for controller API authentication.

10. Do not store secrets in Git.

11. Avoid unnecessary VM reboots or recreation.

12. Ensure reconciliation is idempotent.

13. Handle asynchronous Proxmox API operations correctly.

14. Keep the implementation suitable for eventual deployment outside this Hostinger VPS.

15. The controller must support normal Proxmox installations with KVM acceleration, even though the current development environment does not.

---

## 13. Current Status

Completed:

- Proxmox VE 9.2 installation.
- Proxmox API verification.
- Directory-based storage configuration.
- Private networking.
- NAT and outbound internet connectivity.
- Debian 13 cloud image download.
- Cloud-Init template creation.
- Automated VM cloning.
- Automatic IP configuration.
- Automatic hostname configuration.
- SSH public key provisioning.
- Automatic package installation.
- QEMU Guest Agent installation and startup.

Not completed:

- Dedicated Proxmox API user/token.
- GitOps controller implementation.
- Declarative VM manifest processing.
- Reconciliation engine.
- Ansible integration.
- Role version tracking.
- Reconciliation scheduling.
- Git integration.

The environment is ready for GitOps controller POC development.

## 14. Initial Implementation Goal

Start with a minimal working implementation.

The first milestone should be:

1. Authenticate to the existing Proxmox API.
2. Read a YAML VM definition.
3. Check whether the VM exists.
4. Clone template 9000 if missing.
5. Apply CPU, RAM, disk, and Cloud-Init network configuration.
6. Start the VM.
7. Wait for the Proxmox task to complete.
8. Verify VM status and guest agent availability.
9. Run reconciliation again.
10. Confirm that no unnecessary changes are made.

Do not implement all advanced functionality immediately.

Prioritize a small, working, testable POC with clean architecture that can be extended later.


## Current Environment Status (Updated)

The Proxmox environment is ready for the GitOps controller POC.

- Proxmox VE 9.2 is operational.
- Node: srv2050453
- Debian 13 Cloud-Init template: VMID 9000.
- No regular VMs currently exist.
- Previous test VMs 101 and 102 have been deleted.
- Storage: local (approximately 88 GB available).
- Private bridge: vmbr0 (10.10.10.1/24).
- NAT and outbound internet access are configured.
- Cloud-Init automatically configures hostname, networking, SSH keys, and packages.
- QEMU Guest Agent provisioning has been tested successfully.
- Nested KVM is unavailable. Use kvm: 0 and cpu: qemu64.
- Reserve VMIDs 200–299 for controller-managed test VMs.
- Never modify or delete template 9000 automatically.
- Never delete unmanaged VMs.
- Use Proxmox REST API for provisioning and reconciliation.