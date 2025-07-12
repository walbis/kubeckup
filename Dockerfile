# Multi-stage build for cluster-backup
FROM registry.redhat.io/ubi9/go-toolset:latest AS backup-builder

USER root
WORKDIR /workspace

# Copy Go modules and source code
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o cluster-backup main.go

# Multi-stage build for git-sync
FROM registry.redhat.io/ubi9/go-toolset:latest AS git-sync-builder

USER root
WORKDIR /workspace

# Copy Go modules and source code for git-sync
COPY go.mod go.sum ./
RUN go mod download

COPY git-sync.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o git-sync git-sync.go

# Final runtime image
FROM registry.redhat.io/ubi9/ubi-minimal:latest

# Install required packages
RUN microdnf update -y && \
    microdnf install -y \
        git \
        openssh-clients \
        ca-certificates \
        tar \
        gzip && \
    microdnf clean all && \
    rm -rf /var/cache/yum

# Create non-root user
RUN useradd -r -u 1001 -g root backup-user

# Copy binaries from builders
COPY --from=backup-builder /workspace/cluster-backup /usr/local/bin/cluster-backup
COPY --from=git-sync-builder /workspace/git-sync /usr/local/bin/git-sync

# Set proper permissions
RUN chmod +x /usr/local/bin/cluster-backup /usr/local/bin/git-sync && \
    chown 1001:0 /usr/local/bin/cluster-backup /usr/local/bin/git-sync

# Create necessary directories
RUN mkdir -p /tmp /workspace && \
    chmod 1777 /tmp && \
    chown 1001:0 /workspace

# Switch to non-root user
USER 1001

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/cluster-backup", "--health-check"]

# Default command
CMD ["/usr/local/bin/cluster-backup"]