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

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	cs "ovn4nfv-k8s-plugin/internal/pkg/cniserver"
	pb "ovn4nfv-k8s-plugin/internal/pkg/nfnNotify/proto"
        chaining "ovn4nfv-k8s-plugin/internal/pkg/utils"
	"ovn4nfv-k8s-plugin/internal/pkg/ovn"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kexec "k8s.io/utils/exec"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	//"google.golang.org/grpc/keepalive"

	"ovn4nfv-k8s-plugin/cmd/ovn4nfvk8s-cni/app"

	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var log = logf.Log.WithName("nfn-agent")
var errorChannel chan string
var inSync bool
var pnCreateStore []*pb.Notification_ProviderNwCreate

// subscribe Notifications
func subscribeNotif(client pb.NfnNotifyClient) error {
	log.Info("Subscribe Notification from server")
	ctx := context.Background()
	var n pb.SubscribeContext
	n.NodeName = os.Getenv("NFN_NODE_NAME")
	for {
		stream, err := client.Subscribe(ctx, &n, grpc.WaitForReady(true))
		if err != nil {
			log.Error(err, "Subscribe", "client", client, "status", status.Code(err))
			continue
		}
		log.Info("Subscribe Notification success")

		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				shutDownAgent("Stream closed")
				return err
			}
			if err != nil {
				log.Error(err, "Stream closed from server")
				shutDownAgent("Stream closed from server")
				return err
			}
			log.Info("Got message", "msg", in)
			handleNotif(in)
		}
	}
}

func createVlanProvidernetwork(payload *pb.Notification_ProviderNwCreate) error {
	var err error
	vlanID := payload.ProviderNwCreate.GetVlan().GetVlanId()
	ln := payload.ProviderNwCreate.GetVlan().GetLogicalIntf()
	pn := payload.ProviderNwCreate.GetVlan().GetProviderIntf()
	name := payload.ProviderNwCreate.GetProviderNwName()
	if ln == "" {
		ln = name + "." + vlanID
	}
	err = ovn.CreateVlan(vlanID, pn, ln)
	if err != nil {
		log.Error(err, "Unable to create VLAN", "vlan", ln)
		return err
	}
	err = ovn.CreatePnBridge("nw_"+name, "br-"+name, ln)
	if err != nil {
		log.Error(err, "Unable to create vlan direct bridge", "vlan", pn)
		return err
	}
	return nil
}

func createDirectProvidernetwork(payload *pb.Notification_ProviderNwCreate) error {
	var err error
	pn := payload.ProviderNwCreate.GetDirect().GetProviderIntf()
	name := payload.ProviderNwCreate.GetProviderNwName()
	err = ovn.CreatePnBridge("nw_"+name, "br-"+name, pn)
	if err != nil {
		log.Error(err, "Unable to create direct bridge", "direct", pn)
		return err
	}
	return nil
}

func deleteVlanProvidernetwork(payload *pb.Notification_ProviderNwRemove) {
	ln := payload.ProviderNwRemove.GetVlanLogicalIntf()
	name := payload.ProviderNwRemove.GetProviderNwName()
	ovn.DeleteVlan(ln)
	ovn.DeletePnBridge("nw_"+name, "br-"+name)
}

func deleteDirectProvidernetwork(payload *pb.Notification_ProviderNwRemove) {
	ln := payload.ProviderNwRemove.GetVlanLogicalIntf()
	name := payload.ProviderNwRemove.GetProviderNwName()
	ovn.DeleteVlan(ln)
	ovn.DeletePnBridge("nw_"+name, "br-"+name)
}

func inSyncVlanProvidernetwork() {
	var err error
	// Read config from node
	vlanList := ovn.GetVlan()
	pnBridgeList := ovn.GetPnBridge("nfn")
	diffVlan := make(map[string]bool)
	diffPnBridge := make(map[string]bool)
VLAN:
	for _, pn := range pnCreateStore {
		if pn.ProviderNwCreate.GetVlan() != nil {
			continue
		}
		id := pn.ProviderNwCreate.GetVlan().GetVlanId()
		ln := pn.ProviderNwCreate.GetVlan().GetLogicalIntf()
		pn := pn.ProviderNwCreate.GetVlan().GetProviderIntf()
		if ln == "" {
			ln = pn + "." + id
		}
		for _, vlan := range vlanList {
			if vlan == ln {
				// VLAN already present
				diffVlan[vlan] = true
				continue VLAN
			}
		}
		// Vlan not found
		err = ovn.CreateVlan(id, pn, ln)
		if err != nil {
			log.Error(err, "Unable to create VLAN", "vlan", ln)
			return
		}
	}
PRNETWORK:
	for _, pn := range pnCreateStore {
		if pn.ProviderNwCreate.GetVlan() != nil {
			continue
		}
		ln := pn.ProviderNwCreate.GetVlan().GetLogicalIntf()
		name := pn.ProviderNwCreate.GetProviderNwName()
		for _, br := range pnBridgeList {
			pnName := strings.Replace(br, "br-", "", -1)
			if name == pnName {
				diffPnBridge[br] = true
				continue PRNETWORK
			}
		}
		// Provider Network not found
		ovn.CreatePnBridge("nw_"+name, "br-"+name, ln)
	}
	// Delete VLAN not in the list
	for _, vlan := range vlanList {
		if diffVlan[vlan] == false {
			ovn.DeleteVlan(vlan)
		}
	}
	// Delete Provider Bridge not in the list
	for _, br := range pnBridgeList {
		if diffPnBridge[br] == false {
			name := strings.Replace(br, "br-", "", -1)
			ovn.DeletePnBridge("nw_"+name, "br-"+name)
		}
	}
}

func inSyncDirectProvidernetwork() {
	// Read config from node
	pnBridgeList := ovn.GetPnBridge("nfn")
	diffPnBridge := make(map[string]bool)
DIRECTPRNETWORK:
	for _, pn := range pnCreateStore {
		if pn.ProviderNwCreate.GetDirect() != nil {
			continue
		}
		pr := pn.ProviderNwCreate.GetDirect().GetProviderIntf()
		name := pn.ProviderNwCreate.GetProviderNwName()
		for _, br := range pnBridgeList {
			pnName := strings.Replace(br, "br-", "", -1)
			if name == pnName {
				diffPnBridge[br] = true
				continue DIRECTPRNETWORK
			}
		}
		// Provider Network not found
		ovn.CreatePnBridge("nw_"+name, "br-"+name, pr)
	}
	// Delete Provider Bridge not in the list
	for _, br := range pnBridgeList {
		if diffPnBridge[br] == false {
			name := strings.Replace(br, "br-", "", -1)
			ovn.DeletePnBridge("nw_"+name, "br-"+name)
		}
	}
}

func createNodeOVSInternalPort(payload *pb.Notification_InSync) error {
	nodeIntfIPAddr := strings.Trim(strings.TrimSpace(payload.InSync.GetNodeIntfIpAddress()), "\"")
	nodeIntfMacAddr := strings.Trim(strings.TrimSpace(payload.InSync.GetNodeIntfMacAddress()), "\"")
	nodeName := os.Getenv("NFN_NODE_NAME")

	err := app.CreateNodeOVSInternalPort(nodeIntfIPAddr, nodeIntfMacAddr, nodeName)
	if err != nil {
		return err
	}

	return nil
}

func handleNotif(msg *pb.Notification) {
	switch msg.GetCniType() {
	case "ovn4nfv":
		switch payload := msg.Payload.(type) {
		case *pb.Notification_ProviderNwCreate:
			if !inSync {
				// Store Msgs
				pnCreateStore = append(pnCreateStore, payload)
				return
			}
			if payload.ProviderNwCreate.GetVlan() != nil {
				err := createVlanProvidernetwork(payload)
				if err != nil {
					return
				}
			}

			if payload.ProviderNwCreate.GetDirect() != nil {
				err := createDirectProvidernetwork(payload)
				if err != nil {
					return
				}
			}
		case *pb.Notification_ProviderNwRemove:
			if !inSync {
				// Unexpected Remove message
				return
			}

			if payload.ProviderNwRemove.GetVlanLogicalIntf() != "" {
				deleteVlanProvidernetwork(payload)
			}

			if payload.ProviderNwRemove.GetDirectProviderIntf() != "" {
				deleteDirectProvidernetwork(payload)
			}

		case *pb.Notification_ContainterRtInsert:
			id := payload.ContainterRtInsert.GetContainerId()
			pid, err := chaining.GetPidForContainer(id)
			if err != nil {
				log.Error(err, "Failed to get pid", "containerID", id)
				return
			}
			err = chaining.ContainerAddRoute(pid, payload.ContainterRtInsert.GetRoute())
			if err != nil {
				return
			}

		case *pb.Notification_InSync:
			if payload.InSync.GetNodeIntfIpAddress() != "" && payload.InSync.GetNodeIntfMacAddress() != "" {
				err := createNodeOVSInternalPort(payload)
				if err != nil {
					return
				}
			}
			inSyncVlanProvidernetwork()
			inSyncDirectProvidernetwork()
			pnCreateStore = nil
			inSync = true

		}
	// Add other Types here
	default:
		log.Info("Not supported cni type", "cni type", msg.GetCniType())
	}
}

func shutdownHandler(errorChannel <-chan string) {
	// Register to receive term/int signal.
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	signal.Notify(signalChan, syscall.SIGINT)
	signal.Notify(signalChan, syscall.SIGHUP)

	var reason string
	select {
	case sig := <-signalChan:
		if sig == syscall.SIGHUP {
			log.Info("Received a SIGHUP")
		}
		reason = fmt.Sprintf("Received OS signal %v", sig)
	case reason = <-errorChannel:
		log.Info("Error", "reason", reason)
	}
	log.Info("nfn-agent is shutting down", "reason", reason)
}

func shutDownAgent(reason string) {
	// Send a failure message and give few seconds complete shutdown.
	log.Info("shutDownAgent recieved")
	errorChannel <- reason
	time.Sleep(10 * time.Second)
	// The graceful shutdown failed, terminate the process.
	panic("Shutdown failed. Panicking.")
}

func main() {
	logf.SetLogger(zap.Logger(true))
	log.Info("nfn-agent Started")

	serverAddr := os.Getenv("NFN_OPERATOR_SERVICE_HOST") + ":" + os.Getenv("NFN_OPERATOR_SERVICE_PORT")
	// Setup ovn utilities
	exec := kexec.New()
	err := ovn.SetExec(exec)
	if err != nil {
		log.Error(err, "Unable to setup OVN Utils")
		return
	}
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Error(err, "fail to dial")
		return
	}
	defer conn.Close()
	client := pb.NewNfnNotifyClient(conn)
	errorChannel = make(chan string)

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Unable to create in-cluster config")
		return
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err, "Unable to create clientset for in-cluster config")
		return
	}

	cniserver := cs.NewCNIServer("", clientset)
	err = cniserver.Start(cs.HandleCNIcommandRequest)
	if err != nil {
		log.Error(err, "Unable to start cni server")
		return
	}
	// Run client in background
	go subscribeNotif(client)
	shutdownHandler(errorChannel)

}
