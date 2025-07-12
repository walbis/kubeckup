# Backup Service

Production-ready Kubernetes/OpenShift cluster backup service with enterprise-grade structured logging and comprehensive resource filtering.

## 🎯 Overview

The backup service is a Go-based application that runs as a CronJob in Kubernetes clusters to automatically backup cluster resources to MinIO object storage. It supports flexible resource filtering, OpenShift compatibility, and provides detailed operational metrics.

## 🏗 Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Backup Service                           │
│                                                             │
│  ┌─────────────────┐    ┌─────────────────┐                │
│  │  Configuration  │    │    Discovery    │                │
│  │   - ConfigMap   │    │  - API Groups   │                │
│  │   - Secrets     │    │  - Resources    │                │
│  │   - Env Vars    │    │  - OpenShift    │                │
│  └─────────────────┘    └─────────────────┘                │
│           │                       │                        │
│           └───────┬───────────────┘                        │
│                   │                                        │
│  ┌─────────────────▼─────────────────┐                     │
│  │        Backup Engine              │                     │
│  │  - Resource Filtering             │                     │
│  │  - YAML Processing                │                     │
│  │  - Batch Processing               │                     │
│  │  - Error Handling                 │                     │
│  └─────────────────┬─────────────────┘                     │
│                    │                                       │
│  ┌─────────────────▼─────────────────┐                     │
│  │       Output & Monitoring         │                     │
│  │  - MinIO Upload                   │                     │
│  │  - Prometheus Metrics             │                     │
│  │  - Structured Logging             │                     │
│  └─────────────────────────────────────┘                     │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
              ┌─────────────────────┐
              │    MinIO Storage    │
              │ clusterbackup/      │
              │ ├── cluster-a/      │
              │ ├── cluster-b/      │
              │ └── cluster-c/      │
              └─────────────────────┘
```

## 📋 Core Components

### 1. Configuration Management

**Environment Variables:**
```go
type Config struct {
    ClusterDomain     string
    ClusterName       string
    MinIOEndpoint     string
    MinIOAccessKey    string
    MinIOSecretKey    string
    MinIOBucket       string
    MinIOUseSSL       bool
    BatchSize         int
    RetryAttempts     int
    RetryDelay        time.Duration
}
```

**ConfigMap-based Filtering:**
```go
type BackupConfig struct {
    FilteringMode           string   // "whitelist", "blacklist", "hybrid"
    IncludeResources        []string
    ExcludeResources        []string
    IncludeNamespaces       []string
    ExcludeNamespaces       []string
    IncludeCRDs             []string
    LabelSelector           string
    AnnotationSelector      string
    MaxResourceSize         string
    FollowOwnerReferences   bool
    IncludeManagedFields    bool
    IncludeStatus           bool
    OpenShiftMode           string
    IncludeOpenShiftRes     bool
    ValidateYAML            bool
    SkipInvalidResources    bool
}
```

### 2. Resource Discovery Engine

**API Discovery Process:**
1. **Kubernetes API Discovery**: Discovers all available API groups and resources
2. **OpenShift Detection**: Auto-detects OpenShift environment by checking for specific APIs
3. **CRD Discovery**: Dynamically discovers Custom Resource Definitions
4. **Resource Filtering**: Applies filtering rules based on configuration

**OpenShift Auto-Detection:**
```go
func (cb *ClusterBackup) isOpenShift() bool {
    // Check for OpenShift-specific APIs
    openShiftAPIs := []string{
        "route.openshift.io/v1",
        "build.openshift.io/v1",
        "image.openshift.io/v1",
        "apps.openshift.io/v1",
    }
    // Returns true if any OpenShift API is found
}
```

### 3. Filtering Strategies

**Whitelist Mode**: Only backup specified resources
```yaml
filtering-mode: "whitelist"
include-resources: |
  deployments
  services
  configmaps
```

**Blacklist Mode**: Backup everything except specified resources
```yaml
filtering-mode: "blacklist"
exclude-resources: |
  events
  nodes
  endpoints
```

**Hybrid Mode**: Combine whitelist and blacklist
```yaml
filtering-mode: "hybrid"
include-resources: |
  deployments
  services
exclude-resources: |
  events
```

### 4. Backup Processing Engine

**Resource Processing Flow:**
1. **Namespace Discovery**: Lists all namespaces or filters by include/exclude rules
2. **Resource Enumeration**: For each namespace, lists all resources of each type
3. **Resource Retrieval**: Fetches individual resource definitions
4. **YAML Processing**: Converts to YAML and optionally cleans managed fields
5. **MinIO Upload**: Uploads to structured path in object storage

**Storage Path Structure:**
```
clusterbackup/{cluster-name}/{namespace}/{resource-type}/{resource-name}.yaml
```

Example:
```
clusterbackup/production-east/default/deployments/nginx-deployment.yaml
clusterbackup/production-east/kube-system/services/kube-dns.yaml
```

### 5. Structured Logging System

**Log Entry Format:**
```json
{
  "timestamp": "2025-07-12T22:06:58Z",
  "level": "info",
  "component": "backup",
  "cluster": "production-east",
  "namespace": "default",
  "resource": "deployments",
  "operation": "resource_backup_complete",
  "message": "Resource backup completed successfully",
  "data": {
    "backed_up": 25,
    "skipped": 3,
    "invalid": 0,
    "duration_ms": 1234.56
  },
  "duration_ms": 1234.56
}
```

**Log Levels:**
- **DEBUG**: Detailed operational information, individual resource processing
- **INFO**: Standard operational logs, backup summaries
- **WARN**: Non-critical issues, skipped resources
- **ERROR**: Error conditions, failed operations

### 6. Prometheus Metrics

**Exposed Metrics:**
```go
type BackupMetrics struct {
    BackupDuration     prometheus.Histogram  // Backup operation duration
    BackupErrors       prometheus.Counter    // Total backup errors
    ResourcesBackedUp  prometheus.Counter    // Total resources backed up
    LastBackupTime     prometheus.Gauge      // Last successful backup timestamp
    NamespacesBackedUp prometheus.Gauge      // Number of namespaces backed up
}
```

**Metrics Endpoint**: `:8080/metrics`

## 🚀 Usage

### 1. Environment Configuration

**Required Environment Variables:**
```bash
CLUSTER_NAME=production-east
CLUSTER_DOMAIN=company.local
MINIO_ENDPOINT=minio.example.com:9000
MINIO_ACCESS_KEY=your-access-key
MINIO_SECRET_KEY=your-secret-key
MINIO_BUCKET=cluster-backups
MINIO_USE_SSL=true
LOG_LEVEL=info
```

### 2. ConfigMap Configuration

**Basic Configuration:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: backup-config
  namespace: backup-system
data:
  filtering-mode: "hybrid"
  include-resources: |
    deployments
    services
    configmaps
    secrets
    persistentvolumeclaims
  exclude-resources: |
    events
    nodes
    endpoints
  include-namespaces: |
    production
    staging
  exclude-namespaces: |
    kube-system
    kube-public
  include-openshift-resources: "true"
  include-crds: |
    workflows.argoproj.io
    routes.route.openshift.io
```

### 3. Multi-Cluster Deployment

**Deploy on Each Cluster:**
```bash
# Set unique cluster name for each deployment
CLUSTER_NAME="production-east" kubectl apply -f k8s/backup/backup-cronjob-multicluster.yaml
CLUSTER_NAME="production-west" kubectl apply -f k8s/backup/backup-cronjob-multicluster.yaml
```

**Result in MinIO:**
```
clusterbackup/
├── production-east/
│   ├── default/
│   ├── production/
│   └── staging/
└── production-west/
    ├── default/
    ├── production/
    └── staging/
```

## 🔧 Advanced Configuration

### Custom Resource Definitions (CRDs)

The service automatically discovers and backs up CRDs. You can specify which CRDs to include:

```yaml
include-crds: |
  workflows.argoproj.io
  workflowtemplates.argoproj.io
  applications.argoproj.io
  routes.route.openshift.io
  buildconfigs.build.openshift.io
```

### Label and Annotation Selectors

Filter resources by labels or annotations:

```yaml
label-selector: "app=production,tier!=cache"
annotation-selector: "backup.io/enabled=true"
```

### Resource Size Limits

Set maximum resource size to prevent large objects:

```yaml
max-resource-size: "10Mi"
skip-invalid-resources: "true"
```

## 📊 Monitoring & Observability

### Log Analysis Examples

**Find Backup Errors:**
```bash
kubectl logs -f deployment/backup-service | jq 'select(.level=="error")'
```

**Backup Performance:**
```bash
kubectl logs deployment/backup-service | jq 'select(.operation=="backup_complete") | .data'
```

### Prometheus Queries

**Backup Success Rate:**
```promql
(1 - rate(cluster_backup_errors_total[5m]) / rate(cluster_backup_duration_seconds_count[5m])) * 100
```

**Resources Per Hour:**
```promql
increase(cluster_backup_resources_total[1h])
```

**Average Backup Duration:**
```promql
rate(cluster_backup_duration_seconds_sum[5m]) / rate(cluster_backup_duration_seconds_count[5m])
```

## 🐛 Troubleshooting

### Common Issues

**1. MinIO Connection Failures**
```json
{
  "level": "error",
  "operation": "minio_connect",
  "message": "Failed to connect to MinIO",
  "error": "connection refused"
}
```
- Check MinIO endpoint and credentials
- Verify network connectivity
- Confirm bucket exists

**2. RBAC Permission Errors**
```json
{
  "level": "error",
  "operation": "resource_list",
  "message": "Failed to list resources",
  "error": "forbidden: User cannot list deployments"
}
```
- Verify ClusterRole permissions
- Check ServiceAccount binding
- Ensure proper namespace access

**3. Resource Filtering Issues**
```json
{
  "level": "warn",
  "operation": "resource_filter",
  "message": "Resource filtered out by configuration",
  "data": {"reason": "excluded_by_blacklist"}
}
```
- Review filtering configuration
- Check include/exclude lists
- Verify filtering mode

### Debug Mode

Enable detailed logging:
```bash
kubectl set env cronjob/backup-cronjob LOG_LEVEL=debug
```

## 🔒 Security Considerations

### RBAC Requirements

The service requires read access to:
- All standard Kubernetes resources
- Custom Resource Definitions
- OpenShift-specific resources (if applicable)

### Secret Management

- MinIO credentials stored in Kubernetes secrets
- No credentials logged or exposed in metrics
- Secure MinIO communication with SSL/TLS

### Container Security

- Runs as non-root user (UID 1001)
- Read-only root filesystem
- No privileged capabilities
- Security context constraints compliant

## 📦 Building and Deployment

### Build Container Image

```bash
cd code/backup
docker build -t your-registry/cluster-backup:latest .
docker push your-registry/cluster-backup:latest
```

### Update Deployment

```bash
kubectl set image cronjob/backup-cronjob cluster-backup=your-registry/cluster-backup:latest
```

## 🔄 Integration

The backup service integrates seamlessly with:
- **Git-Sync Service**: Uploads backups for version control
- **Prometheus Monitoring**: Exposes operational metrics
- **Grafana Dashboards**: Visualizes backup performance
- **AlertManager**: Triggers alerts on failures

## 📈 Performance Optimization

### Batch Processing

Configure batch size for optimal performance:
```yaml
batch-size: "50"  # Process 50 resources at a time
```

### Retry Configuration

Configure retry behavior:
```yaml
retry-attempts: "3"
retry-delay: "5s"
```

### Resource Limits

Set appropriate resource limits:
```yaml
resources:
  requests:
    cpu: 200m
    memory: 512Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

This backup service provides a robust, scalable solution for multi-cluster Kubernetes backup with enterprise-grade monitoring and operational visibility.