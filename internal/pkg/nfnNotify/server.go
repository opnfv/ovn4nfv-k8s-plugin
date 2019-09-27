package nfn

import (
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
	pb "ovn4nfv-k8s-plugin/internal/pkg/nfnNotify/proto"
	v1alpha1 "ovn4nfv-k8s-plugin/pkg/apis/k8s/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strings"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientset "ovn4nfv-k8s-plugin/pkg/generated/clientset/versioned"
)

var log = logf.Log.WithName("rpc-server")

type client struct {
	context *pb.SubscribeContext
	stream  pb.NfnNotify_SubscribeServer
}

type serverDB struct {
	name       string
	clientList map[string]client
}

var notifServer *serverDB
var stopChan chan interface{}

var pnClientset *clientset.Clientset
var kubeClientset *kubernetes.Clientset

func newServer() *serverDB {
	return &serverDB{name: "nfnNotifServer", clientList: make(map[string]client)}
}

// Subscribe stores the client information & sends data
func (s *serverDB) Subscribe(sc *pb.SubscribeContext, ss pb.NfnNotify_SubscribeServer) error {
	nodeName := sc.GetNodeName()
	log.Info("Subscribe request from node", "Node Name", nodeName)
	if nodeName == "" {
		return fmt.Errorf("Node name can't be empty")
	}
	cp := client{
		context: sc,
		stream:  ss,
	}
	s.clientList[nodeName] = cp

	providerNetworklist, err := pnClientset.K8sV1alpha1().ProviderNetworks("default").List(v1.ListOptions{})
	if err == nil {
		for _, pn := range providerNetworklist.Items {
			log.Info("Send message", "Provider Network", pn.GetName())
			SendNotif(&pn, "create", nodeName)
		}
	}
	inSyncMsg := pb.Notification{
		CniType: "ovn4nfv",
		Payload: &pb.Notification_InSync{
			InSync: &pb.InSync{},
		},
	}
	log.Info("Send Insync")
	if err = cp.stream.Send(&inSyncMsg); err != nil {
		log.Error(err, "Unable to send sync", "node name", nodeName)
	}
	log.Info("Subscribe Completed")
	// Keep stream open
	for {
		select {
		case <-stopChan:
		}
	}
}

func (s *serverDB) GetClient(nodeName string) client {
	if val, ok := s.clientList[nodeName]; ok {
		return val
	}
	return client{}
}

func updatePnStatus(pn *v1alpha1.ProviderNetwork, status string) error {
	pnCopy := pn.DeepCopy()
	pnCopy.Status.State = status
	_, err := pnClientset.K8sV1alpha1().ProviderNetworks(pn.Namespace).Update(pnCopy)
	return err
}

func createMsg(pn *v1alpha1.ProviderNetwork) pb.Notification {
	msg := pb.Notification{
		CniType: "ovn4nfv",
		Payload: &pb.Notification_ProviderNwCreate{
			ProviderNwCreate: &pb.ProviderNetworkCreate{
				ProviderNwName: pn.Name,
				Vlan: &pb.VlanInfo{
					VlanId:       pn.Spec.Vlan.VlanId,
					ProviderIntf: pn.Spec.Vlan.ProviderInterfaceName,
					LogicalIntf:  pn.Spec.Vlan.LogicalInterfaceName,
				},
			},
		},
	}
	return msg
}

func deleteMsg(pn *v1alpha1.ProviderNetwork) pb.Notification {
	msg := pb.Notification{
		CniType: "ovn4nfv",
		Payload: &pb.Notification_ProviderNwRemove{
			ProviderNwRemove: &pb.ProviderNetworkRemove{
				ProviderNwName:  pn.Name,
				VlanLogicalIntf: pn.Spec.Vlan.LogicalInterfaceName,
			},
		},
	}
	return msg
}

//SendNotif to client
func SendNotif(pn *v1alpha1.ProviderNetwork, msgType string, nodeReq string) error {
	var msg pb.Notification
	var err error

	switch {
	case pn.Spec.CniType == "ovn4nfv":
		switch {
		case pn.Spec.ProviderNetType == "VLAN":
			if msgType == "create" {
				msg = createMsg(pn)
			} else if msgType == "delete" {
				msg = deleteMsg(pn)
			}
			if strings.EqualFold(pn.Spec.Vlan.VlanNodeSelector, "SPECIFIC") {
				for _, label := range pn.Spec.Vlan.NodeLabelList {
					l := strings.Split(label, "=")
					if len(l) == 0 {
						log.Error(fmt.Errorf("Syntax error label: %v", label), "NodeListIterator")
						return nil
					}
				}
				labels := strings.Join(pn.Spec.Vlan.NodeLabelList[:], ",")
				err = sendMsg(msg, labels, "specific", nodeReq)
			} else if strings.EqualFold(pn.Spec.Vlan.VlanNodeSelector, "ALL") {
				err = sendMsg(msg, "", "all", nodeReq)
			} else if strings.EqualFold(pn.Spec.Vlan.VlanNodeSelector, "ANY") {
				if pn.Status.State != v1alpha1.Created {
					err = sendMsg(msg, "", "any", nodeReq)
					if err == nil {
						updatePnStatus(pn, v1alpha1.Created)
					}
				}
			}
		default:
			return fmt.Errorf("Unsupported Provider Network type")
		}
	default:
		return fmt.Errorf("Unsupported CNI type")
	}
	return err
}

// sendMsg send notification to client
func sendMsg(msg pb.Notification, labels string, option string, nodeReq string) error {
	if option == "all" {
		for name, client := range notifServer.clientList {
			if nodeReq != "" && nodeReq != name {
				continue
			}
			if client.stream != nil {
				if err := client.stream.Send(&msg); err != nil {
					log.Error(err, "Msg Send failed", "Node name", name)
				}
			}
		}
		return nil
	} else if option == "any" {
		// Always select the first
		for _, client := range notifServer.clientList {
			if client.stream != nil {
				if err := client.stream.Send(&msg); err != nil {
					return err
				}
				// return after first successful send
				return nil
			}
		}
		return nil
	}
	// This is specific case
	for name := range nodeListIterator(labels) {
		if nodeReq != "" && nodeReq != name {
			continue
		}
		client := notifServer.GetClient(name)
		if client.stream != nil {
			if err := client.stream.Send(&msg); err != nil {
				return err
			}
		}
	}
	return nil
}

func nodeListIterator(labels string) <-chan string {
	ch := make(chan string)

	lo := v1.ListOptions{LabelSelector: labels}
	// List the Nodes matching the Labels
	nodes, err := kubeClientset.CoreV1().Nodes().List(lo)
	if err != nil {
		log.Info("No Nodes found with labels", "list:", lo)
		return nil
	}
	go func() {
		for _, node := range nodes.Items {
			log.Info("Send message to", " node:", node.ObjectMeta.Name)
			ch <- node.ObjectMeta.Name
		}
		close(ch)
	}()
	return ch
}

//SetupNotifServer initilizes the gRpc nfn notif server
func SetupNotifServer(kConfig *rest.Config) {

	log.Info("Starting Notif Server")
	var err error

	// creates the clientset
	pnClientset, err = clientset.NewForConfig(kConfig)
	if err != nil {
		log.Error(err, "Error building clientset")
	}
	kubeClientset, err = kubernetes.NewForConfig(kConfig)
	if err != nil {
		log.Error(err, "Error building Kuberenetes clientset")
	}

	stopChan = make(chan interface{})

	// Start GRPC server
	lis, err := net.Listen("tcp", ":50000")
	if err != nil {
		log.Error(err, "failed to listen")
	}

	s := grpc.NewServer()
	// Intialize Notify server
	notifServer = newServer()
	pb.RegisterNfnNotifyServer(s, notifServer)

	reflection.Register(s)
	log.Info("Initialization Completed")
	if err := s.Serve(lis); err != nil {
		log.Error(err, "failed to serve")
	}
}
