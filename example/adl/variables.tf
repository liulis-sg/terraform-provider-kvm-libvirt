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
    }))
}

variable "hostdev" {
    type = list(string)
    default = ["ubuntu", "windows"]
}

