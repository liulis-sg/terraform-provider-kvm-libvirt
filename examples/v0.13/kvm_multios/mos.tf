terraform {
 required_version = ">= 0.13"
  required_providers {
    libvirt = {
      source  = "dmacvicar/libvirt"
    }
  }
}

provider "libvirt" {
  uri = "qemu:///system"
}

#resource "libvirt_volume" "ubuntu-cloud-uefi" {
#  name   = "ubuntu-cloud-uefi"
#  source = "/home/user/liulis/mtl_ubuntu/ubuntu_bk.qcow2"
#}
#
#resource "libvirt_volume" "volume" {
#  name           = "vm${count.index}"
#  base_volume_id = libvirt_volume.ubuntu-cloud-uefi.id
#  count          = 1
#}

resource "libvirt_domain" "ubuntu" {
  count  = 1
  name   = "ubuntu"
  memory = "4192"
  vcpu   = 6
  arch   = "x86_64"
  machine = "pc-q35-6.0"
  osvariant = "linux"
  emulator = "/usr/bin/qemu-system-x86_64"

  # This file is usually present as part of the ovmf firmware package in many
  # Linux distributions.
  firmware = "/usr/share/OVMF/OVMF_CODE_4M.fd"

  nvram {
    # This is the file which will back the UEFI NVRAM content.
    file = "/var/lib/libvirt/qemu/nvram/vm${count.index}_VARS.fd"

    # This file needs to be provided by the user.
    template = "/usr/share/OVMF/OVMF_VARS_4M.fd"
  }

  disk {
    #volume_id = element(libvirt_volume.volume.*.id, count.index)
    file = "/home/user/liulis/mtl_ubuntu/ubuntu_bk.qcow2"
  }

  ### use VNC as graphics
  #graphics {
  #  type        = "vnc"
  #  listen_type = "address"
  #  listen_address = "0.0.0.0"
  #}
#
  #video {
  #  type = "qxl"
  #}

  ### use GTK as graphics
  graphics {
    type = "gtk"
  }

  cpu {
    mode = "host-passthrough"
  }

  network_interface {
    network_name   = "vm-default"
    bridge         = "vm-virbr0"
  }

  memorybacking {
    hugepages = 2048
    access = "shared"
  }

  hostdev {
    driver = "vfio"
    domain = 0
    bus = 0
    slot = 2
    function = 2
  }

}

resource "libvirt_domain" "windows" {
  count  = 1
  name   = "windows"
  memory = "4192"
  vcpu   = 4
  #metadata = `<libosinfo:libosinfo xmlns:libosinfo="http://libosinfo.org/xmlns/libvirt/domain/1.0"><libosinfo:os id="http://microsoft.com/win/10"/></libosinfo:libosinfo>`
  emulator = "/usr/bin/qemu-system-x86_64"
  machine = "pc-q35-6.0"
  osvariant ="windows"

  # This file is usually present as part of the ovmf firmware package in many
  # Linux distributions.
  #firmware = "/usr/share/OVMF/OVMF_CODE.fd"

  #nvram {
  #  # This is the file which will back the UEFI NVRAM content.
  #  file = "/var/lib/libvirt/qemu/nvram/vm2_VARS.fd"

    # This file needs to be provided by the user.
    # template = "/srv/provisioning/terraform/debian-stable-uefi_VARS.fd"
  #}

  disk {
   #volume_id = element(libvirt_volume.volume.*.id, count.index)
   file = "/home/user/Downloads/windows.qcow2"
   format = "qcow2"
  }

  graphics {
   type        = "gtk"
   #listen_type = "address"
   #listen_address = "0.0.0.0"
  }

  #video {
  #type = "qxl"
  #}

  cpu {
    mode = "host-passthrough"
  }

  memorybacking {
    hugepages = 2048
    access = "shared"
  }

  hostdev {
    driver = "vfio"
    domain = 0
    bus = 0
    slot = 2
    function = 1
  }

  # xml {
  #  xslt = file("win_vm.xsl")
  #}
}