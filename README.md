# Redis Operator (Proof-of-Concept)

A lightweight Kubernetes Operator that manages single-node Redis instances defined through a Custom Resource (CR).

Features
---------
* Automated, **immutable password** generation stored in a Secret
* Declarative **CPU / memory requests & limits**
* Simple **scaling** through `spec.replicas`
* **Health watch** – emits a Warning event on the Redis CR when a Pod restarts
  three or more times
* Automatic clean-up via OwnerReferences (Deployments, Pods and Secret are
  removed when the CR is deleted)

## Quick Start

Prerequisites: a working `kubectl` context, `kubectl apply` permissions and Kubernetes between v1.26 and v1.33 (included).

### 1) Install CRDs and Operator in your Kubernetes Cluster:

**Option A** - apply published manifest (no clone required):
```bash
kubectl apply -f https://raw.githubusercontent.com/GeiserX/redis-operator/config/deploy-redis-operator.yaml
```

**Option B** - build & deploy from source:
```bash
git clone https://github.com/GeiserX/redis-operator.git
cd redis-operator
make docker-build IMG=docker.io/<user>/redis-operator:<tag>
make deploy      IMG=docker.io/<user>/redis-operator:<tag>
```

Or use my prebuild image (`latest` always available too):
```bash
make deploy IMG=docker.io/drumsergio/redis-operator:0.0.1
```

---
In any case: Verify that the controller manager pod is running:
```bash
kubectl -n redis-operator-system get pods
```

### 2) Deploy a Redis instance (Example Redis CR):

Create a file `redis-cr.yaml` with the following contents, as an example:

```yaml
apiVersion: cache.geiser.cloud/v1alpha1
kind: Redis
metadata:
  name: redis-demo
  namespace: default
  labels:
    app.kubernetes.io/name: redis-demo
spec:
  # ────────────── scale ──────────────
  replicas: 1

  # ───────────── image ──────────────
  image: bitnami/redis:8.0

  # ───────── resource requests / limits ─────────
  resources:
    requests:
      cpu: "200m"
      memory: "256Mi"
    limits:
      cpu: "500m"
      memory: "512Mi"

  # ───────── probes ─────────
  probes:
    liveness:
      command: ["sh", "-c", "redis-cli -a \"$REDIS_PASSWORD\" PING"]
      initialDelaySeconds: 20
      periodSeconds: 10
      timeoutSeconds: 3
      failureThreshold: 5
    readiness:
      command: ["sh", "-c", "redis-cli -a \"$REDIS_PASSWORD\" SET readiness_probe OK"]
      initialDelaySeconds: 5
      periodSeconds: 5
      timeoutSeconds: 3
      failureThreshold: 3
```

And then apply:
```bash
kubectl apply -f redis-cr.yaml
```

## Checks

### Check the status of the Redis instance

Confirm that Redis is up (if name and namespace are as default):

```bash
kubectl get redis redis-demo
kubectl get deploy,po -l app=redis-demo -w 
```

### Check the password
To retrieve the password in use:

```bash
kubectl get secret my-redis-secret -o jsonpath='{.data.password}' | base64 --decode
```

### Check E2E test locally
Optionally, you could test the instance locally with `redis-cli`, including more tests on your side:
```bash
# port-forward a Redis pod on 6379
kubectl port-forward deploy/my-redis-deployment 6379:6379 &
redis-cli -a "$(kubectl get secret my-redis-secret -o jsonpath='{.data.password}' | base64 --decode)" PING
```

## Updating a Redis Instance

Increasing replicas (e.g. to 3):

```bash
kubectl patch redis my-redis --type merge -p='{"spec":{"replicas":3}}'
```
You could also edit the Redis CR and reapply.

Changing CPU limits (e.g. to 1):

```bash
kubectl patch redis my-redis --type merge -p='{"spec":{"resources":{"limits":{"cpu":"1"}}}}'
```

## Operator health watch

If any Redis pod restarts three or more times the operator emits a Warning event on the Redis CR:

```bash
kubectl get events --field-selector involvedObject.kind=Redis
```

## Tests

If you would like to execute the tests, make sure exactly you already installed the Operator Framework CLI tooling and execute the following:
```bash
make envtest
export KUBEBUILDER_ASSETS="$(pwd)/bin/k8s/$(go env GOOS)/$(go env GOARCH)"
make test
```

## Limitations

This Redis Operator is explicitly developed as a Proof-Of-Concept (PoC) demonstrating Kubernetes Operator patterns. It has limitations as stated:

- Scaling Limitations: Explicitly scales by adding/removing independent Redis Pods.

- It DOES NOT provision Redis Clusters or implement highly available Redis (no Redis Sentinel or Redis Cluster).

- PVC / persistence, Service, Ingresses, Autoscaling, Observability is not included in this PoC

- Verified with Bitnami Redis 6.2, 7.2 and 8.0 images. Other images or major versions should work but are untested.

## Maintainers

[@GeiserX](https://github.com/GeiserX)

## Contributing

Feel free to dive in! [Open an issue](https://github.com/GeiserX/genieacs-docker/issues/new) or submit PRs. Currently, only the `dev` branch is used for development purposes, and `main` for releases.

Redis Operator (PoC) follows the [Contributor Covenant](http://contributor-covenant.org/version/2/1/) Code of Conduct.
