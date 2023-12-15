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

// KrakenDSpec defines the desired state of KrakenD
type KrakenDSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Replicas int32 `json:"replicas,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Timeout time.Duration `json:"timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	CacheTTL time.Duration `json:"cache_ttl,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Host []string `json:"host,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Port int32 `json:"port,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Address string `json:"listen_ip,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Version int `json:"version,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	OutputEncoding string `json:"output_encoding,omitempty"`
	// // +operator-sdk:csv:customresourcedefinitions:type=spec
	// ExtraConfig map[string]interface{} `json:"extra_config,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReadTimeout time.Duration `json:"read_timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	WriteTimeout time.Duration `json:"write_timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	IdleTimeout time.Duration `json:"idle_timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ReadHeaderTimeout time.Duration `json:"read_header_timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	DisableKeepAlives bool `json:"disable_keep_alives,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	DisableCompression bool `json:"disable_compression,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MaxIdleConns int `json:"max_idle_connections,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	MaxIdleConnsPerHost int `json:"max_idle_connections_per_host,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	IdleConnTimeout time.Duration `json:"idle_connection_timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ResponseHeaderTimeout time.Duration `json:"response_header_timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ExpectContinueTimeout time.Duration `json:"expect_continue_timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	DialerTimeout time.Duration `json:"dialer_timeout,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	DialerFallbackDelay time.Duration `json:"dialer_fallback_delay,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	DialerKeepAlive time.Duration `json:"dialer_keep_alive,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	DisableStrictREST bool `json:"disable_rest,omitempty"`

	// Plugin *Plugin `json:"plugin,omitempty"`
	// TLS *TLS `json:"tls,omitempty"`
	// ClientTLS *ClientTLS `json:"client_tls,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	UseH2C bool `json:"use_h2c,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Debug bool `json:"debug_endpoint,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Echo bool `json:"echo_endpoint,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	SequentialStart bool `json:"sequential_start,omitempty"`
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	AllowInsecureConnections bool `json:"allow_insecure_connections,omitempty"`
}

// KrakenDStatus defines the observed state of KrakenD
type KrakenDStatus struct {
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

// KrakenD is the Schema for the krakends API
// +kubebuilder:subresource:status
type KrakenD struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KrakenDSpec   `json:"spec,omitempty"`
	Status KrakenDStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KrakenDList contains a list of KrakenD
type KrakenDList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KrakenD `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KrakenD{}, &KrakenDList{})
}
