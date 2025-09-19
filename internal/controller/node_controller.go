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

package controller

import (
	"context"
	"net"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	autopeerv1 "github.com/BadAimWeeb/autopeer-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodesReconciler reconciles a Nodes object
type NodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=autopeer.badaimweeb.me,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autopeer.badaimweeb.me,resources=nodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autopeer.badaimweeb.me,resources=nodes/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	nodeData := &autopeerv1.Node{}
	err := r.Get(ctx, req.NamespacedName, nodeData)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Validate data in nodes object and set conditions accordingly

	// Check if local IPv4 for this node is set. If yes, verify it's a valid IPv4 address.
	if nodeData.Spec.IPv4Address != nil {
		// Validate the IPv4 address format
		netIP := net.ParseIP(*nodeData.Spec.IPv4Address)
		if netIP == nil || netIP.To4() == nil {
			// Set Degraded condition to True with appropriate message
			nodeData.Status.Conditions = []metav1.Condition{
				{
					Type:    "Degraded",
					Status:  metav1.ConditionTrue,
					Reason:  "InvalidIPv4Address",
					Message: "Spec.ipv4Address is not a valid IPv4 address.",
				},
			}
			r.Update(ctx, nodeData)
			return ctrl.Result{}, nil
		}
	}

	// If node is of type "external", always return failure (for now).
	// If node is of type "k8s", check if node with given name exists in cluster.
	if nodeData.Spec.Type == "k8s" {
		if nodeData.Spec.NodeName == nil {
			// Set Degraded condition to True with appropriate message
			nodeData.Status.Conditions = []metav1.Condition{
				{
					Type:    "Degraded",
					Status:  metav1.ConditionTrue,
					Reason:  "MissingNodeName",
					Message: "Spec.nodeName is required when Spec.type is 'k8s'.",
				},
			}
			r.Update(ctx, nodeData)
			return ctrl.Result{}, nil
		}

		// Check if node with given name exists in cluster
		k8sNode := &corev1.Node{}
		err = r.Get(ctx, client.ObjectKey{Name: *nodeData.Spec.NodeName}, k8sNode)
		if err != nil {
			// Set Degraded condition to True with appropriate message
			nodeData.Status.Conditions = []metav1.Condition{
				{
					Type:    "Degraded",
					Status:  metav1.ConditionTrue,
					Reason:  "NodeNotFound",
					Message: "Kubernetes node with name '" + *nodeData.Spec.NodeName + "' not found.",
				},
			}
			r.Update(ctx, nodeData)
			return ctrl.Result{}, nil
		}

		// If we reach here, node exists
		// Set Available condition to True and clear other conditions
		nodeData.Status.Conditions = []metav1.Condition{
			{
				Type:    "Available",
				Status:  metav1.ConditionTrue,
				Reason:  "NodeExists",
				Message: "Kubernetes node with name '" + *nodeData.Spec.NodeName + "' exists.",
			},
		}
	} else {
		// External nodes not supported yet
		// Set Degraded condition to True with appropriate message
		nodeData.Status.Conditions = []metav1.Condition{
			{
				Type:    "Degraded",
				Status:  metav1.ConditionTrue,
				Reason:  "ExternalNodesNotSupported",
				Message: "Nodes of type 'external' are not supported yet.",
			},
		}
	}
	r.Update(ctx, nodeData)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&autopeerv1.Node{}).
		Named("node").
		Complete(r)
}
