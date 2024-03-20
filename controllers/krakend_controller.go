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
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gatewayv1alpha1 "github.com/krakend/k8s-operator/api/v1alpha1"
)

// Definitions to manage status conditions
const (
	krakendFinalizer = "gateway.krakend.io/finalizer"

	// typeAvailableKrakenD represents the status of the Deployment reconciliation
	typeAvailableKrakenD = "Available"
	// typeDegradedKrakenD represents the status used when the custom resource is deleted and the finalizer operations are must to occur.
	typeDegradedKrakenD = "Degraded"

	configFilePath = "krakend-k8s.json"
)

type KrakendConfig struct {
	gatewayv1alpha1.KrakenDSpec
	Endpoints []gatewayv1alpha1.EndpointSpec `json:"endpoints"`
}

// KrakenDReconciler reconciles a KrakenD object
type KrakenDReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=gateway.krakend.io,resources=krakends,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=gateway.krakend.io,resources=krakends/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=gateway.krakend.io,resources=krakends/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the KrakenD object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *KrakenDReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Memcached instance
	// The purpose is check if the Custom Resource for the Kind Memcached
	// is applied on the cluster if not we return nil to stop the reconciliation
	krakend := &gatewayv1alpha1.KrakenD{}
	err := r.Get(ctx, req.NamespacedName, krakend)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then, it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("krakend resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get krakend")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status are available
	if krakend.Status.Conditions == nil || len(krakend.Status.Conditions) == 0 {
		meta.SetStatusCondition(&krakend.Status.Conditions, metav1.Condition{Type: typeAvailableKrakenD,
			Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err = r.Status().Update(ctx, krakend); err != nil {
			log.Error(err, "Failed to update KrakenD status")
			return ctrl.Result{}, err
		}

		// Let's re-fetch the krakend Custom Resource after update the status
		// so that we have the latest state of the resource on the cluster and we will avoid
		// raise the issue "the object has been modified, please apply
		// your changes to the latest version and try again" which would re-trigger the reconciliation
		// if we try to update it again in the following operations
		if err := r.Get(ctx, req.NamespacedName, krakend); err != nil {
			log.Error(err, "Failed to re-fetch krakend")
			return ctrl.Result{}, err
		}
	}

	// Let's add a finalizer. Then, we can define some operations which should
	// occurs before the custom resource to be deleted.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers
	if !controllerutil.ContainsFinalizer(krakend, krakendFinalizer) {
		log.Info("Adding Finalizer for KrakenD")
		if ok := controllerutil.AddFinalizer(krakend, krakendFinalizer); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err = r.Update(ctx, krakend); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the Memcached instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	if krakend.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(krakend, krakendFinalizer) {
			log.Info("Performing Finalizer Operations for KrakenD before delete CR")

			// Let's add here an status "Downgrade" to define that this resource begin its process to be terminated.
			meta.SetStatusCondition(&krakend.Status.Conditions, metav1.Condition{Type: typeDegradedKrakenD,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", krakend.Name)})

			if err := r.Status().Update(ctx, krakend); err != nil {
				log.Error(err, "Failed to update KrakenD status")
				return ctrl.Result{}, err
			}

			// Perform all operations required before remove the finalizer and allow
			// the Kubernetes API to remove the custom resource.
			r.doFinalizerOperationsForKrakenD(krakend)

			// TODO(user): If you add operations to the doFinalizerOperationsForMemcached method
			// then you need to ensure that all worked fine before deleting and updating the Downgrade status
			// otherwise, you should requeue here.

			// Re-fetch the krakend Custom Resource before update the status
			// so that we have the latest state of the resource on the cluster and we will avoid
			// raise the issue "the object has been modified, please apply
			// your changes to the latest version and try again" which would re-trigger the reconciliation
			if err := r.Get(ctx, req.NamespacedName, krakend); err != nil {
				log.Error(err, "Failed to re-fetch krakend")
				return ctrl.Result{}, err
			}

			meta.SetStatusCondition(&krakend.Status.Conditions, metav1.Condition{Type: typeDegradedKrakenD,
				Status: metav1.ConditionTrue, Reason: "Finalizing",
				Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", krakend.Name)})

			if err := r.Status().Update(ctx, krakend); err != nil {
				log.Error(err, "Failed to update KrakenD status")
				return ctrl.Result{}, err
			}

			log.Info("Removing Finalizer for KrakenD after successfully perform the operations")
			if ok := controllerutil.RemoveFinalizer(krakend, krakendFinalizer); !ok {
				log.Error(err, "Failed to remove finalizer for KrakenD")
				return ctrl.Result{Requeue: true}, nil
			}

			if err := r.Update(ctx, krakend); err != nil {
				log.Error(err, "Failed to remove finalizer for KrakenD")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: krakend.Name, Namespace: krakend.Namespace}, found)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new configmap
		b, err := json.Marshal(KrakendConfig{
			KrakenDSpec: krakend.Spec,
			Endpoints:   []gatewayv1alpha1.EndpointSpec{},
		})
		if err != nil {
			log.Error(err, "Failed to marshal the new KrakenD: "+err.Error(),
				"KrakenD.Namespace", krakend.Namespace, "KrakenD.Name", krakend.Name)
			return ctrl.Result{}, err
		}
		nscm := getConfigMapNamespacedName(krakend)
		configmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: nscm.Name, Namespace: nscm.Namespace},
			Data:       map[string]string{configFilePath: string(b)},
		}

		if err := ctrl.SetControllerReference(krakend, configmap, r.Scheme); err != nil {
			log.Error(err, "Failed to set controller for the new ConfigMap",
				"ConfigMap.Namespace", krakend.Namespace, "ConfigMap.Name", krakend.Name+"-CM")
		}

		if err := r.Create(ctx, configmap); err != nil {
			log.Error(err, "Failed to create ConfigMap",
				"ConfigMap.Namespace", krakend.Namespace, "ConfigMap.Name", krakend.Name+"-CM")
		}

		// Define a new deployment
		dep, err := r.deploymentForKrakenD(krakend, configmap)
		if err != nil {
			log.Error(err, "Failed to define new Deployment resource for KrakenD")

			// The following implementation will update the status
			meta.SetStatusCondition(&krakend.Status.Conditions, metav1.Condition{Type: typeAvailableKrakenD,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Deployment for the custom resource (%s): (%s)", krakend.Name, err)})

			if err := r.Status().Update(ctx, krakend); err != nil {
				log.Error(err, "Failed to update KrakenD status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new Deployment",
			"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		if err = r.Create(ctx, dep); err != nil {
			log.Error(err, "Failed to create new Deployment",
				"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}

		// Define a new service
		service := &corev1.Service{}
		if err := r.Get(ctx, types.NamespacedName{Name: krakend.Name + "-service", Namespace: krakend.Namespace}, service); err != nil && apierrors.IsNotFound(err) {
			service.ObjectMeta = metav1.ObjectMeta{
				Name:      krakend.Name + "-service",
				Namespace: krakend.Namespace,
			}
			service.Spec = corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Protocol: corev1.ProtocolTCP,
						Port:     krakend.Spec.Port,
					},
				},
				Selector: map[string]string{"app.kubernetes.io/name": "KrakenD"},
			}

			if err := ctrl.SetControllerReference(krakend, service, r.Scheme); err != nil {
				log.Error(err, "Failed to set the controller for the new Service",
					"Service.Namespace", dep.Namespace, "Service.Name", dep.Name+"-service")
				return ctrl.Result{}, err
			}

			log.Info("Creating a new Service",
				"Service.Namespace", dep.Namespace, "Service.Name", dep.Name+"-service")
			if err := r.Create(ctx, service); err != nil {
				log.Error(err, "Failed to create new Service",
					"Service.Namespace", dep.Namespace, "Service.Name", dep.Name+"-service")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			log.Error(err, "Failed to find the service")
		}

		ingress := &networkingv1.Ingress{}
		if err := r.Get(ctx, types.NamespacedName{Name: krakend.Name + "-ingress", Namespace: krakend.Namespace}, ingress); err != nil && apierrors.IsNotFound(err) {
			ingress.ObjectMeta = metav1.ObjectMeta{
				Name:      krakend.Name + "-ingress",
				Namespace: krakend.Namespace,
			}
			prefix := networkingv1.PathTypePrefix
			ingress.Spec = networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &prefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: krakend.Name + "-service",
												Port: networkingv1.ServiceBackendPort{
													Number: krakend.Spec.Port,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}

			if err := ctrl.SetControllerReference(krakend, ingress, r.Scheme); err != nil {
				log.Error(err, "Failed to set the controller for the new Ingress",
					"Ingress.Namespace", dep.Namespace, "Ingress.Name", dep.Name+"-ingress")
				return ctrl.Result{}, err
			}

			log.Info("Creating a new Ingress",
				"Ingress.Namespace", dep.Namespace, "Ingress.Name", dep.Name+"-ingress")
			if err := r.Create(ctx, ingress); err != nil {
				log.Error(err, "Failed to create new Ingress",
					"Ingress.Namespace", dep.Namespace, "Ingress.Name", dep.Name+"-ingress")
				return ctrl.Result{}, err
			}
		}

		// Deployment created successfully
		// We will requeue the reconciliation so that we can ensure the state
		// and move forward for the next operations
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	// update the configmap and check if a rolling restart is required
	configmap := &corev1.ConfigMap{}
	cfnn := getConfigMapNamespacedName(krakend)
	if err = r.Get(ctx, cfnn, configmap); err != nil {
		log.Error(err, "Failed to get ConfigMap")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}
	prevKrakend := &gatewayv1alpha1.KrakenDSpec{}
	json.Unmarshal([]byte(configmap.Data[configFilePath]), prevKrakend)

	if !reflect.DeepEqual(krakend.Spec, *prevKrakend) {
		if res, err := updateConfigMap(ctx, r, req.Namespace, req.Name); err != nil {
			return res, err
		}

		// The CRD API is defining that the Memcached type, have a MemcachedSpec.Size field
		// to set the quantity of Deployment instances is the desired state on the cluster.
		// Therefore, the following code will ensure the Deployment size is the same as defined
		// via the Size spec of the Custom Resource which we are reconciling.
		var shouldUpdateDeplyment bool
		if *found.Spec.Replicas != krakend.Spec.Replicas {
			found.Spec.Replicas = &krakend.Spec.Replicas
			shouldUpdateDeplyment = true
		}
		if found.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort != krakend.Spec.Port {
			found.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = krakend.Spec.Port
			shouldUpdateDeplyment = true
		}

		if shouldUpdateDeplyment {
			if err = r.Update(ctx, found); err != nil {
				log.Error(err, "Failed to update Deployment",
					"Deployment.Namespace", found.Namespace, "Deployment.Name", found.Name)

				// Re-fetch the krakend Custom Resource before update the status
				// so that we have the latest state of the resource on the cluster and we will avoid
				// raise the issue "the object has been modified, please apply
				// your changes to the latest version and try again" which would re-trigger the reconciliation
				if err := r.Get(ctx, req.NamespacedName, krakend); err != nil {
					log.Error(err, "Failed to re-fetch krakend")
					return ctrl.Result{}, err
				}

				// The following implementation will update the status
				meta.SetStatusCondition(&krakend.Status.Conditions, metav1.Condition{Type: typeAvailableKrakenD,
					Status: metav1.ConditionFalse, Reason: "Resizing",
					Message: fmt.Sprintf("Failed to update the size for the custom resource (%s): (%s)", krakend.Name, err)})

				if err := r.Status().Update(ctx, krakend); err != nil {
					log.Error(err, "Failed to update KrakenD status")
					return ctrl.Result{}, err
				}

				return ctrl.Result{}, err
			}

			// Now, that we update the size we want to requeue the reconciliation
			// so that we can ensure that we have the latest state of the resource before
			// update. Also, it will help ensure the desired state on the cluster
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// The following implementation will update the status
	meta.SetStatusCondition(&krakend.Status.Conditions, metav1.Condition{Type: typeAvailableKrakenD,
		Status: metav1.ConditionTrue, Reason: "Reconciling",
		Message: fmt.Sprintf("Deployment for custom resource (%s) with %d replicas created successfully", krakend.Name, krakend.Spec.Replicas)})

	if err := r.Status().Update(ctx, krakend); err != nil {
		log.Error(err, "Failed to update KrakenD status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// doFinalizerOperationsForKrakenD will perform the required operations before delete the CR.
func (r *KrakenDReconciler) doFinalizerOperationsForKrakenD(cr *gatewayv1alpha1.KrakenD) {
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

// deploymentForKrakenD returns a KrakenD Deployment object
func (r *KrakenDReconciler) deploymentForKrakenD(
	krakend *gatewayv1alpha1.KrakenD, configmap *corev1.ConfigMap) (*appsv1.Deployment, error) {
	ls := labelsForKrakenD(krakend.Name)

	// Get the Operand image
	image, err := imageForKrakenD()
	if err != nil {
		return nil, err
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      krakend.Name,
			Namespace: krakend.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &krakend.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					// TODO(user): Uncomment the following code to configure the nodeAffinity expression
					// according to the platforms which are supported by your solution. It is considered
					// best practice to support multiple architectures. build your manager image using the
					// makefile target docker-buildx. Also, you can use docker manifest inspect <image>
					// to check what are the platforms supported.
					// More info: https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity
					//Affinity: &corev1.Affinity{
					//	NodeAffinity: &corev1.NodeAffinity{
					//		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					//			NodeSelectorTerms: []corev1.NodeSelectorTerm{
					//				{
					//					MatchExpressions: []corev1.NodeSelectorRequirement{
					//						{
					//							Key:      "kubernetes.io/arch",
					//							Operator: "In",
					//							Values:   []string{"amd64", "arm64", "ppc64le", "s390x"},
					//						},
					//						{
					//							Key:      "kubernetes.io/os",
					//							Operator: "In",
					//							Values:   []string{"linux"},
					//						},
					//					},
					//				},
					//			},
					//		},
					//	},
					//},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						RunAsUser:    &[]int64{1000}[0],
						// IMPORTANT: seccomProfile was introduced with Kubernetes 1.19
						// If you are looking for to produce solutions to be supported
						// on lower versions you must remove this option.
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configmap.Name,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  configFilePath,
											Path: configFilePath,
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{{
						Image:           image,
						Name:            "krakend",
						ImagePullPolicy: corev1.PullAlways,
						Env: []corev1.EnvVar{
							{
								Name:  "FC_OUT",
								Value: "/etc/krakend/none.json",
							},
						},
						// Ensure restrictive context for the container
						// More info: https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
						SecurityContext: &corev1.SecurityContext{
							// WARNING: Ensure that the image used defines an UserID in the Dockerfile
							// otherwise the Pod will not run and will fail with "container has runAsNonRoot and image has non-numeric user"".
							// If you want your workloads admitted in namespaces enforced with the restricted mode in OpenShift/OKD vendors
							// then, you MUST ensure that the Dockerfile defines a User ID OR you MUST leave the "RunAsNonRoot" and
							// "RunAsUser" fields empty.
							RunAsNonRoot: &[]bool{true}[0],
							// The krakend image does not use a non-zero numeric user as the default user.
							// Due to RunAsNonRoot field being set to true, we need to force the user in the
							// container to a non-zero numeric user. We do this using the RunAsUser field.
							// However, if you are looking to provide solution for K8s vendors like OpenShift
							// be aware that you cannot run under its restricted-v2 SCC if you set this value.
							RunAsUser:                &[]int64{1000}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"ALL"},
							},
						},
						Ports: []corev1.ContainerPort{{
							ContainerPort: krakend.Spec.Port,
							Name:          "krakend",
						}},
						Command: []string{
							"/usr/bin/reflex",
							"--all",
							"-G",
							"none.json",
							"-s",
							"--",
							"/usr/bin/krakend",
							"run",
							"-c",
							"/etc/krakend/" + configFilePath,
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "config",
							MountPath: "/etc/krakend",
							ReadOnly:  true,
						}},
					}},
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type: intstr.Int,
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: krakend.Spec.Replicas/2 + 1,
					},
				},
			},
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(krakend, dep, r.Scheme); err != nil {
		return nil, err
	}
	return dep, nil
}

// labelsForKrakenD returns the labels for selecting the resources
// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
func labelsForKrakenD(name string) map[string]string {
	var imageTag string
	image, err := imageForKrakenD()
	if err == nil {
		imageTag = strings.Split(image, ":")[1]
	}
	return map[string]string{"app.kubernetes.io/name": "KrakenD",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/part-of":    "krakend-operator",
		"app.kubernetes.io/created-by": "controller-manager",
	}
}

// imageForMemcached gets the Operand image which is managed by this controller
// from the KRAKEND_IMAGE environment variable defined in the config/manager/manager.yaml
func imageForKrakenD() (string, error) {
	var imageEnvVar = "KRAKEND_IMAGE"
	image, found := os.LookupEnv(imageEnvVar)
	if !found {
		return "", fmt.Errorf("Unable to find %s environment variable with the image", imageEnvVar)
	}
	return image, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KrakenDReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1alpha1.KrakenD{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

func getConfigMapNamespacedName(krakend *gatewayv1alpha1.KrakenD) types.NamespacedName {
	return types.NamespacedName{Name: krakend.Name + "-cm", Namespace: krakend.Namespace}
}
