# OpenShift Cluster Backup System

Production-grade backup solution for OpenShift clusters that automatically exports Kubernetes resources to MinIO and synchronizes them to Git repositories.

## Overview

This system consists of two main components:

1. **Cluster Backup CronJob**: Runs on each OpenShift cluster to export resources
2. **Git Synchronization CronJob**: Runs centrally to consolidate backups and push to Git

## Architecture

```
[OpenShift Cluster 1] ---> [MinIO] <--- [Git Sync Job] ---> [Git Repository]
[OpenShift Cluster 2] ---> [MinIO] <--- [Git Sync Job] ---> [Git Repository]
[OpenShift Cluster N] ---> [MinIO] <--- [Git Sync Job] ---> [Git Repository]
```

## Security Features

- 🔒 Non-root containers with read-only root filesystem
- 🔑 RBAC with least privilege access
- 🌐 Network policies for traffic restriction
- 🛡️ Security context constraints
- 📊 Resource quotas and limits
- 🔐 Encrypted communication (TLS)
- 🚫 Automatic secret redaction from backups

## Quick Start

### Prerequisites

- OpenShift 4.x cluster
- MinIO server (external)
- Git repository for backup storage
- Prometheus operator (for monitoring)

### 1. Create Namespace

```bash
oc create namespace openshift-backup
oc label namespace openshift-backup name=openshift-backup
```

### 2. Configure Secrets

Edit `secret-template.yaml` with your credentials:

```bash
# Base64 encode your MinIO credentials
echo -n "your-minio-access-key" | base64
echo -n "your-minio-secret-key" | base64

# For SSH key (if using SSH for Git)
cat ~/.ssh/id_rsa | base64 -w 0
```

Apply the secrets:
```bash
oc apply -f secret-template.yaml
```

### 3. Configure Settings

Edit `configmap.yaml` with your cluster-specific settings:
- `cluster-domain`: Your cluster domain
- `cluster-name`: Unique cluster identifier
- `minio-endpoint`: MinIO server endpoint
- `git-repository`: Git repository URL

```bash
oc apply -f configmap.yaml
```

### 4. Deploy RBAC and Security

```bash
oc apply -f rbac.yaml
oc apply -f security-policies.yaml
```

### 5. Deploy Backup CronJob

```bash
oc apply -f backup-cronjob.yaml
```

### 6. Deploy Git Sync (Central Location)

```bash
oc apply -f git-sync-cronjob.yaml
```

### 7. Enable Monitoring

```bash
oc apply -f monitoring.yaml
```

## Configuration

### Backup Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `CLUSTER_DOMAIN` | Cluster domain name | `cluster.local` |
| `CLUSTER_NAME` | Unique cluster identifier | `openshift-cluster` |
| `MINIO_ENDPOINT` | MinIO server endpoint | Required |
| `MINIO_BUCKET` | MinIO bucket name | `cluster-backups` |
| `EXCLUDE_NAMESPACES` | Additional namespaces to exclude | Empty |

### Git Sync Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `GIT_REPOSITORY` | Git repository URL | Required |
| `GIT_BRANCH` | Target branch | `main` |
| `GIT_USERNAME` | Git commit author | `cluster-backup` |
| `GIT_EMAIL` | Git commit email | `cluster-backup@example.com` |

## Backup Structure

Backups are organized in MinIO with the following structure:

```
cluster-backups/
├── cluster.local/
│   ├── production-cluster/
│   │   ├── default/
│   │   │   ├── deployment/
│   │   │   │   ├── app1.yaml
│   │   │   │   └── app2.yaml
│   │   │   └── service/
│   │   │       ├── app1-svc.yaml
│   │   │       └── app2-svc.yaml
│   │   └── myapp-namespace/
│   │       └── ...
│   └── staging-cluster/
│       └── ...
└── other.domain/
    └── ...
```

## Monitoring and Alerting

### Available Metrics

- `cluster_backup_resources_total`: Total resources backed up
- `cluster_backup_duration_seconds`: Backup operation duration
- `cluster_backup_errors_total`: Total backup errors
- `cluster_backup_last_success_timestamp`: Last successful backup timestamp
- `git_sync_duration_seconds`: Git sync operation duration
- `git_sync_errors_total`: Total git sync errors
- `git_sync_last_success_timestamp`: Last successful git sync timestamp

### Alerts

- **ClusterBackupFailed**: Backup job failures
- **ClusterBackupNotRunning**: Backup hasn't run in 24 hours
- **GitSyncFailed**: Git sync failures
- **GitSyncNotRunning**: Git sync hasn't run in 24 hours
- **BackupDurationHigh**: Backup taking longer than 1 hour

### Grafana Dashboard

The system includes a pre-configured Grafana dashboard showing:
- Backup success rates
- Operation durations
- Error rates
- Last successful operations

## Security Considerations

### Excluded Resources

The following resource types are automatically excluded from backups:
- `events`
- `componentstatuses`
- `endpoints`
- `limitranges`
- `persistentvolumes`
- `resourcequotas`
- `nodes`
- `bindings`
- `replicationcontrollers`

### Excluded Namespaces

System namespaces are automatically excluded:
- All `kube-*` namespaces
- All `openshift-*` namespaces (except custom ones)

### Metadata Cleaning

The following metadata fields are automatically removed from backups:
- `status` (entire section)
- `metadata.uid`
- `metadata.resourceVersion`
- `metadata.generation`
- `metadata.creationTimestamp`
- `metadata.managedFields`
- `metadata.selfLink`
- Sensitive annotations

## Troubleshooting

### Common Issues

1. **Backup Job Fails with Permission Errors**
   ```bash
   oc logs -n openshift-backup job/cluster-backup-xxxxx
   oc describe clusterrolebinding cluster-backup-binding
   ```

2. **MinIO Connection Issues**
   ```bash
   # Test MinIO connectivity
   oc run minio-test --image=curlimages/curl --rm -it --restart=Never -- \
     curl -k https://your-minio-endpoint:9000/minio/health/live
   ```

3. **Git Sync SSH Issues**
   ```bash
   # Check SSH key permissions
   oc exec -n openshift-backup deployment/git-sync -- \
     ls -la /etc/git-secrets/
   ```

4. **High Memory Usage**
   - Increase memory limits in CronJob
   - Reduce batch size in configuration
   - Add more excluded namespaces

### Debug Mode

Enable debug logging by setting environment variable:
```yaml
- name: LOG_LEVEL
  value: "debug"
```

## Build and Deploy

### Building Container Images

```bash
# Build the container image
podman build -t your-registry/cluster-backup:latest .

# Push to registry
podman push your-registry/cluster-backup:latest
```

### Update CronJob Image

```bash
oc patch cronjob cluster-backup -n openshift-backup -p \
  '{"spec":{"jobTemplate":{"spec":{"template":{"spec":{"containers":[{"name":"backup","image":"your-registry/cluster-backup:latest"}]}}}}}}'
```

## Backup Verification

### Manual Backup Test

```bash
# Create a test job from the CronJob
oc create job manual-backup --from=cronjob/cluster-backup -n openshift-backup

# Watch the job progress
oc logs -f job/manual-backup -n openshift-backup
```

### Restore Test

To verify backups are valid, periodically test restoration:

```bash
# Download a sample backup
kubectl apply -f downloaded-backup.yaml --dry-run=client -o yaml
```

## Maintenance

### Backup Retention

MinIO lifecycle policies should be configured to manage backup retention:

```json
{
  "Rules": [
    {
      "ID": "backup-retention",
      "Status": "Enabled",
      "Filter": {
        "Prefix": "cluster-backups/"
      },
      "Expiration": {
        "Days": 90
      }
    }
  ]
}
```

### Git Repository Cleanup

Implement Git hooks or actions to:
- Compress old backups
- Remove outdated cluster data
- Maintain repository size

## Support

For issues and questions:
1. Check the logs: `oc logs -n openshift-backup <pod-name>`
2. Review metrics in Grafana dashboard
3. Check Prometheus alerts
4. Verify MinIO and Git connectivity

## License

This project is licensed under the MIT License - see the LICENSE file for details.