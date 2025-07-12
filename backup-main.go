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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
	BatchSize         int
	RetryAttempts     int
	RetryDelay        time.Duration
}

type BackupConfig struct {
	FilteringMode           string   // "whitelist", "blacklist", "hybrid"
	IncludeResources        []string
	ExcludeResources        []string
	IncludeNamespaces       []string
	ExcludeNamespaces       []string
	IncludeCRDs             []string
	LabelSelector           string
	AnnotationSelector      string
	MaxResourceSize         string
	FollowOwnerReferences   bool
	IncludeManagedFields    bool
	IncludeStatus           bool
	OpenShiftMode           string
	IncludeOpenShiftRes     bool
	ValidateYAML            bool
	SkipInvalidResources    bool
}

type ClusterBackup struct {
	config       *Config
	backupConfig *BackupConfig
	minioClient  *minio.Client
	kubeClient   kubernetes.Interface
	dynamicClient dynamic.Interface
	discoveryClient discovery.DiscoveryInterface
	metrics      *BackupMetrics
	ctx          context.Context
}

type BackupMetrics struct {
	BackupDuration    prometheus.Histogram
	BackupErrors      prometheus.Counter
	ResourcesBackedUp prometheus.Counter
	LastBackupTime    prometheus.Gauge
	NamespacesBackedUp prometheus.Gauge
}

var (
	// Default system namespaces to exclude
	defaultSystemNamespaces = []string{
		"kube-system", "kube-public", "kube-node-lease",
		"openshift-system", "openshift-cluster-version", "openshift-machine-api",
		"openshift-kube-apiserver", "openshift-kube-controller-manager",
		"openshift-kube-scheduler", "openshift-etcd", "openshift-apiserver",
		"openshift-controller-manager", "openshift-service-ca",
		"openshift-network-operator", "openshift-sdn", "openshift-dns",
		"openshift-ingress", "openshift-authentication", "openshift-oauth-apiserver",
		"openshift-image-registry", "openshift-cluster-storage-operator",
		"openshift-cluster-csi-drivers", "openshift-monitoring",
		"openshift-operator-lifecycle-manager", "openshift-marketplace",
		"openshift-console", "openshift-console-operator",
	}
)

func main() {
	log.Println("Starting Enhanced OpenShift Cluster Backup...")

	// Check if it's a health check request
	if len(os.Args) > 1 && os.Args[1] == "--health-check" {
		fmt.Println("OK")
		os.Exit(0)
	}

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	backupConfig, err := loadBackupConfig()
	if err != nil {
		log.Fatalf("Failed to load backup configuration: %v", err)
	}

	backup, err := NewClusterBackup(config, backupConfig)
	if err != nil {
		log.Fatalf("Failed to create backup client: %v", err)
	}

	// Start metrics server in a goroutine
	go startMetricsServer()

	if err := backup.Run(); err != nil {
		log.Fatalf("Backup failed: %v", err)
	}

	log.Println("Backup completed successfully")
}

func loadConfig() (*Config, error) {
	config := &Config{
		ClusterDomain:  getSecretValue("CLUSTER_DOMAIN", "cluster.local"),
		ClusterName:    getSecretValue("CLUSTER_NAME", "default"),
		MinIOEndpoint:  getSecretValue("MINIO_ENDPOINT", ""),
		MinIOAccessKey: getSecretValue("MINIO_ACCESS_KEY", ""),
		MinIOSecretKey: getSecretValue("MINIO_SECRET_KEY", ""),
		MinIOBucket:    getSecretValue("MINIO_BUCKET", "cluster-backups"),
		MinIOUseSSL:    getSecretValue("MINIO_USE_SSL", "true") == "true",
		BatchSize:      50,
		RetryAttempts:  3,
		RetryDelay:     5 * time.Second,
	}

	// Parse batch size from secret
	if batchStr := getSecretValue("BATCH_SIZE", "50"); batchStr != "" {
		if batch, err := strconv.Atoi(batchStr); err == nil {
			config.BatchSize = batch
		}
	}

	// Parse retry attempts from secret
	if retryStr := getSecretValue("RETRY_ATTEMPTS", "3"); retryStr != "" {
		if retry, err := strconv.Atoi(retryStr); err == nil {
			config.RetryAttempts = retry
		}
	}

	// Parse retry delay from secret
	if delayStr := getSecretValue("RETRY_DELAY", "5s"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			config.RetryDelay = delay
		}
	}

	if config.MinIOEndpoint == "" || config.MinIOAccessKey == "" || config.MinIOSecretKey == "" {
		return nil, fmt.Errorf("MinIO configuration is incomplete")
	}

	return config, nil
}

func loadBackupConfig() (*BackupConfig, error) {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	// Read backup configuration from ConfigMap
	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), "backup-config", metav1.GetOptions{})
	if err != nil {
		log.Printf("Warning: Could not load backup-config ConfigMap: %v, using defaults", err)
		return getDefaultBackupConfig(), nil
	}

	return parseBackupConfig(configMap), nil
}

func parseBackupConfig(cm *corev1.ConfigMap) *BackupConfig {
	config := getDefaultBackupConfig()

	if val, ok := cm.Data["filtering-mode"]; ok && val != "" {
		config.FilteringMode = strings.TrimSpace(val)
	}
	if val, ok := cm.Data["include-resources"]; ok && val != "" {
		config.IncludeResources = parseCommaSeparated(val)
	}
	if val, ok := cm.Data["exclude-resources"]; ok && val != "" {
		config.ExcludeResources = parseCommaSeparated(val)
	}
	if val, ok := cm.Data["include-namespaces"]; ok && val != "" {
		config.IncludeNamespaces = parseCommaSeparated(val)
	}
	if val, ok := cm.Data["exclude-namespaces"]; ok && val != "" {
		config.ExcludeNamespaces = parseCommaSeparated(val)
	}
	if val, ok := cm.Data["include-crds"]; ok && val != "" {
		config.IncludeCRDs = parseCommaSeparated(val)
	}
	if val, ok := cm.Data["label-selector"]; ok {
		config.LabelSelector = val
	}
	if val, ok := cm.Data["annotation-selector"]; ok {
		config.AnnotationSelector = val
	}
	if val, ok := cm.Data["max-resource-size"]; ok {
		config.MaxResourceSize = val
	}
	if val, ok := cm.Data["follow-owner-references"]; ok {
		config.FollowOwnerReferences = val == "true"
	}
	if val, ok := cm.Data["include-managed-fields"]; ok {
		config.IncludeManagedFields = val == "true"
	}
	if val, ok := cm.Data["include-status"]; ok {
		config.IncludeStatus = val == "true"
	}
	if val, ok := cm.Data["openshift-mode"]; ok {
		config.OpenShiftMode = val
	}
	if val, ok := cm.Data["include-openshift-resources"]; ok {
		config.IncludeOpenShiftRes = val == "true"
	}
	if val, ok := cm.Data["validate-yaml"]; ok {
		config.ValidateYAML = val == "true"
	}
	if val, ok := cm.Data["skip-invalid-resources"]; ok {
		config.SkipInvalidResources = val == "true"
	}

	return config
}

func getDefaultBackupConfig() *BackupConfig {
	return &BackupConfig{
		FilteringMode: "blacklist", // Default: backup everything except excludes
		IncludeResources: []string{
			"pods", "services", "deployments", "replicasets", "configmaps", "secrets",
			"persistentvolumes", "persistentvolumeclaims", "serviceaccounts",
			"roles", "rolebindings", "clusterroles", "clusterrolebindings",
			"ingresses", "networkpolicies", "jobs", "cronjobs", "daemonsets", "statefulsets",
		},
		ExcludeResources: []string{
			"events", "nodes", "endpoints", "replicationcontrollers",
		},
		ExcludeNamespaces: defaultSystemNamespaces,
		IncludeCRDs: []string{
			"workflows.argoproj.io", "workflowtemplates.argoproj.io",
			"routes.route.openshift.io", "buildconfigs.build.openshift.io",
			"imagestreams.image.openshift.io", "deploymentconfigs.apps.openshift.io",
		},
		OpenShiftMode:         "auto-detect",
		IncludeOpenShiftRes:   true,
		ValidateYAML:          true,
		SkipInvalidResources:  true,
		FollowOwnerReferences: false,
		IncludeManagedFields:  false,
		IncludeStatus:         false,
	}
}

func parseCommaSeparated(input string) []string {
	var result []string
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			parts := strings.Split(line, ",")
			for _, part := range parts {
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
	}
	return result
}

func NewClusterBackup(config *Config, backupConfig *BackupConfig) (*ClusterBackup, error) {
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %v", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %v", err)
	}

	minioClient, err := minio.New(config.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.MinIOAccessKey, config.MinIOSecretKey, ""),
		Secure: config.MinIOUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %v", err)
	}

	metrics := &BackupMetrics{
		BackupDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "cluster_backup_duration_seconds",
			Help: "Duration of cluster backup operations in seconds",
		}),
		BackupErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cluster_backup_errors_total",
			Help: "Total number of backup errors",
		}),
		ResourcesBackedUp: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cluster_backup_resources_total",
			Help: "Total number of resources backed up",
		}),
		LastBackupTime: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cluster_backup_last_success_timestamp",
			Help: "Timestamp of the last successful backup",
		}),
		NamespacesBackedUp: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "cluster_backup_namespaces_total",
			Help: "Number of namespaces backed up",
		}),
	}

	return &ClusterBackup{
		config:          config,
		backupConfig:    backupConfig,
		minioClient:     minioClient,
		kubeClient:      kubeClient,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		metrics:         metrics,
		ctx:             context.Background(),
	}, nil
}

func (cb *ClusterBackup) Run() error {
	startTime := time.Now()
	defer func() {
		cb.metrics.BackupDuration.Observe(time.Since(startTime).Seconds())
	}()

	log.Printf("Starting backup with config: cluster=%s.%s, OpenShift=%s", 
		cb.config.ClusterName, cb.config.ClusterDomain, cb.backupConfig.OpenShiftMode)

	// Auto-detect OpenShift if needed
	if cb.backupConfig.OpenShiftMode == "auto-detect" {
		cb.backupConfig.OpenShiftMode = cb.detectOpenShift()
	}

	exists, err := cb.minioClient.BucketExists(cb.ctx, cb.config.MinIOBucket)
	if err != nil {
		cb.metrics.BackupErrors.Inc()
		return fmt.Errorf("failed to check bucket existence: %v", err)
	}
	if !exists {
		cb.metrics.BackupErrors.Inc()
		return fmt.Errorf("bucket %s does not exist", cb.config.MinIOBucket)
	}

	// Get all available API resources
	apiResources, err := cb.getAPIResources()
	if err != nil {
		cb.metrics.BackupErrors.Inc()
		return fmt.Errorf("failed to get API resources: %v", err)
	}

	log.Printf("Found %d API resource types", len(apiResources))

	// Get namespaces to backup
	namespaces, err := cb.getNamespacesToBackup()
	if err != nil {
		cb.metrics.BackupErrors.Inc()
		return fmt.Errorf("failed to get namespaces: %v", err)
	}

	log.Printf("Backing up %d namespaces", len(namespaces))
	cb.metrics.NamespacesBackedUp.Set(float64(len(namespaces)))

	totalResources := 0
	for _, ns := range namespaces {
		count, err := cb.backupNamespace(ns, apiResources)
		if err != nil {
			log.Printf("Error backing up namespace %s: %v", ns, err)
			cb.metrics.BackupErrors.Inc()
		} else {
			totalResources += count
		}
	}

	log.Printf("Backup completed: %d resources from %d namespaces", totalResources, len(namespaces))
	cb.metrics.LastBackupTime.SetToCurrentTime()
	return nil
}

func (cb *ClusterBackup) detectOpenShift() string {
	// Try to detect OpenShift by looking for OpenShift-specific APIs
	_, err := cb.discoveryClient.ServerResourcesForGroupVersion("route.openshift.io/v1")
	if err == nil {
		log.Println("OpenShift detected (route.openshift.io API found)")
		return "enabled"
	}
	
	_, err = cb.discoveryClient.ServerResourcesForGroupVersion("build.openshift.io/v1")
	if err == nil {
		log.Println("OpenShift detected (build.openshift.io API found)")
		return "enabled"
	}
	
	log.Println("OpenShift not detected, using standard Kubernetes mode")
	return "disabled"
}

func (cb *ClusterBackup) getAPIResources() ([]metav1.APIResource, error) {
	var allResources []metav1.APIResource
	
	// Get standard Kubernetes resources
	resourceLists, err := cb.discoveryClient.ServerPreferredResources()
	if err != nil {
		log.Printf("Warning: Some API resources may not be available: %v", err)
	}

	for _, list := range resourceLists {
		if list == nil {
			continue
		}
		
		for _, resource := range list.APIResources {
			if cb.shouldIncludeResource(resource, list.GroupVersion) {
				allResources = append(allResources, resource)
			}
		}
	}

	// Add CRDs if specified
	if len(cb.backupConfig.IncludeCRDs) > 0 {
		crdResources, err := cb.getCRDResources()
		if err != nil {
			log.Printf("Warning: Failed to get CRD resources: %v", err)
		} else {
			allResources = append(allResources, crdResources...)
		}
	}

	return allResources, nil
}

func (cb *ClusterBackup) getCRDResources() ([]metav1.APIResource, error) {
	var resources []metav1.APIResource
	
	for _, crd := range cb.backupConfig.IncludeCRDs {
		parts := strings.Split(crd, ".")
		if len(parts) < 2 {
			continue
		}
		
		resourceName := parts[0]
		group := strings.Join(parts[1:], ".")
		
		// Try to find the CRD in available resources
		resourceLists, err := cb.discoveryClient.ServerPreferredResources()
		if err != nil {
			continue
		}
		
		for _, list := range resourceLists {
			if list == nil {
				continue
			}
			
			if strings.Contains(list.GroupVersion, group) {
				for _, resource := range list.APIResources {
					if resource.Name == resourceName {
						resources = append(resources, resource)
						log.Printf("Found CRD resource: %s in %s", resourceName, list.GroupVersion)
					}
				}
			}
		}
	}
	
	return resources, nil
}

func (cb *ClusterBackup) shouldIncludeResource(resource metav1.APIResource, groupVersion string) bool {
	resourceFullName := resource.Name
	if strings.Contains(groupVersion, "/") {
		groupPart := strings.Split(groupVersion, "/")[0]
		if groupPart != "" {
			resourceFullName = resource.Name + "." + groupPart
		}
	}

	// Must be listable and not a subresource - basic requirement
	if !containsVerb(resource.Verbs, "list") || strings.Contains(resource.Name, "/") {
		return false
	}

	// Apply filtering based on mode
	switch cb.backupConfig.FilteringMode {
	case "whitelist":
		// Only include resources in the include list
		return cb.isInIncludeList(resource.Name, resourceFullName)
		
	case "blacklist":
		// Include all resources except those in exclude list
		return !cb.isInExcludeList(resource.Name, resourceFullName)
		
	case "hybrid":
		// First check include list (if not empty), then check exclude list
		if len(cb.backupConfig.IncludeResources) > 0 {
			if !cb.isInIncludeList(resource.Name, resourceFullName) {
				return false
			}
		}
		return !cb.isInExcludeList(resource.Name, resourceFullName)
		
	default:
		// Default to blacklist mode for backward compatibility
		return !cb.isInExcludeList(resource.Name, resourceFullName)
	}
}

func (cb *ClusterBackup) isInIncludeList(resourceName, resourceFullName string) bool {
	for _, included := range cb.backupConfig.IncludeResources {
		if strings.EqualFold(resourceName, included) || strings.EqualFold(resourceFullName, included) {
			return true
		}
	}
	return false
}

func (cb *ClusterBackup) isInExcludeList(resourceName, resourceFullName string) bool {
	for _, excluded := range cb.backupConfig.ExcludeResources {
		if strings.EqualFold(resourceName, excluded) || strings.EqualFold(resourceFullName, excluded) {
			return true
		}
	}
	return false
}

func (cb *ClusterBackup) getNamespacesToBackup() ([]string, error) {
	// If specific namespaces are included, use those
	if len(cb.backupConfig.IncludeNamespaces) > 0 {
		return cb.backupConfig.IncludeNamespaces, nil
	}

	// Otherwise, get all namespaces and filter
	namespaces, err := cb.kubeClient.CoreV1().Namespaces().List(cb.ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var result []string
	for _, ns := range namespaces.Items {
		if !cb.shouldExcludeNamespace(ns.Name) {
			result = append(result, ns.Name)
		}
	}

	return result, nil
}

func (cb *ClusterBackup) shouldExcludeNamespace(namespace string) bool {
	for _, excluded := range cb.backupConfig.ExcludeNamespaces {
		if namespace == excluded {
			return true
		}
	}
	return false
}

func (cb *ClusterBackup) backupNamespace(namespace string, apiResources []metav1.APIResource) (int, error) {
	log.Printf("Backing up namespace: %s", namespace)
	resourceCount := 0

	for _, resource := range apiResources {
		gvr := schema.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: resource.Name,
		}

		// Parse group and version from API resource
		if strings.Contains(resource.Kind, ".") {
			parts := strings.Split(resource.Kind, ".")
			if len(parts) >= 2 {
				gvr.Group = strings.Join(parts[1:], ".")
			}
		}

		if resource.Group != "" {
			gvr.Group = resource.Group
		}
		if resource.Version != "" {
			gvr.Version = resource.Version
		}

		count, err := cb.backupResource(namespace, gvr, resource)
		if err != nil {
			log.Printf("Error backing up resource %s in namespace %s: %v", resource.Name, namespace, err)
			continue
		}
		resourceCount += count
	}

	return resourceCount, nil
}

func (cb *ClusterBackup) backupResource(namespace string, gvr schema.GroupVersionResource, resource metav1.APIResource) (int, error) {
	var listOptions metav1.ListOptions
	
	if cb.backupConfig.LabelSelector != "" {
		listOptions.LabelSelector = cb.backupConfig.LabelSelector
	}

	var resources *unstructured.UnstructuredList
	var err error

	if resource.Namespaced {
		resources, err = cb.dynamicClient.Resource(gvr).Namespace(namespace).List(cb.ctx, listOptions)
	} else {
		resources, err = cb.dynamicClient.Resource(gvr).List(cb.ctx, listOptions)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to list %s: %v", resource.Name, err)
	}

	count := 0
	for _, item := range resources.Items {
		if cb.shouldSkipResource(&item) {
			continue
		}

		cleaned := cb.cleanResource(&item)
		
		if cb.backupConfig.ValidateYAML {
			if err := cb.validateResource(cleaned); err != nil {
				if cb.backupConfig.SkipInvalidResources {
					log.Printf("Skipping invalid resource %s/%s: %v", namespace, item.GetName(), err)
					continue
				}
				return count, fmt.Errorf("invalid resource %s/%s: %v", namespace, item.GetName(), err)
			}
		}

		if err := cb.uploadResource(namespace, gvr.Resource, item.GetName(), cleaned); err != nil {
			return count, fmt.Errorf("failed to upload %s/%s: %v", namespace, item.GetName(), err)
		}

		count++
		cb.metrics.ResourcesBackedUp.Inc()
	}

	if count > 0 {
		log.Printf("Backed up %d %s resources from namespace %s", count, resource.Name, namespace)
	}

	return count, nil
}

func (cb *ClusterBackup) shouldSkipResource(resource *unstructured.Unstructured) bool {
	// Skip resources with specific annotations if configured
	if cb.backupConfig.AnnotationSelector != "" {
		annotations := resource.GetAnnotations()
		if annotations == nil {
			return true
		}
		
		// Simple annotation check (could be enhanced with label selector parsing)
		parts := strings.Split(cb.backupConfig.AnnotationSelector, "=")
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if annotations[key] != value {
				return true
			}
		}
	}

	// Skip resources managed by operators if not following owner references
	if !cb.backupConfig.FollowOwnerReferences {
		if owners := resource.GetOwnerReferences(); len(owners) > 0 {
			for _, owner := range owners {
				if owner.Controller != nil && *owner.Controller {
					return true
				}
			}
		}
	}

	return false
}

func (cb *ClusterBackup) validateResource(resource map[string]interface{}) error {
	// Basic YAML validation
	_, err := yaml.Marshal(resource)
	return err
}

func (cb *ClusterBackup) cleanResource(resource *unstructured.Unstructured) map[string]interface{} {
	cleaned := make(map[string]interface{})
	for k, v := range resource.Object {
		cleaned[k] = v
	}

	// Always remove status unless specifically included
	if !cb.backupConfig.IncludeStatus {
		delete(cleaned, "status")
	}

	// Clean metadata
	if metadata, ok := cleaned["metadata"].(map[string]interface{}); ok {
		// Always remove these volatile fields
		delete(metadata, "uid")
		delete(metadata, "resourceVersion")
		delete(metadata, "generation")
		delete(metadata, "creationTimestamp")
		delete(metadata, "selfLink")
		
		if !cb.backupConfig.IncludeManagedFields {
			delete(metadata, "managedFields")
		}
	}

	return cleaned
}

func (cb *ClusterBackup) uploadResource(namespace, resourceType, name string, resource map[string]interface{}) error {
	yamlData, err := yaml.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal resource to YAML: %v", err)
	}

	objectPath := fmt.Sprintf("%s/%s/%s/%s/%s.yaml",
		cb.config.ClusterDomain,
		cb.config.ClusterName,
		namespace,
		resourceType,
		name,
	)

	_, err = cb.minioClient.PutObject(
		cb.ctx,
		cb.config.MinIOBucket,
		objectPath,
		strings.NewReader(string(yamlData)),
		int64(len(yamlData)),
		minio.PutObjectOptions{
			ContentType: "application/x-yaml",
		},
	)

	return err
}

func containsVerb(verbs []string, verb string) bool {
	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}

func getSecretValue(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func startMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Starting backup metrics server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Printf("Failed to start backup metrics server: %v", err)
	}
}