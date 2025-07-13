# OpenShift Deployment Instructions

This directory contains OpenShift-specific manifests for deploying the cluster backup system using the **default ServiceAccount** instead of creating a custom ServiceAccount.

## 🎯 Overview

The OpenShift version is designed to work with OpenShift's security model and uses the default ServiceAccount in the backup-system namespace. This simplifies deployment while maintaining security through SecurityContextConstraints (SCC).

## 📋 Prerequisites

- OpenShift 4.8+ cluster
- Cluster admin privileges
- MinIO server accessible from the cluster
- Container images built and pushed to an accessible registry

## 🚀 Quick Deployment

### 1. Create Namespace
```bash
oc apply -f namespace-backup-system.yaml
```

### 2. Apply Security Context Constraints
```bash
oc apply -f scc-backup.yaml
```

### 3. Apply RBAC Configuration
```bash
oc apply -f rbac-default-sa.yaml
```

### 4. Configure Secrets
```bash
# Edit the secret with your actual values
vi backup-secret-openshift.yaml

# Apply the secret
oc apply -f backup-secret-openshift.yaml
```

### 5. Apply Configuration
```bash
oc apply -f configmap-openshift.yaml
```

### 6. Deploy Backup CronJob
```bash
# Update image registry in the YAML file first
vi backup-cronjob-default-sa.yaml

# Apply the CronJob
oc apply -f backup-cronjob-default-sa.yaml
```

### 7. Deploy Monitoring (Optional)
```bash
oc apply -f monitoring-openshift.yaml
```

## 🔧 Configuration

### Required Configuration Changes

Before deploying, update these values in the configuration files:

#### 1. backup-secret-openshift.yaml
```yaml
stringData:
  cluster-name: "your-openshift-cluster"
  minio-endpoint: "your-minio-endpoint:9000"
  minio-access-key: "your-access-key"
  minio-secret-key: "your-secret-key"
```

#### 2. backup-cronjob-default-sa.yaml
```yaml
containers:
- name: backup
  image: your-registry.com/cluster-backup:latest  # Update this
```

### Custom Resource Selection

The ConfigMap (`configmap-openshift.yaml`) is pre-configured with OpenShift-specific settings:

- **Includes**: OpenShift resources (Routes, BuildConfigs, ImageStreams, etc.)
- **Excludes**: OpenShift system namespaces (openshift-*, kube-*)
- **CRDs**: Common OpenShift and application CRDs

## 🔒 Security Features

### SecurityContextConstraints (SCC)

The custom SCC (`scc-backup.yaml`) provides:
- Non-root execution
- Read-only root filesystem
- Dropped capabilities
- Restricted volume types
- Specific UID/GID ranges

### RBAC Configuration

The ClusterRole grants read-only access to:
- Standard Kubernetes resources
- OpenShift-specific resources
- Custom Resource Definitions
- Configuration and operator resources

### Default ServiceAccount Usage

Benefits of using the default ServiceAccount:
- Simplified deployment (no custom SA creation)
- Automatic OpenShift SCC assignment
- Compatible with existing security policies
- Easier integration with OpenShift RBAC

## 📊 Monitoring

### OpenShift Monitoring Integration

The monitoring configuration includes:
- **ServiceMonitor**: Automatic Prometheus scraping
- **PrometheusRule**: OpenShift-specific alerts
- **Metrics Service**: Endpoint for metrics collection

### Available Metrics

- `cluster_backup_duration_seconds`: Backup operation duration
- `cluster_backup_errors_total`: Total backup errors
- `cluster_backup_resources_total`: Total resources backed up
- `cluster_backup_last_success_timestamp`: Last successful backup
- `cluster_backup_cleanup_*`: Cleanup operation metrics

### Alerts

- **OpenShiftBackupJobFailed**: Critical alert for backup failures
- **OpenShiftBackupJobNotRunning**: Warning for missing backups
- **OpenShiftBackupHighDuration**: Warning for slow backups
- **OpenShiftBackupCleanupFailed**: Warning for cleanup failures

## 🐛 Troubleshooting

### Common Issues

#### 1. Permission Denied Errors
```bash
# Check if SCC is applied
oc get scc backup-scc

# Verify ClusterRoleBinding
oc get clusterrolebinding cluster-backup-default-sa-binding

# Check default SA permissions
oc auth can-i list pods --as=system:serviceaccount:backup-system:default
```

#### 2. Pod Security Issues
```bash
# Check pod security context
oc describe pod <backup-pod-name>

# Verify SCC assignment
oc get pod <backup-pod-name> -o yaml | grep scc
```

#### 3. Image Pull Issues
```bash
# Check if image exists and is accessible
oc describe pod <backup-pod-name>

# Verify registry access
oc get events -n backup-system
```

### Debug Commands

```bash
# View backup job logs
oc logs -f job/<job-name> -n backup-system

# Check CronJob status
oc get cronjob cluster-backup-default-sa -n backup-system

# View recent jobs
oc get jobs -n backup-system

# Check metrics endpoint
oc port-forward service/cluster-backup-metrics 8080:8080 -n backup-system
curl http://localhost:8080/metrics
```

## 🔄 Updates and Maintenance

### Updating Configuration
```bash
# Update ConfigMap
oc apply -f configmap-openshift.yaml

# Update Secret (be careful with credentials)
oc apply -f backup-secret-openshift.yaml

# Restart CronJob to pick up changes
oc delete job -l app=cluster-backup -n backup-system
```

### Scaling and Performance
```bash
# Adjust resource limits in backup-cronjob-default-sa.yaml
resources:
  limits:
    cpu: 1000m      # Increase for better performance
    memory: 1Gi     # Increase for large clusters
  requests:
    cpu: 200m
    memory: 512Mi
```

## 📦 Integration with OpenShift Features

### OpenShift Routes (Optional)
If you need external access to metrics:

```yaml
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: backup-metrics
  namespace: backup-system
spec:
  to:
    kind: Service
    name: cluster-backup-metrics
  port:
    targetPort: metrics
  tls:
    termination: edge
```

### OpenShift Operators
This backup system is compatible with:
- Prometheus Operator for monitoring
- Grafana Operator for dashboards
- OpenShift GitOps for deployment automation

## ✅ Verification

After deployment, verify the system is working:

```bash
# Check namespace
oc get namespace backup-system

# Check CronJob
oc get cronjob -n backup-system

# Verify RBAC
oc auth can-i list deployments --as=system:serviceaccount:backup-system:default

# Check metrics
oc get servicemonitor -n backup-system

# Trigger manual backup (optional)
oc create job manual-backup --from=cronjob/cluster-backup-default-sa -n backup-system
```

The OpenShift version provides the same powerful backup capabilities as the standard Kubernetes version while being optimized for OpenShift's security model and operational patterns.