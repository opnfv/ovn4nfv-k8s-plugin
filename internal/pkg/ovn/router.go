package ovn

import (
	"github.com/sirupsen/logrus"
	"ovn4nfv-k8s-plugin/internal/pkg/util"
)

func SetupMaster(name string) error {

	// Make sure br-int is created.
	stdout, stderr, err := util.RunOVSVsctl("--", "--may-exist", "add-br", "br-int")
	if err != nil {
		logrus.Errorf("Failed to create br-int, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	// Create a single common distributed router for the cluster.
	stdout, stderr, err = util.RunOVNNbctlUnix("--", "--may-exist", "lr-add", name, "--", "set", "logical_router", name, "external_ids:ovn4nfv-cluster-router=yes")
	if err != nil {
		logrus.Errorf("Failed to create a single common distributed router for the cluster, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	// Create a logical switch called "ovn4nfv-join" that will be used to connect gateway routers to the distributed router.
	// The "ovn4nfv-join" will be allocated IP addresses in the range 100.64.1.0/24.
	stdout, stderr, err = util.RunOVNNbctlUnix("--may-exist", "ls-add", "ovn4nfv-join")
	if err != nil {
		logrus.Errorf("Failed to create logical switch called \"ovn4nfv-join\", stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	// Connect the distributed router to "ovn4nfv-join".
	routerMac, stderr, err := util.RunOVNNbctlUnix("--if-exist", "get", "logical_router_port", "rtoj-"+name, "mac")
	if err != nil {
		logrus.Errorf("Failed to get logical router port rtoj-%v, stderr: %q, error: %v", name, stderr, err)
		return err
	}
	if routerMac == "" {
		routerMac = util.GenerateMac()
		stdout, stderr, err = util.RunOVNNbctlUnix("--", "--may-exist", "lrp-add", name, "rtoj-"+name, routerMac, "100.64.1.1/24", "--", "set", "logical_router_port", "rtoj-"+name, "external_ids:connect_to_ovn4nfvjoin=yes")
		if err != nil {
			logrus.Errorf("Failed to add logical router port rtoj-%v, stdout: %q, stderr: %q, error: %v", name, stdout, stderr, err)
			return err
		}
	}
	// Connect the switch "ovn4nfv-join" to the router.
	stdout, stderr, err = util.RunOVNNbctlUnix("--", "--may-exist", "lsp-add", "ovn4nfv-join", "jtor-"+name, "--", "set", "logical_switch_port", "jtor-"+name, "type=router", "options:router-port=rtoj-"+name, "addresses="+"\""+routerMac+"\"")
	if err != nil {
		logrus.Errorf("Failed to add logical switch port to logical router, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	return nil
}
