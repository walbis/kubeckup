# Multi-stage build for Git Sync Service
# Production-ready UBI9-based container

FROM registry.redhat.io/ubi9/go-toolset:latest AS builder

USER root
WORKDIR /workspace

# Copy Go modules and download dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code and build statically linked binary
COPY git-sync.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o git-sync git-sync.go

# Final runtime image based on UBI9-minimal
FROM registry.redhat.io/ubi9/ubi-minimal:latest

# Install security updates and required packages
RUN microdnf update -y && \
    microdnf install -y \
        ca-certificates \
        git \
        openssh-clients \
        tar \
        gzip && \
    microdnf clean all && \
    rm -rf /var/cache/yum

# Create non-root user for OpenShift compatibility
RUN useradd -r -u 1001 -g root -s /sbin/nologin git-sync-user

# Copy binary from builder stage
COPY --from=builder /workspace/git-sync /usr/local/bin/git-sync

# Set proper permissions for OpenShift (group-writable, root group ownership)
RUN chmod +x /usr/local/bin/git-sync && \
    chown 1001:root /usr/local/bin/git-sync

# Create necessary directories with OpenShift-compatible permissions
# OpenShift assigns random UIDs but always uses group 0 (root)
RUN mkdir -p /tmp /workspace /data /home/git-sync-user/.ssh && \
    chmod 1777 /tmp && \
    chmod 775 /workspace /data && \
    chmod 700 /home/git-sync-user/.ssh && \
    chgrp -R root /workspace /data /home/git-sync-user && \
    chmod -R g+rwX /workspace /data /home/git-sync-user

# Security labels
LABEL name="openshift-cluster-git-sync" \
      vendor="OpenShift" \
      version="1.0.0" \
      summary="OpenShift Cluster Git Sync Service" \
      description="Production-ready git synchronization service for cluster backups" \
      io.k8s.description="Sync cluster backups from MinIO to Git repositories" \
      io.k8s.display-name="OpenShift Cluster Git Sync" \
      io.openshift.tags="git,sync,backup,openshift,kubernetes"

# Switch to non-root user
USER 1001

# Set HOME environment variable
ENV HOME=/home/git-sync-user

# Expose metrics port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/git-sync", "--health-check"]

# Set working directory
WORKDIR /workspace

# Default command
CMD ["/usr/local/bin/git-sync"]