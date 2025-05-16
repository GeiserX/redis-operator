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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"crypto/rand"
	"encoding/base64"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "github.com/GeiserX/redis-operator/api/v1alpha1"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=cache.geiser.cloud,resources=redis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.geiser.cloud,resources=redis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.geiser.cloud,resources=redis/finalizers,verbs=update
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

	// Secret password
	secretName := redis.Name + "-secret"
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: redis.Namespace}, secret)
	if err != nil && errors.IsNotFound(err) {
		// Secret does not exist. Let's create it.
		password, err := generateRandomPassword(20)
		if err != nil {
			log.Error(err, "Failed to generate random password")
			return ctrl.Result{}, err
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
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, secret); err != nil {
			log.Error(err, "Failed to create Redis Secret", "Secret.Name", secretName)
			return ctrl.Result{}, err
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
			log.Error(err, "Failed to create Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// Handle scaling updates clearly
	desiredReplicas := redis.Spec.Replicas
	if *deployment.Spec.Replicas != desiredReplicas {
		log.Info("Updating deployment replicas", "Deployment.Name", deployment.Name, "from", *deployment.Spec.Replicas, "to", desiredReplicas)
		deployment.Spec.Replicas = &desiredReplicas // Setting desired replicas
		if err := r.Update(ctx, deployment); err != nil {
			log.Error(err, "Failed to update Deployment replicas")
			return ctrl.Result{}, err
		}
		log.Info("Updated Deployment replicas successfully", "Deployment.Name", deploymentName, "Replicas", desiredReplicas)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
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
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}
	// Base64 encode the random bytes to ensure password is printable
	return base64.StdEncoding.EncodeToString(randomBytes)[:length], nil
}

// deploymentForRedis returns a Redis Deployment object
func (r *RedisReconciler) deploymentForRedis(redis *cachev1alpha1.Redis, deploymentName string) *appsv1.Deployment {
	labels := map[string]string{
		"app": redis.Name,
	}
	replicas := redis.Spec.Replicas
	secretName := redis.Name + "-secret" // clearly match the exact secret name we created above

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
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: redis.Spec.Image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 6379,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "REDIS_PASSWORD",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secretName, // Using the defined secret
											},
											Key: "password", // As we defined earlier
										},
									},
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
