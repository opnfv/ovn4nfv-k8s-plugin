package cniserver

import (
        "encoding/json"
        "k8s.io/apimachinery/pkg/util/wait"
        "fmt"
        "net"
        "strconv"
        "net/http"
        "time"
        "k8s.io/klog"

        "k8s.io/client-go/kubernetes"
        "github.com/containernetworking/cni/pkg/types"
        "github.com/containernetworking/cni/pkg/types/current"
        "ovn4nfv-k8s-plugin/internal/pkg/kube"
        "k8s.io/apimachinery/pkg/api/errors"
        "ovn4nfv-k8s-plugin/internal/pkg/config"
        "ovn4nfv-k8s-plugin/cmd/ovn4nfvk8s-cni/app"
)

const (
        ovn4nfvAnnotationTag = "k8s.plugin.opnfv.org/ovnInterfaces"
)

func parseOvnNetworkObject(ovnnetwork string) ([]map[string]string, error) {
	var ovnNet []map[string]string

	if ovnnetwork == "" {
		return nil, fmt.Errorf("parseOvnNetworkObject:error")
	}

	if err := json.Unmarshal([]byte(ovnnetwork), &ovnNet); err != nil {
		return nil, fmt.Errorf("parseOvnNetworkObject: failed to load ovn network err: %v | ovn network: %v", err, ovnnetwork)
	}

	return ovnNet, nil
}

func mergeWithResult(srcObj, dstObj types.Result) (types.Result, error) {

	if dstObj == nil {
		return srcObj, nil
	}
	src, err := current.NewResultFromResult(srcObj)
	if err != nil {
		return nil, fmt.Errorf("Couldn't convert old result to current version: %v", err)
	}
	dst, err := current.NewResultFromResult(dstObj)
	if err != nil {
		return nil, fmt.Errorf("Couldn't convert old result to current version: %v", err)
	}

	ifacesLength := len(dst.Interfaces)

	for _, iface := range src.Interfaces {
		dst.Interfaces = append(dst.Interfaces, iface)
	}
	for _, ip := range src.IPs {
		if ip.Interface != nil && *(ip.Interface) != -1 {
			ip.Interface = current.Int(*(ip.Interface) + ifacesLength)
		}
		dst.IPs = append(dst.IPs, ip)
	}
	for _, route := range src.Routes {
		dst.Routes = append(dst.Routes, route)
	}

	for _, ns := range src.DNS.Nameservers {
		dst.DNS.Nameservers = append(dst.DNS.Nameservers, ns)
	}
	for _, s := range src.DNS.Search {
		dst.DNS.Search = append(dst.DNS.Search, s)
	}
	for _, opt := range src.DNS.Options {
		dst.DNS.Options = append(dst.DNS.Options, opt)
	}
	// TODO: what about DNS.domain?
	return dst, nil
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func isNotFoundError(err error) bool {
        statusErr, ok := err.(*errors.StatusError)
        return ok && statusErr.Status().Code == http.StatusNotFound
}

func (cr *CNIServerRequest) addMultipleInterfaces(ovnAnnotation, namespace, podName string) types.Result {
        klog.Infof("ovn4nfvk8s-cni: addMultipleInterfaces ")
	var ovnAnnotatedMap []map[string]string
	ovnAnnotatedMap, err := parseOvnNetworkObject(ovnAnnotation)
	if err != nil {
		klog.Errorf("addLogicalPort : Error Parsing Ovn Network List %v %v", ovnAnnotatedMap, err)
		return nil
	}
	if namespace == "" || podName == "" {
		klog.Errorf("required CNI variable missing")
		return nil
	}
	var interfacesArray []*current.Interface
	var index int
	var result *current.Result
	var dstResult types.Result
	for _, ovnNet := range ovnAnnotatedMap {
		ipAddress := ovnNet["ip_address"]
		macAddress := ovnNet["mac_address"]
		gatewayIP := ovnNet["gateway_ip"]
		defaultGateway := ovnNet["defaultGateway"]

		if ipAddress == "" || macAddress == "" {
			klog.Errorf("failed in pod annotation key extract")
			return nil
		}

		index++
		interfaceName := ovnNet["interface"]
		if interfaceName == "" {
			klog.Errorf("addMultipleInterfaces: interface can't be null")
			return nil
		}
		klog.Infof("addMultipleInterfaces: ipAddress %v %v", ipAddress, interfaceName)
		interfacesArray, err = app.ConfigureInterface(cr.Netns, cr.SandboxID, cr.IfName, namespace, podName, macAddress, ipAddress, gatewayIP, interfaceName, defaultGateway, index, config.Default.MTU)
		if err != nil {
			klog.Errorf("Failed to configure interface in pod: %v", err)
			return nil
		}
		addr, addrNet, err := net.ParseCIDR(ipAddress)
		if err != nil {
			klog.Errorf("failed to parse IP address %q: %v", ipAddress, err)
			return nil
		}
		ipVersion := "6"
		if addr.To4() != nil {
			ipVersion = "4"
		}
		var routes types.Route
		if defaultGateway == "true" {
			defaultAddr, defaultAddrNet, _ := net.ParseCIDR("0.0.0.0/0")
			routes = types.Route{Dst: net.IPNet{IP: defaultAddr, Mask: defaultAddrNet.Mask}, GW: net.ParseIP(gatewayIP)}

			result = &current.Result{
				Interfaces: interfacesArray,
				IPs: []*current.IPConfig{
					{
						Version:   ipVersion,
						Interface: current.Int(1),
						Address:   net.IPNet{IP: addr, Mask: addrNet.Mask},
						Gateway:   net.ParseIP(gatewayIP),
					},
				},
				Routes: []*types.Route{&routes},
			}
		} else {
			result = &current.Result{
				Interfaces: interfacesArray,
				IPs: []*current.IPConfig{
					{
						Version:   ipVersion,
						Interface: current.Int(1),
						Address:   net.IPNet{IP: addr, Mask: addrNet.Mask},
						Gateway:   net.ParseIP(gatewayIP),
					},
				},
			}

		}
		// Build the result structure to pass back to the runtime
		dstResult, err = mergeWithResult(types.Result(result), dstResult)
		if err != nil {
			klog.Errorf("Failed to merge results: %v", err)
			return nil
		}
	}
	klog.Infof("addMultipleInterfaces: results %s", prettyPrint(dstResult))
	return dstResult
}

func (cr *CNIServerRequest) addRoutes(ovnAnnotation string, dstResult types.Result) types.Result {
	klog.Infof("ovn4nfvk8s-cni: addRoutes ")
	var ovnAnnotatedMap []map[string]string
	ovnAnnotatedMap, err := parseOvnNetworkObject(ovnAnnotation)
	if err != nil {
		klog.Errorf("addLogicalPort : Error Parsing Ovn Route List %v", err)
		return nil
	}

	var result types.Result
	var routes []*types.Route
	for _, ovnNet := range ovnAnnotatedMap {
		dst := ovnNet["dst"]
		gw := ovnNet["gw"]
		dev := ovnNet["dev"]
		if dst == "" || gw == "" || dev == "" {
			klog.Errorf("failed in pod annotation key extract")
			return nil
		}
		err = app.ConfigureRoute(cr.Netns, dst, gw, dev)
		if err != nil {
			klog.Errorf("Failed to configure interface in pod: %v", err)
			return nil
		}
		dstAddr, dstAddrNet, _ := net.ParseCIDR(dst)
		routes = append(routes, &types.Route{
			Dst: net.IPNet{IP: dstAddr, Mask: dstAddrNet.Mask},
			GW:  net.ParseIP(gw),
		})
	}

	result = &current.Result{
		Routes: routes,
	}
	// Build the result structure to pass back to the runtime
	dstResult, err = mergeWithResult(result, dstResult)
	if err != nil {
		klog.Errorf("Failed to merge results: %v", err)
		return nil
	}
	klog.Infof("addRoutes: results %s", prettyPrint(dstResult))
	return dstResult
}

func (cr *CNIServerRequest) cmdAdd(kclient kubernetes.Interface) ([]byte, error) {
        klog.Infof("ovn4nfvk8s-cni: cmdAdd")
	namespace := cr.PodNamespace
        podname := cr.PodName
	if namespace == "" || podname == "" {
		return nil, fmt.Errorf("required CNI variable missing")
	}
        klog.Infof("ovn4nfvk8s-cni: cmdAdd for pod podname:%s and namespace:%s", podname, namespace)
	kubecli := &kube.Kube{KClient: kclient}
	// Get the IP address and MAC address from the API server.
	var annotationBackoff = wait.Backoff{Duration: 1 * time.Second, Steps: 14, Factor: 1.5, Jitter: 0.1}
	var annotation map[string]string
        var err error
	if err = wait.ExponentialBackoff(annotationBackoff, func() (bool, error) {
		annotation, err = kubecli.GetAnnotationsOnPod(namespace, podname)
		if err != nil {
                        if isNotFoundError(err) {
			        return false, fmt.Errorf("Error - pod not found - %v", err)
                        }
                        klog.Infof("ovn4nfvk8s-cni: cmdAdd Warning - Error while obtaining pod annotations - %v", err)
                        return false,nil
		}
		if _, ok := annotation[ovn4nfvAnnotationTag]; ok {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to get pod annotation - %v", err)
	}

	klog.Infof("ovn4nfvk8s-cni: cmdAdd Annotation Found ")
	ovnAnnotation, ok := annotation[ovn4nfvAnnotationTag]
	if !ok {
		return nil, fmt.Errorf("Error while obtaining pod annotations")
	}
        result := cr.addMultipleInterfaces(ovnAnnotation, namespace, podname)
	//Add Routes to the pod if annotation found for routes
	ovnRouteAnnotation, ok := annotation["ovnNetworkRoutes"]
	if ok {
		klog.Infof("ovn4nfvk8s-cni: ovnNetworkRoutes Annotation Found %+v", ovnRouteAnnotation)
		result = cr.addRoutes(ovnRouteAnnotation, result)
	}

        if result == nil {
                klog.Errorf("result struct the ovn4nfv-k8s-plugin cniserver")
                return nil, fmt.Errorf("result is nil from cni server response")
        }

        responseBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pod request response: %v", err)
	}

	return responseBytes, nil
}

func (cr *CNIServerRequest) cmdDel() ([]byte, error) {
        klog.Infof("cmdDel ")
        for i := 0; i < 10; i++ {
		ifaceName := cr.SandboxID[:14] + strconv.Itoa(i)
		done, err := app.PlatformSpecificCleanup(ifaceName)
		if err != nil {
			klog.Errorf("Teardown error: %v", err)
		}
		if done {
			break
		}
	}
        return []byte{}, nil
}
