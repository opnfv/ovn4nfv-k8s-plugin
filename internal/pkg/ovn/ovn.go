package ovn

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	kapi "k8s.io/api/core/v1"
	kexec "k8s.io/utils/exec"
	"strings"
	"time"
)

type Controller struct {
	gatewayCache map[string]string
}

const (
	ovn4nfvRouterName    = "ovn4nfv-master"
	Ovn4nfvAnnotationTag = "k8s.plugin.opnfv.org/ovnInterfaces"
)

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

	if ovnNetObjs == nil {
		return
	}

	if pod.Spec.HostNetwork {
		return
	}

	if _, ok := pod.Annotations[Ovn4nfvAnnotationTag]; ok {
		log.Info("AddLogicalPorts : Pod annotation found")
		return
	}

	var ovnString, outStr string
	ovnString = "["
	var ns netInterface
	for _, net := range ovnNetObjs {

		err := mapstructure.Decode(net, &ns)
		if err != nil {
			log.Error(err, "mapstruct error", "network", net)
			return
		}

		if !findLogicalSwitch(ns.Name) {
			log.Info("Logical Switch not found")
			return
		}
		if ns.Interface == "" {
			log.Info("Interface name must be provided")
			return
		}
		if ns.DefaultGateway == "" {
			ns.DefaultGateway = "false"
		}
		if ns.NetType == "" || ns.NetType != "provider" {
			ns.NetType = "virtual"
		}
		if ns.NetType == "provider" {
			if ns.IPAddress == "" {
				log.Info("ipAddress must be provided for netType Provider")
				return
			}
			if ns.DefaultGateway == "true" {
				log.Info("defaultGateway not supported for provider network - Use ovnNetworkRoutes to add routes")
				return
			}

		}

		outStr = oc.addLogicalPortWithSwitch(pod, ns.Name, ns.IPAddress, ns.MacAddress, ns.Interface, ns.NetType)
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
	last := len(ovnString) - 1
	ovnString = ovnString[:last]
	ovnString += "]"
	key = Ovn4nfvAnnotationTag
	value = ovnString
	return key, value
}

func (oc *Controller) DeleteLogicalPorts(name, namespace string) {

	log.Info("Deleting pod", "name", name)
	logicalPort := fmt.Sprintf("%s_%s", namespace, name)

	// get the list of logical ports from OVN
	stdout, stderr, err := RunOVNNbctl("--data=bare", "--no-heading",
		"--columns=name", "find", "logical_switch_port", "external_ids:pod=true")
	if err != nil {
		log.Error(err, "Error in obtaining list of logical ports ", "stdout", stdout, "stderr", stderr)
		return
	}
	log.Info("Deleting", "Existing Ports", stdout)
	existingLogicalPorts := strings.Fields(stdout)
	for _, existingPort := range existingLogicalPorts {
		if strings.Contains(existingPort, logicalPort) {
			// found, delete this logical port
			log.Info("Deleting", "existingPort", existingPort)
			stdout, stderr, err := RunOVNNbctl("--if-exists", "lsp-del",
				existingPort)
			if err != nil {
				log.Error(err, "Error in deleting pod's logical port ", "stdout", stdout, "stderr", stderr)
			}
		}
	}
	return
}

func (oc *Controller) getGatewayFromSwitch(logicalSwitch string) (string, string, error) {
	var gatewayIPMaskStr, stderr string
	var ok bool
	var err error
	log.Info("getGatewayFromSwitch", "logicalSwitch", logicalSwitch)
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

func (oc *Controller) addLogicalPortWithSwitch(pod *kapi.Pod, logicalSwitch, ipAddress, macAddress, interfaceName, netType string) (annotation string) {
	var out, stderr string
	var err error
	var isStaticIP bool
	if pod.Spec.HostNetwork {
		return
	}

	var portName string
	if interfaceName != "" {
		portName = fmt.Sprintf("%s_%s_%s", pod.Namespace, pod.Name, interfaceName)
	} else {
		return
	}

	log.Info("Creating logical port for on switch", "portName", portName, "logicalSwitch", logicalSwitch)

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

	if netType == "virtual" {
		gatewayIP, mask, err := oc.getGatewayFromSwitch(logicalSwitch)
		if err != nil {
			log.Error(err, "Error obtaining gateway address for switch", "logicalSwitch", logicalSwitch)
			return
		}
		annotation = fmt.Sprintf(`{\"ip_address\":\"%s/%s\", \"mac_address\":\"%s\", \"gateway_ip\": \"%s\"}`, addresses[1], mask, addresses[0], gatewayIP)
	} else {
		annotation = fmt.Sprintf(`{\"ip_address\":\"%s\", \"mac_address\":\"%s\", \"gateway_ip\": \"%s\"}`, addresses[1], addresses[0], "")
	}

	return annotation
}
