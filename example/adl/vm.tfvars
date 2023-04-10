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
    }
}