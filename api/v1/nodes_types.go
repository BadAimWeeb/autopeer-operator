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

// NodeSpec defines the desired state of Node
type NodeSpec struct {
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
	// +default:value="/etc/bird/peers"
	PeerConfigDir string `json:"peerConfigDir,omitempty"`

	// WireGuard configuration directory. The operator will drop WireGuard configs here if needed for peerings.
	// Internally, wg-quick will be used to manage WireGuard interfaces.
	// Default to /etc/wireguard if not set.
	// +default:value="/etc/wireguard"
	WGConfigDir string `json:"wgConfigDir,omitempty"`

	// Image to be deployed to node to configure the relevant parts for peering, should you choose to differ
	// from the default image. Only applicable if type is "k8s".
	// +default:value="ghcr.io/badaimweeb/autopeer-operator/job-worker:20250919-122602-g8f1b850"
	JobWorkerImage string `json:"jobWorkerImage,omitempty"`
}

// NodeStatus defines the observed state of Node.
type NodeStatus struct {
	// conditions represent the current state of the Node resource.
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

// Node is the Schema for the node API
type Node struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Node
	// +required
	Spec NodeSpec `json:"spec"`

	// status defines the observed state of Node
	// +optional
	Status NodeStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// NodeList contains a list of Node
type NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Node `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Node{}, &NodeList{})
}
