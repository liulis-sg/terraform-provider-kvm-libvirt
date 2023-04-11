vmconfig = {
    "ubuntu" = {
        vmtype = "os",
        image = "/home/user/liulis/mtl_ubuntu/ubuntu_bk.qcow2",
        vcpu = 6
        os = "ubuntu"
        graphics = {
            gpusriov = true,
            type =  "vnc",
            listen_type = "address",
            listen_address = "0.0.0.0",
        }
        hostdev_list = [
            {
                name = "vga",
                type = "pci",
                domain = 0,
                bus = 0,
                slot =2,
                function = 1
                driver = "vfio",

            },
            #{
            #    name = "d2",
            #    driver = "vfio",
            #    domain = "0",
            #}
        ]

    },
    "windows" = {
        vmtype = "os",
        image = "/home/user/Downloads/windows.qcow2",
        vcpu = 4
        os = "windows"
        graphics = {
            gpusriov = false,
            type = "vnc",
            listen_type = "address",
            listen_address = "0.0.0.0",
        }
        hostdev_list = [
            #{
            #    name = "d1",
            #    driver = "vfio",
            #    domain = "0",
            #},
            {
                name = "vga",
                type = "pci",
                domain = 0,
                bus = 0,
                slot =2,
                function = 2
                driver = "vfio",
            },
        ]
    }
}