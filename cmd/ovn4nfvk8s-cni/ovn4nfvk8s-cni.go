// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"k8s.io/apimachinery/pkg/util/wait"

	"ovn4nfv-k8s-plugin/internal/pkg/kube"

	"ovn4nfv-k8s-plugin/cmd/ovn4nfvk8s-cni/app"
	"ovn4nfv-k8s-plugin/internal/pkg/config"
)

const (
	ovn4nfvAnnotationTag = "k8s.plugin.opnfv.org/ovnInterfaces"
)

func argString2Map(args string) (map[string]string, error) {
	argsMap := make(map[string]string)

	pairs := strings.Split(args, ";")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 {
			return nil, fmt.Errorf("ARGS: invalid pair %q", pair)
		}
		keyString := kv[0]
		valueString := kv[1]
		argsMap[keyString] = valueString
	}

	return argsMap, nil
}

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

func addMultipleInterfaces(args *skel.CmdArgs, ovnAnnotation, namespace, podName string) types.Result {
	logrus.Infof("ovn4nfvk8s-cni: addMultipleInterfaces ")

	var ovnAnnotatedMap []map[string]string
	ovnAnnotatedMap, err := parseOvnNetworkObject(ovnAnnotation)
	if err != nil {
		logrus.Errorf("addLogicalPort : Error Parsing Ovn Network List %v %v", ovnAnnotatedMap, err)
		return nil
	}
	if namespace == "" || podName == "" {
		logrus.Errorf("required CNI variable missing")
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
			logrus.Errorf("failed in pod annotation key extract")
			return nil
		}

		index++
		interfaceName := ovnNet["interface"]
		if interfaceName == "" {
			logrus.Errorf("addMultipleInterfaces: interface can't be null")
			return nil
		}
		interfacesArray, err = app.ConfigureInterface(args, namespace, podName, macAddress, ipAddress, gatewayIP, interfaceName, defaultGateway, index, config.Default.MTU)
		if err != nil {
			logrus.Errorf("Failed to configure interface in pod: %v", err)
			return nil
		}
		addr, addrNet, err := net.ParseCIDR(ipAddress)
		if err != nil {
			logrus.Errorf("failed to parse IP address %q: %v", ipAddress, err)
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
			logrus.Errorf("Failed to merge results: %v", err)
			return nil
		}
	}
	logrus.Infof("addMultipleInterfaces:  %s", prettyPrint(dstResult))
	return dstResult
}

func addRoutes(args *skel.CmdArgs, ovnAnnotation string, dstResult types.Result) types.Result {
	logrus.Infof("ovn4nfvk8s-cni: addRoutes ")

	var ovnAnnotatedMap []map[string]string
	ovnAnnotatedMap, err := parseOvnNetworkObject(ovnAnnotation)
	if err != nil {
		logrus.Errorf("addLogicalPort : Error Parsing Ovn Route List %v", err)
		return nil
	}

	var result types.Result
	var routes []*types.Route
	for _, ovnNet := range ovnAnnotatedMap {
		dst := ovnNet["dst"]
		gw := ovnNet["gw"]
		dev := ovnNet["dev"]
		if dst == "" || gw == "" || dev == "" {
			logrus.Errorf("failed in pod annotation key extract")
			return nil
		}
		err = app.ConfigureRoute(args, dst, gw, dev)
		if err != nil {
			logrus.Errorf("Failed to configure interface in pod: %v", err)
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
		logrus.Errorf("Failed to merge results: %v", err)
		return nil
	}
	logrus.Infof("addRoutes:  %s", prettyPrint(dstResult))
	return dstResult

}

func cmdAdd(args *skel.CmdArgs) error {
	logrus.Infof("ovn4nfvk8s-cni: cmdAdd ")
	conf := &types.NetConf{}
	if err := json.Unmarshal(args.StdinData, conf); err != nil {
		return fmt.Errorf("failed to load netconf: %v", err)
	}

	argsMap, err := argString2Map(args.Args)
	if err != nil {
		return err
	}

	namespace := argsMap["K8S_POD_NAMESPACE"]
	podName := argsMap["K8S_POD_NAME"]
	if namespace == "" || podName == "" {
		return fmt.Errorf("required CNI variable missing")
	}

	clientset, err := config.NewClientset(&config.Kubernetes)
	if err != nil {
		return fmt.Errorf("Could not create clientset for kubernetes: %v", err)
	}
	kubecli := &kube.Kube{KClient: clientset}

	// Get the IP address and MAC address from the API server.
	var annotationBackoff = wait.Backoff{Duration: 1 * time.Second, Steps: 14, Factor: 1.5, Jitter: 0.1}
	var annotation map[string]string
	if err := wait.ExponentialBackoff(annotationBackoff, func() (bool, error) {
		annotation, err = kubecli.GetAnnotationsOnPod(namespace, podName)
		if err != nil {
			// TODO: check if err is non recoverable
			logrus.Warningf("Error while obtaining pod annotations - %v", err)
			return false, nil
		}
		if _, ok := annotation[ovn4nfvAnnotationTag]; ok {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("failed to get pod annotation - %v", err)
	}
	logrus.Infof("ovn4nfvk8s-cni: Annotation Found ")
	ovnAnnotation, ok := annotation[ovn4nfvAnnotationTag]
	if !ok {
		return fmt.Errorf("Error while obtaining pod annotations")
	}
	result := addMultipleInterfaces(args, ovnAnnotation, namespace, podName)
	// Add Routes to the pod if annotation found for routes
	ovnRouteAnnotation, ok := annotation["ovnNetworkRoutes"]
	if ok {
		logrus.Infof("ovn4nfvk8s-cni: ovnNetworkRoutes Annotation Found %+v", ovnRouteAnnotation)
		result = addRoutes(args, ovnRouteAnnotation, result)
	}

	return result.Print()
}

func cmdDel(args *skel.CmdArgs) error {
	logrus.Infof("ovn4nfvk8s-cni: cmdDel ")
	for i := 0; i < 10; i++ {
		ifaceName := args.ContainerID[:14] + strconv.Itoa(i)
		done, err := app.PlatformSpecificCleanup(ifaceName)
		if err != nil {
			logrus.Errorf("Teardown error: %v", err)
		}
		if done {
			break
		}
	}
	return nil
}

func main() {
	logrus.Infof("ovn4nfvk8s-cni invoked")
	c := cli.NewApp()
	c.Name = "ovn4nfvk8s-cni"
	c.Usage = "a CNI plugin to set up or tear down a additional interfaces with OVN"
	c.Version = "0.0.2"
	c.Flags = config.Flags

	c.Action = func(ctx *cli.Context) error {
		if _, err := config.InitConfig(ctx); err != nil {
			return err
		}

		skel.PluginMain(cmdAdd, nil, cmdDel, version.All, "")
		return nil
	}

	if err := c.Run(os.Args); err != nil {
		// Print the error to stdout in conformance with the CNI spec
		e, ok := err.(*types.Error)
		if !ok {
			e = &types.Error{Code: 100, Msg: err.Error()}
		}
		e.Print()
	}
}
