package kvm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"libvirt.org/go/libvirtxml"
)

const domWaitLeaseStillWaiting = "waiting-addresses"
const domWaitLeaseDone = "all-addresses-obtained"

var errDomainInvalidState = errors.New("invalid state for domain")

//nolint:gomnd
func getIgnitionVolumeKeyFromTerraformID(id string) (string, error) {
	s := strings.SplitN(id, ";", 2)
	if len(s) != 2 {
		return "", fmt.Errorf("%s is not a valid key", id)
	}
	return s[0], nil
}

//nolint:gomnd
func getCloudInitVolumeKeyFromTerraformID(id string) (string, error) {
	s := strings.SplitN(id, ";", 2)
	if len(s) != 2 {
		return "", fmt.Errorf("%s is not a valid key", id)
	}
	return s[0], nil
}

// randomMACAddress returns a randomized MAC address
// with libvirt prefix.
//nolint:gomnd
func randomMACAddress() (string, error) {
	buf := make([]byte, 3)
	//nolint:gosec // math.rand is enough for this
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	// set local bit and unicast
	buf[0] = (buf[0] | 2) & 0xfe
	// Set the local bit
	buf[0] |= 2

	// avoid libvirt-reserved addresses
	if buf[0] == 0xfe {
		buf[0] = 0xee
	}

	return fmt.Sprintf("52:54:00:%02x:%02x:%02x",
		buf[0], buf[1], buf[2]), nil
}

func domainWaitForLeases(ctx context.Context, virConn *libvirt.Libvirt, domain libvirt.Domain, waitForLeases []*libvirtxml.DomainInterface,
	timeout time.Duration, rd *schema.ResourceData) error {
	waitFunc := func() (interface{}, string, error) {

		state, err := domainGetState(virConn, domain)
		if err != nil {
			return false, "", err
		}

		for _, fatalState := range []string{"crashed", "shutoff", "shutdown", "pmsuspended"} {
			if state == fatalState {
				return false, "", errDomainInvalidState
			}
		}

		if state != "running" {
			return false, domWaitLeaseStillWaiting, nil
		}

		// check we have IPs for all the interfaces we are waiting for
		for _, iface := range waitForLeases {
			found, ignore, err := domainIfaceHasAddress(virConn, domain, *iface, rd)
			if err != nil {
				return false, "", err
			}
			if ignore {
				log.Printf("[DEBUG] we don't care about the IP address for %+v", iface)
				continue
			}
			if !found {
				log.Printf("[DEBUG] IP address not found for iface=%+v: will try in a while", strings.ToUpper(iface.MAC.Address))
				return false, domWaitLeaseStillWaiting, nil
			}
		}

		log.Printf("[DEBUG] all the %d IP addresses obtained for the domain", len(waitForLeases))
		return true, domWaitLeaseDone, nil
	}

	stateConf := &resource.StateChangeConf{
		Pending:    []string{domWaitLeaseStillWaiting},
		Target:     []string{domWaitLeaseDone},
		Refresh:    waitFunc,
		Timeout:    timeout,
		MinTimeout: resourceStateMinTimeout,
		Delay:      resourceStateDelay,
	}

	_, err := stateConf.WaitForStateContext(ctx)
	log.Print("[DEBUG] wait-for-leases was successful")
	return err
}

func domainIfaceHasAddress(virConn *libvirt.Libvirt, domain libvirt.Domain,
	iface libvirtxml.DomainInterface,
	rd *schema.ResourceData) (found bool, ignore bool, err error) {

	mac := strings.ToUpper(iface.MAC.Address)
	if mac == "" {
		log.Printf("[DEBUG] Can't wait without a MAC address: ignoring interface %+v.\n", iface)
		// we can't get the ip without a mac address
		return false, true, nil
	}

	log.Printf("[DEBUG] waiting for network address for iface=%s\n", mac)
	ifacesWithAddr, err := domainGetIfacesInfo(virConn, domain, rd)
	if err != nil {
		return false, false, fmt.Errorf("error retrieving interface addresses: %w", err)
	}
	log.Printf("[DEBUG] ifaces with addresses: %+v\n", ifacesWithAddr)

	for _, ifaceWithAddr := range ifacesWithAddr {
		if len(ifaceWithAddr.Hwaddr) > 0 && (mac == strings.ToUpper(ifaceWithAddr.Hwaddr[0])) {
			log.Printf("[DEBUG] found IPs for MAC=%+v: %+v\n", mac, ifaceWithAddr.Addrs)
			return true, false, nil
		}
	}

	log.Printf("[DEBUG] %+v doesn't have IP address(es) yet...\n", mac)
	return false, false, nil
}

func domainGetState(virConn *libvirt.Libvirt, domain libvirt.Domain) (string, error) {
	state, _, err := virConn.DomainGetState(domain, 0)
	if err != nil {
		return "", err
	}

	var stateStr string

	switch libvirt.DomainState(state) {
	case libvirt.DomainNostate:
		stateStr = "nostate"
	case libvirt.DomainRunning:
		stateStr = "running"
	case libvirt.DomainBlocked:
		stateStr = "blocked"
	case libvirt.DomainPaused:
		stateStr = "paused"
	case libvirt.DomainShutdown:
		stateStr = "shutdown"
	case libvirt.DomainCrashed:
		stateStr = "crashed"
	case libvirt.DomainPmsuspended:
		stateStr = "pmsuspended"
	case libvirt.DomainShutoff:
		stateStr = "shutoff"
	default:
		stateStr = fmt.Sprintf("unknown: %v", state)
	}

	return stateStr, nil
}

func domainIsRunning(virConn *libvirt.Libvirt, domain libvirt.Domain) (bool, error) {
	state, _, err := virConn.DomainGetState(domain, 0)
	if err != nil {
		return false, fmt.Errorf("couldn't get state of domain: %w", err)
	}

	return libvirt.DomainState(state) == libvirt.DomainRunning, nil
}

func domainGetIfacesInfo(virConn *libvirt.Libvirt, domain libvirt.Domain, rd *schema.ResourceData) ([]libvirt.DomainInterface, error) {
	domainRunningNow, err := domainIsRunning(virConn, domain)
	if err != nil {
		return []libvirt.DomainInterface{}, err
	}
	if !domainRunningNow {
		log.Print("[DEBUG] no interfaces could be obtained: domain not running")
		return []libvirt.DomainInterface{}, nil
	}

	// setup source of interface address information
	var addrsrc uint32
	qemuAgentEnabled := rd.Get("qemu_agent").(bool)
	if qemuAgentEnabled {
		addrsrc = uint32(libvirt.DomainInterfaceAddressesSrcAgent)
		log.Printf("[DEBUG] qemu-agent used to query interface info")
	} else {
		addrsrc = uint32(libvirt.DomainInterfaceAddressesSrcLease)
		log.Printf("[DEBUG] Obtain interface info from dhcp lease file")
	}

	// get all the interfaces attached to libvirt networks
	var interfaces []libvirt.DomainInterface
	interfaces, err = virConn.DomainInterfaceAddresses(domain, addrsrc, 0)
	if err != nil {
		return interfaces, fmt.Errorf("error retrieving interface addresses: %w", err)
	}
	log.Printf("[DEBUG] Interfaces info obtained with libvirt API:\n%s\n", spew.Sdump(interfaces))

	return interfaces, nil
}

func newDiskForCloudInit(virConn *libvirt.Libvirt, volumeKey string) (libvirtxml.DomainDisk, error) {
	disk := libvirtxml.DomainDisk{
		// HACK mark the disk as belonging to the cloudinit
		// resource so we can ignore it
		Serial: "cloudinit",
		Device: "cdrom",
		Target: &libvirtxml.DomainDiskTarget{
			// Last device letter possible with a single IDE controller on i440FX
			Dev: "hdd",
			Bus: "ide",
		},
		Driver: &libvirtxml.DomainDiskDriver{
			Name: "qemu",
			Type: "raw",
		},
	}

	diskVolume, err := virConn.StorageVolLookupByKey(volumeKey)
	if err != nil {
		return disk, fmt.Errorf("can't retrieve volume %s: %w", volumeKey, err)
	}
	diskVolumeFile, err := virConn.StorageVolGetPath(diskVolume)
	if err != nil {
		return disk, fmt.Errorf("error retrieving volume file: %w", err)
	}

	disk.Source = &libvirtxml.DomainDiskSource{
		File: &libvirtxml.DomainDiskSourceFile{
			File: diskVolumeFile,
		},
	}

	return disk, nil
}

func setCoreOSIgnition(d *schema.ResourceData, domainDef *libvirtxml.Domain, arch string) error {
	if ignition, ok := d.GetOk("coreos_ignition"); ok {
		ignitionKey, err := getIgnitionVolumeKeyFromTerraformID(ignition.(string))
		if err != nil {
			return err
		}

		switch arch {
		case "i686", "x86_64", "aarch64":
			// QEMU and the Linux kernel support the use of the Firmware
			// Configuration Device on these architectures. Ignition will use
			// this mechanism to read its configuration from the hypervisor.

			// `fw_cfg_name` stands for firmware config is defined by a key and a value
			// credits for this cryptic name: https://github.com/qemu/qemu/commit/81b2b81062612ebeac4cd5333a3b15c7d79a5a3d
			if fwCfg, ok := d.GetOk("fw_cfg_name"); ok {
				domainDef.QEMUCommandline = &libvirtxml.DomainQEMUCommandline{
					Args: []libvirtxml.DomainQEMUCommandlineArg{
						{
							Value: "-fw_cfg",
						},
						{
							Value: fmt.Sprintf("name=%s,file=%s", fwCfg, ignitionKey),
						},
					},
				}
			}
		case "s390", "s390x", "ppc64", "ppc64le":
			// System Z and PowerPC do not support the Firmware Configuration
			// device. After a discussion about the best way to support a similar
			// method for qemu in https://github.com/coreos/ignition/issues/928,
			// decided on creating a virtio-blk device with a serial of ignition
			// which contains the ignition config and have ignition support for
			// reading from the device which landed in https://github.com/coreos/ignition/pull/936
			igndisk := libvirtxml.DomainDisk{
				Device: "disk",
				Source: &libvirtxml.DomainDiskSource{
					File: &libvirtxml.DomainDiskSourceFile{
						File: ignitionKey,
					},
				},
				Target: &libvirtxml.DomainDiskTarget{
					Dev: "vdb",
					Bus: "virtio",
				},
				Driver: &libvirtxml.DomainDiskDriver{
					Name: "qemu",
					Type: "raw",
				},
				ReadOnly: &libvirtxml.DomainDiskReadOnly{},
				Serial:   "ignition",
			}
			domainDef.Devices.Disks = append(domainDef.Devices.Disks, igndisk)
		default:
			return fmt.Errorf("ignition not supported on %q", arch)
		}
	}

	return nil
}

func setVideo(d *schema.ResourceData, domainDef *libvirtxml.Domain) {
	prefix := "video.0"
	if _, ok := d.GetOk(prefix); ok {
		domainDef.Devices.Videos = append(domainDef.Devices.Videos, libvirtxml.DomainVideo{
			Model: libvirtxml.DomainVideoModel{
				Type: d.Get(prefix + ".type").(string),
			},
		})
	}
}

func setHostDev(d *schema.ResourceData, domainDef *libvirtxml.Domain) {
	var pcistring string
	//var vmidNumber int

	//if vmid, ok := d.GetOk("vmid"); ok {
	//	vmidNumber = vmid.(int)
	//} else {
	//	fmt.Errorf("missing vmid")
	//	return
	//}

	for i := 0; i < d.Get("hostdev.#").(int); i++ {
		hostdev := libvirtxml.DomainHostdev{}
		prefix := fmt.Sprintf("hostdev.%d", i)

		hostdevType, ok := d.GetOk(prefix + ".type")
		if !ok {
			log.Printf("[DEBUG] hostdev type not found %s", prefix)
			return
		}

		switch hostdevType {
		case "pci":
			//hostdevenabled, ok := d.GetOk(prefix + ".enabled")
			//if !ok || hostdevenabled.(int) != 1 {
			//	//udpateTableContent("VGA", "NA", domainDef, vmidNumber)
			//	return
			//}
			hostdev.SubsysPCI = &libvirtxml.DomainHostdevSubsysPCI{}
			hostdev.SubsysPCI.Source = &libvirtxml.DomainHostdevSubsysPCISource{}
			hostdev.SubsysPCI.Source.Address = &libvirtxml.DomainAddressPCI{}

			if hostdevDriver, ok := d.GetOk(prefix + ".driver"); ok {
				log.Printf("[DEBUG] hostdev driver found %s", prefix)
				hostdev.SubsysPCI.Driver = &libvirtxml.DomainHostdevSubsysPCIDriver{
					Name: hostdevDriver.(string),
				}
			} else {
				fmt.Errorf("driver type for hostdev")
			}

			if sourceDomain, ok := d.GetOk(prefix + ".domain"); ok {
				var adomain uint = (uint)(sourceDomain.(int))
				hostdev.SubsysPCI.Source.Address.Domain = &adomain
				pcistring = pcistring + strconv.Itoa((int)(adomain))
				pcistring += "."
				log.Printf("[DEBUG] hostdev domain found %d", (uint)(sourceDomain.(int)))
			} else {
				pcistring += "0."
			}
			if sourceBus, ok := d.GetOk(prefix + ".bus"); ok {
				var abus uint = (uint)(sourceBus.(int))
				hostdev.SubsysPCI.Source.Address.Bus = &abus
				pcistring = pcistring + strconv.Itoa((int)(abus))
				pcistring += "1."
				log.Printf("[DEBUG] hostdev bus found %s", prefix)
			} else {
				pcistring += "0."
			}
			if sourceSlot, ok := d.GetOk(prefix + ".slot"); ok {
				var aslot uint = (uint)(sourceSlot.(int))
				hostdev.SubsysPCI.Source.Address.Slot = &aslot
				log.Printf("[DEBUG] hostdev slot found %s", prefix)
				pcistring = pcistring + strconv.Itoa((int)(aslot))
				pcistring += "."
			} else {
				pcistring += "0."
			}
			if sourceFunction, ok := d.GetOk(prefix + ".function"); ok {
				var afunction uint = (uint)(sourceFunction.(int))
				hostdev.SubsysPCI.Source.Address.Function = &afunction
				pcistring += strconv.Itoa((int)(afunction))
			} else {
				pcistring += "0"
			}
			domainDef.Devices.Hostdevs = append(domainDef.Devices.Hostdevs, hostdev)
			//udpateTableContent("VGA", pcistring, domainDef, vmidNumber)
		case "usb":
			hostdev.SubsysUSB = &libvirtxml.DomainHostdevSubsysUSB{}
			hostdev.SubsysUSB.Source = &libvirtxml.DomainHostdevSubsysUSBSource{}
			hostdev.SubsysUSB.Source.Address = &libvirtxml.DomainAddressUSB{}
			if deviceName, ok := d.GetOk(prefix + ".name"); ok {
				//hostdevenabled, ok := d.GetOk(prefix + ".enabled")
				//if !ok || hostdevenabled.(int) != 1 {
				//	//udpateTableContent(deviceName.(string), "NA", domainDef, vmidNumber)
				//	return
				//} else {
				//	//udpateTableContent(deviceName.(string), "passthrough", domainDef, vmidNumber)
				//}

				if usbBus, ok := d.GetOk(prefix + ".bus"); ok {
					var aBus uint = (uint)(usbBus.(int))
					hostdev.SubsysUSB.Source.Address.Bus = &aBus
				} else {
					fmt.Errorf("Bus for hostdev USB missing '%s'", deviceName)
				}
				if usbDevice, ok := d.GetOk(prefix + ".device"); ok {
					var aDevice uint = (uint)(usbDevice.(int))
					hostdev.SubsysUSB.Source.Address.Device = &aDevice
				} else {
					fmt.Errorf("Device for hostdev USB missing")
				}

				domainDef.Devices.Hostdevs = append(domainDef.Devices.Hostdevs, hostdev)
			}
		}
	}
}

func setMemoryBacking(d *schema.ResourceData, domainDef *libvirtxml.Domain) {

	prefix := "memorybacking.0"
	if _, ok := d.GetOk(prefix); ok {
		domainDef.MemoryBacking = &libvirtxml.DomainMemoryBacking{}
		hugepages, ok := d.GetOk(prefix + ".hugepages")
		if !ok {
			fmt.Println("hugepage in memorybacking is not properly defined!")
		} else {
			domainDef.MemoryBacking.MemoryHugePages = &libvirtxml.DomainMemoryHugepages{}
			domainDef.MemoryBacking.MemoryHugePages.Hugepages = []libvirtxml.DomainMemoryHugepage{
				{Size: uint(hugepages.(int)), Unit: "KiB"},
			}
			domainDef.MemoryBacking.MemoryNosharepages = &libvirtxml.DomainMemoryNosharepages{}
			domainDef.MemoryBacking.MemorySource = &libvirtxml.DomainMemorySource{
				Type: "memfd",
			}
			domainDef.MemoryBacking.MemoryAccess = &libvirtxml.DomainMemoryAccess{
				Mode: "shared",
			}
			domainDef.MemoryBacking.MemoryAllocation = &libvirtxml.DomainMemoryAllocation{
				Mode: "immediate",
			}
		}
	}
}
func setGraphics(d *schema.ResourceData, domainDef *libvirtxml.Domain, arch string) error {
	// For aarch64, s390x, ppc64 and ppc64le spice is not supported
	var vmidNumber int
	if arch == "aarch64" || arch == "s390x" || strings.HasPrefix(arch, "ppc64") {
		domainDef.Devices.Graphics = nil
		return nil
	}

	prefix := "graphics.0"
	if _, ok := d.GetOk(prefix); ok {
		domainDef.Devices.Graphics = []libvirtxml.DomainGraphic{{}}
		graphicsType, ok := d.GetOk(prefix + ".type")
		if !ok {
			return fmt.Errorf("missing graphics type for domain")
		}

		if graphicsType == "gtk" {
			domainDef.QEMUCommandline = &libvirtxml.DomainQEMUCommandline{}
			domainDef.QEMUCommandline.Args = []libvirtxml.DomainQEMUCommandlineArg{
				{Value: "-device"},
				{Value: "virtio-vga,blob=true"},
				{Value: "-display"},
				{Value: "gtk,gl=on"},
			}
			domainDef.QEMUCommandline.Envs = []libvirtxml.DomainQEMUCommandlineEnv{
				{Name: "DISPLAY", Value: ":0"},
			}
			//setHostDev(d, domainDef)
			udpateTableContent("graphics", graphicsType.(string), domainDef, vmidNumber)
			return nil
		}

		setVideo(d, domainDef)

		autoport := d.Get(prefix + ".autoport").(bool)
		listener := libvirtxml.DomainGraphicListener{}

		if listenType, ok := d.GetOk(prefix + ".listen_type"); ok {
			switch listenType {
			case "address":
				listenAddress := d.Get(prefix + ".listen_address")
				listener.Address = &libvirtxml.DomainGraphicListenerAddress{
					Address: listenAddress.(string),
				}
			case "network":
				listener.Network = &libvirtxml.DomainGraphicListenerNetwork{}
			case "socket":
				listener.Socket = &libvirtxml.DomainGraphicListenerSocket{}
			}
		}

		switch graphicsType {
		case "spice":
			domainDef.Devices.Graphics[0] = libvirtxml.DomainGraphic{
				Spice: &libvirtxml.DomainGraphicSpice{},
			}
			domainDef.Devices.Graphics[0].Spice.AutoPort = formatBoolYesNo(autoport)
			domainDef.Devices.Graphics[0].Spice.Listeners = []libvirtxml.DomainGraphicListener{
				listener,
			}
		case "vnc":
			domainDef.Devices.Graphics[0] = libvirtxml.DomainGraphic{
				VNC: &libvirtxml.DomainGraphicVNC{},
			}
			domainDef.Devices.Graphics[0].VNC.AutoPort = formatBoolYesNo(autoport)
			domainDef.Devices.Graphics[0].VNC.Listeners = []libvirtxml.DomainGraphicListener{
				listener,
			}

			if websocket, ok := d.GetOk(prefix + ".websocket"); ok {
				domainDef.Devices.Graphics[0].VNC.WebSocket = websocket.(int)
			}
		default:
			return fmt.Errorf("this provider only supports vnc/spice as graphics type. Provided: '%s'", graphicsType)
		}
	}
	return nil
}

func setCmdlineArgs(d *schema.ResourceData, domainDef *libvirtxml.Domain) {
	var cmdlineArgs []string
	for i := 0; i < d.Get("cmdline.#").(int); i++ {
		for k, v := range d.Get(fmt.Sprintf("cmdline.%d", i)).(map[string]interface{}) {
			var cmd string
			if k == "_" {
				// keyless cmd (eg: nosplash)
				cmd = fmt.Sprintf("%v", v)
			} else {
				cmd = fmt.Sprintf("%s=%v", k, v)
			}
			cmdlineArgs = append(cmdlineArgs, cmd)
		}
	}
	sort.Strings(cmdlineArgs)
	domainDef.OS.Cmdline = strings.Join(cmdlineArgs, " ")
}

func setFirmware(d *schema.ResourceData, domainDef *libvirtxml.Domain) {
	if firmware, ok := d.GetOk("firmware"); ok {
		firmwareFile := firmware.(string)
		domainDef.OS.Loader = &libvirtxml.DomainLoader{
			Path:     firmwareFile,
			Readonly: "yes",
			Type:     "pflash",
			Secure:   "no",
		}

		if _, ok := d.GetOk("nvram.0"); ok {
			nvramFile := d.Get("nvram.0.file").(string)
			nvramTemplateFile := ""
			if nvramTemplate, ok := d.GetOk("nvram.0.template"); ok {
				nvramTemplateFile = nvramTemplate.(string)
			}
			domainDef.OS.NVRam = &libvirtxml.DomainNVRam{
				NVRam:    nvramFile,
				Template: nvramTemplateFile,
			}
		}
	}
}

func setBootDevices(d *schema.ResourceData, domainDef *libvirtxml.Domain) {
	for i := 0; i < d.Get("boot_device.#").(int); i++ {
		if bootMap, ok := d.GetOk(fmt.Sprintf("boot_device.%d.dev", i)); ok {
			for _, dev := range bootMap.([]interface{}) {
				domainDef.OS.BootDevices = append(domainDef.OS.BootDevices,
					libvirtxml.DomainBootDevice{
						Dev: dev.(string),
					})
			}
		}
	}
}

func setConsoles(d *schema.ResourceData, domainDef *libvirtxml.Domain) {
	for i := 0; i < d.Get("console.#").(int); i++ {
		console := libvirtxml.DomainConsole{}
		prefix := fmt.Sprintf("console.%d", i)
		consoleTargetPortInt, err := strconv.Atoi(d.Get(prefix + ".target_port").(string))
		if err == nil {
			consoleTargetPort := uint(consoleTargetPortInt)
			console.Target = &libvirtxml.DomainConsoleTarget{
				Port: &consoleTargetPort,
			}
		}
		if targetType, ok := d.GetOk(prefix + ".target_type"); ok {
			if console.Target == nil {
				console.Target = &libvirtxml.DomainConsoleTarget{}
			}
			console.Target.Type = targetType.(string)
		}
		switch d.Get(prefix + ".type").(string) {
		case "tcp":
			sourceHost := d.Get(prefix + ".source_host")
			sourceService := d.Get(prefix + ".source_service")
			console.Source = &libvirtxml.DomainChardevSource{
				TCP: &libvirtxml.DomainChardevSourceTCP{
					Mode:    "bind",
					Host:    sourceHost.(string),
					Service: sourceService.(string),
				},
			}
			console.Protocol = &libvirtxml.DomainChardevProtocol{
				Type: "telnet",
			}
		case "pty":
			if sourcePath, ok := d.GetOk(prefix + ".source_path"); ok {
				console.Source = &libvirtxml.DomainChardevSource{
					Pty: &libvirtxml.DomainChardevSourcePty{
						Path: sourcePath.(string),
					},
				}
			}
		default:
			if sourcePath, ok := d.GetOk(prefix + ".source_path"); ok {
				console.Source = &libvirtxml.DomainChardevSource{
					Dev: &libvirtxml.DomainChardevSourceDev{
						Path: sourcePath.(string),
					},
				}
			}
		}
		domainDef.Devices.Consoles = append(domainDef.Devices.Consoles, console)
	}
}

func setDisks(d *schema.ResourceData, domainDef *libvirtxml.Domain, virConn *libvirt.Libvirt) error {
	var scsiDisk = false
	var numOfISOs = 0
	var numOfSCSIs = 0
	log.Printf("[DEBUG] debugSetdisk is local file start")
	for i := 0; i < d.Get("disk.#").(int); i++ {
		disk := newDefDisk(i)

		prefix := fmt.Sprintf("disk.%d", i)
		if d.Get(prefix + ".scsi").(bool) {
			disk.Target = &libvirtxml.DomainDiskTarget{
				Dev: fmt.Sprintf("sd%s", diskLetterForIndex(numOfSCSIs)),
				Bus: "scsi",
			}
			scsiDisk = true
			if wwn, ok := d.GetOk(prefix + ".wwn"); ok {
				disk.WWN = wwn.(string)
			} else {
				//nolint:gomnd
				disk.WWN = randomWWN(10)
			}

			numOfSCSIs++
		}

		if volumeKey, ok := d.GetOk(prefix + ".volume_id"); ok {
			log.Printf("[WARN] Disk volume not supported at the moment  %s", volumeKey.(string))
			return nil
			//	diskVolume, err := virConn.StorageVolLookupByKey(volumeKey.(string))
			//	if err != nil {
			//		return fmt.Errorf("can't retrieve volume %s: %w", volumeKey.(string), err)
			//	}
			//
			//	diskPool, err := virConn.StoragePoolLookupByVolume(diskVolume)
			//	if err != nil {
			//		return fmt.Errorf("can't retrieve pool for volume %s", volumeKey.(string))
			//	}
			//
			//	// find out the format of the volume in order to set the appropriate
			//	// driver
			//	volumeDef, err := newDefVolumeFromLibvirt(virConn, diskVolume)
			//	if err != nil {
			//		return err
			//	}
			//	if volumeDef.Target != nil && volumeDef.Target.Format != nil && volumeDef.Target.Format.Type != "" {
			//		if volumeDef.Target.Format.Type == "qcow2" {
			//			log.Print("[DEBUG] Setting disk driver to 'qcow2' to match disk volume format")
			//			disk.Driver = &libvirtxml.DomainDiskDriver{
			//				Name: "qemu",
			//				Type: "qcow2",
			//			}
			//		}
			//		if volumeDef.Target.Format.Type == "raw" {
			//			log.Print("[DEBUG] Setting disk driver to 'raw' to match disk volume format")
			//			disk.Driver = &libvirtxml.DomainDiskDriver{
			//				Name: "qemu",
			//				Type: "raw",
			//			}
			//		}
			//	} else {
			//		log.Printf("[WARN] Disk volume has no format specified: %s", volumeKey.(string))
			//	}

			//disk.Source = &libvirtxml.DomainDiskSource{
			//	Volume: &libvirtxml.DomainDiskSourceVolume{
			//		Pool:   diskPool.Name,
			//		Volume: diskVolume.Name,
			//	},
			//}
		} else if rawURL, ok := d.GetOk(prefix + ".url"); ok {
			// Support for remote, read-only http disks
			// useful for booting CDs
			url, err := url.Parse(rawURL.(string))
			if err != nil {
				return err
			}

			disk.Source = &libvirtxml.DomainDiskSource{
				Network: &libvirtxml.DomainDiskSourceNetwork{
					Protocol: url.Scheme,
					Name:     url.Path,
					Hosts: []libvirtxml.DomainDiskSourceHost{
						{
							Name: url.Hostname(),
							Port: url.Port(),
						},
					},
				},
			}

			if strings.HasSuffix(url.Path, ".iso") {
				disk.Device = "cdrom"
				disk.Target = &libvirtxml.DomainDiskTarget{
					Dev: fmt.Sprintf("hd%s", diskLetterForIndex(numOfISOs)),
					Bus: "ide",
				}
				disk.Driver = &libvirtxml.DomainDiskDriver{
					Name: "qemu",
				}
				numOfISOs++
			}

			if !strings.HasSuffix(url.Path, ".qcow2") {
				disk.Driver.Type = "raw"
			}
		} else if file, ok := d.GetOk(prefix + ".file"); ok {
			// support for local disks, e.g. CDs
			log.Printf("[DEBUG] debugSetdisk is local file : %s", file.(string))
			disk.Source = &libvirtxml.DomainDiskSource{
				File: &libvirtxml.DomainDiskSourceFile{
					File: file.(string),
				},
			}

			if strings.HasSuffix(file.(string), ".iso") {
				disk.Device = "cdrom"
				disk.Target = &libvirtxml.DomainDiskTarget{
					Dev: fmt.Sprintf("hd%s", diskLetterForIndex(numOfISOs)),
					Bus: "ide",
				}
				disk.Driver = &libvirtxml.DomainDiskDriver{
					Name: "qemu",
					Type: "raw",
				}

				numOfISOs++
			}

			if !strings.HasSuffix(file.(string), ".qcow2") {
				disk.Driver.Type = "raw"
			} else {
				log.Printf("[DEBUG] where is driverType?")
				disk.Driver.Type = "qcow2"
			}
		} else if blockDev, ok := d.GetOk(prefix + ".block_device"); ok {
			disk.Source = &libvirtxml.DomainDiskSource{
				Block: &libvirtxml.DomainDiskSourceBlock{
					Dev: blockDev.(string),
				},
			}

			disk.Driver.Type = "raw"
		}

		domainDef.Devices.Disks = append(domainDef.Devices.Disks, disk)
	}

	log.Printf("[DEBUG] scsiDisk: %t", scsiDisk)
	if scsiDisk {
		domainDef.Devices.Controllers = append(domainDef.Devices.Controllers,
			libvirtxml.DomainController{
				Type:  "scsi",
				Model: "virtio-scsi",
			})
	}

	return nil
}

func setFilesystems(d *schema.ResourceData, domainDef *libvirtxml.Domain) error {
	for i := 0; i < d.Get("filesystem.#").(int); i++ {
		fs := newFilesystemDef()

		prefix := fmt.Sprintf("filesystem.%d", i)
		if accessMode, ok := d.GetOk(prefix + ".accessmode"); ok {
			fs.AccessMode = accessMode.(string)
		}
		if sourceDir, ok := d.GetOk(prefix + ".source"); ok {
			fs.Source = &libvirtxml.DomainFilesystemSource{
				Mount: &libvirtxml.DomainFilesystemSourceMount{
					Dir: sourceDir.(string),
				},
			}
		} else {
			return fmt.Errorf("filesystem entry %d must have a 'source' set", i)
		}
		if targetDir, ok := d.GetOk(prefix + ".target"); ok {
			fs.Target = &libvirtxml.DomainFilesystemTarget{
				Dir: targetDir.(string),
			}
		} else {
			return fmt.Errorf("filesystem entry must have a 'target' set")
		}
		if d.Get(prefix + ".readonly").(bool) {
			fs.ReadOnly = &libvirtxml.DomainFilesystemReadOnly{}
		} else {
			fs.ReadOnly = nil
		}

		domainDef.Devices.Filesystems = append(domainDef.Devices.Filesystems, fs)
	}
	log.Printf("filesystems: %+v\n", domainDef.Devices.Filesystems)
	return nil
}

func setCloudinit(d *schema.ResourceData, domainDef *libvirtxml.Domain, virConn *libvirt.Libvirt) error {
	if cloudinit, ok := d.GetOk("cloudinit"); ok {
		cloudinitID, err := getCloudInitVolumeKeyFromTerraformID(cloudinit.(string))
		if err != nil {
			return err
		}
		disk, err := newDiskForCloudInit(virConn, cloudinitID)
		if err != nil {
			return err
		}
		domainDef.Devices.Disks = append(domainDef.Devices.Disks, disk)
	}

	return nil
}

func setNetworkInterfaces(d *schema.ResourceData, domainDef *libvirtxml.Domain,
	virConn *libvirt.Libvirt, partialNetIfaces map[string]*pendingMapping,
	waitForLeases *[]*libvirtxml.DomainInterface) error {
	for i := 0; i < d.Get("network_interface.#").(int); i++ {
		prefix := fmt.Sprintf("network_interface.%d", i)

		netIface := libvirtxml.DomainInterface{
			Model: &libvirtxml.DomainInterfaceModel{
				Type: "virtio",
			},
		}

		// calculate the MAC address
		var mac string
		if macI, ok := d.GetOk(prefix + ".mac"); ok {
			mac = strings.ToUpper(macI.(string))
		} else {
			var err error
			mac, err = randomMACAddress()
			if err != nil {
				return fmt.Errorf("error generating mac address: %w", err)
			}
		}
		netIface.MAC = &libvirtxml.DomainInterfaceMAC{
			Address: mac,
		}

		// this is not passed to libvirt, but used by waitForAddress
		if waitForLease, ok := d.GetOk(prefix + ".wait_for_lease"); ok {
			if waitForLease.(bool) {
				*waitForLeases = append(*waitForLeases, &netIface)
			}
		}

		// connect to the interface to the network... first, look for the network
		var network libvirt.Network
		var err error

		if networkName, ok := d.GetOk(prefix + ".network_name"); ok {
			network, err = virConn.NetworkLookupByName(networkName.(string))
			if err != nil {
				return fmt.Errorf("can't retrieve network '%s'", networkName.(string))
			}
		} else if networkUUID, ok := d.GetOk(prefix + ".network_id"); ok {
			// when using a "network_id" we are referring to a "network resource"
			// we have defined somewhere else...
			uuid := parseUUID(networkUUID.(string))
			network, err = virConn.NetworkLookupByUUID(uuid)
			if err != nil {
				return fmt.Errorf("can't retrieve network ID %s", networkUUID)
			}

		} else if bridgeNameI, ok := d.GetOk(prefix + ".bridge"); ok {
			netIface.Source = &libvirtxml.DomainInterfaceSource{
				Bridge: &libvirtxml.DomainInterfaceSourceBridge{
					Bridge: bridgeNameI.(string),
				},
			}
		} else if devI, ok := d.GetOk(prefix + ".vepa"); ok {
			netIface.Source = &libvirtxml.DomainInterfaceSource{
				Direct: &libvirtxml.DomainInterfaceSourceDirect{
					Dev:  devI.(string),
					Mode: "vepa",
				},
			}
		} else if devI, ok := d.GetOk(prefix + ".macvtap"); ok {
			netIface.Source = &libvirtxml.DomainInterfaceSource{
				Direct: &libvirtxml.DomainInterfaceSourceDirect{
					Dev:  devI.(string),
					Mode: "bridge",
				},
			}
		} else if devI, ok := d.GetOk(prefix + ".passthrough"); ok {
			netIface.Source = &libvirtxml.DomainInterfaceSource{
				Direct: &libvirtxml.DomainInterfaceSourceDirect{
					Dev:  devI.(string),
					Mode: "passthrough",
				},
			}
		} else {
			log.Printf("[WARN] no network has been specified")
		}

		if network.Name != "" {
			networkDef, err := getXMLNetworkDefFromLibvirt(virConn, network)

			// only for DHCP, we update the host table of the network
			if HasDHCP(networkDef) {
				hostname := domainDef.Name
				if hostnameI, ok := d.GetOk(prefix + ".hostname"); ok {
					hostname = hostnameI.(string)
				}
				if addresses, ok := d.GetOk(prefix + ".addresses"); ok {
					// some IP(s) provided
					for _, addressI := range addresses.([]interface{}) {
						address := addressI.(string)
						ip := net.ParseIP(address)
						if ip == nil {
							return fmt.Errorf("could not parse addresses '%s'", address)
						}

						log.Printf("[INFO] Adding IP/MAC/host=%s/%s/%s to %s", ip.String(), mac, hostname, network.Name)
						if err := updateOrAddHost(virConn, network, ip.String(), mac, hostname); err != nil {
							return err
						}
					}
				} else {
					// no IPs provided: if the hostname has been provided, wait until we get an IP
					wait := false
					for _, iface := range *waitForLeases {
						if iface == &netIface {
							wait = true
							break
						}
					}
					if wait {
						// the resource specifies a hostname but not an IP, so we must wait until we
						// have a valid lease and then read the IP we have been assigned, so we can
						// do the mapping
						log.Printf("[DEBUG] Do not have an IP for '%s' yet: will wait until DHCP provides one...", hostname)
						if err != nil {
							return err
						}
						partialNetIfaces[strings.ToUpper(mac)] = &pendingMapping{
							mac:         strings.ToUpper(mac),
							hostname:    hostname,
							networkName: network.Name,
						}
					}
				}
			}

			netIface.Source = &libvirtxml.DomainInterfaceSource{
				Network: &libvirtxml.DomainInterfaceSourceNetwork{
					Network: network.Name,
				},
			}
		}

		domainDef.Devices.Interfaces = append(domainDef.Devices.Interfaces, netIface)
	}

	return nil
}

func setTPMs(d *schema.ResourceData, domainDef *libvirtxml.Domain) {
	prefix := "tpm.0"
	if _, ok := d.GetOk(prefix); ok {
		tpm := libvirtxml.DomainTPM{}
		if model, ok := d.GetOk(".model"); ok {
			tpm.Model = model.(string)
		}

		if backendType, ok := d.GetOk(prefix + ".backend_type"); ok {
			tpm.Backend = &libvirtxml.DomainTPMBackend{}
			switch backendType {
			case "passthrough":
				tpm.Backend.Passthrough = &libvirtxml.DomainTPMBackendPassthrough{}
			case "emulator":
				tpm.Backend.Emulator = &libvirtxml.DomainTPMBackendEmulator{}
			}
		}

		if tpm.Backend.Passthrough != nil {
			if devicePath, ok := d.GetOk(prefix + ".backend_device_path"); ok {
				tpm.Backend.Passthrough.Device = &libvirtxml.DomainTPMBackendDevice{
					Path: devicePath.(string),
				}
			}
		}
		if tpm.Backend.Emulator != nil {
			if encryptionSecret, ok := d.GetOk(prefix + ".backend_encryption_secret"); ok {
				tpm.Backend.Emulator.Encryption = &libvirtxml.DomainTPMBackendEncryption{
					Secret: encryptionSecret.(string),
				}
			}
			if backendVersion, ok := d.GetOk(prefix + ".backend_version"); ok {
				tpm.Backend.Emulator.Version = backendVersion.(string)
			}
			if backendPersistentState, ok := d.GetOk(prefix + ".backend_persistent_state"); ok {
				tpm.Backend.Emulator.PersistentState = formatBoolYesNo(backendPersistentState.(bool))
			}
		}
		domainDef.Devices.TPMs = append(domainDef.Devices.TPMs, tpm)
	}
}

func destroyDomainByUserRequest(virConn *libvirt.Libvirt, d *schema.ResourceData, domain libvirt.Domain) error {
	if d.Get("running").(bool) {
		return nil
	}

	log.Printf("Destroying libvirt domain %s", uuidString(domain.UUID))
	state, _, err := virConn.DomainGetState(domain, 0)
	if err != nil {
		return fmt.Errorf("couldn't get info about domain: %w", err)
	}

	if libvirt.DomainState(state) == libvirt.DomainRunning || libvirt.DomainState(state) == libvirt.DomainPaused {
		if err := virConn.DomainDestroy(domain); err != nil {
			return fmt.Errorf("couldn't destroy libvirt domain: %w", err)
		}
	}

	return nil
}
