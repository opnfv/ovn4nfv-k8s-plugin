package ovn

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	kapi "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"ovn4nfv-k8s-plugin/internal/pkg/factory"
	"ovn4nfv-k8s-plugin/internal/pkg/kube"
)

// Controller structure is the object which holds the controls for starting
// and reacting upon the watched resources (e.g. pods, endpoints)
type Controller struct {
	kube         kube.Interface
	watchFactory *factory.WatchFactory

	gatewayCache map[string]string
	// A cache of all logical switches seen by the watcher
	logicalSwitchCache map[string]bool
	// A cache of all logical ports seen by the watcher and
	// its corresponding logical switch
	logicalPortCache map[string]string
}

// NewOvnController creates a new OVN controller for creating logical network
// infrastructure and policy
func NewOvnController(kubeClient kubernetes.Interface, wf *factory.WatchFactory) *Controller {
	return &Controller{
		kube:               &kube.Kube{KClient: kubeClient},
		watchFactory:       wf,
		logicalSwitchCache: make(map[string]bool),
		logicalPortCache:   make(map[string]string),
		gatewayCache:       make(map[string]string),
	}
}

// Run starts the actual watching. Also initializes any local structures needed.
func (oc *Controller) Run() error {
	fmt.Println("ovn4nfvk8s watching Pods")
	for _, f := range []func() error{oc.WatchPods} {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

// WatchPods starts the watching of Pod resource and calls back the appropriate handler logic
func (oc *Controller) WatchPods() error {
	_, err := oc.watchFactory.AddPodHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*kapi.Pod)
			if pod.Spec.NodeName != "" {
				oc.addLogicalPort(pod)
			}
		},
		UpdateFunc: func(old, newer interface{}) {
			podNew := newer.(*kapi.Pod)
			podOld := old.(*kapi.Pod)
			if podOld.Spec.NodeName == "" && podNew.Spec.NodeName != "" {
				oc.addLogicalPort(podNew)
			}
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*kapi.Pod)
			oc.deleteLogicalPort(pod)
		},
	}, oc.syncPods)
	return err
}

func (oc *Controller) syncPods(pods []interface{}) {
}

func (oc *Controller) getGatewayFromSwitch(logicalSwitch string) (string, string, error) {
	var gatewayIPMaskStr, stderr string
	var ok bool
	var err error
	logrus.Infof("getGatewayFromSwitch: %s", logicalSwitch)
	if gatewayIPMaskStr, ok = oc.gatewayCache[logicalSwitch]; !ok {
		gatewayIPMaskStr, stderr, err = RunOVNNbctlUnix("--if-exists",
			"get", "logical_switch", logicalSwitch,
			"external_ids:gateway_ip")
		if err != nil {
			logrus.Errorf("Failed to get gateway IP:  %s, stderr: %q, %v",
				gatewayIPMaskStr, stderr, err)
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

func (oc *Controller) deleteLogicalPort(pod *kapi.Pod) {

	if pod.Spec.HostNetwork {
		return
	}

	logrus.Infof("Deleting pod: %s", pod.Name)
	logicalPort := fmt.Sprintf("%s_%s", pod.Namespace, pod.Name)

	// get the list of logical ports from OVN
	output, stderr, err := RunOVNNbctlUnix("--data=bare", "--no-heading",
		"--columns=name", "find", "logical_switch_port", "external_ids:pod=true")
	if err != nil {
		logrus.Errorf("Error in obtaining list of logical ports, "+
			"stderr: %q, err: %v",
			stderr, err)
		return
	}
	logrus.Infof("Exising Ports : %s. ", output)
	existingLogicalPorts := strings.Fields(output)
	for _, existingPort := range existingLogicalPorts {
		if strings.Contains(existingPort, logicalPort) {
			// found, delete this logical port
			logrus.Infof("Deleting: %s. ", existingPort)
			out, stderr, err := RunOVNNbctlUnix("--if-exists", "lsp-del",
				existingPort)
			if err != nil {
				logrus.Errorf("Error in deleting pod's logical port "+
					"stdout: %q, stderr: %q err: %v",
					out, stderr, err)
			} else {
				delete(oc.logicalPortCache, existingPort)
			}
		}
	}
	return
}

func (oc *Controller) addLogicalPortWithSwitch(pod *kapi.Pod, logicalSwitch, ipAddress, macAddress, interfaceName, netType string) (annotation string) {
	var out, stderr string
	var err error
	var isStaticIP bool
	if pod.Spec.HostNetwork {
		return
	}

	if !oc.logicalSwitchCache[logicalSwitch] {
		oc.logicalSwitchCache[logicalSwitch] = true
	}
	var portName string
	if interfaceName != "" {
		portName = fmt.Sprintf("%s_%s_%s", pod.Namespace, pod.Name, interfaceName)
	} else {
		return
	}

	logrus.Infof("Creating logical port for %s on switch %s", portName, logicalSwitch)

	if ipAddress != "" && macAddress != "" {
		isStaticIP = true
	}
	if ipAddress != "" && macAddress == "" {
		macAddress = generateMac()
		isStaticIP = true
	}

	if isStaticIP {
		out, stderr, err = RunOVNNbctlUnix("--may-exist", "lsp-add",
			logicalSwitch, portName, "--", "lsp-set-addresses", portName,
			fmt.Sprintf("%s %s", macAddress, ipAddress), "--", "--if-exists",
			"clear", "logical_switch_port", portName, "dynamic_addresses", "--", "set",
			"logical_switch_port", portName,
			"external-ids:namespace="+pod.Namespace,
			"external-ids:logical_switch="+logicalSwitch,
			"external-ids:pod=true")
		if err != nil {
			logrus.Errorf("Failed to add logical port to switch "+
				"stdout: %q, stderr: %q (%v)",
				out, stderr, err)
			return
		}
	} else {
		out, stderr, err = RunOVNNbctlUnix("--wait=sb", "--",
			"--may-exist", "lsp-add", logicalSwitch, portName,
			"--", "lsp-set-addresses",
			portName, "dynamic", "--", "set",
			"logical_switch_port", portName,
			"external-ids:namespace="+pod.Namespace,
			"external-ids:logical_switch="+logicalSwitch,
			"external-ids:pod=true")
		if err != nil {
			logrus.Errorf("Error while creating logical port %s "+
				"stdout: %q, stderr: %q (%v)",
				portName, out, stderr, err)
			return
		}
	}
	oc.logicalPortCache[portName] = logicalSwitch

	count := 30
	for count > 0 {
		if isStaticIP {
			out, stderr, err = RunOVNNbctlUnix("get",
				"logical_switch_port", portName, "addresses")
		} else {
			out, stderr, err = RunOVNNbctlUnix("get",
				"logical_switch_port", portName, "dynamic_addresses")
		}
		if err == nil && out != "[]" {
			break
		}
		if err != nil {
			logrus.Errorf("Error while obtaining addresses for %s - %v", portName,
				err)
			return
		}
		time.Sleep(time.Second)
		count--
	}
	if count == 0 {
		logrus.Errorf("Error while obtaining addresses for %s "+
			"stdout: %q, stderr: %q, (%v)", portName, out, stderr, err)
		return
	}

	// static addresses have format ["0a:00:00:00:00:01 192.168.1.3"], while
	// dynamic addresses have format "0a:00:00:00:00:01 192.168.1.3".
	outStr := strings.TrimLeft(out, `[`)
	outStr = strings.TrimRight(outStr, `]`)
	outStr = strings.Trim(outStr, `"`)
	addresses := strings.Split(outStr, " ")
	if len(addresses) != 2 {
		logrus.Errorf("Error while obtaining addresses for %s", portName)
		return
	}

	if netType == "virtual" {
		gatewayIP, mask, err := oc.getGatewayFromSwitch(logicalSwitch)
		if err != nil {
			logrus.Errorf("Error obtaining gateway address for switch %s: %s", logicalSwitch, err)
			return
		}
		annotation = fmt.Sprintf(`{\"ip_address\":\"%s/%s\", \"mac_address\":\"%s\", \"gateway_ip\": \"%s\"}`, addresses[1], mask, addresses[0], gatewayIP)
	} else {
		annotation = fmt.Sprintf(`{\"ip_address\":\"%s\", \"mac_address\":\"%s\", \"gateway_ip\": \"%s\"}`, addresses[1], addresses[0], "")
	}

	return annotation
}

func (oc *Controller) findLogicalSwitch(name string) bool {
	// get logical switch from OVN
	output, stderr, err := RunOVNNbctlUnix("--data=bare", "--no-heading",
		"--columns=name", "find", "logical_switch", "name="+name)
	if err != nil {
		logrus.Errorf("Error in obtaining list of logical switch, "+
			"stderr: %q, err: %v",
			stderr, err)
		return false
	}

	if strings.Compare(name, output) == 0 {
		return true
	} else {
		logrus.Errorf("Error finding Switch %v", name)
		return false
	}
}

func (oc *Controller) addLogicalPort(pod *kapi.Pod) {
	var logicalSwitch string
	var ipAddress, macAddress, interfaceName, defaultGateway, netType string

	annotation := pod.Annotations["ovnNetwork"]

	if annotation != "" {
		ovnNetObjs, err := parseOvnNetworkObject(annotation)
		if err != nil {
			logrus.Errorf("addLogicalPort : Error Parsing OvnNetwork List")
			return
		}
		var ovnString, outStr string
		ovnString = "["
		for _, net := range ovnNetObjs {
			logicalSwitch = net["name"].(string)
			if !oc.findLogicalSwitch(logicalSwitch) {
				logrus.Errorf("Logical Switch not found")
				return
			}
			if _, ok := net["interface"]; ok {
				interfaceName = net["interface"].(string)
			} else {
				logrus.Errorf("Interface name must be provided")
				return
			}
			if _, ok := net["ipAddress"]; ok {
				ipAddress = net["ipAddress"].(string)
			} else {
				ipAddress = ""
			}
			if _, ok := net["macAddress"]; ok {
				macAddress = net["macAddress"].(string)
			} else {
				macAddress = ""
			}
			if _, ok := net["defaultGateway"]; ok {
				defaultGateway = net["defaultGateway"].(string)
			} else {
				defaultGateway = "false"
			}
			if _, ok := net["netType"]; ok {
				netType = net["netType"].(string)
			} else {
				netType = "virtual"
			}
			if netType != "provider" && netType != "virtual" {
				logrus.Errorf("netType is not supported")
				return
			}
			if netType == "provider" && ipAddress == "" {
				logrus.Errorf("ipAddress must be provided for netType Provider")
				return
			}
			if netType == "provider" && defaultGateway == "true" {
				logrus.Errorf("defaultGateway not supported for provider network - Use ovnNetworkRoutes to add routes")
				return
			}
			outStr = oc.addLogicalPortWithSwitch(pod, logicalSwitch, ipAddress, macAddress, interfaceName, netType)
			if outStr == "" {
				return
			}
			last := len(outStr) - 1
			tmpString := outStr[:last]
			tmpString += "," + "\\\"defaultGateway\\\":" + "\\\"" + defaultGateway + "\\\""
			tmpString += "," + "\\\"interface\\\":" + "\\\"" + interfaceName + "\\\"}"
			ovnString += tmpString
			ovnString += ","
		}
		last := len(ovnString) - 1
		ovnString = ovnString[:last]
		ovnString += "]"
		logrus.Debugf("ovnIfaceList - %v", ovnString)
		err = oc.kube.SetAnnotationOnPod(pod, "ovnIfaceList", ovnString)
		if err != nil {
			logrus.Errorf("Failed to set annotation on pod %s - %v", pod.Name, err)
		}
	}
}
