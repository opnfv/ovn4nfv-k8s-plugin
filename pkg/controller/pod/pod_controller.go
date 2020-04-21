package pod

import (
	"context"
	"encoding/json"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"ovn4nfv-k8s-plugin/internal/pkg/ovn"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_pod")

const (
	nfnNetworkAnnotation = "k8s.plugin.opnfv.org/nfn-network"
)

type nfnNetwork struct {
	Type      string                   "json:\"type\""
	Interface []map[string]interface{} "json:\"interface\""
}

var enableOvnDefaultIntf bool = true
// Add creates a new Pod Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePod{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {

	// Create a new Controller that will call the provided Reconciler function in response
	// to events.
	c, err := controller.New("pod-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	// Define Predicates On Create and Update function
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			annotaion := e.MetaNew.GetAnnotations()
			// The object doesn't contain annotation ,nfnNetworkAnnotation so the event will be
			// ignored.
			if _, ok := annotaion[nfnNetworkAnnotation]; !ok {
				return false
			}
			// If pod is already processed by OVN don't add event
			if _, ok := annotaion[ovn.Ovn4nfvAnnotationTag]; ok {
				return false
			}
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			// The object doesn't contain annotation ,nfnNetworkAnnotation so the event will be
			// ignored.
			/*annotaion := e.Meta.GetAnnotations()
			if _, ok := annotaion[nfnNetworkAnnotation]; !ok {
				return false
			}*/
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// The object doesn't contain annotation ,nfnNetworkAnnotation so the event will be
			// ignored.
			/*annotaion := e.Meta.GetAnnotations()
			if _, ok := annotaion[nfnNetworkAnnotation]; !ok {
				return false
			}*/
			return true
		},
	}

	// Watch for Pod create / update / delete events and call Reconcile
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcuilePod implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePod{}

// ReconcilePod reconciles a ProviderNetwork object
type ReconcilePod struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile function
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePod) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Enter Reconciling Pod")

	// Fetch the Pod instance
	instance := &corev1.Pod{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Delete Pod", "request", request)
			r.deleteLogicalPorts(request.Name, request.Namespace)
			reqLogger.Info("Exit Reconciling Pod")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	if instance.Name == "" || instance.Namespace == "" {
		return reconcile.Result{}, nil
	}
	if instance.Spec.HostNetwork {
		return reconcile.Result{}, nil
	}

	err = r.addLogicalPorts(instance)
	if err != nil && err.Error() == "Failed to add ports" {
		// Requeue the object
		return reconcile.Result{}, err
	}
	reqLogger.Info("Exit Reconciling Pod")
	return reconcile.Result{}, nil
}

// annotatePod annotates pod with the given annotations
func (r *ReconcilePod) setPodAnnotation(pod *corev1.Pod, key, value string) error {

	patchData := fmt.Sprintf(`{"metadata":{"annotations":{"%s":"%s"}}}`, key, value)
	err := r.client.Patch(context.TODO(), pod, client.ConstantPatch(types.MergePatchType, []byte(patchData)))
	if err != nil {
		log.Error(err, "Updating pod failed", "pod", pod, "key", key, "value", value)
		return err
	}
	return nil
}

func (r *ReconcilePod) addLogicalPorts(pod *corev1.Pod) error {

	nfn, err := r.readPodAnnotation(pod)
	if err != nil {
		// No annotation for multiple interfaces
		nfn = &nfnNetwork {Interface: nil}
		if enableOvnDefaultIntf == true {
			nfn.Type = "ovn4nfv"
		} else {
			return err
		}
	}

	switch {
	case nfn.Type == "ovn4nfv":
		ovnCtl, err := ovn.GetOvnController()
		if err != nil {
			return err
		}
        if _, ok := pod.Annotations[ovn.Ovn4nfvAnnotationTag]; ok {
			return fmt.Errorf("Pod annotation found")
		}
		key, value := ovnCtl.AddLogicalPorts(pod, nfn.Interface)
		if len(key) > 0 {
			return r.setPodAnnotation(pod, key, value)
		}
		return fmt.Errorf("Failed to add ports")
	default:
		return fmt.Errorf("Unsupported Networking type %s", nfn.Type)
	// Add other types here
	}
}

func (r *ReconcilePod) deleteLogicalPorts(name, namesapce string) error {

	// Run delete for all controllers; pod annonations inaccessible
	ovnCtl, err := ovn.GetOvnController()
	if err != nil {
		return err
	}
	log.Info("Calling DeleteLogicalPorts")
	ovnCtl.DeleteLogicalPorts(name, namesapce)
	return nil
	// Add other types here
}

func (r *ReconcilePod) readPodAnnotation(pod *corev1.Pod) (*nfnNetwork, error) {
	annotaion, ok := pod.Annotations[nfnNetworkAnnotation]
	if !ok {
		return nil, fmt.Errorf("Invalid annotations")
	}
	var nfn nfnNetwork
	err := json.Unmarshal([]byte(annotaion), &nfn)
	if err != nil {
		log.Error(err, "Invalid nfn annotaion", "annotaiton", annotaion)
		return nil, err
	}
	return &nfn, nil
}
