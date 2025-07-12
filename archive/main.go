package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Config struct {
	ClusterDomain     string
	ClusterName       string
	MinIOEndpoint     string
	MinIOAccessKey    string
	MinIOSecretKey    string
	MinIOBucket       string
	MinIOUseSSL       bool
	ExcludeNamespaces []string
	BatchSize         int
	RetryAttempts     int
	RetryDelay        time.Duration
}

type BackupMetrics struct {
	ResourcesBackedUp prometheus.Counter
	BackupDuration    prometheus.Histogram
	BackupErrors      prometheus.Counter
	LastBackupTime    prometheus.Gauge
}

type ClusterBackup struct {
	config       *Config
	k8sClient    dynamic.Interface
	minioClient  *minio.Client
	metrics      *BackupMetrics
	ctx          context.Context
}

var (
	systemNamespaces = []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"openshift-system",
		"openshift-operator-lifecycle-manager",
		"openshift-machine-config-operator",
		"openshift-cluster-version",
		"openshift-authentication",
		"openshift-authentication-operator",
		"openshift-cluster-storage-operator",
		"openshift-console",
		"openshift-console-operator",
		"openshift-dns",
		"openshift-dns-operator",
		"openshift-etcd",
		"openshift-etcd-operator",
		"openshift-image-registry",
		"openshift-ingress",
		"openshift-ingress-operator",
		"openshift-kube-apiserver",
		"openshift-kube-controller-manager",
		"openshift-kube-scheduler",
		"openshift-machine-api",
		"openshift-monitoring",
		"openshift-network-operator",
		"openshift-node",
		"openshift-oauth-apiserver",
		"openshift-operator-lifecycle-manager",
		"openshift-service-ca",
		"openshift-service-ca-operator",
	}
)

func main() {
	log.Println("Starting OpenShift Cluster Backup...")

	// Check if it's a health check request
	if len(os.Args) > 1 && os.Args[1] == "--health-check" {
		fmt.Println("OK")
		os.Exit(0)
	}

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	backup, err := NewClusterBackup(config)
	if err != nil {
		log.Fatalf("Failed to initialize cluster backup: %v", err)
	}

	// Start metrics server in a goroutine
	go startMetricsServer()

	if err := backup.Run(); err != nil {
		log.Fatalf("Backup failed: %v", err)
	}

	log.Println("Backup completed successfully")
}

func startMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Starting metrics server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Printf("Failed to start metrics server: %v", err)
	}
}

func loadConfig() (*Config, error) {
	config := &Config{
		ClusterDomain:     getEnvOrDefault("CLUSTER_DOMAIN", "cluster.local"),
		ClusterName:       getEnvOrDefault("CLUSTER_NAME", "openshift-cluster"),
		MinIOEndpoint:     getEnvOrDefault("MINIO_ENDPOINT", ""),
		MinIOAccessKey:    getEnvOrDefault("MINIO_ACCESS_KEY", ""),
		MinIOSecretKey:    getEnvOrDefault("MINIO_SECRET_KEY", ""),
		MinIOBucket:       getEnvOrDefault("MINIO_BUCKET", "cluster-backups"),
		MinIOUseSSL:       getEnvOrDefault("MINIO_USE_SSL", "true") == "true",
		BatchSize:         50,
		RetryAttempts:     3,
		RetryDelay:        5 * time.Second,
	}

	// Parse batch size
	if batchStr := getEnvOrDefault("BATCH_SIZE", "50"); batchStr != "" {
		if batch, err := strconv.Atoi(batchStr); err == nil {
			config.BatchSize = batch
		}
	}

	// Parse retry attempts
	if retryStr := getEnvOrDefault("RETRY_ATTEMPTS", "3"); retryStr != "" {
		if retry, err := strconv.Atoi(retryStr); err == nil {
			config.RetryAttempts = retry
		}
	}

	// Parse retry delay
	if delayStr := getEnvOrDefault("RETRY_DELAY", "5s"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			config.RetryDelay = delay
		}
	}

	excludeNS := getEnvOrDefault("EXCLUDE_NAMESPACES", "")
	if excludeNS != "" {
		config.ExcludeNamespaces = strings.Split(excludeNS, ",")
	}
	config.ExcludeNamespaces = append(config.ExcludeNamespaces, systemNamespaces...)

	if config.MinIOEndpoint == "" || config.MinIOAccessKey == "" || config.MinIOSecretKey == "" {
		return nil, fmt.Errorf("MinIO configuration is incomplete")
	}

	return config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func NewClusterBackup(config *Config) (*ClusterBackup, error) {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %v", err)
	}

	k8sClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	minioClient, err := minio.New(config.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.MinIOAccessKey, config.MinIOSecretKey, ""),
		Secure: config.MinIOUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %v", err)
	}

	metrics := &BackupMetrics{
		ResourcesBackedUp: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cluster_backup_resources_total",
			Help: "Total number of resources backed up",
		}),
		BackupDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "cluster_backup_duration_seconds",
			Help: "Duration of backup operations in seconds",
		}),
		BackupErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cluster_backup_errors_total",
			Help: "Total number of backup errors",
		}),
		LastBackupTime: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cluster_backup_last_success_timestamp",
			Help: "Timestamp of the last successful backup",
		}),
	}

	return &ClusterBackup{
		config:      config,
		k8sClient:   k8sClient,
		minioClient: minioClient,
		metrics:     metrics,
		ctx:         context.Background(),
	}, nil
}

func (cb *ClusterBackup) Run() error {
	startTime := time.Now()
	defer func() {
		cb.metrics.BackupDuration.Observe(time.Since(startTime).Seconds())
	}()

	exists, err := cb.minioClient.BucketExists(cb.ctx, cb.config.MinIOBucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket existence: %v", err)
	}
	if !exists {
		return fmt.Errorf("bucket %s does not exist", cb.config.MinIOBucket)
	}

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to create in-cluster config: %v", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %v", err)
	}

	apiResources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		log.Printf("Warning: Some API resources may not be available: %v", err)
	}

	namespaces, err := cb.getNamespaces()
	if err != nil {
		return fmt.Errorf("failed to get namespaces: %v", err)
	}

	for _, namespace := range namespaces {
		if cb.isExcludedNamespace(namespace.GetName()) {
			log.Printf("Skipping excluded namespace: %s", namespace.GetName())
			continue
		}

		log.Printf("Backing up namespace: %s", namespace.GetName())
		if err := cb.backupNamespace(namespace.GetName(), apiResources); err != nil {
			log.Printf("Error backing up namespace %s: %v", namespace.GetName(), err)
			cb.metrics.BackupErrors.Inc()
			continue
		}
	}

	cb.metrics.LastBackupTime.SetToCurrentTime()
	return nil
}

func (cb *ClusterBackup) getNamespaces() ([]unstructured.Unstructured, error) {
	namespaceGVR := schema.GroupVersionResource{
		Version:  "v1",
		Resource: "namespaces",
	}

	list, err := cb.k8sClient.Resource(namespaceGVR).List(cb.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return list.Items, nil
}

func (cb *ClusterBackup) isExcludedNamespace(namespace string) bool {
	for _, excluded := range cb.config.ExcludeNamespaces {
		if namespace == excluded {
			return true
		}
	}
	return false
}

func (cb *ClusterBackup) backupNamespace(namespace string, apiResources []*metav1.APIResourceList) error {
	for _, apiResourceList := range apiResources {
		if apiResourceList == nil {
			continue
		}

		for _, apiResource := range apiResourceList.APIResources {
			if !cb.shouldBackupResource(apiResource) {
				continue
			}

			gvr := schema.GroupVersionResource{
				Group:    apiResourceList.GroupVersion,
				Version:  apiResourceList.GroupVersion,
				Resource: apiResource.Name,
			}

			if strings.Contains(apiResourceList.GroupVersion, "/") {
				parts := strings.Split(apiResourceList.GroupVersion, "/")
				gvr.Group = parts[0]
				gvr.Version = parts[1]
			} else {
				gvr.Group = ""
				gvr.Version = apiResourceList.GroupVersion
			}

			if err := cb.backupResource(namespace, gvr, apiResource.Kind); err != nil {
				log.Printf("Error backing up resource %s in namespace %s: %v", apiResource.Name, namespace, err)
				continue
			}
		}
	}
	return nil
}

func (cb *ClusterBackup) shouldBackupResource(apiResource metav1.APIResource) bool {
	if !contains(apiResource.Verbs, "get") || !contains(apiResource.Verbs, "list") {
		return false
	}

	excludedResources := []string{
		"events",
		"componentstatuses",
		"endpoints",
		"limitranges",
		"persistentvolumes",
		"resourcequotas",
		"nodes",
		"bindings",
		"replicationcontrollers",
	}

	for _, excluded := range excludedResources {
		if apiResource.Name == excluded {
			return false
		}
	}

	if strings.Contains(apiResource.Name, "/") {
		return false
	}

	return true
}

func (cb *ClusterBackup) backupResource(namespace string, gvr schema.GroupVersionResource, kind string) error {
	var list *unstructured.UnstructuredList
	var err error

	if namespace != "" {
		list, err = cb.k8sClient.Resource(gvr).Namespace(namespace).List(cb.ctx, metav1.ListOptions{})
	} else {
		list, err = cb.k8sClient.Resource(gvr).List(cb.ctx, metav1.ListOptions{})
	}

	if err != nil {
		return err
	}

	for _, item := range list.Items {
		if err := cb.uploadResource(namespace, kind, &item); err != nil {
			log.Printf("Error uploading resource %s/%s: %v", item.GetNamespace(), item.GetName(), err)
			continue
		}
		cb.metrics.ResourcesBackedUp.Inc()
	}

	return nil
}

func (cb *ClusterBackup) uploadResource(namespace, kind string, resource *unstructured.Unstructured) error {
	cleanedResource := cb.cleanResource(resource)

	yamlData, err := yaml.Marshal(cleanedResource)
	if err != nil {
		return fmt.Errorf("failed to marshal resource to YAML: %v", err)
	}

	objectPath := fmt.Sprintf("%s/%s/%s/%s/%s.yaml",
		cb.config.ClusterDomain,
		cb.config.ClusterName,
		namespace,
		strings.ToLower(kind),
		resource.GetName(),
	)

	_, err = cb.minioClient.PutObject(
		cb.ctx,
		cb.config.MinIOBucket,
		objectPath,
		strings.NewReader(string(yamlData)),
		int64(len(yamlData)),
		minio.PutObjectOptions{
			ContentType: "application/yaml",
		},
	)

	if err != nil {
		return fmt.Errorf("failed to upload to MinIO: %v", err)
	}

	log.Printf("Uploaded: %s", objectPath)
	return nil
}

func (cb *ClusterBackup) cleanResource(resource *unstructured.Unstructured) map[string]interface{} {
	cleaned := make(map[string]interface{})

	for k, v := range resource.Object {
		cleaned[k] = v
	}

	delete(cleaned, "status")

	if metadata, ok := cleaned["metadata"].(map[string]interface{}); ok {
		delete(metadata, "uid")
		delete(metadata, "resourceVersion")
		delete(metadata, "generation")
		delete(metadata, "creationTimestamp")
		delete(metadata, "managedFields")
		delete(metadata, "selfLink")

		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
			sensitiveAnnotations := []string{
				"kubectl.kubernetes.io/last-applied-configuration",
				"deployment.kubernetes.io/revision",
			}
			for _, key := range sensitiveAnnotations {
				delete(annotations, key)
			}
		}

		cleaned["metadata"] = metadata
	}

	if spec, ok := cleaned["spec"].(map[string]interface{}); ok {
		if clusterIP, exists := spec["clusterIP"]; exists && clusterIP == "None" {
			delete(spec, "clusterIP")
		}
		if nodeName, exists := spec["nodeName"]; exists && nodeName != "" {
			delete(spec, "nodeName")
		}
		cleaned["spec"] = spec
	}

	return cleaned
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}