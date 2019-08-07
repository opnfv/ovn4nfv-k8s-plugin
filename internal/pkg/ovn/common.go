package ovn

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strings"
	"time"
)

var log = logf.Log.WithName("ovn")

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

// Find if switch exists
func findLogicalSwitch(name string) bool {
	// get logical switch from OVN
	output, stderr, err := RunOVNNbctl("--data=bare", "--no-heading",
		"--columns=name", "find", "logical_switch", "name="+name)
	if err != nil {
		log.Error(err, "Error in obtaining list of logical switch", "stderr", stderr)
		return false
	}

	if strings.Compare(name, output) == 0 {
		return true
	}
	return false
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
