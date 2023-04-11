variable "vmconfig" {
    type = map(object({
       vmtype = string,
       image = string,
       vcpu = number
       #firmware = string,
       os = string,
       graphics = object( {
        type = string,
        listen_type = string,
        listen_address = string,
       })
       hostdev = list(string)
       hostdev_list = list(object({
            type = string,
            name = string,
            domain = number,
            driver = string,
            slot = number,
            function = number,
            bus = number,
       }))
    }))
}
