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

	"k8s.io/apimachinery/pkg/api/meta"
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
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Available")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Progressing")
		meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
			Type:    "Degraded",
			Status:  metav1.ConditionTrue,
			Reason:  "NodeNotFound",
			Message: "The specified Node resource was not found.",
		})

		err = r.Status().Update(ctx, peering)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, err
	}

	if node.Spec.Type != "k8s" {
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Available")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Progressing")
		meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
			Type:    "Degraded",
			Status:  metav1.ConditionTrue,
			Reason:  "InvalidNodeType",
			Message: "The specified Node resource has an invalid type. Only 'k8s' type is supported.",
		})

		err = r.Status().Update(ctx, peering)

		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	}

	nodeAvailable := false
	for _, cond := range node.Status.Conditions {
		if cond.Type == "Available" && cond.Status == metav1.ConditionTrue {
			nodeAvailable = true
			break
		}
	}

	if !nodeAvailable {
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Available")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Progressing")
		meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
			Type:    "Degraded",
			Status:  metav1.ConditionTrue,
			Reason:  "NodeNotReady",
			Message: "The specified Node resource is not in an Available state.",
		})

		err = r.Status().Update(ctx, peering)

		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, nil
	}

	// If the Peering resource is marked for deletion, handle cleanup
	if !peering.ObjectMeta.DeletionTimestamp.IsZero() || !peering.Spec.Enabled {
		// Deletion logic: create a Job to clean up the peering
		return r.KubernetesDeletePeering(ctx, req, peering, node)
	} else {
		// Add a finalizer if it doesn't exist
		if !controllerutil.ContainsFinalizer(peering, finalizer) {
			controllerutil.AddFinalizer(peering, finalizer)
			if err := r.Update(ctx, peering); err != nil {
				return ctrl.Result{}, err
			}
		}

		// Re-read the resource to ensure we have the latest version
		err = r.Get(ctx, req.NamespacedName, peering)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return r.KubernetesAddOrUpdatePeering(ctx, req, peering, node)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PeeringReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&autopeerv1.Peering{}).
		Named("peering").
		Complete(r)
}

func (r *PeeringReconciler) KubernetesDeletePeering(ctx context.Context, req ctrl.Request, peering *autopeerv1.Peering, node *autopeerv1.Node) (ctrl.Result, error) {
	logf := logf.FromContext(ctx)

	// Get old add Job, if exists, delete it
	jobName := peering.Name + "-apply-peering"
	job := batchv1.Job{}

	err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: peering.Namespace}, &job)
	if err == nil {
		propagationPolicy := metav1.DeletePropagationForeground

		// Job exists, delete it
		if delErr := r.Delete(ctx, &job, &client.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		}); delErr != nil {
			logf.Error(delErr, "Failed to delete apply-peering Job during cleanup")
			return ctrl.Result{}, delErr
		}
	}

	// Remove the Secret associated with this peering
	secretName := peering.Name + "-peering-config"
	secret := corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Name: secretName, Namespace: peering.Namespace}, &secret); err == nil {
		if delErr := r.Delete(ctx, &secret); delErr != nil {
			logf.Error(delErr, "Failed to delete peering Secret during cleanup")
			return ctrl.Result{}, delErr
		}
	}

	// Create a cleanup Job to remove the peering from the node
	jobName = peering.Name + "-cleanup-peering"
	cleanupJob := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: peering.Namespace,
			Labels: map[string]string{
				"autopeer.badaimweeb.me/job": "true",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": jobName,
					},
				},
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
							Name:  "cleanup-peering",
							Image: node.Spec.JobWorkerImage,
							SecurityContext: &corev1.SecurityContext{
								Privileged: func() *bool { b := true; return &b }(),
							},
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
									Value: node.Spec.PeerConfigDir,
								},
								{
									Name:  "WIREGUARD_DIR",
									Value: node.Spec.WGConfigDir,
								},
								{
									Name:  "ASN",
									Value: fmt.Sprintf("%d", peering.Spec.PeerASN),
								},
								{
									Name:  "TASK",
									Value: "rmpeer",
								},
							},
						},
					},
				},
			},
		},
	}
	if err := controllerutil.SetControllerReference(peering, &cleanupJob, r.Scheme); err != nil {
		logf.Error(err, "Failed to set controller reference on cleanup Job")
		return ctrl.Result{}, err
	}
	// Create the cleanup Job if it doesn't exist
	err = client.IgnoreAlreadyExists(r.Create(ctx, &cleanupJob))
	if err != nil {
		logf.Error(err, "Failed to create cleanup Job")
		return ctrl.Result{}, err
	}

	// Wait for the cleanup Job to complete
	err = r.Get(ctx, client.ObjectKey{Name: cleanupJob.Name, Namespace: cleanupJob.Namespace}, &cleanupJob)
	if err != nil {
		logf.Error(err, "Failed to get cleanup Job")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if cleanupJob.Status.Succeeded == 0 && cleanupJob.Status.Failed == 0 {
		// Job is still running
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if cleanupJob.Status.Failed > 0 {
		logf.Error(fmt.Errorf("cleanup job failed"), "Cleanup Job failed")
		return ctrl.Result{}, fmt.Errorf("cleanup job failed")
	}

	// Remove the cleanup Job after it succeeded
	propagationPolicy := metav1.DeletePropagationForeground
	if delErr := r.Delete(ctx, &cleanupJob, &client.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}); delErr != nil {
		logf.Error(delErr, "Failed to delete cleanup Job after completion")
		return ctrl.Result{}, delErr
	}

	// Remove finalizer if object is being deleted
	finalizer := "autopeer.badaimweeb.me/finalizer"
	if !peering.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(peering, finalizer) {
			controllerutil.RemoveFinalizer(peering, finalizer)
			if err := r.Update(ctx, peering); err != nil {
				logf.Error(err, "Failed to remove finalizer from Peering")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *PeeringReconciler) KubernetesAddOrUpdatePeering(ctx context.Context, req ctrl.Request, peering *autopeerv1.Peering, node *autopeerv1.Node) (ctrl.Result, error) {
	logf := logf.FromContext(ctx)

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

	res, err := controllerutil.CreateOrUpdate(ctx, r.Client, &secret, func() error {
		secret.Data = secretData
		return controllerutil.SetControllerReference(peering, &secret, r.Scheme)
	})

	if err != nil {
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Available")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Progressing")
		meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
			Type:    "Degraded",
			Status:  metav1.ConditionTrue,
			Reason:  "SecretError",
			Message: "Failed to create or update the Secret for peering configuration: " + err.Error(),
		})

		err = r.Status().Update(ctx, peering)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, err
	}

	// check if peering has succeeed
	isSuccessBefore := meta.IsStatusConditionTrue(peering.Status.Conditions, "Available")

	switch res {
	case controllerutil.OperationResultNone:
		if isSuccessBefore {
			// do nothing
			return ctrl.Result{}, nil
		}
	case controllerutil.OperationResultUpdated:
		if isSuccessBefore {
			// delete job and its associated pods to rerun
			jobName := peering.Name + "-apply-peering"
			job := batchv1.Job{}
			err = client.IgnoreNotFound(r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: peering.Namespace}, &job))

			if err != nil {
				logf.Error(err, "Failed to update existing Job")
				meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
					Type:    "Degraded",
					Status:  metav1.ConditionTrue,
					Reason:  "JobUpdateFailed",
					Message: "Cannot delete old Job to update configuration",
				})
			}

			propagationPolicy := metav1.DeletePropagationForeground
			err = r.Delete(ctx, &job, &client.DeleteOptions{
				PropagationPolicy: &propagationPolicy,
			})

			if err != nil {
				logf.Error(err, "Failed to delete existing Job")
				meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
					Type:    "Degraded",
					Status:  metav1.ConditionTrue,
					Reason:  "JobDeleteFailed",
					Message: "Cannot delete old Job to update configuration",
				})
			}
		}
	}

	// Set Progressing condition
	meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
		Type:    "Progressing",
		Status:  metav1.ConditionTrue,
		Reason:  "ConfigUpdated",
		Message: "Peering configuration Secret created or updated successfully.",
	})
	meta.RemoveStatusCondition(&peering.Status.Conditions, "Degraded")

	// Create a job to apply the peering configuration on the node
	jobLabels := map[string]string{
		"app": peering.Name + "-apply-peering",
	}

	workerImage := node.Spec.JobWorkerImage
	peerConfigDir := node.Spec.PeerConfigDir
	wgConfigDir := node.Spec.WGConfigDir

	priv := true
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      peering.Name + "-apply-peering",
			Namespace: peering.Namespace,
			Labels: map[string]string{
				"autopeer.badaimweeb.me/job": "true",
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: jobLabels,
				},
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
							SecurityContext: &corev1.SecurityContext{
								Privileged: &priv,
							},
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
									Name: "WG_CONF",
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
		},
	}
	err = controllerutil.SetControllerReference(peering, &job, r.Scheme)

	if err != nil {
		logf.Error(err, "Failed to set controller reference on the Job for applying peering configuration")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Available")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Progressing")
		meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
			Type:    "Degraded",
			Status:  metav1.ConditionTrue,
			Reason:  "JobError",
			Message: "Failed to set controller reference on the Job for applying peering configuration: " + err.Error(),
		})

		err = r.Status().Update(ctx, peering)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, err
	}

	// Create the Job
	err = client.IgnoreAlreadyExists(r.Create(ctx, &job))

	if err != nil {
		logf.Error(err, "Failed to create the Job for applying peering configuration")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Available")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Progressing")
		meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
			Type:    "Degraded",
			Status:  metav1.ConditionTrue,
			Reason:  "JobError",
			Message: "Failed to create the Job for applying peering configuration: " + err.Error(),
		})

		err = r.Status().Update(ctx, peering)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, err
	}

	// Query for latest data before updating
	err = r.Get(ctx, req.NamespacedName, peering)
	if err != nil {
		logf.Error(err, "Failed to get the Peering resource before updating status")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	err = r.Get(ctx, client.ObjectKey{Name: job.Name, Namespace: job.Namespace}, &job)
	if err != nil {
		logf.Error(err, "Failed to get the Job for applying peering configuration")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Available")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Progressing")
		meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
			Type:    "Degraded",
			Status:  metav1.ConditionTrue,
			Reason:  "JobError",
			Message: "Failed to get the Job for applying peering configuration: " + err.Error(),
		})

		err = r.Status().Update(ctx, peering)
		if err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		return ctrl.Result{}, err
	}

	// Check Job status, then set Available condition if successful
	success := false
	if job.Status.Succeeded > 0 {
		meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
			Type:    "Available",
			Status:  metav1.ConditionTrue,
			Reason:  "PeeringApplied",
			Message: "Peering configuration applied successfully on the node.",
		})

		meta.RemoveStatusCondition(&peering.Status.Conditions, "Progressing")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Degraded")

		success = true
	}

	if job.Status.Failed > 0 {
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Available")
		meta.RemoveStatusCondition(&peering.Status.Conditions, "Progressing")
		meta.SetStatusCondition(&peering.Status.Conditions, metav1.Condition{
			Type:    "Degraded",
			Status:  metav1.ConditionTrue,
			Reason:  "JobFailed",
			Message: "The Job for applying peering configuration has failed.",
		})

		success = false
	}

	err = r.Status().Update(ctx, peering)
	if err != nil {
		logf.Error(err, "Failed to update Peering status")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !success {
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	return ctrl.Result{}, nil
}
