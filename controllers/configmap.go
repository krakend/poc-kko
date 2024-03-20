package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	gatewayv1alpha1 "github.com/krakend/k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ReadWriter interface {
	client.Reader
	client.Writer
}

func updateConfigMap(ctx context.Context, r ReadWriter, namespace, owner string) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	// retrieve the krakend data
	krakend := &gatewayv1alpha1.KrakenD{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: namespace, Name: owner}, krakend); err != nil {
		log.Error(err, "Failed to get parent KrakenD")
		return ctrl.Result{}, err
	}

	// retrieve all the endpoints belonging to the parent
	endpointList := &gatewayv1alpha1.EndpointList{}
	if err := r.List(ctx, endpointList); err != nil {
		log.Error(err, "Failed to get the endpoints")
		return ctrl.Result{}, err
	}

	endpoints := []gatewayv1alpha1.EndpointSpec{}
	for _, e := range endpointList.Items {
		if e.Spec.Parent != krakend.Name {
			continue
		}
		endpoints = append(endpoints, e.Spec)
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].Endpoint < endpoints[j].Endpoint })

	cfg := KrakendConfig{
		KrakenDSpec: krakend.Spec,
		Endpoints:   endpoints,
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		log.Error(err, "Failed to marshal the new configmap")
		return ctrl.Result{}, err
	}

	// retrieve the parent's configmap
	configmap := &corev1.ConfigMap{}
	if err := r.Get(ctx, getConfigMapNamespacedName(krakend), configmap); err != nil {
		log.Error(err, "Failed to get configmap")
		return ctrl.Result{}, err
	}

	if configmap.Data[configFilePath] == string(b) {
		log.Info("ConfigMap up to date",
			"ConfigMap.Namespace", configmap.Namespace, "ConfigMap.Name", configmap.Name)
		return ctrl.Result{}, nil
	}

	log.Info(fmt.Sprintf("Total retrieved endpoints: %d", len(cfg.Endpoints)))

	configmap.Data[configFilePath] = string(b)

	// save the modified parent's configmap
	if err := r.Update(ctx, configmap); err != nil {
		log.Error(err, "Failed to update the ConfigMap",
			"ConfigMap.Namespace", configmap.Namespace, "ConfigMap.Name", configmap.Name)
		return ctrl.Result{}, err
	}

	log.Info("ConfigMap updated", "ConfigMap.Namespace", configmap.Namespace, "ConfigMap.Name", configmap.Name)

	return ctrl.Result{}, nil
}
