package kvm

import (
	"log"
	"strconv"

	"github.com/atsushinee/go-markdown-generator/doc"
	"libvirt.org/go/libvirtxml"
)

var markdownIndex int = 0
var bookMd *doc.MarkDownDoc = nil
var tblMd *doc.Table = nil
var tbl_ubuntu_vm_info *doc.Table
var tbl_windows_vm_info *doc.Table
var kvm_mos_md_index map[string]int
var kvm_mos_md_desc map[string]string
var vmcount int

func createMarkdownTbl(size int) (*doc.MarkDownDoc, *doc.Table) {
	if bookMd == nil || tblMd == nil {
		log.Print("[DEBUG] create markdown table")
		bookMd = doc.NewMarkDown()
		tblMd = doc.NewTable(11, size+2)
		tbl_ubuntu_vm_info = doc.NewTable(2, 2)
		tbl_windows_vm_info = doc.NewTable(2, 2)

		kvm_mos_md_index = map[string]int{
			"name":      0,
			"memory":    1,
			"vcpu":      2,
			"arch":      3,
			"machine":   4,
			"osvariant": 5,
			"firmware":  6,
			"graphics":  7,
			"network":   8,
			"VGA":       9,
			"USB-Mouse": 10,
		}

		kvm_mos_md_desc = map[string]string{
			"name":      "Name of the VM",
			"memory":    "vm memory in MiB",
			"vcpu":      "vcpu number",
			"arch":      "arch",
			"machine":   "machine types",
			"osvariant": "vm os type",
			"firmware":  "loaded firmware",
			"graphics":  "graphics settings",
			"network":   "network interface setting",
			"VGA":       "VGA PCI configuration",
			"USB-Mouse": "usb mouse configuration",
		}

		bookMd.WriteTitle("Supported Configurations", doc.LevelNormal)

		for i := 1; i <= size; i++ {
			tblMd.SetTitle(i, "VM Guest"+strconv.Itoa(i))
		}
		tblMd.SetTitle(0, "Component")
		//tblMd.SetTitle(1, "VM Guest1")
		//tblMd.SetTitle(2, "VM Guest2")
		//tblMd.SetTitle(3, "Description")
		tblMd.SetTitle(size+1, "Description")

		for key, value := range kvm_mos_md_index {
			tblMd.SetContent(value, 0, key)
			tblMd.SetContent(value, size+1, kvm_mos_md_desc[key])
		}
	}
	return bookMd, tblMd

}

func getMarkdownTbl() (*doc.MarkDownDoc, *doc.Table) {
	if bookMd == nil || tblMd == nil {
		log.Print("[DEBUG] ###### MarkdownTBL not defined!!!")
	}
	return bookMd, tblMd
}

func getvmguestTbl() (*doc.Table, *doc.Table) {
	return tbl_ubuntu_vm_info, tbl_windows_vm_info
}

func udpateTableContent(key string, content string, domainDef *libvirtxml.Domain, col int) {

	if key == "" || key == "null" || content == "" || content == "null" {
		log.Print("[DEBUG] udpateTableContent error input ", col, "  ", content)
	} else {
		log.Print("[DEBUG] update table for vmid ", col, "  ", content)
	}
	//kvm_mos_md_index_te := map[string]int{
	//	"name":      0,
	//	"memory":    1,
	//	"vcpu":      2,
	//	"arch":      3,
	//	"machine":   4,
	//	"osvariant": 5,
	//	"firmware":  6,
	//	"graphics":  7,
	//	"network":   8,
	//	"VGA":       9,
	//	"USB-Mouse": 10,
	//}

	if _, ok := kvm_mos_md_index[key]; !ok {
		log.Print("[DEBUG] udpateTableContent mapkey not exist", key)
		return
	} else {
		_, tbl := getMarkdownTbl()
		if tbl != nil {
			log.Print("[DEBUG] udpateTableContent updated vmid:", col, " key:", key, " content:", content, " column: ", col, "row: ", kvm_mos_md_index[key])
			tbl.SetContent(kvm_mos_md_index[key], col, content)
		} else {
			log.Print("[DEBUG] udpateTableContent tbl not exist", key)
		}
	}

}

func exportMarkdown(book *doc.MarkDownDoc, t *doc.Table, filename string, totalvmNumber int) {
	if book == nil || t == nil {
		log.Print("[DEBUG] error in create Markdown")
		return
	} else {
		vmcount = vmcount + 1
		if vmcount == totalvmNumber {
			book.WriteTable(t)
			//bookMd.WriteTitle("Ubuntu Guest", doc.LevelNormal)
			//book.WriteTable(tbl_ubuntu_vm_info)
			//bookMd.WriteTitle("Windows Guest", doc.LevelNormal)
			//book.WriteTable(tbl_windows_vm_info)
			err := book.Export(filename)
			if err != nil {
				log.Print("[DEBUG] Markdown is created")
			}
		}
	}

}
