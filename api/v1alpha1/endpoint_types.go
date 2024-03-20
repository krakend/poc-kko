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

package v1alpha1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EndpointSpec defines the desired state of Endpoint
type EndpointSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Parent string `json:"parent"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Endpoint string `json:"endpoint"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Method string `json:"method,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ConcurrentCalls int `json:"concurrent_calls,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Timeout time.Duration `json:"timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	CacheTTL time.Duration `json:"cache_ttl,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	OutputEncoding string `json:"output_encoding,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	QueryString []string `json:"input_query_strings,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	HeadersToPass []string `json:"input_headers,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Backend []BackendForEndpoint `json:"backend,omitempty"`
	// // +operator-sdk:csv:customresourcedefinitions:type=spec
	// ExtraConfig ExtraConfig `json:"extra_config,omitempty"`
}

type BackendForEndpoint struct {
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	Group string `json:"group,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	Method string `json:"method,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	Host []string `json:"host,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	HostSanitizationDisabled bool `json:"disable_host_sanitize,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	URLPattern string `json:"url_pattern,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	AllowList []string `json:"allow,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	DenyList []string `json:"deny,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	Mapping map[string]string `json:"mapping,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	Encoding string `json:"encoding,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	IsCollection bool `json:"is_collection,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	Target string `json:"target,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	SD string `json:"sd,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	SDScheme string `json:"sd_scheme,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	HeadersToPass []string `json:"input_headers,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=backend
	QueryStringsToPass []string `json:"input_query_strings,omitempty"`
	// // +operator-sdk:csv:customresourcedefinitions:type=backend
	// ExtraConfig ExtraConfig `json:"extra_config,omitempty"`
}

//type ExtraConfig map[string]interface{}

// EndpointStatus defines the observed state of Endpoint
type EndpointStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Represents the observations of a Memcached's current state.
	// Memcached.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// Memcached.status.conditions.status are one of True, False, Unknown.
	// Memcached.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// Memcached.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Conditions store the status conditions of the KrakenD instances
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Endpoint is the Schema for the endpoints API
type Endpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EndpointSpec   `json:"spec,omitempty"`
	Status EndpointStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EndpointList contains a list of Endpoint
type EndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Endpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Endpoint{}, &EndpointList{})
}
