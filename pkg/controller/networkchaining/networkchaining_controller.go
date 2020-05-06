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

package networkchaining

import (
	"fmt"
	"context"
	notif "ovn4nfv-k8s-plugin/internal/pkg/nfnNotify"
	chaining "ovn4nfv-k8s-plugin/internal/pkg/utils"
	k8sv1alpha1 "ovn4nfv-k8s-plugin/pkg/apis/k8s/v1alpha1"
	"ovn4nfv-k8s-plugin/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_networkchaining")

// Add creates a new NetworkChaining Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileNetworkChaining{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("networkchaining-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource NetworkChaining
	err = c.Watch(&source.Kind{Type: &k8sv1alpha1.NetworkChaining{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileNetworkChaining implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileNetworkChaining{}

// ReconcileNetworkChaining reconciles a NetworkChaining object
type ReconcileNetworkChaining struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}
type reconcileFun func(instance *k8sv1alpha1.NetworkChaining, reqLogger logr.Logger) error
// Reconcile reads that state of the cluster for a NetworkChaining object and makes changes based on the state read
// and what is in the NetworkChaining.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNetworkChaining) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling NetworkChaining")

	// Fetch the NetworkChaining instance
	instance := &k8sv1alpha1.NetworkChaining{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	for _, fun := range []reconcileFun{
		r.reconcileFinalizers,
		r.createChain,
	} {
		if err = fun(instance, reqLogger); err != nil {
			return reconcile.Result{}, err
		}
	}


	return reconcile.Result{}, nil
}
const (
	nfnNetworkChainFinalizer = "nfnCleanUpNetworkChain"
)

func (r *ReconcileNetworkChaining) createChain(cr *k8sv1alpha1.NetworkChaining, reqLogger logr.Logger) error {

	if !cr.DeletionTimestamp.IsZero() {
		// Marked for deletion
		return nil
	}
	switch {
	case cr.Spec.ChainType == "Routing":
		routeList, err := chaining.CalculateRoutes(cr)
		if err != nil {
			return err
		}
		err = notif.SendRouteNotif(routeList, "create")
		if err != nil {
			cr.Status.State = k8sv1alpha1.CreateInternalError
			reqLogger.Error(err, "Error Sending Message")
		} else {
			cr.Status.State = k8sv1alpha1.Created
		}

		err = r.client.Status().Update(context.TODO(), cr)
		if err != nil {
			return err
		}
		return nil
	// Add other Chaining types here
	}
	reqLogger.Info("Chaining type not supported", "name", cr.Spec.ChainType)
	return fmt.Errorf("Chaining type not supported")
}

func (r *ReconcileNetworkChaining) deleteChain(cr *k8sv1alpha1.NetworkChaining, reqLogger logr.Logger) error {

	reqLogger.Info("Delete Chain not implementated")
	return fmt.Errorf("Not Implemented")
}

func (r *ReconcileNetworkChaining) reconcileFinalizers(instance *k8sv1alpha1.NetworkChaining, reqLogger logr.Logger) (err error) {

	if !instance.DeletionTimestamp.IsZero() {
		// Instance marked for deletion
		if utils.Contains(instance.ObjectMeta.Finalizers, nfnNetworkChainFinalizer) {
			reqLogger.V(1).Info("Finalizer found - delete chain")
			if err = r.deleteChain(instance, reqLogger); err != nil {
				reqLogger.Error(err, "Delete chain")
			}
			// Remove the finalizer even if Delete Network fails. Fatal error retry will not resolve
			instance.ObjectMeta.Finalizers = utils.Remove(instance.ObjectMeta.Finalizers, nfnNetworkChainFinalizer)
			if err = r.client.Update(context.TODO(), instance); err != nil {
				reqLogger.Error(err, "Removing Finalizer")
				return err
			}
		}

	} else {
		// If finalizer doesn't exist add it
		if !utils.Contains(instance.GetFinalizers(), nfnNetworkChainFinalizer) {
			instance.SetFinalizers(append(instance.GetFinalizers(), nfnNetworkChainFinalizer))
			if err = r.client.Update(context.TODO(), instance); err != nil {
				reqLogger.Error(err, "Adding Finalizer")
				return err
			}
			reqLogger.V(1).Info("Finalizer added")
		}
	}
	return nil
}
