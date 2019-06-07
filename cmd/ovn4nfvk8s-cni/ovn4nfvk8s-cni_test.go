// +build linux

package main

import (
	"encoding/json"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"net"
	"ovn4nfv-k8s-plugin/cmd/ovn4nfvk8s-cni/app"
	"reflect"
	"testing"
)

func TestAddMultipleInterfaces(t *testing.T) {
	oldConfigureInterface := app.ConfigureInterface
	// as we are exiting, revert ConfigureInterface back  at end of function
	defer func() { app.ConfigureInterface = oldConfigureInterface }()
	app.ConfigureInterface = func(args *skel.CmdArgs, namespace, podName, macAddress, ipAddress, gatewayIP, interfaceName, defaultGateway string, mtu int) ([]*current.Interface, error) {
		return []*current.Interface{
			{
				Name:    "pod",
				Mac:     "0a:00:00:00:00:0c",
				Sandbox: "102103104",
			}}, nil
	}
	oldConfigureRoute := app.ConfigureRoute
	defer func() { app.ConfigureRoute = oldConfigureRoute }()
	app.ConfigureRoute = func(args *skel.CmdArgs, dst, gw, dev string) error {
		return nil
	}

	args := &skel.CmdArgs{"102103104", "default", "eth0", "", "", nil}
	ovnAnnotation := "[{\"ip_address\":\"172.16.24.2/24\", \"mac_address\":\"0a:00:00:00:00:0c\", \"gateway_ip\": \"172.16.24.1\",\"interface\":\"net0\"}] "
	result := addMultipleInterfaces(args, ovnAnnotation, "default", "pod")
	if result == nil {
		t.Errorf("Failed addMultipleInterfaces %+v", ovnAnnotation)
	}
	resultSave := result
	ovnAnnotation = "[{\"ip_address\":\"172.16.24.2/24\", \"mac_address\":\"0a:00:00:00:00:0c\", \"gateway_ip\": \"172.16.24.1\",\"defaultGateway\":\"true\",\"interface\":\"net0\"}] "
	result = addMultipleInterfaces(args, ovnAnnotation, "default", "pod")
	if result == nil {
		t.Errorf("Failed addMultipleInterfaces %+v", ovnAnnotation)
	}
	ovnAnnotation = "[{\"ip_address\":\"172.16.24.2/24\", \"mac_address\":\"0a:00:00:00:00:0c\", \"gateway_ip\": \"172.16.24.1\"}] "
	result = addMultipleInterfaces(args, ovnAnnotation, "default", "pod")
	if result != nil {
		t.Errorf("Failed addMultipleInterfaces %+v", ovnAnnotation)
	}
	ovnAnnotation = "[{\"mac_address\":\"0a:00:00:00:00:0c\", \"gateway_ip\": \"172.16.24.1\",\"interface\":\"net0\"}] "
	result = addMultipleInterfaces(args, ovnAnnotation, "default", "pod")
	if result != nil {
		t.Errorf("Failed addMultipleInterfaces %+v", ovnAnnotation)
	}
	ovnAnnotation = "[{\"ip_address\":\"172.16.24.2/24\", \"mac_address\":\"0a:00:00:00:00:0c\", \"gateway_ip\": \"172.16.24.1\",\"interface\":\"net0\"}, {\"ip_address\":\"172.16.25.2/24\", \"mac_address\":\"0a:00:00:00:00:0d\", \"gateway_ip\": \"172.16.25.1\",\"interface\":\"net1\"}]"
	result = addMultipleInterfaces(args, ovnAnnotation, "default", "pod")
	if result == nil {
		t.Errorf("Failed addMultipleInterfaces %+v", ovnAnnotation)
	}
	ovnAnnotation = "[{\"ip_address\":\"172.16.24.2/24\", \"mac_address\":\"0a:00:00:00:00:0c\", \"gateway_ip\": \"172.16.24.1\",\"interface\":\"net0\", \"defaultGateway\":\"true\"}, {\"ip_address\":\"172.16.25.2/24\", \"mac_address\":\"0a:00:00:00:00:0d\", \"gateway_ip\": \"172.16.25.1\",\"interface\":\"net1\"}]"
	result = addMultipleInterfaces(args, ovnAnnotation, "default", "pod")
	if result == nil {
		t.Errorf("Failed addMultipleInterfaces %+v", ovnAnnotation)
	}
	// Test add route feature
	ovnRoutesAnnotation := "[{ \"dst\": \"172.16.29.0/24\", \"gw\": \"172.16.24.1\", \"dev\": \"eth0\" }]"
	result = addRoutes(args, ovnRoutesAnnotation, resultSave)
	if result == nil {
		t.Errorf("Failed addRoutes %+v", ovnRoutesAnnotation)
	}

	ovnRoutesAnnotation = "[{ \"dst\": \"172.16.30.0/24\", \"gw\": \"172.16.25.1\", \"dev\": \"eth0\"}, { \"dst\": \"172.16.31.0/24\", \"gw\": \"172.16.26.1\", \"dev\": \"eth1\" }]"
	result = addRoutes(args, ovnRoutesAnnotation, resultSave)
	if result == nil {
		t.Errorf("Failed addRoutes %+v", ovnRoutesAnnotation)
	}
	newResult, err := current.NewResultFromResult(result)
	if err != nil {
		t.Errorf("Failed addMultipleInterfaces %+v", newResult)
	}
	addr1, addrNet1, _ := net.ParseCIDR("172.16.24.2/24")
	addr2, addrNet2, _ := net.ParseCIDR("172.16.29.0/24")
	addr3, addrNet3, _ := net.ParseCIDR("172.16.30.0/24")
	addr4, addrNet4, _ := net.ParseCIDR("172.16.31.0/24")
	expectedResult := &current.Result{
		CNIVersion: "0.3.1",
		Interfaces: []*current.Interface{
			{
				Name:    "pod",
				Mac:     "0a:00:00:00:00:0c",
				Sandbox: "102103104",
			},
		},
		IPs: []*current.IPConfig{
			{
				Version:   "4",
				Interface: current.Int(1),
				Address:   net.IPNet{IP: addr1, Mask: addrNet1.Mask},
				Gateway:   net.ParseIP("172.16.24.1"),
			},
		},
		Routes: []*types.Route{
			{
				Dst: net.IPNet{IP: addr2, Mask: addrNet2.Mask},
				GW:  net.ParseIP("172.16.24.1"),
			},
			{
				Dst: net.IPNet{IP: addr3, Mask: addrNet3.Mask},
				GW:  net.ParseIP("172.16.25.1"),
			},
			{
				Dst: net.IPNet{IP: addr4, Mask: addrNet4.Mask},
				GW:  net.ParseIP("172.16.26.1"),
			},
		},
		DNS: types.DNS{},
	}
	jsonBytes1, err := json.Marshal(newResult.Routes)
	jsonBytes2, err := json.Marshal(expectedResult.Routes)
	if !reflect.DeepEqual(jsonBytes1, jsonBytes2) {
		t.Errorf("Routes are not correct")
	}

}
