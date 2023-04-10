# Terraform Provider Libvirt

### Requirements
   * [Golang 1.13+ installed and configured.](https://golang.org/doc/install)
   * [Terraform 0.14+ CLI](https://learn.hashicorp.com/tutorials/terraform/install-cli) installed locally

### Set up override in ~/.terraformrc
```
  dev_overrides {
      "local.com/nex/kvm" = "/home/user/go/bin"
  }

```

### Build and install provider
* Run the following command to rebuild and install to ~/go/bin
    ```
    $ make
    ```
### Running example
* On root folder run terraform with example configuration
    ```
    $ sudo -i
    $ source mos_config.sh
    $ terraform init
    $ terraform apply -var-file vm.tfars
    ```