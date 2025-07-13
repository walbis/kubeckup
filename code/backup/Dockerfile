# Multi-stage build for OpenShift Cluster Backup
# Production-ready UBI9-based container

FROM registry.redhat.io/ubi9/go-toolset:latest AS builder

USER root
WORKDIR /workspace

# Copy Go modules and download dependencies
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code and build statically linked binary
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o cluster-backup main.go

# Final runtime image based on UBI9-minimal
FROM registry.redhat.io/ubi9/ubi-minimal:latest

# Install security updates and required packages
RUN microdnf update -y && \
    microdnf install -y ca-certificates && \
    microdnf clean all && \
    rm -rf /var/cache/yum

# Create non-root user for OpenShift compatibility
RUN useradd -r -u 1001 -g root -s /sbin/nologin backup-user

# Copy binary from builder stage
COPY --from=builder /workspace/cluster-backup /usr/local/bin/cluster-backup

# Set proper permissions for OpenShift (group-writable, root group ownership)
RUN chmod +x /usr/local/bin/cluster-backup && \
    chown 1001:root /usr/local/bin/cluster-backup

# Create necessary directories with OpenShift-compatible permissions
# OpenShift assigns random UIDs but always uses group 0 (root)
RUN mkdir -p /tmp /workspace /data && \
    chmod 1777 /tmp && \
    chmod 775 /workspace /data && \
    chgrp -R root /workspace /data && \
    chmod -R g+rwX /workspace /data

# Security labels
LABEL name="openshift-cluster-backup" \
      vendor="OpenShift" \
      version="1.0.0" \
      summary="OpenShift Cluster Backup Service" \
      description="Production-ready backup service for OpenShift clusters" \
      io.k8s.description="Backup OpenShift cluster resources to MinIO storage" \
      io.k8s.display-name="OpenShift Cluster Backup" \
      io.openshift.tags="backup,openshift,kubernetes,minio"

# Switch to non-root user
USER 1001

# Expose metrics port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/cluster-backup", "--health-check"]

# Set working directory
WORKDIR /workspace

# Default command
CMD ["/usr/local/bin/cluster-backup"]