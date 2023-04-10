package main

import (
	"math/rand"
	"terraform-provider-kvm/kvm"
	"time"

	//"github.com/dmacvicar/terraform-provider-libvirt/libvirt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	defer kvm.CleanupLibvirtConnections()

	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: kvm.Provider,
	})
}
