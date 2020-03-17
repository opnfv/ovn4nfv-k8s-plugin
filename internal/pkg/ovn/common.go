package ovn

import (
	"encoding/json"
	"fmt"
	"github.com/vishvananda/netlink"
	"math/big"
	"math/rand"
	"net"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strings"
	"time"
)

var log = logf.Log.WithName("ovn")

// CreateVlan creates VLAN with vlanID
func CreateVlan(vlanID, interfaceName, logicalInterfaceName string) error {
	if interfaceName == "" || vlanID == "" || logicalInterfaceName == "" {
		return fmt.Errorf("CreateVlan invalid parameters: %v %v %v", interfaceName, vlanID, logicalInterfaceName)
	}
	_, err := netlink.LinkByName(logicalInterfaceName)
	if err == nil {
		return err
	}
	stdout, stderr, err := RunIP("link", "add", "link", interfaceName, "name", logicalInterfaceName, "type", "vlan", "id", vlanID)
	if err != nil {
		log.Error(err, "Failed to create Vlan", "stdout", stdout, "stderr", stderr)
		return err
	}
	stdout, stderr, err = RunIP("link", "set", logicalInterfaceName, "alias", "nfn-"+logicalInterfaceName)
	if err != nil {
		log.Error(err, "Failed to create Vlan", "stdout", stdout, "stderr", stderr)
		return err
	}
        stdout, stderr, err = RunIP("link", "set", "dev", logicalInterfaceName, "up")
        if err != nil {
                log.Error(err, "Failed to enable Vlan", "stdout", stdout, "stderr", stderr)
                return err
        }
	return nil
}

// DeleteVlan deletes VLAN with logicalInterface Name
func DeleteVlan(logicalInterfaceName string) error {
	if logicalInterfaceName == "" {
		return fmt.Errorf("DeleteVlan invalid parameters")
	}
	stdout, stderr, err := RunIP("link", "del", "dev", logicalInterfaceName)
	if err != nil {
		log.Error(err, "Failed to create Vlan", "stdout", stdout, "stderr", stderr)
		return err
	}
	return nil
}

// GetVlan returns a list of VLAN configured on the node
func GetVlan() []string {
	var intfList []string
	links, err := netlink.LinkList()
	if err != nil {

	}
	for _, l := range links {
		if strings.Contains(l.Attrs().Alias, "nfn-") {
			intfList = append(intfList, l.Attrs().Name)
		}
	}
	return intfList
}

// CreatePnBridge creates Provider network bridge and mappings
func CreatePnBridge(nwName, brName, intfName string) error {
	if nwName == "" || brName == "" || intfName == "" {
		return fmt.Errorf("CreatePnBridge invalid parameters")
	}
	// Create Bridge
	stdout, stderr, err := RunOVSVsctl("--may-exist", "add-br", brName)
	if err != nil {
		log.Error(err, "Failed to create Bridge", "stdout", stdout, "stderr", stderr)
		return err
	}
	stdout, stderr, err = RunOVSVsctl("--may-exist", "add-port", brName, intfName)
	if err != nil {
		log.Error(err, "Failed to add port to Bridge", "stdout", stdout, "stderr", stderr)
		return err
	}
	stdout, stderr, err = RunOVSVsctl("set", "bridge", brName, "external_ids:nfn="+nwName)
	if err != nil {
		log.Error(err, "Failed to set nfn-alias", "stdout", stdout, "stderr", stderr)
		return err
	}
	// Update ovn-bridge-mappings
	updateOvnBridgeMapping(brName, nwName, "add")
	return nil
}

// DeletePnBridge creates Provider network bridge and mappings
func DeletePnBridge(nwName, brName string) error {
	if nwName == "" || brName == "" {
		return fmt.Errorf("DeletePnBridge invalid parameters")
	}
	// Delete Bridge
	stdout, stderr, err := RunOVSVsctl("--if-exist", "del-br", brName)
	if err != nil {
		log.Error(err, "Failed to delete Bridge", "stdout", stdout, "stderr", stderr)
		return err
	}
	updateOvnBridgeMapping(brName, nwName, "delete")

	return nil
}

// GetPnBridge returns Provider networks with external ids
func GetPnBridge(externalID string) []string {
	if externalID == "" {
		log.Error(fmt.Errorf("GetBridge invalid parameters"), "Invalid")
	}
	stdout, stderr, err := RunOVSVsctl("list-br")
	if err != nil {
		log.Error(err, "No bridges found", "stdout", stdout, "stderr", stderr)
		return nil
	}
	brNames := strings.Split(stdout, "\n")
	var brList []string
	for _, name := range brNames {
		stdout, stderr, err = RunOVSVsctl("get", "bridge", name, "external_ids:"+externalID)
		if err != nil {
			if !strings.Contains(stderr, "no key") {
				log.Error(err, "Unknown error reading external_ids", "stdout", stdout, "stderr", stderr)
			}
			continue
		}
		if stdout == "" {
			continue
		} else {
			brList = append(brList, name)
		}
	}
	return brList
}

// Update ovn-bridge-mappings
func updateOvnBridgeMapping(brName, nwName, action string) error {
	stdout, stderr, err := RunOVSVsctl("get", "open", ".", "external-ids:ovn-bridge-mappings")
	if err != nil {
		if !strings.Contains(stderr, "no key") {
			log.Error(err, "Failed to get ovn-bridge-mappings", "stdout", stdout, "stderr", stderr)
			return err
		}
	}
	// Convert csv string to map
	mm := make(map[string]string)
	if len(stdout) > 0 {
		am := strings.Split(stdout, ",")
		for _, label := range am {
			l := strings.Split(label, ":")
			if len(l) == 0 {
				log.Error(fmt.Errorf("Syntax error label: %v", label), "ovnBridgeMapping")
				return nil
			}
			mm[strings.TrimSpace(l[0])] = strings.TrimSpace(l[1])
		}
	}
	if action == "add" {
		mm[nwName] = brName
	} else if action == "delete" {
		delete(mm, nwName)
		if len(mm) == 0 {
			// No mapping needed
			stdout, stderr, err = RunOVSVsctl("remove", "open", ".", "external-ids", "ovn-bridge-mappings")
			if err != nil {
				log.Error(err, "Failed to remove ovn-bridge-mappings", "stdout", stdout, "stderr", stderr)
				return err
			}
			return nil
		}
	} else {
		return fmt.Errorf("Invalid action %s", action)
	}
	var mapping string
	for key, value := range mm {
		mapping = mapping + fmt.Sprintf("%s:%s,", key, value)
	}
	// Remove trailing ,
	mapping = mapping[:len(mapping)-1]
	extIDMap := "external-ids:ovn-bridge-mappings=" + mapping

	stdout, stderr, err = RunOVSVsctl("set", "open", ".", extIDMap)
	if err != nil {
		log.Error(err, "Failed to set ovn-bridge-mappings", "stdout", stdout, "stderr", stderr)
		return err
	}
	return nil
}

func parseOvnNetworkObject(ovnnetwork string) ([]map[string]interface{}, error) {
	var ovnNet []map[string]interface{}

	if ovnnetwork == "" {
		return nil, fmt.Errorf("parseOvnNetworkObject:error")
	}

	if err := json.Unmarshal([]byte(ovnnetwork), &ovnNet); err != nil {
		return nil, fmt.Errorf("parseOvnNetworkObject: failed to load ovn network err: %v | ovn network: %v", err, ovnnetwork)
	}

	return ovnNet, nil
}

func setupDistributedRouter(name string) error {

	// Create a single common distributed router for the cluster.
	stdout, stderr, err := RunOVNNbctl("--", "--may-exist", "lr-add", name, "--", "set", "logical_router", name, "external_ids:ovn4nfv-cluster-router=yes")
	if err != nil {
		log.Error(err, "Failed to create a single common distributed router for the cluster", "stdout", stdout, "stderr", stderr)
		return err
	}
	// Create a logical switch called "ovn4nfv-join" that will be used to connect gateway routers to the distributed router.
	// The "ovn4nfv-join" will be allocated IP addresses in the range 100.64.1.0/24.
	stdout, stderr, err = RunOVNNbctl("--may-exist", "ls-add", "ovn4nfv-join")
	if err != nil {
		log.Error(err, "Failed to create logical switch called \"ovn4nfv-join\"", "stdout", stdout, "stderr", stderr)
		return err
	}
	// Connect the distributed router to "ovn4nfv-join".
	routerMac, stderr, err := RunOVNNbctl("--if-exist", "get", "logical_router_port", "rtoj-"+name, "mac")
	if err != nil {
		log.Error(err, "Failed to get logical router port rtoj-", "name", name, "stdout", stdout, "stderr", stderr)
		return err
	}
	if routerMac == "" {
		routerMac = generateMac()
		stdout, stderr, err = RunOVNNbctl("--", "--may-exist", "lrp-add", name, "rtoj-"+name, routerMac, "100.64.1.1/24", "--", "set", "logical_router_port", "rtoj-"+name, "external_ids:connect_to_ovn4nfvjoin=yes")
		if err != nil {
			log.Error(err, "Failed to add logical router port rtoj", "name", name, "stdout", stdout, "stderr", stderr)
			return err
		}
	}
	// Connect the switch "ovn4nfv-join" to the router.
	stdout, stderr, err = RunOVNNbctl("--", "--may-exist", "lsp-add", "ovn4nfv-join", "jtor-"+name, "--", "set", "logical_switch_port", "jtor-"+name, "type=router", "options:router-port=rtoj-"+name, "addresses="+"\""+routerMac+"\"")
	if err != nil {
		log.Error(err, "Failed to add logical switch port to logical router", "stdout", stdout, "stderr", stderr)
		return err
	}
	return nil
}

// CreateNetwork in OVN controller
func createOvnLS(name, subnet, gatewayIP, excludeIps string) (gatewayIPMask string, err error) {
	var stdout, stderr string

	output, stderr, err := RunOVNNbctl("--data=bare", "--no-heading",
		"--columns=name", "find", "logical_switch", "name="+name)
	if err != nil {
		log.Error(err, "Error in reading logical switch", "stderr", stderr)
		return
	}

	if strings.Compare(name, output) == 0 {
		log.V(1).Info("Logical Switch already exists, delete first to update/recreate", "name", name)
		return "", fmt.Errorf("LS exists")
	}

	_, cidr, err := net.ParseCIDR(subnet)
	if err != nil {
		log.Error(err, "ovnNetwork '%s' invalid subnet CIDR", "name", name)
		return

	}
	firstIP := NextIP(cidr.IP)
	n, _ := cidr.Mask.Size()

	var gwIP net.IP
	if gatewayIP != "" {
		gwIP, _, err = net.ParseCIDR(gatewayIP)
		if err != nil {
			// Check if this is a valid IP address
			gwIP = net.ParseIP(gatewayIP)
		}
	}
	// If no valid Gateway use the first IP address for GatewayIP
	if gwIP == nil {
		gatewayIPMask = fmt.Sprintf("%s/%d", firstIP.String(), n)
	} else {
		gatewayIPMask = fmt.Sprintf("%s/%d", gwIP.String(), n)
	}

	// Create a logical switch and set its subnet.
	if excludeIps != "" {
		stdout, stderr, err = RunOVNNbctl("--wait=hv", "--", "--may-exist", "ls-add", name, "--", "set", "logical_switch", name, "other-config:subnet="+subnet, "external-ids:gateway_ip="+gatewayIPMask, "other-config:exclude_ips="+excludeIps)
	} else {
		stdout, stderr, err = RunOVNNbctl("--wait=hv", "--", "--may-exist", "ls-add", name, "--", "set", "logical_switch", name, "other-config:subnet="+subnet, "external-ids:gateway_ip="+gatewayIPMask)
	}
	if err != nil {
		log.Error(err, "Failed to create a logical switch", "name", name, "stdout", stdout, "stderr", stderr)
		return
	}
	return
}

// generateMac generates mac address.
func generateMac() string {
	prefix := "00:00:00"
	newRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	mac := fmt.Sprintf("%s:%02x:%02x:%02x", prefix, newRand.Intn(255), newRand.Intn(255), newRand.Intn(255))
	return mac
}

// NextIP returns IP incremented by 1
func NextIP(ip net.IP) net.IP {
	i := ipToInt(ip)
	return intToIP(i.Add(i, big.NewInt(1)))
}

func ipToInt(ip net.IP) *big.Int {
	if v := ip.To4(); v != nil {
		return big.NewInt(0).SetBytes(v)
	}
	return big.NewInt(0).SetBytes(ip.To16())
}

func intToIP(i *big.Int) net.IP {
	return net.IP(i.Bytes())
}
