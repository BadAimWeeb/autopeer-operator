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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	autopeerv1 "github.com/BadAimWeeb/autopeer-operator/api/v1"
	"github.com/BadAimWeeb/autopeer-operator/internal/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PeeringReconciler reconciles a Peering object
type PeeringReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=autopeer.badaimweeb.me,resources=peerings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autopeer.badaimweeb.me,resources=peerings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=autopeer.badaimweeb.me,resources=peerings/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PeeringReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = logf.FromContext(ctx)

	peering := &autopeerv1.Peering{}
	err := r.Get(ctx, req.NamespacedName, peering)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	finalizer := "autopeer.badaimweeb.me/finalizer"

	node := &autopeerv1.Node{}
	err = r.Get(ctx, client.ObjectKey{Name: peering.Spec.Node, Namespace: peering.Namespace}, node)
	if err != nil {
		peering.Status.Conditions = []metav1.Condition{
			{
				Type:    "Degraded",
				Status:  metav1.ConditionTrue,
				Reason:  "NodeNotFound",
				Message: "The specified Node resource was not found.",
			},
		}
		err = r.Status().Update(ctx, peering)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, err
	}

	if node.Spec.Type != "k8s" {
		peering.Status.Conditions = []metav1.Condition{
			{
				Type:    "Degraded",
				Status:  metav1.ConditionTrue,
				Reason:  "InvalidNodeType",
				Message: "The specified Node resource has an invalid type. Only 'k8s' type is supported.",
			},
		}
		err = r.Status().Update(ctx, peering)

		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	}

	nodeAvailable := false
	for _, cond := range peering.Status.Conditions {
		if cond.Type == "Available" && cond.Status == metav1.ConditionTrue {
			nodeAvailable = true
			break
		}
	}

	if !nodeAvailable {
		peering.Status.Conditions = []metav1.Condition{
			{
				Type:    "Degraded",
				Status:  metav1.ConditionTrue,
				Reason:  "NodeNotReady",
				Message: "The specified Node resource is not in an Available state.",
			},
		}
		err = r.Status().Update(ctx, peering)

		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	}

	// If the Peering resource is marked for deletion, handle cleanup
	if !peering.ObjectMeta.DeletionTimestamp.IsZero() {
		// Deletion logic: create a Job to clean up the peering
		return ctrl.Result{}, nil
	} else {
		// Add a finalizer if it doesn't exist
		if !controllerutil.ContainsFinalizer(peering, finalizer) {
			controllerutil.AddFinalizer(peering, finalizer)
			if err := r.Update(ctx, peering); err != nil {
				return ctrl.Result{}, err
			}
		}

		return r.KubernetesAddOrUpdatePeering(ctx, peering, node)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PeeringReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&autopeerv1.Peering{}).
		Named("peering").
		Complete(r)
}

func (r *PeeringReconciler) KubernetesAddOrUpdatePeering(ctx context.Context, peering *autopeerv1.Peering, node *autopeerv1.Node) (ctrl.Result, error) {
	// Generate required data for peering
	config := utils.GenerateConfig(&peering.Spec, &node.Spec)

	// Create or update the Secret with the peering configuration
	secretName := peering.Name + "-peering-config"

	wgBytes := []byte{}
	if config.WireGuard != nil {
		wgBytes = []byte(*config.WireGuard)
	}

	secretData := map[string][]byte{
		"bird": []byte(config.BIRD),
		"wg":   wgBytes,
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: peering.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, &secret, func() error {
		secret.Data = secretData
		return controllerutil.SetControllerReference(peering, &secret, r.Scheme)
	})

	if err != nil {
		peering.Status.Conditions = []metav1.Condition{
			{
				Type:    "Degraded",
				Status:  metav1.ConditionTrue,
				Reason:  "SecretError",
				Message: "Failed to create or update the Secret for peering configuration: " + err.Error(),
			},
		}
		err = r.Status().Update(ctx, peering)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, err
	}

	// Set Progressing condition
	peering.Status.Conditions = []metav1.Condition{
		{
			Type:    "Progressing",
			Status:  metav1.ConditionTrue,
			Reason:  "ConfigUpdated",
			Message: "Peering configuration Secret created or updated successfully.",
		},
	}

	// Create a job to apply the peering configuration on the node
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      peering.Name + "-apply-peering",
			Namespace: peering.Namespace,
		},
	}

	workerImage := node.Spec.JobWorkerImage
	peerConfigDir := node.Spec.PeerConfigDir
	wgConfigDir := node.Spec.WGConfigDir

	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, &job, func() error {
		job.Spec = batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeName: *node.Spec.NodeName,
					HostPID:  true,
					Volumes: []corev1.Volume{{
						Name: "root-mount",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/",
							},
						},
					}},
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "apply-peering",
							Image: workerImage,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "root-mount",
									MountPath: "/mnt/node-root",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "NODE_ROOT",
									Value: "/mnt/node-root",
								},
								{
									Name:  "BIRD_PEERS_DIR",
									Value: peerConfigDir,
								},
								{
									Name:  "WIREGUARD_DIR",
									Value: wgConfigDir,
								},
								{
									Name: "BIRD_CONF",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "bird",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretName,
											},
										},
									},
								},
								{
									Name: "WIREGUARD_CONF",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											Key: "wg",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretName,
											},
										},
									},
								},
								{
									Name:  "ASN",
									Value: fmt.Sprintf("%d", peering.Spec.PeerASN),
								},
								{
									Name:  "TASK",
									Value: "peer",
								},
							},
						},
					},
				},
			},
		}
		return controllerutil.SetControllerReference(peering, &job, r.Scheme)
	})

	if err != nil {
		peering.Status.Conditions = []metav1.Condition{
			{
				Type:    "Degraded",
				Status:  metav1.ConditionTrue,
				Reason:  "JobError",
				Message: "Failed to create or update the Job for applying peering configuration: " + err.Error(),
			},
		}

		err = r.Status().Update(ctx, peering)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, err
	}

	// Check Job status, then set Available condition if successful
	success := false
	if job.Status.Succeeded > 0 {
		peering.Status.Conditions = []metav1.Condition{
			{
				Type:    "Available",
				Status:  metav1.ConditionTrue,
				Reason:  "PeeringApplied",
				Message: "Peering configuration applied successfully on the node.",
			},
		}

		success = true
	}

	if job.Status.Failed > 0 {
		peering.Status.Conditions = []metav1.Condition{
			{
				Type:    "Degraded",
				Status:  metav1.ConditionTrue,
				Reason:  "JobFailed",
				Message: "The Job for applying peering configuration has failed.",
			},
		}

		success = false
	}

	err = r.Status().Update(ctx, peering)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !success {
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	return ctrl.Result{}, nil
}
