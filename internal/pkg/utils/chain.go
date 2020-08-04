/*
 * Copyright 2020 Intel Corporation, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package nfn

import (
	"context"
	"fmt"
	"ovn4nfv-k8s-plugin/internal/pkg/network"
	"ovn4nfv-k8s-plugin/internal/pkg/ovn"
	k8sv1alpha1 "ovn4nfv-k8s-plugin/pkg/apis/k8s/v1alpha1"
	"strings"

	pb "ovn4nfv-k8s-plugin/internal/pkg/nfnNotify/proto"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/docker/docker/client"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("chaining")

type RoutingInfo struct {
	Name                 string            // Name of the pod
	Namespace            string            // Namespace of the Pod
	Id                   string            // Container ID for pod
	Node                 string            // Hostname where Pod is scheduled
	LeftNetworkRoute     k8sv1alpha1.Route // TODO: Update to support multiple networks
	RightNetworkRoute    k8sv1alpha1.Route // TODO: Update to support multiple networks
	DynamicNetworkRoutes []k8sv1alpha1.Route
}

var chainRoutingInfo []RoutingInfo

// Calcuate route to get to left and right edge networks and other networks (not adjacent) in the chain
func calculateDeploymentRoutes(namespace, label string, pos int, num int, ln []k8sv1alpha1.RoutingNetwork, rn []k8sv1alpha1.RoutingNetwork, networkList, deploymentList []string) (r RoutingInfo, err error) {

	var nextLeftIP string
	var nextRightIP string

	r.Namespace = namespace
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		return RoutingInfo{}, err
	}
	var k *kubernetes.Clientset
	k, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "Error building Kuberenetes clientset")
		return RoutingInfo{}, err
	}
	lo := v1.ListOptions{LabelSelector: label}
	// List the Pods matching the Labels
	pods, err := k.CoreV1().Pods(namespace).List(lo)
	if err != nil {
		log.Error(err, "Deloyment with label not found", "label", label)
		return RoutingInfo{}, err
	}
	// LOADBALANCER NOT YET SUPPORTED - Assuming deployment has only one Pod
	if len(pods.Items) <= 0 {
		log.Error(err, "Deloyment with label not found", "label", label)
		return RoutingInfo{}, fmt.Errorf("Pod not found")
	}
	// Get the containerID of the first container
	r.Id = strings.TrimPrefix(pods.Items[0].Status.ContainerStatuses[0].ContainerID, "docker://")
	r.Name = pods.Items[0].GetName()
	r.Node = pods.Items[0].Spec.NodeName
	// Calcluate IP addresses for next neighbours on both sides
	if pos == 0 {
		nextLeftIP = ln[0].GatewayIP
	} else {
		name := strings.Split(deploymentList[pos-1], "=")
		nextLeftIP, err = ovn.GetIPAdressForPod(networkList[pos-1], name[1])
		if err != nil {
			return RoutingInfo{}, err
		}
	}
	if pos == num-1 {
		nextRightIP = rn[0].GatewayIP
	} else {
		name := strings.Split(deploymentList[pos+1], "=")
		nextRightIP, err = ovn.GetIPAdressForPod(networkList[pos], name[1])
		if err != nil {
			return RoutingInfo{}, err
		}
	}
	// Calcuate left right Route to be inserted in Pod
	r.LeftNetworkRoute.Dst = ln[0].Subnet
	r.LeftNetworkRoute.GW = nextLeftIP
	r.RightNetworkRoute.Dst = rn[0].Subnet
	r.RightNetworkRoute.GW = nextRightIP
	// For each network that is not adjacent add route
	for i := 0; i < len(networkList); i++ {
		if i == pos || i == pos-1 {
			continue
		} else {
			var rt k8sv1alpha1.Route
			rt.Dst, err = ovn.GetNetworkSubnet(networkList[i])
			if err != nil {
				return RoutingInfo{}, err
			}
			if i > pos {
				rt.GW = nextRightIP
			} else {
				rt.GW = nextLeftIP
			}
			r.DynamicNetworkRoutes = append(r.DynamicNetworkRoutes, rt)
		}
	}

	//Add Default Route based on Right Network
	rt := k8sv1alpha1.Route{
		GW:  nextRightIP,
		Dst: "0.0.0.0",
	}
	r.DynamicNetworkRoutes = append(r.DynamicNetworkRoutes, rt)
	return
}

func CalculateRoutes(cr *k8sv1alpha1.NetworkChaining) ([]RoutingInfo, error) {
	//
	var deploymentList []string
	var networkList []string

	// TODO: Add Validation of Input to this function
	ln := cr.Spec.RoutingSpec.LeftNetwork
	rn := cr.Spec.RoutingSpec.RightNetwork
	chains := strings.Split(cr.Spec.RoutingSpec.NetworkChain, ",")
	i := 0
	for _, chain := range chains {
		if i%2 == 0 {
			deploymentList = append(deploymentList, chain)
		} else {
			networkList = append(networkList, chain)
		}
		i++
	}
	num := len(deploymentList)
	log.Info("Display the num", "num", num)
	log.Info("Display the ln", "ln", ln)
	log.Info("Display the rn", "rn", rn)
	log.Info("Display the networklist", "networkList", networkList)
	log.Info("Display the deploymentlist", "deploymentList", deploymentList)
	for i, deployment := range deploymentList {
		r, err := calculateDeploymentRoutes(cr.Namespace, deployment, i, num, ln, rn, networkList, deploymentList)
		if err != nil {
			return nil, err
		}
		chainRoutingInfo = append(chainRoutingInfo, r)
	}
	return chainRoutingInfo, nil
}

func ContainerAddRoute(containerPid int, route []*pb.RouteData) error {
	str := fmt.Sprintf("/host/proc/%d/ns/net", containerPid)

	hostNet, err := network.GetHostNetwork()
	if err != nil {
		log.Error(err, "Failed to get host network")
		return err
	}

	nms, err := ns.GetNS(str)
	if err != nil {
		log.Error(err, "Failed namesapce", "containerID", containerPid)
		return err
	}
	defer nms.Close()
	err = nms.Do(func(_ ns.NetNS) error {
		podGW, err := network.GetDefaultGateway()
		if err != nil {
			log.Error(err, "Failed to get pod default gateway")
			return err
		}

		stdout, stderr, err := ovn.RunIP("route", "add", hostNet, "via", podGW)
		if err != nil && !strings.Contains(stderr, "RTNETLINK answers: File exists") {
			log.Error(err, "Failed to ip route add", "stdout", stdout, "stderr", stderr)
			return err
		}

		for _, r := range route {
			dst := r.GetDst()
			gw := r.GetGw()
			// Replace default route
			if dst == "0.0.0.0" {
				stdout, stderr, err := ovn.RunIP("route", "replace", "default", "via", gw)
				if err != nil {
					log.Error(err, "Failed to ip route replace", "stdout", stdout, "stderr", stderr)
					return err
				}
			} else {
				stdout, stderr, err := ovn.RunIP("route", "add", dst, "via", gw)
				if err != nil && !strings.Contains(stderr, "RTNETLINK answers: File exists") {
					log.Error(err, "Failed to ip route add", "stdout", stdout, "stderr", stderr)
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Error(err, "Failed Netns Do", "containerID", containerPid)
		return err
	}
	return nil
}

func GetPidForContainer(id string) (int, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		fmt.Println("Unable to create docker client")
		return 0, err
	}
	cli.NegotiateAPIVersion(context.Background())
	cj, err := cli.ContainerInspect(context.Background(), id)
	if err != nil {
		fmt.Println("Unable to Inspect docker container")
		return 0, err
	}
	if cj.State.Pid == 0 {
		return 0, fmt.Errorf("Container not found %s", id)
	}
	return cj.State.Pid, nil

}
