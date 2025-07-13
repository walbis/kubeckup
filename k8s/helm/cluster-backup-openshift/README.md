# Cluster Backup OpenShift Helm Chart

Production-ready Helm chart for deploying cluster backup system on OpenShift with MinIO storage and Git synchronization.

## 🎯 Features

- ✅ **OpenShift Optimized**: Native OpenShift integration with Routes, SCC, and monitoring
- ✅ **Helm 3 Compatible**: Modern Helm chart with template best practices
- ✅ **Configurable**: Comprehensive values.yaml for all settings
- ✅ **Multi-Cluster Ready**: Support for multiple OpenShift clusters
- ✅ **Monitoring Included**: Prometheus ServiceMonitor and alerting rules
- ✅ **Security Hardened**: OpenShift-compatible security contexts and RBAC
- ✅ **Git Integration**: Optional git synchronization for backup versioning
- ✅ **Cleanup Management**: Automatic backup retention and cleanup

## 📋 Prerequisites

- OpenShift 4.8+
- Helm 3.0+
- MinIO server accessible from the cluster
- Git repository (optional, for git-sync)
- Prometheus Operator (optional, for monitoring)

## 🚀 Quick Start

### 1. Add Helm Repository (if published)

```bash
helm repo add cluster-backup https://your-org.github.io/kubeckup
helm repo update
```

### 2. Install from Local Chart

```bash
# Clone the repository
git clone https://github.com/walbis/kubeckup.git
cd kubeckup/k8s/helm

# Install the chart
helm install my-openshift-backup cluster-backup-openshift \
  --namespace backup-system \
  --create-namespace \
  --set cluster.name="my-openshift-cluster" \
  --set minio.endpoint="minio.apps.my-openshift.com" \
  --set minio.credentials.accessKey="your-access-key" \
  --set minio.credentials.secretKey="your-secret-key"
```

## 🔧 Configuration

### Required Values

The following values must be configured for a successful deployment:

```yaml
# values.yaml
cluster:
  name: "production-openshift"  # REQUIRED: Unique cluster identifier

minio:
  endpoint: "minio.apps.openshift.example.com"  # REQUIRED
  credentials:
    accessKey: "your-minio-access-key"          # REQUIRED
    secretKey: "your-minio-secret-key"          # REQUIRED
```

### Complete Example Values

```yaml
# Example production configuration
cluster:
  name: "production-openshift"
  domain: "cluster.local"

# MinIO configuration
minio:
  endpoint: "minio.apps.openshift.example.com"
  bucket: "openshift-cluster-backups"
  useSSL: true
  credentials:
    accessKey: "prod-backup-access-key"
    secretKey: "prod-backup-secret-key"

# Git synchronization (optional)
git:
  enabled: true
  repository: "https://github.com/your-org/openshift-backups.git"
  branch: "main"
  user:
    name: "openshift-backup-bot"
    email: "backup@yourcompany.com"
  auth:
    token: "ghp_your_github_token_here"

# Backup configuration
backup:
  enabled: true
  schedule: "0 2 * * *"  # Daily at 2 AM
  filtering:
    mode: "hybrid"
    includeResources:
      - deployments
      - services
      - routes
      - buildconfigs
    excludeResources:
      - events
      - pods
  config:
    enableCleanup: true
    retentionDays: 14  # Keep backups for 2 weeks
    logLevel: "info"

# Git-sync (deploy only on one cluster)
gitSync:
  enabled: false  # Enable only on central cluster

# Monitoring
monitoring:
  enabled: true
  serviceMonitor:
    enabled: true
  prometheusRule:
    enabled: true

# OpenShift Routes (optional)
routes:
  enabled: true
  backup:
    enabled: true
    host: "backup-metrics.apps.openshift.example.com"
    tls:
      termination: edge
      insecureEdgeTerminationPolicy: Redirect
```

## 📊 Deployment Scenarios

### 1. Single Cluster with Git Sync

```bash
helm install openshift-backup cluster-backup-openshift \
  --namespace backup-system \
  --create-namespace \
  --set cluster.name="openshift-prod" \
  --set minio.endpoint="minio.apps.openshift.com" \
  --set minio.credentials.accessKey="access-key" \
  --set minio.credentials.secretKey="secret-key" \
  --set git.enabled=true \
  --set git.repository="https://github.com/org/backups.git" \
  --set git.auth.token="ghp_token" \
  --set gitSync.enabled=true
```

### 2. Multi-Cluster Setup

**Central Cluster (with git-sync):**
```bash
helm install central-backup cluster-backup-openshift \
  --namespace backup-system \
  --create-namespace \
  --values central-cluster-values.yaml
```

**Remote Clusters (backup only):**
```bash
helm install remote-backup cluster-backup-openshift \
  --namespace backup-system \
  --create-namespace \
  --set cluster.name="openshift-remote-1" \
  --set gitSync.enabled=false \
  --values remote-cluster-values.yaml
```

### 3. Development/Testing

```bash
helm install dev-backup cluster-backup-openshift \
  --namespace backup-system \
  --create-namespace \
  --set development.enabled=true \
  --set backup.schedule="*/15 * * * *"  # Every 15 minutes for testing
  --set backup.config.retentionDays=1 \
  --values dev-values.yaml
```

## 🔍 Monitoring & Observability

### Prometheus Integration

The chart automatically creates:
- **ServiceMonitor**: For Prometheus scraping
- **PrometheusRule**: Alerting rules for backup failures
- **Service**: Metrics endpoint exposure

### Available Metrics

```promql
# Backup success rate
(1 - rate(cluster_backup_errors_total[5m]) / rate(cluster_backup_duration_seconds_count[5m])) * 100

# Resources backed up per hour
increase(cluster_backup_resources_total[1h])

# Time since last successful backup
time() - cluster_backup_last_success_timestamp
```

### OpenShift Routes

Access metrics via OpenShift Routes:

```yaml
routes:
  enabled: true
  backup:
    enabled: true
    host: "backup-metrics.apps.openshift.example.com"
```

## 🛠 Management Commands

### Upgrade Chart

```bash
helm upgrade openshift-backup cluster-backup-openshift \
  --namespace backup-system \
  --values updated-values.yaml
```

### Check Status

```bash
# View all resources
helm status openshift-backup -n backup-system

# Check backup jobs
oc get cronjob -n backup-system
oc get jobs -l app.kubernetes.io/component=backup -n backup-system

# View logs
oc logs -l app.kubernetes.io/component=backup -n backup-system
```

### Manual Backup

```bash
# Trigger manual backup job
oc create job manual-backup-$(date +%s) \
  --from=cronjob/my-openshift-backup-cluster-backup-openshift-backup \
  -n backup-system
```

### Uninstall

```bash
# Remove chart but keep PVCs
helm uninstall openshift-backup -n backup-system

# Complete cleanup (including PVCs)
helm uninstall openshift-backup -n backup-system
oc delete pvc -l app.kubernetes.io/instance=openshift-backup -n backup-system
```

## 🎛 Configuration Reference

### Key Configuration Sections

| Section | Description | Required |
|---------|-------------|----------|
| `cluster.name` | Unique cluster identifier | ✅ |
| `minio.endpoint` | MinIO server endpoint | ✅ |
| `minio.credentials` | MinIO access credentials | ✅ |
| `backup.filtering` | Resource filtering rules | ⚪ |
| `git.*` | Git synchronization config | ⚪ |
| `monitoring.*` | Prometheus monitoring | ⚪ |
| `routes.*` | OpenShift Routes config | ⚪ |

### OpenShift-Specific Features

- **Routes**: External access to metrics endpoints
- **SCC Compatibility**: Works with restricted-v2 SCC
- **OpenShift Resources**: Routes, BuildConfigs, ImageStreams
- **Namespace Filtering**: Excludes OpenShift system namespaces
- **Monitoring Integration**: Native Prometheus Operator support

## 🐛 Troubleshooting

### Common Issues

1. **Permission Denied**
   ```bash
   # Check RBAC
   oc auth can-i list deployments --as=system:serviceaccount:backup-system:cluster-backup
   ```

2. **MinIO Connection**
   ```bash
   # Test MinIO connectivity
   oc run minio-test --rm -it --image=minio/mc -- mc ls minio-alias/bucket
   ```

3. **Git Authentication**
   ```bash
   # Check git credentials
   oc get secret cluster-backup-secrets -o yaml
   ```

### Debug Mode

```bash
# Enable debug logging
helm upgrade openshift-backup cluster-backup-openshift \
  --set backup.config.logLevel=debug \
  --reuse-values
```

## 🔒 Security Considerations

- Uses OpenShift's restricted-v2 SecurityContextConstraints
- Read-only RBAC permissions for cluster resources
- Secrets are encrypted at rest in etcd
- Container runs as non-root user
- Optional network policies for traffic isolation

## 🏷 Chart Values

For complete configuration options, see the [values.yaml](values.yaml) file which contains detailed comments for all available settings.

## 📞 Support

- **Documentation**: [GitHub Repository](https://github.com/walbis/kubeckup)
- **Issues**: [GitHub Issues](https://github.com/walbis/kubeckup/issues)
- **Discussions**: [GitHub Discussions](https://github.com/walbis/kubeckup/discussions)

## 📝 License

This chart is licensed under the same license as the main project.