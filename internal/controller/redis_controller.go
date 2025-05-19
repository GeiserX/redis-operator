/*
Copyright 2025.

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
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/GeiserX/redis-operator/api/v1alpha1"
)

const (
	redisSecretVolume     = "redis-secret"
	redisSecretMountPath  = "/etc/redis-secret" // mount path inside the Pod
	CondPasswordGenerated = "PasswordGenerated"
	CondDeploymentReady   = "DeploymentReady"
	CondReady             = "Ready"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cache.geiser.cloud,resources=redis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.geiser.cloud,resources=redis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.geiser.cloud,resources=redis/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *RedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Redis instance (the CR)
	redis := &cachev1alpha1.Redis{}
	if err := r.Get(ctx, req.NamespacedName, redis); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Redis resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Redis")
		return ctrl.Result{}, err
	}
	original := redis.DeepCopy()
	redis.SetDefaults()
	if err := r.Patch(ctx, redis, client.MergeFrom(original)); err != nil {
		log.Error(err, "Failed to explicitly persist default values back to Redis CR")
		return ctrl.Result{}, err
	}

	// Preparations for custom Status
	patch := client.MergeFrom(redis.DeepCopy()) // explicitly capture initial state to patch later
	redis.Status.Status = "Reconciling"
	redis.Status.Message = "Reconciling Redis in progress"
	redis.Status.Nodes = nil
	if err := r.Status().Patch(ctx, redis, patch); err != nil {
		log.Error(err, "Failed to explicitly update Redis status to Reconciling")
		return ctrl.Result{}, err
	}

	// Secret password
	secretName := redis.Name + "-secret"
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: redis.Namespace}, secret)
	if err != nil && errors.IsNotFound(err) {
		// Secret does not exist. Let's create it.
		rawBytes, err := generateRandomPassword()
		if err != nil {
			log.Error(err, "Failed to generate random password")
			setCondition(redis, CondPasswordGenerated,
				metav1.ConditionFalse, "SecretError", err.Error())

			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Password generation failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)

			setCondition(redis, CondDeploymentReady,
				metav1.ConditionFalse, "DeploymentError", err.Error())
			reconcileReady(redis)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly delayed retry
		}

		immutable := ptr.To(true)
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: redis.Namespace,
			},
			Type:      corev1.SecretTypeOpaque,
			Immutable: immutable,
			Data: map[string][]byte{
				"password": rawBytes,
			},
		}

		// set Redis CR as the owner
		if err := ctrl.SetControllerReference(redis, secret, r.Scheme); err != nil {
			log.Error(err, "Failed to set owner reference for secret")

			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Owner reference for secret failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)

			setCondition(redis, CondDeploymentReady,
				metav1.ConditionFalse, "DeploymentError", err.Error())
			reconcileReady(redis)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly delayed retry
		}

		if err := r.Create(ctx, secret); err != nil {
			log.Error(err, "Failed explicitly creating Redis Secret", "Secret.Name", secretName)

			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Secret creation failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)

			setCondition(redis, CondDeploymentReady,
				metav1.ConditionFalse, "DeploymentError", err.Error())
			reconcileReady(redis)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // Delayed retry
		}

		log.Info("Created new Redis Secret", "Secret.Name", secretName)
		setCondition(redis, CondPasswordGenerated,
			metav1.ConditionTrue, "SecretPresent", "Password secret is present")
		reconcileReady(redis)
		_ = r.Status().Patch(ctx, redis, client.MergeFrom(redis.DeepCopy()))
		// Requeue to immediately move to next reconcile step (deployment creation/update)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Redis Secret")
		return ctrl.Result{}, err
	} else {
		// Secret found – be sure we still own it
		if !isControlledBy(secret, redis) {
			setCondition(redis, CondPasswordGenerated,
				metav1.ConditionTrue, "SecretPresent", "Password secret is present")
			reconcileReady(redis)
			_ = r.Status().Patch(ctx, redis, client.MergeFrom(redis.DeepCopy()))
			log.Info("Secret exists but is not owned by this Redis CR – patching ownerReference",
				"Secret", secret.Name)
			if err := controllerutil.SetControllerReference(redis, secret, r.Scheme); err != nil {
				return ctrl.Result{}, fmt.Errorf("patch ownerRef on Secret: %w", err)
			}
			if err := r.Update(ctx, secret); err != nil {
				return ctrl.Result{}, fmt.Errorf("update Secret ownerRef: %w", err)
			}
		}
	}

	// Define desired Deployment name
	deploymentName := redis.Name + "-deployment"

	// Check if Deployment already exists
	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: redis.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		// No Deployment found, create one
		log.Info("Creating new Deployment", "Deployment.Namespace", redis.Namespace, "Deployment.Name", deploymentName)
		deployment = r.deploymentForRedis(ctx, redis, deploymentName)
		if err = r.Create(ctx, deployment); err != nil {
			log.Error(err, "Failed to create Deployment explicitly", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Deployment creation failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)
			setCondition(redis, CondDeploymentReady,
				metav1.ConditionFalse, "DeploymentError", err.Error())
			reconcileReady(redis)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly delayed retry
		}
		patch = client.MergeFrom(redis.DeepCopy())
		redis.Status.Status = "Ready"
		redis.Status.Message = "Deployment created successfully"
		setCondition(redis, CondDeploymentReady,
			metav1.ConditionFalse, "Creating", "Deployment just created")
		reconcileReady(redis)
		if err := r.Status().Patch(ctx, redis, patch); err != nil {
			log.Error(err, "Failed updating Redis status after creating Deployment")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly delayed retry
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		patch = client.MergeFrom(redis.DeepCopy())
		redis.Status.Status = "Error"
		redis.Status.Message = fmt.Sprintf("Failed to get deployment state: %v", err)
		_ = r.Status().Patch(ctx, redis, patch)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Handle scaling updates clearly
	desiredReplicas := redis.Spec.Replicas
	if *deployment.Spec.Replicas != desiredReplicas {
		log.Info("Updating deployment replicas", "Deployment.Name", deployment.Name, "from", *deployment.Spec.Replicas, "to", desiredReplicas)
		deployment.Spec.Replicas = &desiredReplicas // Setting desired replicas
		if err := r.Update(ctx, deployment); err != nil {
			log.Error(err, "Failed explicitly updating Deployment replicas")
			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Scaling update failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)
			setCondition(redis, CondDeploymentReady,
				metav1.ConditionFalse, "DeploymentError", err.Error())
			reconcileReady(redis)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly retry after delay
		}
		log.Info("Updated Deployment replicas successfully", "Deployment.Name", deploymentName, "Replicas", desiredReplicas)
		setCondition(redis, CondDeploymentReady,
			metav1.ConditionTrue, "AllReplicasReady", "Deployment ready")
		reconcileReady(redis)
		patch = client.MergeFrom(redis.DeepCopy())
		redis.Status.Status = "Ready"
		redis.Status.Message = fmt.Sprintf("Deployment updated to %d replicas", redis.Spec.Replicas)
		if err := r.Status().Patch(ctx, redis, patch); err != nil {
			log.Error(err, "Failed updating Redis status after scaling deployment")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check and reconcile resources requirements
	if needsResourceUpdate(*deployment, *redis) {
		log.Info("Resource requirements updated, reconciling deployment", "Deployment.Name", deploymentName)
		updatedDeployment := r.deploymentForRedis(ctx, redis, deploymentName)
		deployment.Spec.Template.Spec = updatedDeployment.Spec.Template.Spec
		if err := r.Update(ctx, deployment); err != nil {
			log.Error(err, "Failed explicitly updating deployment resources")
			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Resource requirements update failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)
			setCondition(redis, CondDeploymentReady,
				metav1.ConditionFalse, "DeploymentError", err.Error())
			reconcileReady(redis)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		log.Info("Deployment updated successfully with new resources. Pods restarting..")
		// after r.Update(ctx, deployment) once resource is updated successfully
		patch = client.MergeFrom(redis.DeepCopy())
		redis.Status.Status = "Ready"
		redis.Status.Message = "Deployment resources updated successfully"
		if err := r.Status().Patch(ctx, redis, patch); err != nil {
			log.Error(err, "Failed updating Redis status after resource update")
			return ctrl.Result{}, err
		}
		setCondition(redis, CondDeploymentReady,
			metav1.ConditionFalse, "DeploymentError", err.Error())
		reconcileReady(redis)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// List pods matching Redis CR labels
	podList := &corev1.PodList{}
	labels := &client.MatchingLabels{"app": req.Name}
	if err := r.List(ctx, podList, client.InNamespace(req.Namespace), labels); err != nil {
		log.Error(err, "Failed listing pods for Redis")
		return ctrl.Result{}, err
	}

	unhealthyPods := []corev1.Pod{}
	for _, pod := range podList.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				unhealthyPods = append(unhealthyPods, pod)
				break
			}
		}
	}

	// Take action depending on unhealthy pods
	if len(unhealthyPods) > 0 {
		for _, pod := range unhealthyPods {
			restartCount := pod.Status.ContainerStatuses[0].RestartCount
			podAge := time.Since(pod.ObjectMeta.CreationTimestamp.Time)

			// If pod restart count exceeds 3 within short period, escalate & alert rather than trying again
			if restartCount >= 3 && podAge < 5*time.Minute {
				// Emit Kubernetes event, and set Redis.Status “Error”
				log.Info("Pod is repeatedly unhealthy! Alerting!", "pod", pod.Name)
				r.emitRedisEvent(ctx, req, fmt.Sprintf("Pod %s repeatedly unhealthy. Manual intervention required.", pod.Name), corev1.EventTypeWarning)
				r.updateRedisCRStatus(ctx, req, "Degraded", fmt.Sprintf("Pod %s repeatedly unhealthy. Manual intervention required.", pod.Name))
			} else {
				// Try to recover automatically by deleting the unhealthy pod (deployment recreates pod)
				log.Info("Deleting unhealthy pod for automatic recovery", "pod", pod.Name)
				if err := r.Delete(ctx, &pod); err != nil {
					log.Error(err, "Failed deleting unhealthy pod", "pod", pod.Name)
					continue
				}
				r.emitRedisEvent(ctx, req, fmt.Sprintf("Deleted unhealthy Pod %s for automated recovery", pod.Name), corev1.EventTypeNormal)
			}
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	desired := redis.Spec.Replicas
	isReady := deployment.Status.ReadyReplicas == desired
	if isReady {
		setCondition(redis, CondDeploymentReady,
			metav1.ConditionTrue, "AllReplicasReady", "Deployment ready")
	} else {
		setCondition(redis, CondDeploymentReady,
			metav1.ConditionFalse, "Updating", "Waiting for replicas")
	}
	reconcileReady(redis)
	_ = r.Status().Patch(ctx, redis, client.MergeFrom(redis.DeepCopy()))
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Redis{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}

// generateRandomPassword returns a slice with 32 random bytes
// (256-bit entropy).  The caller decides how to encode it.
func generateRandomPassword() ([]byte, error) {
	const bytesLen = 32 // 32*8 = 256 bits
	raw := make([]byte, bytesLen)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// deploymentForRedis returns a Redis Deployment object
func (r *RedisReconciler) deploymentForRedis(ctx context.Context, redis *cachev1alpha1.Redis, deploymentName string) *appsv1.Deployment {
	labels := map[string]string{
		"app": redis.Name,
	}
	replicas := redis.Spec.Replicas
	secretName := redis.Name + "-secret"

	// Container definition with resources
	container := corev1.Container{
		Name:  "redis",
		Image: redis.Spec.Image,
		Ports: []corev1.ContainerPort{
			{ContainerPort: 6379},
		},
		Env: []corev1.EnvVar{
			// plain env-var for backward compatibility
			{
				Name: "REDIS_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "password",
					},
				},
			},
			// *_FILE convention that bitnami/redis also supports
			{
				Name:  "REDIS_PASSWORD_FILE",
				Value: filepath.Join(redisSecretMountPath, "password"),
			},
		},

		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      redisSecretVolume,
				MountPath: redisSecretMountPath,
				ReadOnly:  true,
			},
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: redis.Spec.Probes.Readiness.Command,
				},
			},
			InitialDelaySeconds: redis.Spec.Probes.Readiness.InitialDelaySeconds,
			PeriodSeconds:       redis.Spec.Probes.Readiness.PeriodSeconds,
			TimeoutSeconds:      redis.Spec.Probes.Readiness.TimeoutSeconds,
			FailureThreshold:    redis.Spec.Probes.Readiness.FailureThreshold,
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: redis.Spec.Probes.Liveness.Command,
				},
			},
			InitialDelaySeconds: redis.Spec.Probes.Liveness.InitialDelaySeconds,
			PeriodSeconds:       redis.Spec.Probes.Liveness.PeriodSeconds,
			TimeoutSeconds:      redis.Spec.Probes.Liveness.TimeoutSeconds,
			FailureThreshold:    redis.Spec.Probes.Liveness.FailureThreshold,
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				"cpu":    resource.MustParse(redis.Spec.Resources.Requests.CPU),
				"memory": resource.MustParse(redis.Spec.Resources.Requests.Memory),
			},
			Limits: corev1.ResourceList{
				"cpu":    resource.MustParse(redis.Spec.Resources.Limits.CPU),
				"memory": resource.MustParse(redis.Spec.Resources.Limits.Memory),
			},
		},
	}

	// get current secret hash for rollout purposes
	secretHashStr := "unknown"
	sec := &corev1.Secret{}
	if err := r.Client.Get(
		ctx,
		types.NamespacedName{Name: secretName, Namespace: redis.Namespace},
		sec,
	); err == nil {
		if pw, ok := sec.Data["password"]; ok {
			hash := sha256.Sum256(pw)
			secretHashStr = hex.EncodeToString(hash[:])
		}
	}

	// Deployment creation
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: redis.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"redis.cache.geiser.cloud/secret-hash": secretHashStr,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{container},
					Volumes: []corev1.Volume{
						{
							Name: redisSecretVolume,
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretName,
								},
							},
						},
					},
				},
			},
		},
	}

	ctrl.SetControllerReference(redis, deployment, r.Scheme)
	return deployment
}

func needsResourceUpdate(existing appsv1.Deployment, redis cachev1alpha1.Redis) bool {
	container := existing.Spec.Template.Spec.Containers[0]
	reqCpu := resource.MustParse(redis.Spec.Resources.Requests.CPU)
	reqMem := resource.MustParse(redis.Spec.Resources.Requests.Memory)
	limCpu := resource.MustParse(redis.Spec.Resources.Limits.CPU)
	limMem := resource.MustParse(redis.Spec.Resources.Limits.Memory)

	return container.Resources.Requests["cpu"] != reqCpu ||
		container.Resources.Requests["memory"] != reqMem ||
		container.Resources.Limits["cpu"] != limCpu ||
		container.Resources.Limits["memory"] != limMem
}

// emitRedisEvent is utility for emitting Kubernetes events clearly
func (r *RedisReconciler) emitRedisEvent(ctx context.Context, req ctrl.Request, message, eventType string) {
	redisInstance := &cachev1alpha1.Redis{}
	if err := r.Get(ctx, req.NamespacedName, redisInstance); err == nil {
		r.EventRecorder.Event(redisInstance, eventType, "HealthCheck", message)
	}
}

// updateRedisCRStatus for updating custom Redis CR status clearly
func (r *RedisReconciler) updateRedisCRStatus(ctx context.Context, req ctrl.Request, status, message string) {
	redisInstance := &cachev1alpha1.Redis{}
	if err := r.Get(ctx, req.NamespacedName, redisInstance); err == nil {
		patch := client.MergeFrom(redisInstance.DeepCopy())
		redisInstance.Status.Status = status
		redisInstance.Status.Message = message
		if err := r.Status().Patch(ctx, redisInstance, patch); err != nil {
			log.FromContext(ctx).Error(err, "Failed updating Redis Status")
		}
	}
}

// isControlledBy returns true when `child` has an OwnerReference whose UID matches `parent` *and* the OwnerReference is marked as
// controller=true.  It is functionally identical to controllerutil.IsControlledBy, but implemented inline so you do not
// have to import the whole controller-runtime helper package.
func isControlledBy(child metav1.Object, parent metav1.Object) bool {
	for _, ref := range child.GetOwnerReferences() {
		// We care only about references that designate the controller.
		if ref.Controller == nil || !*ref.Controller {
			continue
		}

		// A match is on UID; name/kind are not necessary.
		if ref.UID == parent.GetUID() {
			return true
		}
	}
	return false
}

// setCondition adds/replaces a condition on the CR instance
func setCondition(r *cachev1alpha1.Redis, condType string,
	status metav1.ConditionStatus, reason, msg string) {

	c := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		LastTransitionTime: metav1.Now(),
	}
	meta.SetStatusCondition(&r.Status.Conditions, c)
}

// recompute Ready = PasswordGenerated && DeploymentReady
func reconcileReady(cr *cachev1alpha1.Redis) {
	pg := meta.FindStatusCondition(cr.Status.Conditions, CondPasswordGenerated)
	dep := meta.FindStatusCondition(cr.Status.Conditions, CondDeploymentReady)

	readyStatus := metav1.ConditionFalse
	reason, msg := "Waiting", "Waiting for password and deployment"
	if pg != nil && pg.Status == metav1.ConditionTrue &&
		dep != nil && dep.Status == metav1.ConditionTrue {
		readyStatus, reason, msg = metav1.ConditionTrue, "AllGood", "Redis is ready"
	}
	setCondition(cr, CondReady, readyStatus, reason, msg)
}
