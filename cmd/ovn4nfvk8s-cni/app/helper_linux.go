// +build linux

package app

import (
	"fmt"
	"net"
	"os/exec"
	"ovn4nfv-k8s-plugin/internal/pkg/config"
	"ovn4nfv-k8s-plugin/internal/pkg/network"
	"ovn4nfv-k8s-plugin/internal/pkg/ovn"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/prometheus/common/log"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

func renameLink(curName, newName string) error {
	link, err := netlink.LinkByName(curName)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetDown(link); err != nil {
		return err
	}
	if err := netlink.LinkSetName(link, newName); err != nil {
		return err
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return err
	}

	return nil
}

//Todo Comments
func CreateNodeOVSInternalPort(nodeintfipaddr, nodeintfmacaddr, node string) error {
	nodeName := strings.ToLower(node)
	nodeOVSInternalIntfName := config.GetNodeIntfName(nodeName)

	hwAddr, err := net.ParseMAC(nodeintfmacaddr)
	if err != nil {
		logrus.Errorf("Error is converting %q to net hwaddr: %v", nodeOVSInternalIntfName, err)
		return fmt.Errorf("Error is converting %q to net hwaddr: %v", nodeOVSInternalIntfName, err)
	}

	ovsArgs := []string{
		"add-port", "br-int", nodeOVSInternalIntfName, "--", "set",
		"interface", nodeOVSInternalIntfName, "type=internal",
		fmt.Sprintf("mac_in_use=%s", strings.ReplaceAll(hwAddr.String(), ":", "\\:")),
		fmt.Sprintf("mac=%s", strings.ReplaceAll(hwAddr.String(), ":", "\\:")),
		fmt.Sprintf("external_ids:iface-id=%s", nodeOVSInternalIntfName),
	}
	logrus.Infof("ovs-vsctl args - %v", ovsArgs)

	//var out []byte
	out, err := exec.Command("ovs-vsctl", ovsArgs...).CombinedOutput()
	if err != nil {
		logrus.Errorf("failure in creating Node OVS internal port - %s: %v - %q", nodeOVSInternalIntfName, err, string(out))
		return fmt.Errorf("failure in creating Node OVS internal port - %s: %v - %q", nodeOVSInternalIntfName, err, string(out))
	}
	logrus.Infof("ovs-vsctl args - %v output:%v", ovsArgs, string(out))

	link, err := netlink.LinkByName(nodeOVSInternalIntfName)
	if err != nil {
		logrus.Errorf("failed to get netlink for Node OVS internal port %s: %v", nodeOVSInternalIntfName, err)
		return fmt.Errorf("failed to get netlink for Node OVS internal port %s: %v", nodeOVSInternalIntfName, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		logrus.Errorf("failed to set up netlink for Node OVS internal port %s: %v", nodeOVSInternalIntfName, err)
		return fmt.Errorf("failed to set up netlink for Node OVS internal port %s: %v", nodeOVSInternalIntfName, err)
	}

	addr, err := netlink.ParseAddr(nodeintfipaddr)
	if err != nil {
		logrus.Errorf("failed to parse IP addr %s: %v", nodeintfipaddr, err)
		return fmt.Errorf("failed to parse IP addr %s: %v", nodeintfipaddr, err)
	}
	err = netlink.AddrAdd(link, addr)
	if err != nil {
		logrus.Errorf("failed to parse IP addr %s: %v", nodeintfipaddr, err)
		return fmt.Errorf("failed to add IP addr %s to %s: %v", nodeintfipaddr, nodeOVSInternalIntfName, err)
	}

	err = network.SetupAndEnsureIPTables(network.MasqRules(nodeOVSInternalIntfName))
	if err != nil {
		logrus.Errorf("failed to apply snat rule for %s: %v", nodeOVSInternalIntfName, err)
		return fmt.Errorf("failed to apply snat rule for %s: %v", nodeOVSInternalIntfName, err)
	}

	return nil
}

func setupInterface(netns ns.NetNS, containerID, ifName, macAddress, ipAddress, gatewayIP, defaultGateway string, idx, mtu int, isDefaultGW bool) (*current.Interface, *current.Interface, error) {
	hostIface := &current.Interface{}
	contIface := &current.Interface{}
	var hostNet string

	if defaultGateway == "false" && isDefaultGW == true && ifName == "eth0" {
		var err error
		hostNet, err = network.GetHostNetwork()
		if err != nil {
			log.Error(err, "Failed to get host network")
			return nil, nil, fmt.Errorf("failed to get host network: %v", err)
		}
	}

	var oldHostVethName string
	err := netns.Do(func(hostNS ns.NetNS) error {
		// create the veth pair in the container and move host end into host netns
		hostVeth, containerVeth, err := ip.SetupVeth(ifName, mtu, hostNS)
		if err != nil {
			return fmt.Errorf("failed to setup veth %s: %v", ifName, err)
			//return err
		}
		hostIface.Mac = hostVeth.HardwareAddr.String()
		contIface.Name = containerVeth.Name

		link, err := netlink.LinkByName(contIface.Name)
		if err != nil {
			return fmt.Errorf("failed to lookup %s: %v", contIface.Name, err)
		}

		hwAddr, err := net.ParseMAC(macAddress)
		if err != nil {
			return fmt.Errorf("failed to parse mac address for %s: %v", contIface.Name, err)
		}
		err = netlink.LinkSetHardwareAddr(link, hwAddr)
		if err != nil {
			return fmt.Errorf("failed to add mac address %s to %s: %v", macAddress, contIface.Name, err)
		}
		contIface.Mac = macAddress
		contIface.Sandbox = netns.Path()

		addr, err := netlink.ParseAddr(ipAddress)
		if err != nil {
			return err
		}
		err = netlink.AddrAdd(link, addr)
		if err != nil {
			return fmt.Errorf("failed to add IP addr %s to %s: %v", ipAddress, contIface.Name, err)
		}

		if defaultGateway == "true" {
			gw := net.ParseIP(gatewayIP)
			if gw == nil {
				return fmt.Errorf("parse ip of gateway failed")
			}
			err = ip.AddRoute(nil, gw, link)
			if err != nil {
				logrus.Errorf("ip.AddRoute failed %v gw %v link %v", err, gw, link)
				return err
			}
		}

		if defaultGateway == "false" && isDefaultGW == true && ifName == "eth0" {
			stdout, stderr, err := ovn.RunIP("route", "add", hostNet, "via", gatewayIP)
			if err != nil && !strings.Contains(stderr, "RTNETLINK answers: File exists") {
				logrus.Errorf("Failed to ip route add stout %s, stderr %s, err %v", stdout, stderr, err)
				return fmt.Errorf("Failed to ip route add stout %s, stderr %s, err %v", stdout, stderr, err)
			}
		}

		oldHostVethName = hostVeth.Name

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// rename the host end of veth pair
	hostIface.Name = containerID[:14] + strconv.Itoa(idx)
	if err := renameLink(oldHostVethName, hostIface.Name); err != nil {
		return nil, nil, fmt.Errorf("failed to rename %s to %s: %v", oldHostVethName, hostIface.Name, err)
	}

	return hostIface, contIface, nil
}

// ConfigureInterface sets up the container interface
var ConfigureInterface = func(containerNetns, containerID, ifName, namespace, podName, macAddress, ipAddress, gatewayIP, interfaceName, defaultGateway string, idx, mtu int, isDefaultGW bool) ([]*current.Interface, error) {
	netns, err := ns.GetNS(containerNetns)
	if err != nil {
		return nil, fmt.Errorf("failed to open netns %q: %v", containerNetns, err)
	}
	defer netns.Close()

	var ifaceID string
	if interfaceName != "*" {
		ifaceID = fmt.Sprintf("%s_%s_%s", namespace, podName, interfaceName)
	} else {
		ifaceID = fmt.Sprintf("%s_%s", namespace, podName)
		interfaceName = ifName
	}
	hostIface, contIface, err := setupInterface(netns, containerID, interfaceName, macAddress, ipAddress, gatewayIP, defaultGateway, idx, mtu, isDefaultGW)
	if err != nil {
		return nil, err
	}

	ovsArgs := []string{
		"add-port", "br-int", hostIface.Name, "--", "set",
		"interface", hostIface.Name,
		fmt.Sprintf("external_ids:attached_mac=%s", macAddress),
		fmt.Sprintf("external_ids:iface-id=%s", ifaceID),
		fmt.Sprintf("external_ids:ip_address=%s", ipAddress),
		fmt.Sprintf("external_ids:sandbox=%s", containerID),
	}

	var out []byte
	out, err = exec.Command("ovs-vsctl", ovsArgs...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failure in plugging pod interface: %v\n  %q", err, string(out))
	}

	return []*current.Interface{hostIface, contIface}, nil
}

func setupRoute(netns ns.NetNS, dst, gw, dev string) error {
	// Add Route to the namespace
	err := netns.Do(func(_ ns.NetNS) error {
		dstAddr, dstAddrNet, _ := net.ParseCIDR(dst)
		ipNet := net.IPNet{IP: dstAddr, Mask: dstAddrNet.Mask}
		link, err := netlink.LinkByName(dev)
		err = ip.AddRoute(&ipNet, net.ParseIP(gw), link)
		if err != nil {
			logrus.Errorf("ip.AddRoute failed %v dst %v gw %v", err, dst, gw)
		}
		return err
	})
	return err
}

// ConfigureRoute sets up the container routes
var ConfigureRoute = func(containerNetns, dst, gw, dev string) error {
	netns, err := ns.GetNS(containerNetns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", containerNetns, err)
	}
	defer netns.Close()
	err = setupRoute(netns, dst, gw, dev)
	return err
}

// PlatformSpecificCleanup deletes the OVS port
func PlatformSpecificCleanup(ifaceName string) (bool, error) {
	done := false
	ovsArgs := []string{
		"del-port", "br-int", ifaceName,
	}
	out, err := exec.Command("ovs-vsctl", ovsArgs...).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "no port named") {
		// DEL should be idempotent; don't return an error just log it
		logrus.Warningf("failed to delete OVS port %s: %v\n  %q", ifaceName, err, string(out))
		done = true
	}

	return done, nil
}
