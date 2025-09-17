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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	autopeerv1 "github.com/BadAimWeeb/autopeer-operator/api/v1"
)

// PeeringReconciler reconciles a Peering object
type PeeringReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=autopeer.badaimweeb.me,resources=peerings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autopeer.badaimweeb.me,resources=peerings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autopeer.badaimweeb.me,resources=peerings/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PeeringReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	peering := &autopeerv1.Peering{}
	err := r.Get(ctx, req.NamespacedName, peering)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// If the Peering resource is marked for deletion, handle cleanup
	if !peering.ObjectMeta.DeletionTimestamp.IsZero() {
		// Create a job to remove the peer from the target server

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PeeringReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&autopeerv1.Peering{}).
		Named("peering").
		Complete(r)
}
