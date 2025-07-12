# Cluster Backup Helm Chart

A Helm chart for deploying the production-ready multi-cluster Kubernetes backup system with MinIO storage and Git synchronization.

## Installation

### Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- MinIO server accessible from the cluster
- Git repository for backup storage (optional, for git-sync)

### Add Helm Repository (if published)

```bash
helm repo add cluster-backup https://your-org.github.io/kubeckup
helm repo update
```

### Install from Local Chart

```bash
# Clone the repository
git clone https://github.com/walbis/kubeckup.git
cd kubeckup/k8s/helm

# Install the chart
helm install my-backup cluster-backup \
  --namespace backup-system \
  --create-namespace \
  --set cluster.name="my-cluster" \
  --set minio.endpoint="minio.example.com:9000" \
  --set minio.credentials.accessKey="your-access-key" \
  --set minio.credentials.secretKey="your-secret-key"
```

## Configuration

### Required Values

The following values must be configured for a successful deployment:

```yaml
# Unique cluster identifier
cluster:
  name: "production-east"

# MinIO configuration
minio:
  endpoint: "minio.example.com:9000"
  credentials:
    accessKey: "your-access-key"
    secretKey: "your-secret-key"
```

### Backup Service Configuration

```yaml
backup:
  enabled: true
  schedule: "0 2 * * *"  # Daily at 2 AM
  filtering:
    mode: "hybrid"
    includeResources:
      - deployments
      - services
      - configmaps
    excludeResources:
      - events
      - nodes
```

### Git-Sync Configuration

```yaml
gitSync:
  enabled: true  # Enable only on one cluster
  schedule: "0 3 * * *"  # After backup completes

git:
  repository: "https://github.com/your-org/backups.git"
  branch: "main"
  user:
    name: "backup-bot"
    email: "backup@yourcompany.com"
  auth:
    token: "ghp_your_github_token"
```

## Deployment Scenarios

### Single Cluster Deployment

```bash
helm install backup cluster-backup \
  --namespace backup-system \
  --create-namespace \
  --values single-cluster-values.yaml
```

### Multi-Cluster Deployment

**Deploy backup service on all clusters:**

```bash
# Cluster A
helm install backup-east cluster-backup \
  --namespace backup-system \
  --create-namespace \
  --set cluster.name="production-east" \
  --set gitSync.enabled=false \
  --values common-values.yaml

# Cluster B  
helm install backup-west cluster-backup \
  --namespace backup-system \
  --create-namespace \
  --set cluster.name="production-west" \
  --set gitSync.enabled=false \
  --values common-values.yaml
```

**Deploy git-sync service on central cluster:**

```bash
helm install git-sync cluster-backup \
  --namespace backup-system \
  --set backup.enabled=false \
  --set gitSync.enabled=true \
  --values git-sync-values.yaml
```

## Values Reference

### Global Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `global.imageRegistry` | Global image registry | `""` |
| `global.imagePullSecrets` | Global image pull secrets | `[]` |
| `global.namespaceOverride` | Override namespace | `""` |

### Cluster Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `cluster.name` | Unique cluster identifier (required) | `"my-cluster"` |
| `cluster.domain` | Cluster domain | `"cluster.local"` |

### Image Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.backup.repository` | Backup service image | `"your-org/cluster-backup"` |
| `image.backup.tag` | Backup service tag | `"latest"` |
| `image.gitSync.repository` | Git-sync service image | `"your-org/git-sync"` |
| `image.gitSync.tag` | Git-sync service tag | `"latest"` |

### MinIO Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `minio.endpoint` | MinIO server endpoint (required) | `"minio.example.com:9000"` |
| `minio.bucket` | MinIO bucket name | `"cluster-backups"` |
| `minio.useSSL` | Use SSL for MinIO | `true` |
| `minio.credentials.accessKey` | MinIO access key (required) | `""` |
| `minio.credentials.secretKey` | MinIO secret key (required) | `""` |

### Backup Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `backup.enabled` | Enable backup service | `true` |
| `backup.schedule` | Backup cron schedule | `"0 2 * * *"` |
| `backup.filtering.mode` | Filtering mode (whitelist/blacklist/hybrid) | `"hybrid"` |
| `backup.filtering.includeResources` | Resources to include | See values.yaml |
| `backup.filtering.excludeResources` | Resources to exclude | See values.yaml |
| `backup.config.logLevel` | Log level | `"info"` |

### Git Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `git.enabled` | Enable git integration | `true` |
| `git.repository` | Git repository URL (required if enabled) | `""` |
| `git.branch` | Git branch | `"main"` |
| `git.user.name` | Git commit username | `"cluster-backup-bot"` |
| `git.auth.token` | Git authentication token | `""` |

### Monitoring Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `monitoring.enabled` | Enable monitoring | `true` |
| `monitoring.serviceMonitor.enabled` | Enable ServiceMonitor | `true` |
| `monitoring.prometheusRule.enabled` | Enable PrometheusRule | `true` |

## Example Values Files

### Production Multi-Cluster

```yaml
# production-values.yaml
cluster:
  name: "production-east"
  
minio:
  endpoint: "minio.prod.company.com:9000"
  useSSL: true
  credentials:
    accessKey: "prod-access-key"
    secretKey: "prod-secret-key"

backup:
  schedule: "0 1 * * *"  # 1 AM daily
  filtering:
    mode: "hybrid"
    includeResources:
      - deployments
      - services
      - configmaps
      - secrets
      - ingresses
    excludeNamespaces:
      - kube-system
      - monitoring
  resources:
    requests:
      cpu: 500m
      memory: 1Gi
    limits:
      cpu: 2000m
      memory: 2Gi

gitSync:
  enabled: false  # Enable on central cluster only

monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
    interval: 30s
```

### Central Git-Sync

```yaml
# git-sync-values.yaml
backup:
  enabled: false

gitSync:
  enabled: true
  schedule: "0 2 * * *"  # After backups complete

git:
  repository: "https://github.com/company/cluster-backups.git"
  branch: "main"
  user:
    name: "cluster-backup-bot"
    email: "devops@company.com"
  auth:
    token: "ghp_your_production_token"

resources:
  requests:
    cpu: 200m
    memory: 512Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

## Monitoring

The chart includes comprehensive monitoring configuration:

### Prometheus Metrics

- Backup operation duration and success rates
- Resource count and processing statistics
- Git synchronization performance metrics

### Alerting Rules

- Backup job failures
- Missing backup executions
- Git synchronization failures
- High backup duration warnings

### Grafana Dashboard

Import the provided dashboard for visualization:

```bash
kubectl get configmap cluster-backup-grafana-dashboard \
  -o jsonpath='{.data.dashboard\.json}' > backup-dashboard.json
```

## Troubleshooting

### Check Helm Release Status

```bash
helm status my-backup -n backup-system
```

### View Backup Logs

```bash
kubectl logs -l app.kubernetes.io/component=backup -n backup-system
```

### Test Configuration

```bash
# Dry-run installation
helm install my-backup cluster-backup \
  --dry-run --debug \
  --values my-values.yaml

# Template rendering
helm template my-backup cluster-backup \
  --values my-values.yaml
```

### Common Issues

1. **MinIO Connection Failed**
   - Verify endpoint and credentials
   - Check network connectivity
   - Confirm bucket exists

2. **Git Authentication Failed**
   - Verify token permissions
   - Check repository URL
   - Ensure token is not expired

3. **RBAC Permission Denied**
   - Verify ClusterRole permissions
   - Check ServiceAccount creation
   - Confirm namespace access

## Upgrading

```bash
# Upgrade to new version
helm upgrade my-backup cluster-backup \
  --namespace backup-system \
  --values my-values.yaml

# Rollback if needed
helm rollback my-backup 1 -n backup-system
```

## Uninstalling

```bash
helm uninstall my-backup -n backup-system
```

## Development

### Testing Locally

```bash
# Lint the chart
helm lint k8s/helm/cluster-backup

# Test template rendering
helm template test k8s/helm/cluster-backup \
  --values test-values.yaml

# Package the chart
helm package k8s/helm/cluster-backup
```

### Contributing

1. Make changes to templates or values
2. Update version in Chart.yaml
3. Test with different configurations
4. Update this README if needed

## Support

- GitHub Issues: https://github.com/walbis/kubeckup/issues
- Documentation: https://github.com/walbis/kubeckup/blob/main/README.md