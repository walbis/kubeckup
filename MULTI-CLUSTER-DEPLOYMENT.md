# Multi-Cluster Deployment Guide

This guide explains how to deploy the KubeBackup system across multiple clusters with centralized Git synchronization.

## Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Cluster 1     │    │   Cluster 2     │    │   Cluster 3     │
│  (Production)   │    │   (Staging)     │    │ (Development)   │
│                 │    │                 │    │                 │
│  ┌───────────┐  │    │  ┌───────────┐  │    │  ┌───────────┐  │
│  │ Backup    │  │    │  │ Backup    │  │    │  │ Backup    │  │
│  │ CronJob   │  │    │  │ CronJob   │  │    │  │ CronJob   │  │
│  └─────┬─────┘  │    │  └─────┬─────┘  │    │  └─────┬─────┘  │
└────────┼────────┘    └────────┼────────┘    └────────┼────────┘
         │                      │                      │
         └──────────────────────┼──────────────────────┘
                                │
                    ┌───────────▼───────────┐
                    │      MinIO Storage    │
                    │                       │
                    │ clusterbackup/        │
                    │ ├── production-cluster│
                    │ ├── staging-cluster   │
                    │ └── dev-cluster       │
                    └───────────┬───────────┘
                                │
                    ┌───────────▼───────────┐
                    │  Central Git-Sync     │
                    │     (Cluster 1)       │
                    │                       │
                    │  ┌─────────────────┐  │
                    │  │ Git-Sync CronJob│  │
                    │  │ (Incremental)   │  │
                    │  └─────────────────┘  │
                    └───────────┬───────────┘
                                │
                    ┌───────────▼───────────┐
                    │   Git Repository      │
                    │                       │
                    │ All clusters          │
                    │ consolidated          │
                    └───────────────────────┘
```

## Deployment Steps

### 1. Prepare Common Resources

First, ensure you have the required images built and available:

```bash
# Build backup image
docker build -f backup-dockerfile -t your-registry/cluster-backup:latest .
docker push your-registry/cluster-backup:latest

# Build git-sync image
docker build -f git-sync-dockerfile -t your-registry/git-sync:latest .
docker push your-registry/git-sync:latest
```

### 2. Deploy Backup on ALL Clusters

Deploy backup components on every cluster you want to backup:

```bash
# On each cluster, run these commands:

# 1. Create namespace and RBAC
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/security-policies.yaml

# 2. Create backup secrets (customize MinIO credentials)
kubectl apply -f k8s/backup-secret.yaml

# 3. Deploy backup CronJob (CUSTOMIZE CLUSTER_NAME for each cluster)
# Edit k8s/backup-cronjob-multicluster.yaml:
# - Set CLUSTER_NAME to unique value for each cluster
# - Examples: "production-cluster", "staging-cluster", "dev-cluster"
kubectl apply -f k8s/backup-cronjob-multicluster.yaml
```

**Important**: Customize `CLUSTER_NAME` environment variable for each cluster:

**Cluster 1 (Production)**:
```yaml
- name: CLUSTER_NAME
  value: "production-cluster"
```

**Cluster 2 (Staging)**:
```yaml
- name: CLUSTER_NAME
  value: "staging-cluster"
```

**Cluster 3 (Development)**:
```yaml
- name: CLUSTER_NAME
  value: "dev-cluster"
```

### 3. Deploy Git-Sync on ONE Cluster Only

Choose one cluster (preferably your management/central cluster) for Git synchronization:

```bash
# Only on the central cluster:

# 1. Create git-sync secrets
kubectl apply -f k8s/git-sync-secret.yaml

# 2. Deploy central git-sync CronJob
kubectl apply -f k8s/git-sync-cronjob-central.yaml

# 3. Enable monitoring (optional)
kubectl apply -f k8s/monitoring.yaml
```

### 4. Verify Deployment

Check that backups are working on all clusters:

```bash
# On each cluster:
kubectl get cronjobs -n backup-system
kubectl get jobs -n backup-system
kubectl logs -n backup-system -l app=cluster-backup

# On central cluster only:
kubectl get cronjobs -n backup-system
kubectl logs -n backup-system -l app=git-sync
```

## MinIO Folder Structure

With this setup, your MinIO bucket will have the following structure:

```
cluster-backups/
└── clusterbackup/
    ├── production-cluster/
    │   ├── default/
    │   │   ├── deployment/
    │   │   │   ├── app1.yaml
    │   │   │   └── app2.yaml
    │   │   └── service/
    │   │       ├── app1-svc.yaml
    │   │       └── app2-svc.yaml
    │   ├── kube-system/
    │   └── production-apps/
    ├── staging-cluster/
    │   ├── default/
    │   ├── staging-apps/
    │   └── test-apps/
    └── dev-cluster/
        ├── default/
        ├── development/
        └── experimental/
```

## Git Repository Structure

The final Git repository will contain all clusters:

```
repository/
└── clusterbackup/
    ├── production-cluster/
    │   ├── default/
    │   ├── production-apps/
    │   └── kube-system/
    ├── staging-cluster/
    │   ├── default/
    │   ├── staging-apps/
    │   └── test-apps/
    └── dev-cluster/
        ├── default/
        ├── development/
        └── experimental/
```

## Configuration Customization

### Backup Filtering Per Cluster

You can customize backup filtering for each cluster using ConfigMaps:

```yaml
# Example: Different filtering for production vs development
apiVersion: v1
kind: ConfigMap
metadata:
  name: backup-config
  namespace: backup-system
data:
  filtering-mode: "blacklist"
  
  # Production: Exclude test namespaces
  exclude-namespaces: |
    kube-system,test-*,dev-*
  
  # Development: Include everything except system
  exclude-namespaces: |
    kube-system,kube-public
```

### Schedule Coordination

Recommended schedule to avoid conflicts:

```yaml
# Backup schedules (staggered across clusters)
# Cluster 1: "0 2 * * *"   # 2:00 AM
# Cluster 2: "15 2 * * *"  # 2:15 AM  
# Cluster 3: "30 2 * * *"  # 2:30 AM

# Git-sync schedule (after all backups)
# Central: "0 3 * * *"     # 3:00 AM
```

## Monitoring and Troubleshooting

### Check Backup Status

```bash
# View all backup jobs across clusters
kubectl get jobs -n backup-system -o wide

# Check backup metrics
kubectl port-forward -n backup-system svc/backup-metrics 8080:8080
curl http://localhost:8080/metrics
```

### Check Git-Sync Status

```bash
# View git-sync logs
kubectl logs -n backup-system -l app=git-sync --tail=100

# Check git-sync metrics  
kubectl port-forward -n backup-system svc/git-sync-metrics 8080:8080
curl http://localhost:8080/metrics
```

### Troubleshooting Common Issues

1. **Clusters backing up to wrong paths**:
   - Verify `CLUSTER_NAME` environment variable is unique per cluster
   - Check MinIO bucket structure

2. **Git-sync not finding backups**:
   - Ensure git-sync has same MinIO credentials as backup jobs
   - Check if `clusterbackup/` prefix is present in MinIO

3. **Multiple git-sync jobs conflicting**:
   - Ensure git-sync CronJob is deployed on only ONE cluster
   - Use `concurrencyPolicy: Forbid` in CronJob spec

4. **No incremental changes pushed**:
   - Git-sync only pushes when changes are detected
   - Check git logs in git-sync pod for change statistics

## Benefits of This Architecture

✅ **Scalable**: Add new clusters by just deploying backup CronJob
✅ **Centralized**: All cluster backups in one Git repository  
✅ **Efficient**: Only changed files are pushed to Git
✅ **Reliable**: Each cluster backs up independently
✅ **Organized**: Clear folder structure per cluster
✅ **Incremental**: Git-sync does clone + diff + push for minimal overhead

## Security Considerations

- Each cluster only needs MinIO write access for its own path
- Git-sync cluster needs MinIO read access to all paths
- Git credentials are only stored on central cluster
- RBAC follows least privilege principle
- All containers run non-root with read-only filesystems