package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type GitSyncConfig struct {
	MinIOEndpoint   string
	MinIOAccessKey  string
	MinIOSecretKey  string
	MinIOBucket     string
	MinIOUseSSL     bool
	GitRepository   string
	GitBranch       string
	GitUsername     string
	GitEmail        string
	GitToken        string
	SSHKeyPath      string
	WorkDir         string
	RetryAttempts   int
	RetryDelay      time.Duration
}

type GitSyncMetrics struct {
	SyncDuration     prometheus.Histogram
	SyncErrors       prometheus.Counter
	FilesProcessed   prometheus.Counter
	LastSyncTime     prometheus.Gauge
	ClustersBackedUp prometheus.Gauge
}

type GitSync struct {
	config      *GitSyncConfig
	minioClient *minio.Client
	metrics     *GitSyncMetrics
	ctx         context.Context
	logger      *GitSyncLogger
}

type GitSyncLogger struct {
	logLevel string
}

type GitSyncLogEntry struct {
	Timestamp   string      `json:"timestamp"`
	Level       string      `json:"level"`
	Component   string      `json:"component"`
	Operation   string      `json:"operation"`
	Message     string      `json:"message"`
	Data        interface{} `json:"data,omitempty"`
	Error       string      `json:"error,omitempty"`
	Duration    float64     `json:"duration_ms,omitempty"`
}

func main() {
	logger := NewGitSyncLogger()
	logger.Info("startup", "Starting Git Sync service...", nil)

	// Check if it's a health check request
	if len(os.Args) > 1 && os.Args[1] == "--health-check" {
		fmt.Println("OK")
		os.Exit(0)
	}

	config, err := loadGitSyncConfig()
	if err != nil {
		logger.Fatal("config_load", "Failed to load configuration", map[string]interface{}{"error": err.Error()})
	}

	gitSync, err := NewGitSync(config, logger)
	if err != nil {
		logger.Fatal("git_sync_init", "Failed to initialize git sync", map[string]interface{}{"error": err.Error()})
	}

	logger.Info("config_loaded", "Git sync configuration loaded successfully", map[string]interface{}{
		"git_repository": config.GitRepository,
		"git_branch": config.GitBranch,
		"minio_bucket": config.MinIOBucket,
		"work_dir": config.WorkDir,
	})

	// Start metrics server in a goroutine
	go startGitSyncMetricsServer()

	if err := gitSync.Run(); err != nil {
		logger.Fatal("git_sync_run", "Git sync failed", map[string]interface{}{"error": err.Error()})
	}

	logger.Info("git_sync_complete", "Git sync completed successfully", nil)
}

func startGitSyncMetricsServer() {
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Starting Git sync metrics server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Printf("Failed to start git sync metrics server: %v", err)
	}
}

func loadGitSyncConfig() (*GitSyncConfig, error) {
	// Use environment variable for work directory, fallback to temp
	workDir := getEnvOrDefault("WORK_DIR", "/tmp/git-sync-work")

	config := &GitSyncConfig{
		MinIOEndpoint:  getEnvOrDefault("MINIO_ENDPOINT", ""),
		MinIOAccessKey: getEnvOrDefault("MINIO_ACCESS_KEY", ""),
		MinIOSecretKey: getEnvOrDefault("MINIO_SECRET_KEY", ""),
		MinIOBucket:    getEnvOrDefault("MINIO_BUCKET", "cluster-backups"),
		MinIOUseSSL:    getEnvOrDefault("MINIO_USE_SSL", "true") == "true",
		GitRepository:  getEnvOrDefault("GIT_REPOSITORY", ""),
		GitBranch:      getEnvOrDefault("GIT_BRANCH", "main"),
		GitUsername:    getEnvOrDefault("GIT_USERNAME", "cluster-backup"),
		GitEmail:       getEnvOrDefault("GIT_EMAIL", "cluster-backup@example.com"),
		GitToken:       getEnvOrDefault("GIT_TOKEN", ""),
		SSHKeyPath:     getEnvOrDefault("SSH_KEY_PATH", "/etc/git-secrets/ssh-private-key"),
		WorkDir:        workDir,
		RetryAttempts:  3,
		RetryDelay:     5 * time.Second,
	}

	if config.MinIOEndpoint == "" || config.MinIOAccessKey == "" || config.MinIOSecretKey == "" {
		return nil, fmt.Errorf("MinIO configuration is incomplete")
	}

	// Git repository is optional - if not provided, only MinIO download will be performed
	if config.GitRepository == "" {
		log.Println("No git repository configured - will run in download-only mode")
	}

	return config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func NewGitSyncLogger() *GitSyncLogger {
	return &GitSyncLogger{
		logLevel: getEnvOrDefault("LOG_LEVEL", "info"),
	}
}

func (gsl *GitSyncLogger) log(level, operation, message string, data map[string]interface{}, err error) {
	entry := GitSyncLogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Component: "git-sync",
		Operation: operation,
		Message:   message,
		Data:      data,
	}
	
	if err != nil {
		entry.Error = err.Error()
	}
	
	// Add duration from data if available
	if data != nil {
		if dur, ok := data["duration_ms"].(float64); ok {
			entry.Duration = dur
		}
	}
	
	logJSON, _ := json.Marshal(entry)
	fmt.Println(string(logJSON))
	
	// Also log to standard logger for backward compatibility
	if level == "error" || level == "fatal" {
		log.Printf("[%s] %s: %s", level, operation, message)
		if err != nil {
			log.Printf("Error details: %v", err)
		}
	}
}

func (gsl *GitSyncLogger) Debug(operation, message string, data map[string]interface{}) {
	if gsl.logLevel == "debug" {
		gsl.log("debug", operation, message, data, nil)
	}
}

func (gsl *GitSyncLogger) Info(operation, message string, data map[string]interface{}) {
	gsl.log("info", operation, message, data, nil)
}

func (gsl *GitSyncLogger) Warn(operation, message string, data map[string]interface{}) {
	gsl.log("warn", operation, message, data, nil)
}

func (gsl *GitSyncLogger) Error(operation, message string, data map[string]interface{}) {
	gsl.log("error", operation, message, data, nil)
}

func (gsl *GitSyncLogger) ErrorWithErr(operation, message string, data map[string]interface{}, err error) {
	gsl.log("error", operation, message, data, err)
}

func (gsl *GitSyncLogger) Fatal(operation, message string, data map[string]interface{}) {
	gsl.log("fatal", operation, message, data, nil)
	os.Exit(1)
}

func NewGitSync(config *GitSyncConfig, logger *GitSyncLogger) (*GitSync, error) {
	minioClient, err := minio.New(config.MinIOEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.MinIOAccessKey, config.MinIOSecretKey, ""),
		Secure: config.MinIOUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %v", err)
	}

	metrics := &GitSyncMetrics{
		SyncDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "git_sync_duration_seconds",
			Help: "Duration of git sync operations in seconds",
		}),
		SyncErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "git_sync_errors_total",
			Help: "Total number of git sync errors",
		}),
		FilesProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "git_sync_files_processed_total",
			Help: "Total number of files processed during sync",
		}),
		LastSyncTime: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "git_sync_last_success_timestamp",
			Help: "Timestamp of the last successful sync",
		}),
		ClustersBackedUp: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "git_sync_clusters_backed_up",
			Help: "Number of clusters backed up in the last sync",
		}),
	}

	return &GitSync{
		config:      config,
		minioClient: minioClient,
		metrics:     metrics,
		ctx:         context.Background(),
		logger:      logger,
	}, nil
}

func (gs *GitSync) Run() error {
	startTime := time.Now()
	defer func() {
		gs.metrics.SyncDuration.Observe(time.Since(startTime).Seconds())
	}()

	exists, err := gs.minioClient.BucketExists(gs.ctx, gs.config.MinIOBucket)
	if err != nil {
		gs.metrics.SyncErrors.Inc()
		return fmt.Errorf("failed to check bucket existence: %v", err)
	}
	if !exists {
		gs.metrics.SyncErrors.Inc()
		return fmt.Errorf("bucket %s does not exist", gs.config.MinIOBucket)
	}

	if err := gs.setupWorkDirectory(); err != nil {
		gs.metrics.SyncErrors.Inc()
		return fmt.Errorf("failed to setup work directory: %v", err)
	}
	defer gs.cleanup()

	if err := gs.setupGitConfig(); err != nil {
		gs.metrics.SyncErrors.Inc()
		return fmt.Errorf("failed to setup git config: %v", err)
	}

	// Skip git operations if repository is not configured
	if gs.config.GitRepository == "" {
		log.Println("No git repository configured, performing MinIO download only")
		clusterCount, err := gs.downloadAndMergeBackups()
		if err != nil {
			gs.metrics.SyncErrors.Inc()
			return fmt.Errorf("failed to download backups: %v", err)
		}
		log.Printf("Download-only mode completed successfully: %d clusters processed", clusterCount)
		gs.metrics.ClustersBackedUp.Set(float64(clusterCount))
		return nil
	}

	if err := gs.cloneOrPullRepository(); err != nil {
		gs.metrics.SyncErrors.Inc()
		return fmt.Errorf("failed to clone/pull repository: %v", err)
	}

	clusterCount, err := gs.downloadAndMergeBackups()
	if err != nil {
		gs.metrics.SyncErrors.Inc()
		return fmt.Errorf("failed to download and merge backups: %v", err)
	}

	if err := gs.commitAndPushChanges(); err != nil {
		gs.metrics.SyncErrors.Inc()
		return fmt.Errorf("failed to commit and push changes: %v", err)
	}

	gs.metrics.LastSyncTime.SetToCurrentTime()
	gs.metrics.ClustersBackedUp.Set(float64(clusterCount))
	return nil
}

func (gs *GitSync) setupWorkDirectory() error {
	// Remove existing work directory and recreate (now it's a subdirectory)
	log.Printf("Setting up work directory: %s", gs.config.WorkDir)
	
	// Remove existing directory if it exists
	if err := os.RemoveAll(gs.config.WorkDir); err != nil {
		log.Printf("Warning: failed to remove existing work directory: %v", err)
	}
	
	// Create fresh work directory
	if err := os.MkdirAll(gs.config.WorkDir, 0755); err != nil {
		return fmt.Errorf("failed to create work directory: %v", err)
	}
	
	log.Printf("Work directory ready: %s", gs.config.WorkDir)
	return nil
}

func (gs *GitSync) cleanup() {
	// Only clean contents, don't remove the work directory itself
	// as it might be a mounted volume
	if entries, err := os.ReadDir(gs.config.WorkDir); err == nil {
		for _, entry := range entries {
			entryPath := filepath.Join(gs.config.WorkDir, entry.Name())
			if err := os.RemoveAll(entryPath); err != nil {
				log.Printf("Warning: cleanup failed for %s: %v", entryPath, err)
			}
		}
	}
}

func (gs *GitSync) setupGitConfig() error {
	// Create a local git config in the work directory to avoid read-only filesystem issues
	gitConfigPath := filepath.Join(gs.config.WorkDir, ".gitconfig")
	
	gitConfigContent := fmt.Sprintf(`[user]
	name = %s
	email = %s
[init]
	defaultBranch = main
[safe]
	directory = *
`, gs.config.GitUsername, gs.config.GitEmail)

	if err := os.WriteFile(gitConfigPath, []byte(gitConfigContent), 0644); err != nil {
		return fmt.Errorf("failed to write git config: %v", err)
	}

	// Set GIT_CONFIG_GLOBAL to use our local config file
	os.Setenv("GIT_CONFIG_GLOBAL", gitConfigPath)
	
	log.Printf("Git config set up in work directory: %s", gitConfigPath)

	// Only setup SSH key if it exists (for HTTPS we don't need it)
	if gs.config.SSHKeyPath != "" {
		if _, err := os.Stat(gs.config.SSHKeyPath); err == nil {
			if err := gs.setupSSHKey(); err != nil {
				log.Printf("Warning: failed to setup SSH key: %v", err)
				// Continue with HTTPS authentication
			}
		} else {
			log.Println("SSH key not found, using HTTPS authentication")
		}
	}

	return nil
}

func (gs *GitSync) setupSSHKey() error {
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return err
	}

	keyPath := filepath.Join(sshDir, "id_rsa")
	if err := gs.runCommand("cp", gs.config.SSHKeyPath, keyPath); err != nil {
		return err
	}

	if err := os.Chmod(keyPath, 0600); err != nil {
		return err
	}

	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	knownHosts := `github.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCj7ndNxQowgcQnjshcLrqPEiiphnt+VTTvDP6mHBL9j1aNUkY4Ue1gvwnGLVlOhGeYrnZaMgRK6+PKCUXaDbC7qtbW8gIkhL7aGCsOr/C56SJMy/BCZfxd1nWzAOxSDPgVsmerOBYfNqltV9/hWCqBywINIR+5dIg6JTJ72pcEpEjcYgXkE2YEFXV1JHnsKgbLWNlhScqb2UmyRkQyytRLtL+38TGxkxCflmO+5Z8CSSNY7GidjMIZ7Q4zMjA2n1nGrlTDkzwDCsw+wqFPGQA179cnfGWOWRVruj16z6XyvxvjJwbz0wQZ75XK5tKSb7FNyeIEs4TT4jk+S4dhPeAUC5y+bDYirYgM4GC7uEnztnZyaVWQ7B381AK4Qdrwt51ZqExKbQpTUNn+EjqoTwvqNj4kqx5QUCI0ThS/YkOxJCXmPUWZbhjpCg56i+2aB6CmK2JGhn57K5mj0MNdBXA4/WnwH6XoPWJzK5Nyu2zB3nAZp+S5hpQs+p1vN1/wsjk=
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9`

	return os.WriteFile(knownHostsPath, []byte(knownHosts), 0644)
}

func (gs *GitSync) cloneOrPullRepository() error {
	repoDir := filepath.Join(gs.config.WorkDir, "repository")
	
	// Check if authentication is available for private repositories
	authURL := gs.config.GitRepository
	hasAuth := false
	
	if gs.config.GitToken != "" && strings.HasPrefix(gs.config.GitRepository, "https://") {
		// Convert https://github.com/user/repo.git to https://token@github.com/user/repo.git
		authURL = strings.Replace(gs.config.GitRepository, "https://", fmt.Sprintf("https://%s@", gs.config.GitToken), 1)
		hasAuth = true
	} else if gs.config.SSHKeyPath != "" {
		hasAuth = true
	}

	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		log.Println("Cloning repository for incremental sync...")
		
		// Try to clone with authentication first, then fallback to public access
		if err := gs.runCommand("git", "clone", "-b", gs.config.GitBranch, authURL, repoDir); err != nil {
			if hasAuth {
				return fmt.Errorf("failed to clone repository with authentication: %v", err)
			} else {
				// If no auth provided, try public access (might work for public repos)
				log.Printf("No authentication provided, attempting public clone...")
				if err := gs.runCommand("git", "clone", "-b", gs.config.GitBranch, gs.config.GitRepository, repoDir); err != nil {
					return fmt.Errorf("failed to clone repository (no authentication): %v - please provide GIT_TOKEN or SSH key for private repositories", err)
				}
			}
		}
		log.Println("Repository cloned successfully")
		return nil
	}

	log.Println("Repository exists, pulling latest changes for incremental sync...")
	// Reset any local changes first
	if err := gs.runCommandInDir(repoDir, "git", "reset", "--hard", "HEAD"); err != nil {
		log.Printf("Warning: failed to reset repository: %v", err)
	}
	
	// Pull latest changes
	if err := gs.runCommandInDir(repoDir, "git", "pull", "origin", gs.config.GitBranch); err != nil {
		return fmt.Errorf("failed to pull latest changes: %v", err)
	}
	
	log.Println("Repository updated successfully")
	return nil
}

func (gs *GitSync) downloadAndMergeBackups() (int, error) {
	log.Println("Downloading multi-cluster backups from MinIO...")

	backupDir := filepath.Join(gs.config.WorkDir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return 0, err
	}

	clusters := make(map[string]bool)
	objectCh := gs.minioClient.ListObjects(gs.ctx, gs.config.MinIOBucket, minio.ListObjectsOptions{
		Prefix:    "clusterbackup/", // Only process centralized cluster backups
		Recursive: true,
	})

	downloadCount := 0
	for object := range objectCh {
		if object.Err != nil {
			log.Printf("Error listing object: %v", object.Err)
			continue
		}

		// Parse new structure: clusterbackup/{cluster-name}/{namespace}/{resource-type}/{resource-name}.yaml
		parts := strings.Split(object.Key, "/")
		if len(parts) >= 2 && parts[0] == "clusterbackup" {
			clusterName := parts[1]
			clusters[clusterName] = true
		}

		localPath := filepath.Join(backupDir, object.Key)
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			log.Printf("Error creating directory for %s: %v", localPath, err)
			continue
		}

		if err := gs.downloadFile(object.Key, localPath); err != nil {
			log.Printf("Error downloading %s: %v", object.Key, err)
			continue
		}

		downloadCount++
		gs.metrics.FilesProcessed.Inc()
	}

	log.Printf("Downloaded %d files from %d clusters", downloadCount, len(clusters))

	repoDir := filepath.Join(gs.config.WorkDir, "repository")
	if err := gs.mergeBackupsToRepo(backupDir, repoDir); err != nil {
		return 0, err
	}

	return len(clusters), nil
}

func (gs *GitSync) downloadFile(objectKey, localPath string) error {
	object, err := gs.minioClient.GetObject(gs.ctx, gs.config.MinIOBucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer object.Close()

	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, object)
	return err
}

func (gs *GitSync) mergeBackupsToRepo(backupDir, repoDir string) error {
	log.Println("Merging backups to repository...")

	return filepath.Walk(backupDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(backupDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(repoDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		return gs.copyFile(path, destPath)
	})
}

func (gs *GitSync) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func (gs *GitSync) commitAndPushChanges() error {
	repoDir := filepath.Join(gs.config.WorkDir, "repository")

	log.Println("Analyzing changes for incremental push...")
	
	// Add all changes
	if err := gs.runCommandInDir(repoDir, "git", "add", "."); err != nil {
		return fmt.Errorf("failed to add changes: %v", err)
	}

	// Check if there are any changes to commit
	if err := gs.runCommandInDir(repoDir, "git", "diff-index", "--quiet", "HEAD", "--"); err == nil {
		log.Println("No changes detected - skipping commit and push")
		return nil
	}

	// Get detailed change statistics
	changeStats, err := gs.getChangeStatistics(repoDir)
	if err != nil {
		log.Printf("Warning: failed to get change statistics: %v", err)
		changeStats = "Changes detected"
	}

	// Create detailed commit message with change summary
	commitMessage := fmt.Sprintf(`Multi-cluster backup sync - %s

%s

Incremental sync from MinIO to Git repository.
Only changed files are included in this commit.

Generated by KubeBackup Git-Sync Service`, 
		time.Now().Format("2006-01-02 15:04:05 UTC"),
		changeStats)

	log.Printf("Committing incremental changes...")
	if err := gs.runCommandInDir(repoDir, "git", "commit", "-m", commitMessage); err != nil {
		return fmt.Errorf("failed to commit changes: %v", err)
	}

	// Set up authenticated remote for push if token is provided
	if gs.config.GitToken != "" && strings.HasPrefix(gs.config.GitRepository, "https://") {
		authURL := strings.Replace(gs.config.GitRepository, "https://", fmt.Sprintf("https://%s@", gs.config.GitToken), 1)
		if err := gs.runCommandInDir(repoDir, "git", "remote", "set-url", "origin", authURL); err != nil {
			log.Printf("Warning: failed to set authenticated remote URL: %v", err)
		}
	}

	log.Println("Pushing incremental changes to remote repository...")
	if err := gs.runCommandInDir(repoDir, "git", "push", "origin", gs.config.GitBranch); err != nil {
		return fmt.Errorf("failed to push changes: %v", err)
	}

	log.Println("Incremental push completed successfully")
	return nil
}

func (gs *GitSync) getChangeStatistics(repoDir string) (string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--stat")
	cmd.Dir = repoDir
	
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	stats := strings.TrimSpace(string(output))
	if stats == "" {
		return "No detailed statistics available", nil
	}
	
	return stats, nil
}

func (gs *GitSync) runCommand(args ...string) error {
	return gs.runCommandInDir("", args...)
}

func (gs *GitSync) runCommandInDir(dir string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	log.Printf("Running command: %s", strings.Join(args, " "))
	return cmd.Run()
}