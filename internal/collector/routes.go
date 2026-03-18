//go:build linux

package collector

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// Route mirrors models.Route but lives in the agent.
type Route struct {
	Destination   string
	Gateway       string
	InterfaceName string
	Metric        int
	Flags         string
}

// CollectRoutes reads the kernel routing table via netlink.
func CollectRoutes() ([]Route, error) {
	nlRoutes, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return nil, fmt.Errorf("netlink route list: %w", err)
	}

	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("netlink link list for routes: %w", err)
	}
	linkByIdx := make(map[int]string, len(links))
	for _, l := range links {
		linkByIdx[l.Attrs().Index] = l.Attrs().Name
	}

	var routes []Route
	for _, r := range nlRoutes {
		// Skip local/broadcast/multicast table entries.
		if r.Table != 254 { // RT_TABLE_MAIN
			continue
		}

		dst := "0.0.0.0/0"
		if r.Dst != nil {
			dst = r.Dst.String()
		}

		gw := ""
		if r.Gw != nil {
			gw = r.Gw.String()
		}

		ifaceName := linkByIdx[r.LinkIndex]

		flags := routeFlags(r)

		routes = append(routes, Route{
			Destination:   dst,
			Gateway:       gw,
			InterfaceName: ifaceName,
			Metric:        r.Priority,
			Flags:         flags,
		})
	}
	return routes, nil
}

// routeFlags builds a human-readable flags string similar to `route -n`.
func routeFlags(r netlink.Route) string {
	flags := "U" // route is up
	if r.Gw != nil {
		flags += "G"
	}
	if r.Dst != nil {
		if ones, bits := r.Dst.Mask.Size(); ones == bits {
			flags += "H" // host route
		}
	}
	if isLoopback(r) {
		flags += "L"
	}
	return flags
}

func isLoopback(r netlink.Route) bool {
	if r.Dst == nil {
		return false
	}
	return r.Dst.IP.IsLoopback() || net.IP(r.Dst.IP).Equal(net.IPv4(127, 0, 0, 0))
}
