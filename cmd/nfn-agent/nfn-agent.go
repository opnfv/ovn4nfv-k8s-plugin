package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"io"
	kexec "k8s.io/utils/exec"
	"os"
	"os/signal"
	pb "ovn4nfv-k8s-plugin/internal/pkg/nfnNotify/proto"
	"ovn4nfv-k8s-plugin/internal/pkg/ovn"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strings"
	"syscall"
	"time"
	//"google.golang.org/grpc/keepalive"
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

func handleNotif(msg *pb.Notification) {
	var err error
	switch msg.GetCniType() {
	case "ovn4nfv":
		switch payload := msg.Payload.(type) {
		case *pb.Notification_ProviderNwCreate:
			if !inSync {
				// Store Msgs
				pnCreateStore = append(pnCreateStore, payload)
				return
			}
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
				return
			}
			ovn.CreatePnBridge("nw_"+name, "br-"+name, ln)
		case *pb.Notification_ProviderNwRemove:
			if !inSync {
				// Unexpected Remove message
				return
			}
			ln := payload.ProviderNwRemove.GetVlanLogicalIntf()
			name := payload.ProviderNwRemove.GetProviderNwName()
			ovn.DeleteVlan(ln)
			ovn.DeletePnBridge("nw_"+name, "br-"+name)
		case *pb.Notification_InSync:
			// Read config from node
			vlanList := ovn.GetVlan()
			pnBridgeList := ovn.GetPnBridge("nfn")
			diffVlan := make(map[string]bool)
			diffPnBridge := make(map[string]bool)
		VLAN:
			for _, pn := range pnCreateStore {
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

	// Run client in background
	go subscribeNotif(client)
	shutdownHandler(errorChannel)

}
