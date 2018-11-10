package ovn

import (
	"fmt"
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
