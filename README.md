# redis-operator
Redis Operator for Kubernetes

## Scaling Limitations

The provided Redis Operator implementation scales Redis by adding independent Redis instances.
Please note, it does NOT automatically provision Redis Clusters or utilize Redis Sentinel. Each pod is isolated:
- Data is not replicated automatically between Redis instances.
- Suitable for testing or lightweight ephemeral caching use-cases.

For production deployments needing real Redis high availability and data replication, consider using:
- [Bitnami Redis Helm Chart](https://github.com/bitnami/charts/tree/master/bitnami/redis)
- Redis Cluster or Redis Sentinel deployment models explicitly.


kubectl patch redis my-redis -n default \
  --type merge --patch '{"spec":{"replicas":3}}'


kubectl patch redis my-redis -n default --type merge --patch='
{
  "spec": {
    "resources": {
      "requests": {
        "cpu": "200m",
        "memory": "256Mi"
      },
      "limits": {
        "cpu": "500m",
        "memory": "512Mi"
      }
    }
  }
}'