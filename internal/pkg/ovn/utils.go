package ovn

import (
	"bytes"
	"fmt"
	kexec "k8s.io/utils/exec"
	"os"
	"reflect"
	"strings"
	"time"
)

const (
	ovsCommandTimeout = 15
	ovnNbctlCommand   = "ovn-nbctl"
	ovsVsctlCommand   = "ovs-vsctl"
	ipCommand         = "ip"
)

// Exec runs various OVN and OVS utilities
type execHelper struct {
	exec      kexec.Interface
	nbctlPath string
	vsctlPath string
	ipPath    string
	hostIP    string
	hostPort  string
}

var runner *execHelper

// SetupOvnUtils does internal OVN initialization
var SetupOvnUtils = func() error {
	// Setup Distributed Router
	err := setupDistributedRouter(ovn4nfvRouterName)
	if err != nil {
		log.Error(err, "Failed to initialize OVN Distributed Router")
		return err
	}

	log.Info("OVN Network", "OVN Default NW", Ovn4nfvDefaultNw, "OVN Subnet", ovnConf.Subnet, "OVN Gateway IP", ovnConf.GatewayIP, "OVN ExcludeIPs", ovnConf.ExcludeIPs)
	_, err = createOvnLS(Ovn4nfvDefaultNw, ovnConf.Subnet, ovnConf.GatewayIP, ovnConf.ExcludeIPs)
	if err != nil && !reflect.DeepEqual(err, fmt.Errorf("LS exists")) {
		log.Error(err, "Failed to create ovn4nfvk8s default nw")
		return err
	}
	return nil
}

// SetExec validates executable paths and saves the given exec interface
// to be used for running various OVS and OVN utilites
func SetExec(exec kexec.Interface) error {
	var err error

	runner = &execHelper{exec: exec}
	runner.nbctlPath, err = exec.LookPath(ovnNbctlCommand)
	if err != nil {
		return err
	}
	runner.vsctlPath, err = exec.LookPath(ovsVsctlCommand)
	if err != nil {
		return err
	}
	runner.ipPath, err = exec.LookPath(ipCommand)
	if err != nil {
		return err
	}
	runner.hostIP = os.Getenv("HOST_IP")
	// OVN Host Port
	runner.hostPort = "6641"
	log.Info("Host Port", "IP", runner.hostIP, "Port", runner.hostPort)

	return nil
}

// Run the ovn-ctl command and retry if "Connection refused"
// poll waitng for service to become available
func runOVNretry(cmdPath string, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {

	retriesLeft := 200
	for {
		stdout, stderr, err := run(cmdPath, args...)
		if err == nil {
			return stdout, stderr, err
		}
		// Connection refused
		// Master may not be up so keep trying
		if strings.Contains(stderr.String(), "Connection refused") {
			if retriesLeft == 0 {
				return stdout, stderr, err
			}
			retriesLeft--
			time.Sleep(2 * time.Second)
		} else {
			// Some other problem for caller to handle
			return stdout, stderr, err
		}
	}
}

func run(cmdPath string, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := runner.exec.Command(cmdPath, args...)
	cmd.SetStdout(stdout)
	cmd.SetStderr(stderr)
	log.V(1).Info("exec:", "cmdPath", cmdPath, "args", strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		log.Info("ovs", "Error:", err, "cmdPath", cmdPath, "args", strings.Join(args, " "), "stdout", stdout, "stderr", stderr)
	} else {
		log.V(1).Info("output:", "stdout", stdout)
	}
	return stdout, stderr, err
}

// RunOVNNbctlWithTimeout runs command via ovn-nbctl with a specific timeout
func RunOVNNbctlWithTimeout(timeout int, args ...string) (string, string, error) {
	var cmdArgs []string
	if len(runner.hostIP) > 0 {
		cmdArgs = []string{
			fmt.Sprintf("--db=tcp:%s:%s", runner.hostIP, runner.hostPort),
		}
	}
	cmdArgs = append(cmdArgs, fmt.Sprintf("--timeout=%d", timeout))
	cmdArgs = append(cmdArgs, args...)
	stdout, stderr, err := runOVNretry(runner.nbctlPath, cmdArgs...)
	return strings.Trim(strings.TrimSpace(stdout.String()), "\""), stderr.String(), err
}

// RunOVNNbctl runs a command via ovn-nbctl.
func RunOVNNbctl(args ...string) (string, string, error) {
	return RunOVNNbctlWithTimeout(ovsCommandTimeout, args...)
}

// RunIP runs a command via the iproute2 "ip" utility
func RunIP(args ...string) (string, string, error) {
	stdout, stderr, err := run(runner.ipPath, args...)
	return strings.TrimSpace(stdout.String()), stderr.String(), err
}

// RunOVSVsctl runs a command via ovs-vsctl.
func RunOVSVsctl(args ...string) (string, string, error) {
	cmdArgs := []string{fmt.Sprintf("--timeout=%d", ovsCommandTimeout)}
	cmdArgs = append(cmdArgs, args...)
	stdout, stderr, err := run(runner.vsctlPath, cmdArgs...)
	return strings.Trim(strings.TrimSpace(stdout.String()), "\""), stderr.String(), err
}
