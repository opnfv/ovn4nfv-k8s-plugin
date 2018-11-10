// +build linux

package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types/current"
	"ovn4nfv-k8s-plugin/cmd/ovn4nfvk8s-cni/app"
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
	args := &skel.CmdArgs{"102103104", "default", "eth0", "", "", nil}

	ovnAnnotation := "[{\"ip_address\":\"172.16.24.2/24\", \"mac_address\":\"0a:00:00:00:00:0c\", \"gateway_ip\": \"172.16.24.1\",\"interface\":\"net0\"}] "
	result := addMultipleInterfaces(args, ovnAnnotation, "default", "pod")
	if result == nil {
		t.Errorf("Failed addMultipleInterfaces %+v", ovnAnnotation)
	}
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
}
