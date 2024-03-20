/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gatewayv1alpha1 "github.com/krakend/k8s-operator/api/v1alpha1"
)

// Definitions to manage status conditions
const (
	// typeAvailableEndpoint represents the status of the Deployment reconciliation
	typeAvailableEndpoint = "Available"
	// typeDegradedEndpoint represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedEndpoint = "Degraded"
)

// EndpointReconciler reconciles a Endpoint object
type EndpointReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=gateway.krakend.io,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.krakend.io,resources=endpoints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.krakend.io,resources=endpoints/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Endpoint object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *EndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Endpoint instance
	// The purpose is check if the Custom Resource for the Kind Endpoint
	// is applied on the cluster if not we return nil to stop the reconciliation
	endpoint := &gatewayv1alpha1.Endpoint{}
	if err := r.Get(ctx, req.NamespacedName, endpoint); err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("endpoint resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get endpoint")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status are available
	if endpoint.Status.Conditions == nil || len(endpoint.Status.Conditions) == 0 {
		meta.SetStatusCondition(&endpoint.Status.Conditions, metav1.Condition{Type: typeAvailableEndpoint,
			Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Status().Update(ctx, endpoint); err != nil {
			log.Error(err, "Failed to update endpoint status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the endpoint Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, endpoint); err != nil {
			log.Error(err, "Failed to re-fetch endpoint")
			return ctrl.Result{}, err
		}
	}

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(endpoint, krakendFinalizer) {
		log.Info("Adding Finalizer for KrakenD Endpoint")
		if ok := controllerutil.AddFinalizer(endpoint, krakendFinalizer); !ok {
			log.Error(nil, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, endpoint); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the Endpoint instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if endpoint.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(endpoint, krakendFinalizer) {
			log.Info("Performing Finalizer Operations for KrakenD Endpoint before delete CR")

			// Let's add here an status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&endpoint.Status.Conditions, metav1.Condition{Type: typeDegradedKrakenD,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", endpoint.Name)})

			if err := r.Status().Update(ctx, endpoint); err != nil {
				log.Error(err, "Failed to update KrakenD Endpoint status")
				return ctrl.Result{}, err
			}

			// TODO: remove the endpoint from the configmap

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperationsForEndpoint(endpoint)

			// TODO(user): If you add operations to the doFinalizerOperationsForMemcached method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Re-fetch the krakend Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, endpoint); err != nil {
				log.Error(err, "Failed to re-fetch krakend")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&endpoint.Status.Conditions, metav1.Condition{Type: typeDegradedKrakenD,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", endpoint.Name)})

			if err := r.Status().Update(ctx, endpoint); err != nil {
				log.Error(err, "Failed to update KrakenD status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for KrakenD after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(endpoint, krakendFinalizer); !ok {
				log.Error(nil, "Failed to remove finalizer for KrakenD")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, endpoint); err != nil {
				log.Error(err, "Failed to remove finalizer for KrakenD")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return updateConfigMap(ctx, r, req.Namespace, endpoint.Spec.Parent)
}

// doFinalizerOperationsForEndpoint will perform the required operations before delete the CR.
func (r *EndpointReconciler) doFinalizerOperationsForEndpoint(cr *gatewayv1alpha1.Endpoint) {
	// TODO(user): Add the cleanup steps that the operator
	// needs to do before the CR can be deleted. Examples
	// of finalizers include performing backups and deleting
	// resources that are not owned by this CR, like a PVC.

	// Note: It is not recommended to use finalizers with the purpose of delete resources which are
	// created and managed in the reconciliation. These ones, such as the Deployment created on this reconcile,
	// are defined as depended of the custom resource. See that we use the method ctrl.SetControllerReference.
	// to set the ownerRef which means that the Deployment will be deleted by the Kubernetes API.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/use-cascading-deletion/

	// The following implementation will raise an event
	r.Recorder.Event(cr, "Warning", "Deleting",
		fmt.Sprintf("Custom Resource %s is being deleted from the namespace %s",
			cr.Name,
			cr.Namespace))
}

// SetupWithManager sets up the controller with the Manager.
func (r *EndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1alpha1.Endpoint{}).
		Complete(r)
}
