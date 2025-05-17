# Redis Operator for Kubernetes

The Redis Operator is a Proof-of-Concept Kubernetes Operator that manages Redis instances defined through Custom Resources (CR). It provides secure automated password management, controlled configuration of Redis deployments, persistent storage, resource management (CPU and Memory), and simple scaling operations.

---

## Quick Start (Setup Instructions)

### 1) Install CRDs and Operator in your Kubernetes Cluster:

```bash
kubectl apply -f https://raw.githubusercontent.com/GeiserX/redis-operator/config/deploy-redis-operator.yaml
```
### 2) Deploy a Redis instance (Example Redis CR):

Create a file `redis-cr.yaml` with the following contents:

```yaml
apiVersion: cache.geiser.cloud/v1alpha1
kind: Redis
metadata:
  name: my-redis
  namespace: default
spec:
  replicas: 3
  resources:
    requests:
      cpu: "200m"
      memory: "256Mi"
    limits:
      cpu: "500m"
      memory: "512Mi"
  persistence:
    enabled: true
    storageClass: "standard"
    size: "1Gi"
```

And then apply:
```bash
kubectl apply -f redis-cr.yaml
```

## Feature set

### Automated Password Management
Operator generates a robust random password for Redis when creating the related deployment. Stores secure passwords inside Kubernetes Secrets by default named <redis-name>-secret. It is used by then by the Redis deployment.

Retrieve the password for your application explicitly with:

```bash
kubectl get secret my-redis-secret -n default -o jsonpath='{.data.password}' | base64 --decode
```

### Scaling
The Operator supports replica scaling (Please check the Scaling Limitations section below):
```bash
kubectl patch redis my-redis -n default --type merge -p='{"spec":{"replicas": 3}}'
```
Or reapply the modified CR.

### Managing Resource Requests and Limits
The Redis operator lets you explicitly define CPU and Memory limits and requests

### Persistent Storage Configuration 
You can easily configure persistent storage explicitly.
```bash
kubectl patch redis my-redis -n default --type merge --patch='{
  "spec": {
    "persistence": {
      "enabled": true,
      "storageClass": "standard",
      "size": "5Gi"
    }
  }
}'
```
IMPORTANT NOTE: Decreasing volume sizes is unsupported explicitly due to Kubernetes PVC design limits.

### Automatic cleanup
All Kubernetes resources (Deployments, Secrets, PVC) explicitly auto-cleaned by Kubernetes via ownership references.

## Limitations

This Redis Operator is explicitly developed as a Proof-Of-Concept (PoC) demonstrating Kubernetes Operator patterns. It has explicit limitations clearly stated:

- Scaling Limitations: Explicitly scales by adding/removing independent Redis Pods.

- Explicitly DOES NOT provision Redis Clusters or implement highly available Redis (no Redis Sentinel or Redis Cluster).

- Lack of Data Replication: Redis pods explicitly run standalone—NO Sentinel, No automatic redis data replication or HA. Explicitly recommend for lightweight tests, demos only—not production workloads.

## Maintainers

[@GeiserX](https://github.com/GeiserX).

## Contributing

Feel free to dive in! [Open an issue](https://github.com/GeiserX/genieacs-docker/issues/new) or submit PRs.

GenieACS-Docker follows the [Contributor Covenant](http://contributor-covenant.org/version/2/1/) Code of Conduct.
