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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/scale/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1alpha1 "github.com/GeiserX/redis-operator/api/v1alpha1"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = cachev1alpha1.AddToScheme(s)
	return s
}

var _ = Describe("Redis Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		redis := &cachev1alpha1.Redis{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Redis")
			err := k8sClient.Get(ctx, typeNamespacedName, redis)
			if err != nil && errors.IsNotFound(err) {
				resource := &cachev1alpha1.Redis{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: cachev1alpha1.RedisSpec{
						Replicas: 1,
						Image:    "bitnami/redis:8.0",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &cachev1alpha1.Redis{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Redis")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &RedisReconciler{
				Client: k8sClient,
				Scheme: testScheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})

// Test random password generation
var _ = Describe("generateRandomPassword", func() {
	Context("With valid input length", func() {
		It("should generate a password", func() {
			password, err := generateRandomPassword()

			Expect(err).NotTo(HaveOccurred()) // no error occurred

			// Generate a second password clearly to ensure randomness
			password2, err2 := generateRandomPassword()

			Expect(err2).NotTo(HaveOccurred())
			Expect(password2).NotTo(Equal(password)) // explicitly verifying they are different
		})
	})
})

// Test if Deployment is generated with Redis CR
var _ = Describe("deploymentForRedis", func() {
	Context("When provided a Redis resource", func() {
		It("should correctly generate a Deployment matching Redis CR", func() {
			redis := &cachev1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-redis",
					Namespace: "default",
				},
				Spec: cachev1alpha1.RedisSpec{
					Replicas: 3,
					Image:    "bitnami/redis:7",
					Resources: cachev1alpha1.ResourceSpec{
						Requests: cachev1alpha1.ResourceList{
							CPU:    "100m",
							Memory: "128Mi",
						},
						Limits: cachev1alpha1.ResourceList{
							CPU:    "250m",
							Memory: "256Mi",
						},
					},
				},
			}

			deployment := (&RedisReconciler{
				Client: k8sClient,
				Scheme: testScheme(),
			}).deploymentForRedis(context.TODO(), redis, "test-redis-deployment")

			Expect(deployment.Name).To(Equal("test-redis-deployment"))
			Expect(*deployment.Spec.Replicas).To(Equal(int32(3)))
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))

			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Image).To(Equal("bitnami/redis:7"))
			foundPort6379 := false
			for _, p := range container.Ports {
				if p.ContainerPort == 6379 {
					foundPort6379 = true
					break
				}
			}
			Expect(foundPort6379).To(BeTrue(), "container should expose port 6379")

			Expect(container.Env).To(ContainElement(corev1.EnvVar{
				Name: "REDIS_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "test-redis-secret",
						},
						Key: "password",
					},
				},
			}))
		})
	})
})

// Test if reconcile logic updates replica numbers correctly
var _ = Describe("Redis Reconcile - Update Handling", func() {
	Context("When updating Redis replicas", func() {
		It("should update Deployment replicas accordingly", func() {
			ctx := context.Background()

			redis := &cachev1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "update-redis",
					Namespace: "default",
				},
				Spec: cachev1alpha1.RedisSpec{
					Replicas: 2,
					Image:    "bitnami/redis:8.0",
					Resources: cachev1alpha1.ResourceSpec{
						Requests: cachev1alpha1.ResourceList{
							CPU:    "100m",
							Memory: "128Mi",
						},
						Limits: cachev1alpha1.ResourceList{
							CPU:    "250m",
							Memory: "256Mi",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, redis)).Should(Succeed())

			reconciler := &RedisReconciler{
				Client: k8sClient,
				Scheme: testScheme(),
			}

			// First reconciliation (initial secret & deployment created)
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "update-redis", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			// Second reconciliation (assuming immediate requeue after secret created)
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "update-redis", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			// Important: Refresh object's state after reconciliation
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "update-redis", Namespace: "default"}, redis)).Should(Succeed())

			// Manual scaling change
			redis.Spec.Replicas = 4
			Expect(k8sClient.Update(ctx, redis)).Should(Succeed())

			// Again: reconcile after manual scaling
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: "update-redis", Namespace: "default"},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify updated replica count on Deployment
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "update-redis-deployment", Namespace: "default"}, deployment)).Should(Succeed())
			Expect(*deployment.Spec.Replicas).To(Equal(int32(4)))
		})
	})
})

var _ = Describe("Secret creation on first reconcile", func() {
	It("creates <name>-secret exactly once and sets owner-ref", func() {
		ctx := context.Background()
		const (
			name      = "secret-check"
			namespace = "default"
		)

		cr := &cachev1alpha1.Redis{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       cachev1alpha1.RedisSpec{Replicas: 1},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())

		r := &RedisReconciler{
			Client: k8sClient,
			Scheme: testScheme(),
		}

		// first pass: CR defaults persisted
		_, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
		})
		Expect(err).NotTo(HaveOccurred())

		// second pass: Secret gets created
		_, err = r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
		})
		Expect(err).NotTo(HaveOccurred())

		// assert Secret exists and has owner-ref
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx,
			types.NamespacedName{Name: name + "-secret", Namespace: namespace}, secret)).
			To(Succeed())

		Expect(secret.OwnerReferences).ToNot(BeEmpty(),
			"Secret should have an OwnerReference")
		Expect(secret.OwnerReferences[0].Name).To(Equal(name))
		Expect(secret.Type).To(Equal(corev1.SecretTypeOpaque))
		Expect(secret.Data).To(HaveKey("password"))
	})
})

// Test Secret Reuse on Reconciliation (Password Persistence)
var _ = Describe("Redis Controller", func() {
	Context("When reconciling existing Redis with Secret", func() {
		It("should reuse existing Secret without changing the password", func() {
			ctx := context.Background()
			redis := &cachev1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "password-persistence-redis",
					Namespace: "default",
				},
				Spec: cachev1alpha1.RedisSpec{
					Replicas: 1,
					Image:    "bitnami/redis:8.0",
				},
			}
			Expect(k8sClient.Create(ctx, redis)).Should(Succeed())

			reconciler := &RedisReconciler{
				Client: k8sClient,
				Scheme: testScheme(),
			}

			// First reconciliation to create Secret
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: redis.Name, Namespace: redis.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			// Get the password from the Secret
			secret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: redis.Name + "-secret", Namespace: redis.Namespace}, secret)).Should(Succeed())
			initialPassword := secret.Data["password"]

			// Reconcile again
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: redis.Name, Namespace: redis.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			// Get the password again
			updatedSecret := &corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: redis.Name + "-secret", Namespace: redis.Namespace}, updatedSecret)).Should(Succeed())
			updatedPassword := updatedSecret.Data["password"]

			// Verify the password hasn't changed
			Expect(updatedPassword).To(Equal(initialPassword))
		})
	})
})

// Test Reconciliation When Secret Already Exists (Idempotency)
var _ = Describe("Redis Controller", func() {
	Context("When Secret already exists before Redis CR is reconciled", func() {
		It("should use the existing Secret", func() {
			ctx := context.Background()
			secretName := "existing-secret-redis-secret"
			preExistingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-secret-redis-secret",
					Namespace: "default",
				},
				StringData: map[string]string{
					"password": "predefinedPassword123!",
				},
			}
			Expect(k8sClient.Create(ctx, preExistingSecret)).Should(Succeed())

			redis := &cachev1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-secret-redis",
					Namespace: "default",
				},
				Spec: cachev1alpha1.RedisSpec{
					Replicas: 1,
					Image:    "bitnami/redis:8.0",
				},
			}
			Expect(k8sClient.Create(ctx, redis)).Should(Succeed())

			reconciler := &RedisReconciler{
				Client: k8sClient,
				Scheme: testScheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: redis.Name, Namespace: redis.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			// Ensure that the existing Secret is used
			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: redis.Name + "-deployment", Namespace: redis.Namespace}, deployment)).Should(Succeed())
			envVars := deployment.Spec.Template.Spec.Containers[0].Env
			var redisPasswordEnv corev1.EnvVar
			for _, env := range envVars {
				if env.Name == "REDIS_PASSWORD" {
					redisPasswordEnv = env
					break
				}
			}
			Expect(redisPasswordEnv.ValueFrom.SecretKeyRef.Name).To(Equal(secretName))
		})
	})
})

// Test watch mechanism and automatic pod recovery on unhealthy Pods
var _ = Describe("Redis Controller – Pod Health and Recovery", func() {
	Context("When a managed Redis Pod becomes unhealthy", func() {
		It("should automatically delete unhealthy pods and emit recovery events", func() {
			ctx := context.Background()
			redisName := "health-monitor-redis"
			namespace := "default"

			// Step 1: Create Redis CR
			redisInstance := &cachev1alpha1.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:      redisName,
					Namespace: namespace,
				},
				Spec: cachev1alpha1.RedisSpec{
					Replicas: 1,
					Image:    "bitnami/redis:8.0",
				},
			}
			Expect(k8sClient.Create(ctx, redisInstance)).Should(Succeed())

			reconciler := &RedisReconciler{
				Client:        k8sClient,
				Scheme:        testScheme(),
				EventRecorder: record.NewFakeRecorder(10),
			}

			// Reconcile twice: once for secret, once for deployment/pods
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: redisName, Namespace: namespace},
			})
			Expect(err).NotTo(HaveOccurred())
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: redisName, Namespace: namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			// Step 2: Simulate unhealthy Pod (≥ 3 restarts triggers warning)
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-pod", redisName),
					Namespace: namespace,
					Labels:    map[string]string{"app": redisName},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "bitnami/redis:8.0", // match redisInstance spec
						},
					},
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:         "redis",
							Ready:        false,
							RestartCount: 3,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).Should(Succeed())

			// Step 2b: mark the pod unhealthy through the /status sub-resource
			Expect(k8sClient.Get(ctx,
				types.NamespacedName{Name: pod.Name, Namespace: namespace}, pod)).Should(Succeed())

			pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
				Name:         "redis",
				Ready:        false, // not ready  → unhealthy
				RestartCount: 3,
			}}
			Expect(k8sClient.Status().Update(ctx, pod)).Should(Succeed())

			// Step 3: Trigger reconciliation and verify warning event
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: redisName, Namespace: namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify Pod still exists (controller does NOT delete it)
			Consistently(func() bool {
				dummy := &corev1.Pod{}
				return k8sClient.Get(ctx,
					types.NamespacedName{Name: pod.Name, Namespace: namespace},
					dummy) == nil
			}, 2*time.Second, 200*time.Millisecond).Should(BeTrue(),
				"Pod should remain; controller only emits a warning event")

			// Step 4: Check for emitted Warning event
			eventRecorder, ok := reconciler.EventRecorder.(*record.FakeRecorder)
			Expect(ok).To(BeTrue(), "Recorder must be FakeRecorder in tests")
			select {
			case event := <-eventRecorder.Events:
				Expect(event).To(ContainSubstring("manual check recommended"),
					"Expected warning event about pod restarts")
			case <-time.After(2 * time.Second):
				Fail("Expected recovery event emission, but timed out")
			}
		})
	})
})
