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

// PeeringSpec defines the desired state of Peering
type PeeringSpec struct {
	// Target node to peer with
	Node string `json:"targetNode"`

	// Peer ASN
	PeerASN uint32 `json:"peerASN"`

	// Peer IPv4 address. Optional if this peer only routes IPv6, or we're using Extended Next-Hop.
	// +optional
	PeerIPv4 *string `json:"peerIPv4,omitempty"`

	// Peer IPv6 address, can be Link-local or Global/ULA. Optional if this peer only routes IPv4.
	// +optional
	PeerIPv6 *string `json:"peerIPv6,omitempty"`

	// Local IPv6 address, must be Link-local if PeerIPv6 is Link-local. Optional if this peer only routes IPv4.
	// +optional
	LocalIPv6 *string `json:"localIPv6,omitempty"`

	// Indicates whether to use Multiprotocol BGP (MP-BGP) to exchange IPv6 routes.
	// If false, there will be multiple BGP sessions for IPv4 and IPv6 (if configured).
	// If true, there will be only one BGP session over IPv6 for both IPv4 and IPv6 routes.
	// +optional
	MPBGP bool `json:"mpbgp,omitempty"`

	// Extended Next-Hop (ENH) allows routing IPv4 traffic over an IPv6-only link.
	// +optional
	ENH bool `json:"enh,omitempty"`

	// Cost to reach this peer, lower values are preferred.
	// Recommended to be consistent with your IGP cost to be able to fine-tune path selection.
	// Leave blank to not use cost.
	// +kubebuilder:default=0
	// +optional
	Cost int32 `json:"cost,omitempty"`

	// Whether to enable this peering. Disabling a peering will shut it down immediately.
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// What tunnel mechanism to use to reach this peer. Default to "wireguard" if not specified.
	// +kubebuilder:default="wireguard"
	// +optional
	TunnelType string `json:"tunnelType,omitempty"`

	// WireGuard configuration to be used if TunnelType is "wireguard".
	// +optional
	WireGuard *WireGuardConfig `json:"wireguard,omitempty"`
}

type WireGuardConfig struct {
	// Local port for the WireGuard interface. Leave blank to use a random port (you'll only make outbound connections).
	// +kubebuilder:default=0
	// +optional
	LocalPort int32 `json:"localPort,omitempty"`

	// Peer endpoint for the WireGuard interface. Empty if you want to let remote peer initiate the connection.
	// +optional
	Endpoint *string `json:"endpoint,omitempty"`

	// Local WireGuard private key. If not provided, a new keypair will be generated.
	// +optional
	PrivateKey *string `json:"privateKey,omitempty"`

	// Peer WireGuard public key.
	// +required
	PeerPublicKey string `json:"peerPublicKey"`

	// Pre-shared key for the WireGuard interface. Optional but recommended for additional security.
	// +optional
	PresharedKey *string `json:"presharedKey,omitempty"`

	// MTU for the WireGuard interface. Leave blank to use system default (usually 1420 or 1422).
	// +kubebuilder:default=0
	// +optional
	MTU int32 `json:"mtu,omitempty"`
}

// PeeringStatus defines the observed state of Peering.
type PeeringStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the Peering resource.
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

	// Is this peering currently provisioned on the node?
	// +default:value=false
	Provisioned bool `json:"provisioned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Peering is the Schema for the peerings API
type Peering struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of Peering
	// +required
	Spec PeeringSpec `json:"spec"`

	// status defines the observed state of Peering
	// +optional
	Status PeeringStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// PeeringList contains a list of Peering
type PeeringList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Peering `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Peering{}, &PeeringList{})
}
