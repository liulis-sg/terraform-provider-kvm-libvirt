variable "os_config" {
    type = map(number)
    description = "VGA PCI Passthrough"
    default = {
        ubuntu = 0
        windows = 1
    }
}

variable "vm_config" {
    type = list(object({
        os = string
        vcpu = number
        graphics_type = string
        qcow_file = string
        firmware = string
        vga_pci = object({
            enable = number
            type = string
            driver = string
            domain = number
            bus = number
            slot = number
            function = number
        })
        mouse_usb = object ({
               enable = number
               type = string
               bus = number
               device = number
        })
        network = object ({
            enable = number
            name   = string
            bridge         = string
        })
        release_config = object ({
          os = string
          kernel = string
        })
    }))
    description = "configuration of ubuntu VM"
    default = [
        {
           os = "ubuntu"
           vcpu = 6
           graphics_type = "gtk"
           qcow_file = "/home/user/liulis/mtl_ubuntu/ubuntu_bk.qcow2"
           firmware = "/usr/share/OVMF/OVMF_CODE_4M.fd"
           vga_pci = {
               enable = 1
               type = "pci"
               driver = "vfio"
               domain = 0
               bus = 0
               slot = 2
               function = 1
           }
           mouse_usb = {
               enable = 0
               type = "usb"
               bus = 0
               device = 0
           }
           network = {
               enable = 0
               name = "vm-default"
               bridge = "vm-virbr0"
           }
           release_config = {
              os = "ubuntu 22.04 Jammy Jellyfish"
              kernel = "6.0 iotg-next-overlay-v6.0-ubuntu-230117t080120z"
           }
        },
        {
            os = "windows"
            vcpu = 6
            graphics_type = "vnc"
            qcow_file = "/home/user/Downloads/windows.qcow2"
            firmware = null
            vga_pci = {
                enable = 1
                type = "pci"
                driver = "vfio"
                domain = 0
                bus = 0
                slot = 2
                function = 2
            }
           mouse_usb = {
               enable = 0
               type = "usb"
               bus = 0
               device = 0
           }
            network = {
               enable = 0
               name = "vm-default"
               bridge = "vm-virbr0"
           }
            release_config = {
               os = "windows"
               kernel = "Windows 10 IOT Enterprise LTSC 21H2"
           }
        }
]
}



variable "ubuntu_vga_pci" {
    type = map(number)
    description = "VGA PCI Passthrough"
    default = {
        enabled = 1
        domain = 0
        bus = 0
        slot = 2
        function = 1
    }
}

variable "ubuntu_graphics" {
    type = map(string)
    description = "graphics"
    default = {
        type = "vnc"
        listen_address = "0.0.0.0"
    }
    sensitive = "false"
}

variable "ubuntu_network" {
    type = map(string)
    description = "network interface"
    default = {
        enabled = "1"
        network_name   = "vm-default"
        bridge         = "vm-virbr0"
    }
}

variable "ubuntu_hostdev_usb" {
    type = map(number)
    description = "USB device Passthrough"
    default = {
        enabled = 0
        bus = 3
        device = 8
    }
}

variable "windows_graphics" {
    type = map(string)
    description = "graphics"
    default = {
        type = "gtk"
        listen_address = "0.0.0.0"
    }
    sensitive = "false"
}

variable "windows_vga_pci" {
    type = map(number)
    description = "VGA PCI Passthrough"
    default = {
        enabled = 1
        domain = 0
        bus = 0
        slot = 2
        function = 2
    }
}


variable "windows_hostdev_usb" {
    type = map(number)
    description = "USB device Passthrough"
    default = {
        enabled = 0
        bus = 3
        device = 4
    }
}

variable "windows_network" {
    type = map(string)
    description = "network interface"
    default = {
        enabled = "1"
        network_name   = "vm-default"
        bridge         = "vm-virbr0"
    }
}
