//go:build linux

// Package collector gathers network state from the Linux kernel.
package collector

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// Interface mirrors models.InterfaceInfo but lives in the agent to avoid
// importing the center module.
type Interface struct {
	Name      string
	MAC       string
	State     string // "up" | "down"
	MTU       int
	SpeedMbps int64
	Addresses []Address
}

// Address is a single IP address (with prefix) assigned to an interface.
type Address struct {
	Address string // CIDR, e.g. "192.168.1.10/24"
	Family  string // "ipv4" | "ipv6"
}

// CollectInterfaces returns the current list of network interfaces with
// their assigned IP addresses.
func CollectInterfaces() ([]Interface, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("netlink link list: %w", err)
	}

	var ifaces []Interface
	for _, link := range links {
		attrs := link.Attrs()
		if attrs.Name == "lo" {
			continue // skip loopback
		}

		state := "down"
		if attrs.Flags&net.FlagUp != 0 {
			state = "up"
		}

		iface := Interface{
			Name:  attrs.Name,
			MAC:   attrs.HardwareAddr.String(),
			State: state,
			MTU:   attrs.MTU,
		}

		// Gather both IPv4 and IPv6 addresses.
		for _, family := range []int{netlink.FAMILY_V4, netlink.FAMILY_V6} {
			addrs, err := netlink.AddrList(link, family)
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				if addr.IP.IsLinkLocalUnicast() {
					continue // skip link-local
				}
				fam := "ipv4"
				if family == netlink.FAMILY_V6 {
					fam = "ipv6"
				}
				iface.Addresses = append(iface.Addresses, Address{
					Address: addr.IPNet.String(),
					Family:  fam,
				})
			}
		}

		ifaces = append(ifaces, iface)
	}
	return ifaces, nil
}
