package network

import (
	"errors"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
)

//GetDefaultGateway return default gateway of the network namespace
func GetDefaultGateway() (string, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return "", err
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			if route.Gw.To4() == nil {
				return "", errors.New("Found default route but could not determine gateway")
			}
			return route.Gw.To4().String(), nil
		}
	}

	return "", errors.New("Unable to find default route")
}

// GetDefaultGatewayInterface return default gateway interface link
func GetDefaultGatewayInterface() (*net.Interface, error) {
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return nil, err
	}

	for _, route := range routes {
		if route.Dst == nil || route.Dst.String() == "0.0.0.0/0" {
			if route.LinkIndex <= 0 {
				return nil, errors.New("Found default route but could not determine interface")
			}
			return net.InterfaceByIndex(route.LinkIndex)
		}
	}

	return nil, errors.New("Unable to find default route")
}

func getIfaceAddrs(iface *net.Interface) ([]netlink.Addr, error) {

	link := &netlink.Device{
		netlink.LinkAttrs{
			Index: iface.Index,
		},
	}

	return netlink.AddrList(link, syscall.AF_INET)
}

//GetInterfaceIP4Addr return IP4addr of a interface
func GetInterfaceIP4Addr(iface *net.Interface) (netlink.Addr, error) {
	addrs, err := getIfaceAddrs(iface)
	if err != nil {
		return netlink.Addr{}, err
	}

	// prefer non link-local addr
	var ll netlink.Addr

	for _, addr := range addrs {
		if addr.IP.To4() == nil {
			continue
		}

		if addr.IP.IsGlobalUnicast() {
			return addr, nil
		}

		if addr.IP.IsLinkLocalUnicast() {
			ll = addr
		}
	}

	if ll.IP.To4() != nil {
		// didn't find global but found link-local. it'll do.
		return ll, nil
	}

	return netlink.Addr{}, errors.New("No IPv4 address found for given interface")
}

//GetHostNetwork return default gateway interface network
func GetHostNetwork() (string, error) {

	iface, err := GetDefaultGatewayInterface()
	if err != nil {
		log.Error(err, "error in gettting default gateway interface")
		return "", err
	}

	ipv4addr, err := GetInterfaceIP4Addr(iface)
	if err != nil {
		log.Error(err, "error in gettting default gateway interface IPv4 address")
		return "", err
	}

	_, ipv4Net, err := net.ParseCIDR(ipv4addr.IPNet.String())
	if err != nil {
		log.Error(err, "error in gettting default gateway interface network")
		return "", err
	}

	return ipv4Net.String(), nil
}
