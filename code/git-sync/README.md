# Git-Sync Service

Centralized git synchronization service for multi-cluster Kubernetes backup system with incremental change detection and comprehensive operational logging.

## 🎯 Overview

The git-sync service is a Go-based application that runs as a CronJob on a central cluster to download backup files from MinIO object storage and synchronize them to a Git repository. It supports incremental synchronization, change detection, and provides detailed operational metrics.

## 🏗 Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   Git-Sync Service                          │
│                                                             │
│  ┌─────────────────┐    ┌─────────────────┐                │
│  │   MinIO Client  │    │   Git Client    │                │
│  │  - Download     │    │  - Clone/Pull   │                │
│  │  - Multi-Cluster│    │  - Commit       │                │
│  │  - Change Detect│    │  - Push         │                │
│  └─────────────────┘    └─────────────────┘                │
│           │                       │                        │
│           └───────┬───────────────┘                        │
│                   │                                        │
│  ┌─────────────────▼─────────────────┐                     │
│  │        Sync Engine                │                     │
│  │  - Incremental Download           │                     │
│  │  - Change Detection               │                     │
│  │  - Git Operations                 │                     │
│  │  - Conflict Resolution            │                     │
│  └─────────────────┬─────────────────┘                     │
│                    │                                       │
│  ┌─────────────────▼─────────────────┐                     │
│  │     Monitoring & Logging          │                     │
│  │  - Prometheus Metrics             │                     │
│  │  - Structured Logging             │                     │
│  │  - Operation Tracking             │                     │
│  └─────────────────────────────────────┘                     │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
           ┌─────────────────────────────────┐
           │        Git Repository           │
           │   (Version Controlled Backups)  │
           │                                 │
           │  clusterbackup/                 │
           │  ├── production-east/           │
           │  │   ├── default/               │
           │  │   └── kube-system/           │
           │  ├── production-west/           │
           │  │   ├── default/               │
           │  │   └── kube-system/           │
           │  └── staging/                   │
           │      ├── default/               │
           │      └── kube-system/           │
           └─────────────────────────────────┘
```

## 📋 Core Components

### 1. Configuration Management

**Environment Variables:**
```go
type GitSyncConfig struct {
    MinIOEndpoint   string        // MinIO server endpoint
    MinIOAccessKey  string        // MinIO access key
    MinIOSecretKey  string        // MinIO secret key
    MinIOBucket     string        // MinIO bucket name
    MinIOUseSSL     bool          // Use SSL for MinIO
    GitRepository   string        // Git repository URL
    GitBranch       string        // Target git branch
    GitUsername     string        // Git commit username
    GitEmail        string        // Git commit email
    GitToken        string        // Git authentication token
    SSHKeyPath      string        // SSH private key path (alternative)
    WorkDir         string        // Working directory path
    RetryAttempts   int           // Number of retry attempts
    RetryDelay      time.Duration // Delay between retries
}
```

### 2. MinIO Integration

**Download Process:**
1. **Bucket Discovery**: Lists all objects in the MinIO bucket
2. **Multi-Cluster Detection**: Identifies different cluster directories
3. **Incremental Download**: Downloads only changed files (optional)
4. **File Organization**: Maintains the same directory structure locally

**Download Modes:**
- **Full Download**: Downloads all backup files from MinIO
- **Download-Only**: Skips git operations (useful for testing)
- **Incremental**: Downloads only files newer than last sync (planned feature)

### 3. Git Operations Engine

**Git Workflow:**
1. **Repository Setup**: Clones repository or updates existing clone
2. **Change Detection**: Compares local files with git working tree
3. **Staging**: Adds new and modified files to git index
4. **Commit Creation**: Creates commit with comprehensive metadata
5. **Push Operation**: Pushes changes to remote repository

**Authentication Methods:**
- **HTTPS with Token**: Uses personal access token for GitHub/GitLab
- **SSH Keys**: Uses SSH private key for git operations (planned)

### 4. Change Detection System

**Change Types Detected:**
- **New Files**: Files added since last sync
- **Modified Files**: Files with different content
- **Deleted Files**: Files removed from MinIO but present in git
- **Directory Changes**: New or removed cluster directories

**Commit Message Format:**
```
Automated backup sync - 2025-07-12 22:06:58 UTC

Clusters processed: 3
- production-east: 245 files
- production-west: 189 files  
- staging: 67 files

Total files: 501
Changes: 23 new, 15 modified, 2 deleted
```

### 5. Structured Logging System

**Log Entry Format:**
```json
{
  "timestamp": "2025-07-12T22:06:58Z",
  "level": "info",
  "component": "git-sync",
  "operation": "sync_complete",
  "message": "Git synchronization completed successfully",
  "data": {
    "clusters_processed": 3,
    "files_downloaded": 501,
    "files_changed": 40,
    "git_commit": "abc123def456",
    "duration_ms": 45678.90
  },
  "duration_ms": 45678.90
}
```

**Key Operations Tracked:**
- **MinIO Operations**: Download progress, file counts, errors
- **Git Operations**: Clone, commit, push status and timing
- **Change Detection**: File comparison results and statistics
- **Error Handling**: Detailed error context and recovery attempts

### 6. Prometheus Metrics

**Exposed Metrics:**
```go
type GitSyncMetrics struct {
    SyncDuration     prometheus.Histogram  // Sync operation duration
    SyncErrors       prometheus.Counter    // Total sync errors
    FilesProcessed   prometheus.Counter    // Total files processed
    LastSyncTime     prometheus.Gauge      // Last successful sync timestamp
    ClustersBackedUp prometheus.Gauge      // Number of clusters processed
}
```

**Additional Metrics:**
- `git_sync_duration_seconds`: Complete sync operation duration
- `git_sync_errors_total`: Total number of sync failures
- `git_sync_files_processed_total`: Cumulative files processed
- `git_sync_last_success_timestamp`: Unix timestamp of last success
- `git_sync_clusters_backed_up`: Number of clusters in last sync

**Metrics Endpoint**: `:8080/metrics`

## 🚀 Usage

### 1. Environment Configuration

**Required Environment Variables:**
```bash
# MinIO Configuration
MINIO_ENDPOINT=minio.example.com:9000
MINIO_ACCESS_KEY=your-access-key
MINIO_SECRET_KEY=your-secret-key
MINIO_BUCKET=cluster-backups
MINIO_USE_SSL=true

# Git Configuration
GIT_REPOSITORY=https://github.com/your-org/cluster-backups.git
GIT_BRANCH=main
GIT_USERNAME=cluster-backup-bot
GIT_EMAIL=backup@yourcompany.com
GIT_TOKEN=ghp_your_github_token_here

# Working Directory
WORK_DIR=/workspace

# Retry Configuration
RETRY_ATTEMPTS=3
RETRY_DELAY=5s

# Logging
LOG_LEVEL=info
```

### 2. Deployment Modes

**Central Deployment (Recommended):**
Deploy git-sync on only one cluster to avoid conflicts:
```bash
# Deploy on central/primary cluster only
kubectl apply -f k8s/git-sync/git-sync-cronjob-central.yaml
```

**Distributed Deployment:**
Deploy on multiple clusters with different branches:
```yaml
# Cluster A
GIT_BRANCH: "backups/production-east"

# Cluster B  
GIT_BRANCH: "backups/production-west"
```

### 3. Authentication Setup

**GitHub Token Authentication:**
```bash
# Create GitHub personal access token with repo permissions
# Set token in secret
kubectl create secret generic git-sync-secrets \
  --from-literal=git-token=ghp_your_token_here \
  --namespace=backup-system
```

**SSH Key Authentication (Future):**
```bash
# Generate SSH key pair
ssh-keygen -t rsa -b 4096 -f git-sync-key

# Add public key to Git provider
# Store private key in secret
kubectl create secret generic git-sync-secrets \
  --from-file=ssh-private-key=git-sync-key \
  --namespace=backup-system
```

## 🔧 Advanced Configuration

### 1. Working Directory Structure

The service creates and manages this directory structure:
```
/workspace/
├── .git/                 # Git repository metadata
├── .gitconfig           # Local git configuration
├── clusterbackup/       # Downloaded backup files
│   ├── production-east/
│   │   ├── default/
│   │   └── kube-system/
│   ├── production-west/
│   │   ├── default/
│   │   └── kube-system/
│   └── staging/
│       ├── default/
│       └── kube-system/
└── sync-metadata.json   # Sync operation metadata
```

### 2. Download-Only Mode

For testing or when git repository is not configured:
```bash
# Set empty git repository to enable download-only mode
GIT_REPOSITORY=""
```

The service will:
- Download all files from MinIO
- Skip git operations
- Log download statistics
- Expose download metrics

### 3. Custom Scheduling

**High-Frequency Sync:**
```yaml
spec:
  schedule: "*/15 * * * *"  # Every 15 minutes
```

**Low-Frequency Sync:**
```yaml
spec:
  schedule: "0 4 * * *"     # Daily at 4 AM
```

**Business Hours Only:**
```yaml
spec:
  schedule: "0 9-17 * * 1-5"  # Weekdays 9 AM to 5 PM
```

## 📊 Monitoring & Observability

### Log Analysis Examples

**Track Sync Performance:**
```bash
kubectl logs -f deployment/git-sync | jq 'select(.operation=="sync_complete") | .data'
```

**Monitor Git Operations:**
```bash
kubectl logs deployment/git-sync | jq 'select(.operation | startswith("git_"))'
```

**Find Sync Errors:**
```bash
kubectl logs deployment/git-sync | jq 'select(.level=="error")'
```

### Prometheus Queries

**Sync Success Rate:**
```promql
(1 - rate(git_sync_errors_total[5m]) / rate(git_sync_duration_seconds_count[5m])) * 100
```

**Files Processed Per Hour:**
```promql
increase(git_sync_files_processed_total[1h])
```

**Average Sync Duration:**
```promql
rate(git_sync_duration_seconds_sum[5m]) / rate(git_sync_duration_seconds_count[5m])
```

**Time Since Last Sync:**
```promql
time() - git_sync_last_success_timestamp
```

### Health Checks

**Health Check Endpoint:**
```bash
# Command line health check
./git-sync --health-check

# HTTP health check (if implemented)
curl http://localhost:8080/health
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
**Solution:**
- Verify MinIO endpoint and credentials
- Check network connectivity from git-sync pod
- Confirm bucket exists and is accessible

**2. Git Authentication Failures**
```json
{
  "level": "error", 
  "operation": "git_push",
  "message": "Failed to push to repository",
  "error": "authentication failed"
}
```
**Solution:**
- Verify git token is valid and has repo permissions
- Check repository URL is correct
- Ensure token is properly stored in secret

**3. Git Merge Conflicts**
```json
{
  "level": "error",
  "operation": "git_commit",
  "message": "Git conflict detected",
  "error": "merge conflict in file.yaml"
}
```
**Solution:**
- Ensure only one git-sync instance per repository/branch
- Use different branches for different clusters
- Manual conflict resolution may be needed

**4. Working Directory Issues**
```json
{
  "level": "error",
  "operation": "setup_workdir",
  "message": "Failed to create working directory",
  "error": "permission denied"
}
```
**Solution:**
- Check volume mount permissions
- Verify container runs with correct user ID
- Ensure sufficient disk space

### Debug Mode

Enable detailed logging:
```bash
kubectl set env cronjob/git-sync-cronjob LOG_LEVEL=debug
```

Debug mode provides:
- Detailed git command output
- File-by-file download progress
- Git diff information
- MinIO operation details

## 🔒 Security Considerations

### Authentication Security

**Token Management:**
- Git tokens stored in Kubernetes secrets
- Tokens should have minimal required permissions
- Regular token rotation recommended

**Container Security:**
- Runs as non-root user
- Read-only root filesystem where possible
- No privileged capabilities required

### Git Repository Security

**Access Control:**
- Use private repositories for backup data
- Implement branch protection rules
- Enable audit logging for repository access

**Data Sensitivity:**
- Backup data may contain sensitive information
- Consider repository encryption
- Implement proper access controls

## 📦 Building and Deployment

### Build Container Image

```bash
cd code/git-sync
docker build -t your-registry/git-sync:latest .
docker push your-registry/git-sync:latest
```

### Update Deployment

```bash
kubectl set image cronjob/git-sync-cronjob git-sync=your-registry/git-sync:latest
```

### Local Development

```bash
# Set environment variables
export MINIO_ENDPOINT=localhost:9000
export GIT_REPOSITORY=https://github.com/your-org/test-backups.git
# ... other vars

# Run locally
go run main.go
```

## 🔄 Integration

The git-sync service integrates with:

**Upstream Services:**
- **Backup Services**: Consumes backup files from multiple clusters
- **MinIO Storage**: Downloads organized backup data

**Downstream Services:**
- **Git Repositories**: Provides version-controlled backup history
- **CI/CD Pipelines**: Triggers on backup updates
- **Monitoring Systems**: Exposes operational metrics

**Monitoring Integration:**
- **Prometheus**: Metrics collection and alerting
- **Grafana**: Performance visualization  
- **AlertManager**: Failure notifications

## 📈 Performance Optimization

### Resource Allocation

**CPU Requirements:**
- Base: 100m CPU
- During sync: 200-500m CPU
- Large clusters: 1000m CPU

**Memory Requirements:**
- Base: 128Mi RAM
- During sync: 256-512Mi RAM
- Large clusters: 1Gi RAM

**Storage Requirements:**
- Work directory: 2-5Gi depending on cluster size
- Git history: Additional storage for commit history

### Optimization Strategies

**Concurrent Downloads:**
```go
// Configure concurrent MinIO downloads
semaphore := make(chan struct{}, 10) // 10 concurrent downloads
```

**Git Shallow Clones:**
```bash
git clone --depth 1 <repository>  # Faster initial clone
```

**Selective Sync:**
```yaml
# Sync only specific clusters or namespaces
# (Future feature)
```

This git-sync service provides robust, centralized synchronization of multi-cluster backup data with comprehensive monitoring and operational visibility.