# Production-Ready Multi-Cluster Kubernetes Backup System

A comprehensive backup solution for multi-cluster Kubernetes/OpenShift environments with enterprise-grade structured logging, monitoring, and centralized coordination.

## 🚀 Features

### Core Backup Capabilities
- **Multi-Cluster Support**: Centralized backup coordination across multiple clusters
- **OpenShift Compatibility**: Auto-detection and support for OpenShift resources
- **Flexible Resource Filtering**: Whitelist, blacklist, and hybrid filtering modes
- **Custom Resource Definitions (CRDs)**: Full support for custom resources
- **MinIO Integration**: Secure object storage with organized folder structure
- **Git Synchronization**: Incremental git sync with change detection

### Production Features
- **Structured Logging**: JSON-formatted logs with comprehensive operational data
- **Prometheus Metrics**: Built-in monitoring and alerting capabilities
- **Security Hardened**: Non-root containers, read-only filesystems, comprehensive RBAC
- **Resource Optimization**: Configurable batch processing and retry mechanisms
- **Health Checks**: Built-in health endpoints for container orchestration
- **Automatic Cleanup**: Configurable backup retention with automatic old file removal

## 📋 Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Cluster A     │    │   Cluster B     │    │   Cluster C     │
│                 │    │                 │    │                 │
│  ┌─────────────┐│    │  ┌─────────────┐│    │  ┌─────────────┐│
│  │Backup CronJob││    │  │Backup CronJob││    │  │Backup CronJob││
│  └─────────────┘│    │  └─────────────┘│    │  └─────────────┘│
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
         ┌─────────────────────────────────────────────────┐
         │                MinIO Storage                    │
         │   clusterbackup/                               │
         │   ├── cluster-a/                               │
         │   ├── cluster-b/                               │
         │   └── cluster-c/                               │
         └─────────────────────────────────────────────────┘
                                 │
         ┌─────────────────────────────────────────────────┐
         │          Central Git-Sync Service               │
         │         (Deployed on one cluster)              │
         └─────────────────────────────────────────────────┘
                                 │
         ┌─────────────────────────────────────────────────┐
         │              Git Repository                     │
         │         (Backup Version Control)                │
         └─────────────────────────────────────────────────┘
```

## 🛠 Quick Start

### 1. Deploy Backup Service (All Clusters)

```bash
# Deploy backup service on each cluster with unique CLUSTER_NAME
CLUSTER_NAME="production-east" kubectl apply -f k8s/backup/
```

### 2. Deploy Git-Sync Service (Central Cluster Only)

```bash
# Deploy git-sync service on one central cluster
kubectl apply -f k8s/git-sync/
```

### 3. Deploy Monitoring (Optional)

```bash
# Deploy monitoring configuration
kubectl apply -f k8s/monitoring/
```

### 4. Deploy Shared Resources (If needed)

```bash
# Deploy shared/common resources
kubectl apply -f k8s/shared/
```

### 5. OpenShift Deployment (Alternative)

For OpenShift clusters using default ServiceAccount:

```bash
# Deploy OpenShift-specific manifests
oc apply -f k8s/openshift/

# See k8s/openshift/deployment-instructions.md for detailed guide
```

## 📊 Enhanced Structured Logging

### Log Format

All components produce structured JSON logs for enterprise observability:

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

### Log Levels

Configure logging verbosity via environment variable:

- `LOG_LEVEL=debug`: Detailed operational information
- `LOG_LEVEL=info`: Standard operational logs (default)
- `LOG_LEVEL=warn`: Warnings and non-critical issues
- `LOG_LEVEL=error`: Error conditions only

### Key Operations Tracked

**Backup Component:**
- Configuration loading and validation
- MinIO connectivity and bucket verification
- OpenShift auto-detection results
- API resource discovery and filtering
- Per-namespace backup statistics
- Individual resource processing
- Error categorization and context

**Git-Sync Component:**
- Git repository operations (clone/pull/push)
- MinIO download progress and statistics
- Change detection and commit analysis
- Multi-cluster coordination
- Authentication and authorization events

## 🎯 Resource Filtering & Cleanup

### Filtering Modes & Retention

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: backup-config
  namespace: backup-system
data:
  # Choose filtering strategy
  filtering-mode: "hybrid"  # whitelist, blacklist, or hybrid
  
  # Whitelist: only backup these resources
  include-resources: |
    deployments
    services
    configmaps
    secrets
  
  # Blacklist: backup everything except these
  exclude-resources: |
    events
    nodes
    endpoints
  
  # Namespace filtering
  include-namespaces: |
    production
    staging
  
  # OpenShift resources
  include-openshift-resources: "true"
  
  # Custom Resource Definitions
  include-crds: |
    workflows.argoproj.io
    routes.route.openshift.io
  
  # Backup retention and cleanup (NEW)
  enable-cleanup: "true"
  retention-days: "7"
  cleanup-on-startup: "false"
```

## 📈 Monitoring & Metrics

### Prometheus Metrics

Both components expose metrics on `:8080/metrics`:

**Backup Metrics:**
- `cluster_backup_duration_seconds`: Backup operation duration
- `cluster_backup_errors_total`: Total backup errors
- `cluster_backup_resources_total`: Total resources backed up
- `cluster_backup_last_success_timestamp`: Last successful backup time
- `cluster_backup_namespaces_total`: Number of namespaces backed up

**Git-Sync Metrics:**
- `git_sync_duration_seconds`: Sync operation duration
- `git_sync_errors_total`: Total sync errors
- `git_sync_files_processed_total`: Total files processed
- `git_sync_last_success_timestamp`: Last successful sync time
- `git_sync_clusters_backed_up`: Number of clusters processed

### Prometheus Setup

1. **Install Prometheus (Helm)**:
```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace
```

2. **Deploy Monitoring Configuration**:
```bash
kubectl apply -f k8s/monitoring/
```

3. **Access Metrics**:
- Prometheus: `http://prometheus.monitoring.svc.cluster.local:9090`
- Grafana: `http://grafana.monitoring.svc.cluster.local:3000`

### Key Metrics Queries

```promql
# Backup success rate
(1 - rate(cluster_backup_errors_total[5m]) / rate(cluster_backup_duration_seconds_count[5m])) * 100

# Resources backed up per hour
increase(cluster_backup_resources_total[1h])

# Average backup duration
rate(cluster_backup_duration_seconds_sum[5m]) / rate(cluster_backup_duration_seconds_count[5m])

# Time since last successful backup
time() - cluster_backup_last_success_timestamp
```

### Built-in Alerts

- **BackupJobFailed**: Triggers when backup fails
- **BackupJobNotRunning**: Triggers when no backup in 24h
- **GitSyncFailed**: Triggers when git sync fails
- **HighBackupDuration**: Triggers when backup takes >1 hour

## 🔒 Security Features

- **RBAC**: Minimal required permissions with comprehensive resource access
- **Non-root Containers**: Security-hardened container execution
- **Read-only Filesystems**: Immutable container filesystems where possible
- **Secret Management**: Secure credential handling via Kubernetes secrets
- **Network Policies**: Optional network segmentation support

## 🚀 Production Deployment

### High Availability
- Deploy backup jobs across multiple nodes using node selectors
- Configure pod disruption budgets for critical workloads
- Use persistent volumes for git-sync work directories

### Monitoring Integration
- Configure Prometheus scraping for metrics collection
- Set up Grafana dashboards for operational visibility
- Create alerts for backup failures and performance issues

### Log Aggregation
- Forward structured logs to ELK stack, Splunk, or similar
- Configure log retention policies
- Set up log-based alerting for critical events

## 📁 Directory Structure

```
├── code/
│   ├── backup/           # Backup service source code
│   │   ├── main.go       # Enhanced backup application
│   │   ├── Dockerfile    # Container image definition
│   │   ├── go.mod        # Go module dependencies
│   │   └── go.sum        # Dependency checksums
│   └── git-sync/         # Git synchronization service
│       ├── main.go       # Enhanced git-sync application
│       ├── Dockerfile    # Container image definition
│       ├── go.mod        # Go module dependencies
│       └── go.sum        # Dependency checksums
└── k8s/                  # Kubernetes manifests (organized by service)
    ├── backup/           # Backup service manifests
    │   ├── backup-cronjob-multicluster.yaml
    │   ├── backup-cronjob.yaml
    │   ├── backup-secret.yaml
    │   ├── namespace-backup-system.yaml
    │   └── rbac-backup-system.yaml
    ├── git-sync/         # Git-sync service manifests
    │   ├── git-sync-cronjob-central.yaml
    │   ├── git-sync-cronjob.yaml
    │   └── git-sync-secret.yaml
    ├── helm/             # Helm chart for deployment automation
    │   └── cluster-backup/
    │       ├── Chart.yaml
    │       ├── values.yaml
    │       ├── templates/
    │       └── examples/
    ├── openshift/        # OpenShift-specific manifests (NEW)
    │   ├── rbac-default-sa.yaml
    │   ├── backup-cronjob-default-sa.yaml
    │   ├── scc-backup.yaml
    │   ├── configmap-openshift.yaml
    │   ├── backup-secret-openshift.yaml
    │   ├── namespace-backup-system.yaml
    │   ├── monitoring-openshift.yaml
    │   └── deployment-instructions.md
    ├── monitoring/       # Monitoring manifests
    │   ├── monitoring.yaml
    │   └── prometheus-monitoring.yaml
    └── shared/           # Shared/common manifests
        ├── namespace.yaml
        ├── rbac.yaml
        └── security-policies.yaml
```

## 🔧 Configuration Reference

### Environment Variables

**Backup Service:**
- `CLUSTER_NAME`: Unique cluster identifier
- `CLUSTER_DOMAIN`: Cluster domain (default: cluster.local)
- `MINIO_ENDPOINT`: MinIO server endpoint
- `MINIO_ACCESS_KEY`: MinIO access credentials
- `MINIO_SECRET_KEY`: MinIO secret credentials
- `MINIO_BUCKET`: Target bucket name
- `MINIO_USE_SSL`: Enable SSL/TLS (default: true)
- `LOG_LEVEL`: Logging verbosity (debug, info, warn, error)
- `ENABLE_CLEANUP`: Enable automatic cleanup (default: true)
- `RETENTION_DAYS`: Number of days to retain backups (default: 7)
- `CLEANUP_ON_STARTUP`: Perform cleanup on startup vs after backup (default: false)

**Git-Sync Service:**
- `GIT_REPOSITORY`: Git repository URL
- `GIT_BRANCH`: Target branch (default: main)
- `GIT_USERNAME`: Git username for commits
- `GIT_EMAIL`: Git email for commits
- `GIT_TOKEN`: Git authentication token
- `WORK_DIR`: Working directory path
- `LOG_LEVEL`: Logging verbosity (debug, info, warn, error)

## 🐛 Troubleshooting

### Common Issues

1. **Permission Denied Errors**
   - Verify RBAC configuration
   - Check service account permissions
   - Ensure proper namespace access

2. **MinIO Connection Issues**
   - Validate MinIO credentials
   - Check network connectivity
   - Verify bucket existence

3. **Git Authentication Failures**
   - Confirm git token validity
   - Check repository permissions
   - Verify SSH key configuration

### Debug Mode

Enable detailed logging for troubleshooting:

```bash
kubectl set env cronjob/backup-cronjob LOG_LEVEL=debug
kubectl set env cronjob/git-sync-cronjob LOG_LEVEL=debug
```

## 📞 Support

For issues and questions:
- Check the troubleshooting section above
- Review structured logs for detailed error context
- Monitor Prometheus metrics for operational insights
- Validate RBAC and security configurations

## 🏆 Production Ready

This backup system is production-ready with:
- ✅ Enterprise-grade structured logging
- ✅ Comprehensive monitoring and metrics
- ✅ Security hardening and RBAC
- ✅ Multi-cluster coordination
- ✅ Incremental git synchronization
- ✅ OpenShift compatibility
- ✅ Flexible resource filtering
- ✅ Error handling and retry mechanisms
- ✅ Automatic backup cleanup and retention management