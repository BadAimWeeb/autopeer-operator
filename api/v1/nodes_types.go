/*
Copyright 2025 BadAimWeeb.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodesSpec defines the desired state of Nodes
type NodesSpec struct {
	// Type of node referenced: "k8s" for nodes in the same K8s cluster, "external" for nodes outside the cluster.
	// If node is "external", it must run a companion agent to receive Peering configs and apply them (WIP).
	Type string `json:"type"`

	// Kubernetes node name. Required if type is "k8s".
	// +optional
	NodeName *string `json:"nodeName,omitempty"`

	// External address. Required if type is "external".
	// +optional
	ExternalAddress *string `json:"externalAddress,omitempty"`

	// Local IPv4 address for this node to be used for peerings.
	// Optional if this node only routes IPv6, or we're using Extended Next-Hop.
	// +optional
	IPv4Address *string `json:"ipv4Address,omitempty"`

	// BIRD peering configuration directory. The operator will drop peering configs here.
	// Default to /etc/bird/peers if not set.
	// +optional
	PeerConfigDir *string `json:"peerConfigDir,omitempty"`

	// WireGuard configuration directory. The operator will drop WireGuard configs here if needed for peerings.
	// Internally, wg-quick will be used to manage WireGuard interfaces.
	// Default to /etc/wireguard if not set.
	// +optional
	WGConfigDir *string `json:"wgConfigDir,omitempty"`
}

// NodesStatus defines the observed state of Nodes.
type NodesStatus struct {
	// conditions represent the current state of the Nodes resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Nodes is the Schema for the nodes API
type Nodes struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Nodes
	// +required
	Spec NodesSpec `json:"spec"`

	// status defines the observed state of Nodes
	// +optional
	Status NodesStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// NodesList contains a list of Nodes
type NodesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Nodes `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Nodes{}, &NodesList{})
}
