package ovn

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	kapi "k8s.io/api/core/v1"
	kexec "k8s.io/utils/exec"
	"math/rand"
	"os"
	k8sv1alpha1 "ovn4nfv-k8s-plugin/pkg/apis/k8s/v1alpha1"
	"strings"
	"time"
)

type Controller struct {
	gatewayCache map[string]string
}

type OVNNetworkConf struct {
	Subnet     string
	GatewayIP  string
	ExcludeIPs string
}

const (
	ovn4nfvRouterName = "ovn4nfv-master"
	// Ovn4nfvAnnotationTag tag on already processed Pods
	Ovn4nfvAnnotationTag = "k8s.plugin.opnfv.org/ovnInterfaces"
	// OVN Default Network name
	Ovn4nfvDefaultNw = "ovn4nfvk8s-default-nw"
)

var ovnConf *OVNNetworkConf

func GetOvnNetConf() error {
	ovnConf = &OVNNetworkConf{}

	ovnConf.Subnet = os.Getenv("OVN_SUBNET")
	if ovnConf.Subnet == "" {
		fmt.Errorf("OVN subnet is not set in nfn-operator configmap env")
	}

	ovnConf.GatewayIP = os.Getenv("OVN_GATEWAYIP")
	if ovnConf.GatewayIP == "" {
		fmt.Errorf("OVN gatewayIP is not set in nfn-operator configmap env")
	}

	ovnConf.ExcludeIPs = os.Getenv("OVN_EXCLUDEIPS")
	if ovnConf.ExcludeIPs == "" {
		fmt.Errorf("OVN excludeIPs is not set in nfn-operator configmap env")
	}

	return nil
}

type netInterface struct {
	Name           string
	Interface      string
	NetType        string
	DefaultGateway string
	IPAddress      string
	MacAddress     string
}

var ovnCtl *Controller

// NewOvnController creates a new OVN controller for creating logical networks
func NewOvnController(exec kexec.Interface) (*Controller, error) {

	if exec == nil {
		exec = kexec.New()
	}
	if err := SetExec(exec); err != nil {
		log.Error(err, "Failed to initialize exec helper")
		return nil, err
	}

	if err := GetOvnNetConf(); err != nil {
		log.Error(err, "nfn-operator OVN Network configmap is not set")
		return nil, err
	}
	if err := SetupOvnUtils(); err != nil {
		log.Error(err, "Failed to initialize OVN State")
		return nil, err
	}

	ovnCtl = &Controller{
		gatewayCache: make(map[string]string),
	}
	return ovnCtl, nil
}

// GetOvnController returns OVN controller for creating logical networks
func GetOvnController() (*Controller, error) {
	if ovnCtl != nil {
		return ovnCtl, nil
	}
	return nil, fmt.Errorf("OVN Controller not initialized")
}

// AddLogicalPorts adds ports to the Pod
func (oc *Controller) AddLogicalPorts(pod *kapi.Pod, ovnNetObjs []map[string]interface{}) (key, value string) {

	if pod.Spec.HostNetwork {
		return
	}

	if _, ok := pod.Annotations[Ovn4nfvAnnotationTag]; ok {
		log.V(1).Info("AddLogicalPorts : Pod annotation found")
		return
	}

	var ovnString, outStr string
	var defaultInterface bool

	ovnString = "["
	var ns netInterface
	for _, net := range ovnNetObjs {
		err := mapstructure.Decode(net, &ns)
		if err != nil {
			log.Error(err, "mapstruct error", "network", net)
			return
		}
		if !oc.FindLogicalSwitch(ns.Name) {
			log.Info("Logical Switch not found")
			return
		}
		if ns.Name == Ovn4nfvDefaultNw {
			defaultInterface = true
		}
		if ns.Interface == "" && ns.Name != Ovn4nfvDefaultNw {
			log.Info("Interface name must be provided")
			return
		}
		if ns.DefaultGateway == "" {
			ns.DefaultGateway = "false"
		}
		var portName string
		if ns.Interface != "" {
			portName = fmt.Sprintf("%s_%s_%s", pod.Namespace, pod.Name, ns.Interface)
		} else {
			portName = fmt.Sprintf("%s_%s", pod.Namespace, pod.Name)
			ns.Interface = "*"
		}
		outStr = oc.addLogicalPortWithSwitch(pod, ns.Name, ns.IPAddress, ns.MacAddress, portName)
		if outStr == "" {
			return
		}
		last := len(outStr) - 1
		tmpString := outStr[:last]
		tmpString += "," + "\\\"defaultGateway\\\":" + "\\\"" + ns.DefaultGateway + "\\\""
		tmpString += "," + "\\\"interface\\\":" + "\\\"" + ns.Interface + "\\\"}"
		ovnString += tmpString
		ovnString += ","
	}
	var last int
	if defaultInterface == false {
		// Add Default interface
		portName := fmt.Sprintf("%s_%s", pod.Namespace, pod.Name)
		outStr = oc.addLogicalPortWithSwitch(pod, Ovn4nfvDefaultNw, "", "", portName)
		if outStr == "" {
			return
		}
		last := len(outStr) - 1
		tmpString := outStr[:last]
		tmpString += "," + "\\\"interface\\\":" + "\\\"" + "*" + "\\\"}"
		ovnString += tmpString
		ovnString += ","
	}
	last = len(ovnString) - 1
	ovnString = ovnString[:last]
	ovnString += "]"
	key = Ovn4nfvAnnotationTag
	value = ovnString
	return key, value
}

// DeleteLogicalPorts deletes the OVN ports for the pod
func (oc *Controller) DeleteLogicalPorts(name, namespace string) {

	log.Info("DeleteLogicalPorts")
	logicalPort := fmt.Sprintf("%s_%s", namespace, name)

	// get the list of logical ports from OVN
	stdout, stderr, err := RunOVNNbctl("--data=bare", "--no-heading",
		"--columns=name", "find", "logical_switch_port", "external_ids:pod=true")
	if err != nil {
		log.Error(err, "Error in obtaining list of logical ports ", "stdout", stdout, "stderr", stderr)
		return
	}
	existingLogicalPorts := strings.Fields(stdout)
	for _, existingPort := range existingLogicalPorts {
		if strings.Contains(existingPort, logicalPort) {
			// found, delete this logical port
			log.Info("Deleting", "Port", existingPort)
			stdout, stderr, err := RunOVNNbctl("--if-exists", "lsp-del",
				existingPort)
			if err != nil {
				log.Error(err, "Error in deleting pod's logical port ", "stdout", stdout, "stderr", stderr)
			}
		}
	}
	return
}

// CreateNetwork in OVN controller
func (oc *Controller) CreateNetwork(cr *k8sv1alpha1.Network) error {
	var stdout, stderr string

	// Currently only these fields are supported
	name := cr.Name
	subnet := cr.Spec.Ipv4Subnets[0].Subnet
	gatewayIP := cr.Spec.Ipv4Subnets[0].Gateway
	excludeIps := cr.Spec.Ipv4Subnets[0].ExcludeIps

	gatewayIPMask, err := createOvnLS(name, subnet, gatewayIP, excludeIps)
	if err != nil {
		return err
	}

	routerMac, stderr, err := RunOVNNbctl("--if-exist", "get", "logical_router_port", "rtos-"+name, "mac")
	if err != nil {
		log.Error(err, "Failed to get logical router port", "stderr", stderr)
		return err
	}
	if routerMac == "" {
		prefix := "00:00:00"
		newRand := rand.New(rand.NewSource(time.Now().UnixNano()))
		routerMac = fmt.Sprintf("%s:%02x:%02x:%02x", prefix, newRand.Intn(255), newRand.Intn(255), newRand.Intn(255))
	}

	_, stderr, err = RunOVNNbctl("--wait=hv", "--may-exist", "lrp-add", ovn4nfvRouterName, "rtos-"+name, routerMac, gatewayIPMask)
	if err != nil {
		log.Error(err, "Failed to add logical port to router", "stderr", stderr)
		return err
	}

	// Connect the switch to the router.
	stdout, stderr, err = RunOVNNbctl("--wait=hv", "--", "--may-exist", "lsp-add", name, "stor-"+name, "--", "set", "logical_switch_port", "stor-"+name, "type=router", "options:router-port=rtos-"+name, "addresses="+"\""+routerMac+"\"")
	if err != nil {
		log.Error(err, "Failed to add logical port to switch", "stderr", stderr, "stdout", stdout)
		return err
	}

	return nil
}

// DeleteNetwork in OVN controller
func (oc *Controller) DeleteNetwork(cr *k8sv1alpha1.Network) error {

	name := cr.Name
	stdout, stderr, err := RunOVNNbctl("--if-exist", "--wait=hv", "lrp-del", "rtos-"+name)
	if err != nil {
		log.Error(err, "Failed to delete router port", "name", name, "stdout", stdout, "stderr", stderr)
		return err
	}
	stdout, stderr, err = RunOVNNbctl("--if-exist", "--wait=hv", "ls-del", name)
	if err != nil {
		log.Error(err, "Failed to delete switch", "name", name, "stdout", stdout, "stderr", stderr)
		return err
	}
	return nil
}

// CreateProviderNetwork in OVN controller
func (oc *Controller) CreateProviderNetwork(cr *k8sv1alpha1.ProviderNetwork) error {
	var stdout, stderr string

	// Currently only these fields are supported
	name := cr.Name
	subnet := cr.Spec.Ipv4Subnets[0].Subnet
	gatewayIP := cr.Spec.Ipv4Subnets[0].Gateway
	excludeIps := cr.Spec.Ipv4Subnets[0].ExcludeIps
	_, err := createOvnLS(name, subnet, gatewayIP, excludeIps)
	if err != nil {
		return err
	}

	// Add localnet port.
	stdout, stderr, err = RunOVNNbctl("--wait=hv", "--", "--may-exist", "lsp-add", name, "server-localnet_"+name, "--",
		"lsp-set-addresses", "server-localnet_"+name, "unknown", "--",
		"lsp-set-type", "server-localnet_"+name, "localnet", "--",
		"lsp-set-options", "server-localnet_"+name, "network_name=nw_"+name)
	if err != nil {
		log.Error(err, "Failed to add logical port to switch", "stderr", stderr, "stdout", stdout)
		return err
	}

	return nil
}

// DeleteProviderNetwork in OVN controller
func (oc *Controller) DeleteProviderNetwork(cr *k8sv1alpha1.ProviderNetwork) error {

	name := cr.Name
	stdout, stderr, err := RunOVNNbctl("--if-exist", "--wait=hv", "ls-del", name)
	if err != nil {
		log.Error(err, "Failed to delete switch", "name", name, "stdout", stdout, "stderr", stderr)
		return err
	}
	return nil
}

// FindLogicalSwitch returns true if switch exists
func (oc *Controller) FindLogicalSwitch(name string) bool {
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

func (oc *Controller) getGatewayFromSwitch(logicalSwitch string) (string, string, error) {
	var gatewayIPMaskStr, stderr string
	var ok bool
	var err error
	if gatewayIPMaskStr, ok = oc.gatewayCache[logicalSwitch]; !ok {
		gatewayIPMaskStr, stderr, err = RunOVNNbctl("--if-exists",
			"get", "logical_switch", logicalSwitch,
			"external_ids:gateway_ip")
		if err != nil {
			log.Error(err, "Failed to get gateway IP", "stderr", stderr, "gatewayIPMaskStr", gatewayIPMaskStr)
			return "", "", err
		}
		if gatewayIPMaskStr == "" {
			return "", "", fmt.Errorf("Empty gateway IP in logical switch %s",
				logicalSwitch)
		}
		oc.gatewayCache[logicalSwitch] = gatewayIPMaskStr
	}
	gatewayIPMask := strings.Split(gatewayIPMaskStr, "/")
	if len(gatewayIPMask) != 2 {
		return "", "", fmt.Errorf("Failed to get IP and Mask from gateway CIDR:  %s",
			gatewayIPMaskStr)
	}
	gatewayIP := gatewayIPMask[0]
	mask := gatewayIPMask[1]
	return gatewayIP, mask, nil
}

func (oc *Controller) addLogicalPortWithSwitch(pod *kapi.Pod, logicalSwitch, ipAddress, macAddress, portName string) (annotation string) {
	var out, stderr string
	var err error
	var isStaticIP bool
	if pod.Spec.HostNetwork {
		return
	}

	log.V(1).Info("Creating logical port for on switch", "portName", portName, "logicalSwitch", logicalSwitch)

	if ipAddress != "" && macAddress != "" {
		isStaticIP = true
	}
	if ipAddress != "" && macAddress == "" {
		macAddress = generateMac()
		isStaticIP = true
	}

	if isStaticIP {
		out, stderr, err = RunOVNNbctl("--may-exist", "lsp-add",
			logicalSwitch, portName, "--", "lsp-set-addresses", portName,
			fmt.Sprintf("%s %s", macAddress, ipAddress), "--", "--if-exists",
			"clear", "logical_switch_port", portName, "dynamic_addresses", "--", "set",
			"logical_switch_port", portName,
			"external-ids:namespace="+pod.Namespace,
			"external-ids:logical_switch="+logicalSwitch,
			"external-ids:pod=true")
		if err != nil {
			log.Error(err, "Failed to add logical port to switch", "out", out, "stderr", stderr)
			return
		}
	} else {
		out, stderr, err = RunOVNNbctl("--wait=sb", "--",
			"--may-exist", "lsp-add", logicalSwitch, portName,
			"--", "lsp-set-addresses",
			portName, "dynamic", "--", "set",
			"logical_switch_port", portName,
			"external-ids:namespace="+pod.Namespace,
			"external-ids:logical_switch="+logicalSwitch,
			"external-ids:pod=true")
		if err != nil {
			log.Error(err, "Error while creating logical port %s ", "portName", portName, "stdout", out, "stderr", stderr)
			return
		}
	}

	count := 30
	for count > 0 {
		if isStaticIP {
			out, stderr, err = RunOVNNbctl("get",
				"logical_switch_port", portName, "addresses")
		} else {
			out, stderr, err = RunOVNNbctl("get",
				"logical_switch_port", portName, "dynamic_addresses")
		}
		if err == nil && out != "[]" {
			break
		}
		if err != nil {
			log.Error(err, "Error while obtaining addresses for", "portName", portName)
			return
		}
		time.Sleep(time.Second)
		count--
	}
	if count == 0 {
		log.Error(err, "Error while obtaining addresses for", "portName", portName, "stdout", out, "stderr", stderr)
		return
	}

	// static addresses have format ["0a:00:00:00:00:01 192.168.1.3"], while
	// dynamic addresses have format "0a:00:00:00:00:01 192.168.1.3".
	outStr := strings.TrimLeft(out, `[`)
	outStr = strings.TrimRight(outStr, `]`)
	outStr = strings.Trim(outStr, `"`)
	addresses := strings.Split(outStr, " ")
	if len(addresses) != 2 {
		log.Info("Error while obtaining addresses for", "portName", portName)
		return
	}

	gatewayIP, mask, err := oc.getGatewayFromSwitch(logicalSwitch)
	if err != nil {
		log.Error(err, "Error obtaining gateway address for switch", "logicalSwitch", logicalSwitch)
		return
	}
	annotation = fmt.Sprintf(`{\"ip_address\":\"%s/%s\", \"mac_address\":\"%s\", \"gateway_ip\": \"%s\"}`, addresses[1], mask, addresses[0], gatewayIP)

	return annotation
}
