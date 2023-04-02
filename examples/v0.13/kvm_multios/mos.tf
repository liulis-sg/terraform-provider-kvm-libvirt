terraform {
 required_version = ">= 0.13"
  required_providers {
    libvirt = {
      source  = "dmacvicar/libvirt"
    }
  }
}

locals {
  generate_xml_only = "false"
  vms =["ubuntu", "windows"]
  #vms =["ubuntu"]
  ## provide the os type for the vm, 0: ubuntu 1: windows, 2: android
  os_id = [ 0, 1 ]
}

provider "libvirt" {
  uri = "qemu:///system"
}

resource "libvirt_domain" "vmdomain" {
  count  = length(local.vms)
  vmid = count.index + 1
  name   = "${local.vms[count.index]}-${count.index}"
  memory = "4192"
  osvariant = local.vms[count.index]
  vcpu   = var.vm_config[local.os_id[count.index]].vcpu
  arch   = "x86_64"
  machine = "pc-q35-6.0"
  emulator = "/usr/bin/qemu-system-x86_64"
  tpl_gen = local.generate_xml_only == "true" ? true : false
  total_vms = length(local.vms)
  last_instance = (count.index +1) == length(local.vms) ? true : false

  ## This file is usually present as part of the ovmf firmware package in many
  ## Linux distributions.
  # "/usr/share/OVMF/OVMF_CODE_4M.fd"
  firmware = var.vm_config[local.os_id[count.index]].firmware
  nvram {
    # This is the file which will back the UEFI NVRAM content.
    file = "/var/lib/libvirt/qemu/nvram/vm${count.index}_VARS.fd"
    # This file needs to be provided by the user.
    template = "/usr/share/OVMF/OVMF_VARS_4M.fd"
  }

  disk {
    #volume_id = element(libvirt_volume.volume.*.id, count.index)
    file = var.vm_config[local.os_id[count.index]].qcow_file
  }

  ## use VNC as graphics
  graphics {
   type        = var.vm_config[local.os_id[count.index]].graphics_type
   listen_type = "address"
   listen_address = "0.0.0.0"
  }

  video {
    type = "qxl"
  }

  cpu {
    mode = "host-passthrough"
  }

  network_interface {
    enabled = var.vm_config[local.os_id[count.index]].network.enable
    network_name   = var.vm_config[local.os_id[count.index]].network.name
    bridge         = var.vm_config[local.os_id[count.index]].network.bridge
  }
  memorybacking {
    hugepages = 2048
    access = "shared"
  }

  hostdev {
    type = var.vm_config[local.os_id[count.index]].vga_pci.type
    driver = var.vm_config[local.os_id[count.index]].vga_pci.driver
    enabled = var.vm_config[local.os_id[count.index]].vga_pci.enable
    domain = var.vm_config[local.os_id[count.index]].vga_pci.domain
    bus = var.vm_config[local.os_id[count.index]].vga_pci.bus
    slot = var.vm_config[local.os_id[count.index]].vga_pci.slot
    function = var.vm_config[local.os_id[count.index]].vga_pci.function
  }

  hostdev {
    type = var.vm_config[local.os_id[count.index]].mouse_usb.type
    name = "USB-Mouse"
    enabled = var.vm_config[local.os_id[count.index]].mouse_usb.enable
    bus = var.vm_config[local.os_id[count.index]].mouse_usb.bus
    device = var.vm_config[local.os_id[count.index]].mouse_usb.device
  }

  release_config {
    os = var.vm_config[local.os_id[count.index]].release_config.os
    kernel = var.vm_config[local.os_id[count.index]].release_config.kernel
  }
}
