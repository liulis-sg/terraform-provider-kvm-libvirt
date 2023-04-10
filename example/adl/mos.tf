terraform {
 required_version = ">= 0.13"
  required_providers {
    kvm = {
      source  = "local.com/nex/kvm"
    }
  }
}

provider "kvm" {
  uri = "qemu:///system"
}

locals {
  vms = ["ubuntu", "windows"]
}

resource "kvm_domain" "vms" {
  for_each = toset(local.vms)
  name = each.value
  vcpu =  var.vmconfig[each.value].vcpu
  machine = "pc-q35-6.0"
  arch = "x86_64"
  emulator = "/usr/bin/qemu-system-x86_64"
  osvariant = var.vmconfig[each.value].os
  memory = "4192"

  cpu {
    mode = "host-passthrough"
  }

  memorybacking {
    hugepages = 2048
    access = "shared"
  }


  disk {
    file = var.vmconfig[each.value].image
  }

  nvram {
    # This is the file which will back the UEFI NVRAM content.
    file = "/var/lib/libvirt/qemu/nvram/vm${index(local.vms, each.value)}_VARS.fd"

    # This file needs to be provided by the user.
    template = "/usr/share/OVMF/OVMF_VARS_4M.fd"
  }

  video {
   type = "qxl"
  }

  graphics {
    type = var.vmconfig[each.value].graphics.type
    listen_address = var.vmconfig[each.value].graphics.listen_address
    listen_type = var.vmconfig[each.value].graphics.listen_type
  }

}


