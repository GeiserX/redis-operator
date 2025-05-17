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
	"fmt"
	"math/big"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/GeiserX/redis-operator/api/v1alpha1"
)

const passwordCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+"
const charsetLength = len(passwordCharset)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Scheme *runtime.Scheme
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
		password, err := generateRandomPassword(20)
		if err != nil {
			log.Error(err, "Failed to generate random password")

			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Password generation failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)

			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly delayed retry
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: redis.Namespace,
			},
			StringData: map[string]string{
				"password": password,
			},
		}

		// set Redis CR as the owner
		if err := ctrl.SetControllerReference(redis, secret, r.Scheme); err != nil {
			log.Error(err, "Failed to set owner reference for secret")

			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Owner reference for secret failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)

			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly delayed retry
		}

		if err := r.Create(ctx, secret); err != nil {
			log.Error(err, "Failed explicitly creating Redis Secret", "Secret.Name", secretName)

			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Secret creation failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)

			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // Delayed retry
		}

		log.Info("Created new Redis Secret", "Secret.Name", secretName)
		// Requeue to immediately move to next reconcile step (deployment creation/update)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Redis Secret")
		return ctrl.Result{}, err
	}

	// Define desired Deployment name
	deploymentName := redis.Name + "-deployment"

	// Check if Deployment already exists
	deployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: redis.Namespace}, deployment)
	if err != nil && errors.IsNotFound(err) {
		// No Deployment found, create one
		log.Info("Creating new Deployment", "Deployment.Namespace", redis.Namespace, "Deployment.Name", deploymentName)
		deployment = r.deploymentForRedis(redis, deploymentName)
		if err = r.Create(ctx, deployment); err != nil {
			log.Error(err, "Failed to create Deployment explicitly", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Deployment creation failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly delayed retry
		}
		patch = client.MergeFrom(redis.DeepCopy())
		redis.Status.Status = "Ready"
		redis.Status.Message = "Deployment created successfully"
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
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly retry after delay
		}
		log.Info("Updated Deployment replicas successfully", "Deployment.Name", deploymentName, "Replicas", desiredReplicas)
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
		updatedDeployment := r.deploymentForRedis(redis, deploymentName)
		deployment.Spec.Template.Spec = updatedDeployment.Spec.Template.Spec
		if err := r.Update(ctx, deployment); err != nil {
			log.Error(err, "Failed explicitly updating deployment resources")
			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Resource requirements update failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)
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
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// Check and reconcile persistence PVC
	if redis.Spec.Persistence.Enabled {
		if err := r.reconcilePersistence(ctx, redis); err != nil {
			log.Error(err, "Failed reconciling persistence")
			patch = client.MergeFrom(redis.DeepCopy())
			redis.Status.Status = "Error"
			redis.Status.Message = fmt.Sprintf("Persistent storage reconciliation failed: %v", err)
			_ = r.Status().Patch(ctx, redis, patch)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil // explicitly delay retry
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Redis{}).
		Complete(r)
}

// generateRandomPassword creates a secure random string of specified length
func generateRandomPassword(length int) (string, error) {
	password := make([]byte, length)
	for i := range password {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(charsetLength)))
		if err != nil {
			return "", err
		}
		password[i] = passwordCharset[num.Int64()]
	}
	return string(password), nil
}

// deploymentForRedis returns a Redis Deployment object
func (r *RedisReconciler) deploymentForRedis(redis *cachev1alpha1.Redis, deploymentName string) *appsv1.Deployment {
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
			{
				Name: "REDIS_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
						Key:                  "password",
					},
				},
			},
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

	// Handle persistence
	podVolumes := []corev1.Volume{}
	if redis.Spec.Persistence.Enabled {
		podVolumes = append(podVolumes, corev1.Volume{
			Name: "redis-data",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: redis.Name + "-pvc",
				},
			},
		})
		container.VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "redis-data",
				MountPath: "/data",
			},
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
				},
				Spec: corev1.PodSpec{
					Volumes:    podVolumes,
					Containers: []corev1.Container{container},
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

func (r *RedisReconciler) reconcilePersistence(ctx context.Context, redis *cachev1alpha1.Redis) error {
	if !redis.Spec.Persistence.Enabled {
		return nil
	}

	pvcName := redis.Name + "-pvc"
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: redis.Namespace}, pvc)
	if err != nil && errors.IsNotFound(err) {
		// No existing PVC, create it
		storageQuantity, err := resource.ParseQuantity(redis.Spec.Persistence.Size)
		if err != nil {
			return err
		}
		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: redis.Namespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &redis.Spec.Persistence.StorageClass,
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: storageQuantity,
					},
				},
			},
		}
		ctrl.SetControllerReference(redis, pvc, r.Scheme)
		return r.Create(ctx, pvc)
	} else if err != nil {
		return err
	}
	// Note: resizing PVC downward is unsafe—here, conservatively do nothing if requested size smaller.
	return nil
}
