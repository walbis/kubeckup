#!/bin/bash

# OpenShift Cluster Backup System Deployment Script
set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
CLUSTER_NAME="${CLUSTER_NAME:-openshift-cluster}"
CLUSTER_DOMAIN="${CLUSTER_DOMAIN:-cluster.local}"
MINIO_ENDPOINT="${MINIO_ENDPOINT:-}"
MINIO_BUCKET="${MINIO_BUCKET:-cluster-backups}"
GIT_REPOSITORY="${GIT_REPOSITORY:-}"
NAMESPACE="openshift-backup"
MODE="${MODE:-backup}"  # backup or git-sync

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if oc command exists
    if ! command -v oc &> /dev/null; then
        log_error "oc command not found. Please install OpenShift CLI."
        exit 1
    fi
    
    # Check if logged in to OpenShift
    if ! oc whoami &> /dev/null; then
        log_error "Not logged in to OpenShift. Please run 'oc login' first."
        exit 1
    fi
    
    # Check cluster admin permissions
    if ! oc auth can-i create clusterroles &> /dev/null; then
        log_error "Insufficient permissions. Cluster admin access required."
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

validate_config() {
    log_info "Validating configuration..."
    
    if [[ -z "$MINIO_ENDPOINT" ]]; then
        log_error "MINIO_ENDPOINT is required"
        exit 1
    fi
    
    if [[ "$MODE" == "git-sync" && -z "$GIT_REPOSITORY" ]]; then
        log_error "GIT_REPOSITORY is required for git-sync mode"
        exit 1
    fi
    
    log_success "Configuration validation passed"
}

create_namespace() {
    log_info "Creating namespace: $NAMESPACE"
    
    if oc get namespace $NAMESPACE &> /dev/null; then
        log_warning "Namespace $NAMESPACE already exists"
    else
        oc apply -f namespace.yaml
        log_success "Namespace created"
    fi
}

setup_secrets() {
    log_info "Setting up secrets..."
    
    # Check if secrets already exist
    if oc get secret backup-secrets -n $NAMESPACE &> /dev/null; then
        log_warning "backup-secrets already exists, skipping creation"
    else
        log_warning "Please create backup-secrets manually using secret-template.yaml"
        log_warning "You need to provide MinIO credentials in base64 format"
    fi
    
    if [[ "$MODE" == "git-sync" ]]; then
        if oc get secret git-sync-secrets -n $NAMESPACE &> /dev/null; then
            log_warning "git-sync-secrets already exists, skipping creation"
        else
            log_warning "Please create git-sync-secrets manually using secret-template.yaml"
            log_warning "You need to provide Git credentials in base64 format"
        fi
    fi
}

update_configmap() {
    log_info "Updating configuration..."
    
    # Create temporary configmap with updated values
    cat > /tmp/configmap-updated.yaml << EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: backup-config
  namespace: $NAMESPACE
  labels:
    app: cluster-backup
    component: configuration
data:
  cluster-domain: "$CLUSTER_DOMAIN"
  cluster-name: "$CLUSTER_NAME"
  minio-endpoint: "$MINIO_ENDPOINT"
  minio-bucket: "$MINIO_BUCKET"
  minio-use-ssl: "true"
  exclude-namespaces: "test-namespace,development"
  batch-size: "50"
  retry-attempts: "3"
  retry-delay: "5s"
  log-level: "info"
EOF

    if [[ "$MODE" == "git-sync" ]]; then
        cat > /tmp/git-sync-config-updated.yaml << EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: git-sync-config
  namespace: $NAMESPACE
  labels:
    app: git-sync
    component: configuration
data:
  minio-endpoint: "$MINIO_ENDPOINT"
  minio-bucket: "$MINIO_BUCKET"
  minio-use-ssl: "true"
  git-repository: "$GIT_REPOSITORY"
  git-branch: "main"
  work-dir: "/workspace"
  gitconfig: |
    [user]
        name = cluster-backup
        email = cluster-backup@example.com
    [init]
        defaultBranch = main
    [safe]
        directory = *
    [core]
        sshCommand = ssh -i /etc/git-secrets/ssh-private-key -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no
EOF
        oc apply -f /tmp/git-sync-config-updated.yaml
        rm /tmp/git-sync-config-updated.yaml
    fi
    
    oc apply -f /tmp/configmap-updated.yaml
    rm /tmp/configmap-updated.yaml
    
    log_success "Configuration updated"
}

deploy_rbac() {
    log_info "Deploying RBAC..."
    oc apply -f rbac.yaml
    log_success "RBAC deployed"
}

deploy_security_policies() {
    log_info "Deploying security policies..."
    
    # Check if SecurityContextConstraints are supported (OpenShift)
    if oc api-resources | grep -q securitycontextconstraints; then
        oc apply -f security-policies.yaml
        log_success "Security policies deployed"
    else
        log_warning "SecurityContextConstraints not supported, skipping SCC creation"
        # Apply only NetworkPolicies and other resources
        oc apply -f security-policies.yaml || log_warning "Some security policies may not be supported"
    fi
}

deploy_backup_cronjob() {
    log_info "Deploying backup CronJob..."
    oc apply -f backup-cronjob.yaml
    log_success "Backup CronJob deployed"
}

deploy_git_sync() {
    log_info "Deploying Git Sync CronJob..."
    oc apply -f git-sync-cronjob.yaml
    log_success "Git Sync CronJob deployed"
}

deploy_monitoring() {
    log_info "Deploying monitoring..."
    
    # Check if Prometheus operator is available
    if oc api-resources | grep -q servicemonitors; then
        oc apply -f monitoring.yaml
        log_success "Monitoring deployed"
    else
        log_warning "Prometheus operator not found, skipping monitoring deployment"
    fi
}

run_test_backup() {
    log_info "Running test backup..."
    
    # Create a manual job from the CronJob
    JOB_NAME="manual-backup-$(date +%s)"
    oc create job $JOB_NAME --from=cronjob/cluster-backup -n $NAMESPACE
    
    log_info "Waiting for job to complete..."
    oc wait --for=condition=complete job/$JOB_NAME -n $NAMESPACE --timeout=600s
    
    if oc get job $JOB_NAME -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}' | grep -q True; then
        log_success "Test backup completed successfully"
    else
        log_error "Test backup failed"
        oc logs job/$JOB_NAME -n $NAMESPACE
        exit 1
    fi
    
    # Clean up test job
    oc delete job $JOB_NAME -n $NAMESPACE
}

show_status() {
    log_info "Deployment status:"
    echo
    echo "Namespace:"
    oc get namespace $NAMESPACE
    echo
    echo "CronJobs:"
    oc get cronjobs -n $NAMESPACE
    echo
    echo "Secrets:"
    oc get secrets -n $NAMESPACE | grep -E "(backup|git-sync)"
    echo
    echo "ConfigMaps:"
    oc get configmaps -n $NAMESPACE | grep -E "(backup|git-sync)"
    echo
    
    if [[ "$MODE" == "backup" ]]; then
        echo "Next backup scheduled for:"
        oc get cronjob cluster-backup -n $NAMESPACE -o jsonpath='{.spec.schedule}'
        echo
    fi
    
    if [[ "$MODE" == "git-sync" ]]; then
        echo "Next git sync scheduled for:"
        oc get cronjob git-sync -n $NAMESPACE -o jsonpath='{.spec.schedule}'
        echo
    fi
}

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Deploy OpenShift Cluster Backup System

Options:
    -m, --mode MODE              Deployment mode: 'backup' or 'git-sync' (default: backup)
    -c, --cluster-name NAME      Cluster name (default: openshift-cluster)
    -d, --cluster-domain DOMAIN  Cluster domain (default: cluster.local)
    -e, --minio-endpoint URL     MinIO endpoint (required)
    -b, --minio-bucket BUCKET    MinIO bucket (default: cluster-backups)
    -g, --git-repository URL     Git repository URL (required for git-sync mode)
    -t, --test                   Run test backup after deployment
    -h, --help                   Show this help message

Environment Variables:
    CLUSTER_NAME                 Cluster name
    CLUSTER_DOMAIN              Cluster domain
    MINIO_ENDPOINT              MinIO endpoint
    MINIO_BUCKET                MinIO bucket
    GIT_REPOSITORY              Git repository URL

Examples:
    # Deploy backup system
    $0 --mode backup --cluster-name prod-cluster --minio-endpoint minio.example.com:9000

    # Deploy git sync system
    $0 --mode git-sync --minio-endpoint minio.example.com:9000 --git-repository git@github.com:org/backups.git

    # Deploy with test
    $0 --mode backup --minio-endpoint minio.example.com:9000 --test

EOF
}

# Parse command line arguments
TEST_BACKUP=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -m|--mode)
            MODE="$2"
            shift 2
            ;;
        -c|--cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        -d|--cluster-domain)
            CLUSTER_DOMAIN="$2"
            shift 2
            ;;
        -e|--minio-endpoint)
            MINIO_ENDPOINT="$2"
            shift 2
            ;;
        -b|--minio-bucket)
            MINIO_BUCKET="$2"
            shift 2
            ;;
        -g|--git-repository)
            GIT_REPOSITORY="$2"
            shift 2
            ;;
        -t|--test)
            TEST_BACKUP=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Main deployment flow
main() {
    log_info "Starting OpenShift Cluster Backup System deployment..."
    log_info "Mode: $MODE"
    log_info "Cluster: $CLUSTER_NAME"
    log_info "Domain: $CLUSTER_DOMAIN"
    log_info "MinIO: $MINIO_ENDPOINT"
    
    check_prerequisites
    validate_config
    create_namespace
    setup_secrets
    update_configmap
    deploy_rbac
    deploy_security_policies
    
    if [[ "$MODE" == "backup" ]]; then
        deploy_backup_cronjob
    elif [[ "$MODE" == "git-sync" ]]; then
        deploy_git_sync
    else
        log_error "Invalid mode: $MODE. Must be 'backup' or 'git-sync'"
        exit 1
    fi
    
    deploy_monitoring
    
    if [[ "$TEST_BACKUP" == "true" && "$MODE" == "backup" ]]; then
        run_test_backup
    fi
    
    show_status
    
    log_success "Deployment completed successfully!"
    
    if [[ "$MODE" == "backup" ]]; then
        log_info "Don't forget to:"
        log_info "1. Create the backup-secrets with your MinIO credentials"
        log_info "2. Verify the backup CronJob schedule meets your requirements"
        log_info "3. Configure MinIO lifecycle policies for backup retention"
    fi
    
    if [[ "$MODE" == "git-sync" ]]; then
        log_info "Don't forget to:"
        log_info "1. Create the git-sync-secrets with your Git credentials"
        log_info "2. Verify the Git repository is accessible"
        log_info "3. Configure appropriate Git repository permissions"
    fi
}

main "$@"