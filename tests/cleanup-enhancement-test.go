package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Test and demonstrate the need for cleanup mechanism
func main() {
	// MinIO configuration
	endpoint := "localhost:9000"
	accessKey := "minioadmin"
	secretKey := "minioadmin"
	bucketName := "cleanup-test-bucket"
	useSSL := false

	// Create MinIO client
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}

	ctx := context.Background()

	// Create bucket if it doesn't exist
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		log.Fatalf("Failed to check bucket: %v", err)
	}
	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			log.Fatalf("Failed to create bucket: %v", err)
		}
		log.Printf("Created bucket: %s", bucketName)
	}

	// Test 1: Demonstrate storage growth without cleanup
	log.Println("\n=== Test 1: Storage Growth Without Cleanup ===")
	testStorageGrowthWithoutCleanup(ctx, minioClient, bucketName)

	// Test 2: Test with proposed cleanup mechanism
	log.Println("\n=== Test 2: Storage With Cleanup Mechanism ===")
	testStorageWithCleanup(ctx, minioClient, bucketName)

	// Test 3: Cleanup performance impact
	log.Println("\n=== Test 3: Cleanup Performance Analysis ===")
	testCleanupPerformance(ctx, minioClient, bucketName)
}

func testStorageGrowthWithoutCleanup(ctx context.Context, client *minio.Client, bucket string) {
	cluster := "no-cleanup-cluster"
	
	log.Println("📈 Simulating 30 days of backups without cleanup...")
	
	// Simulate backup every day for 30 days
	for day := 1; day <= 30; day++ {
		// Each day, backup 10 different resources
		for resource := 1; resource <= 10; resource++ {
			objectPath := fmt.Sprintf("clusterbackup/%s/default/deployments/app-%d.yaml", cluster, resource)
			content := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-%d
  namespace: default
  cluster: %s
  backup-day: %d
  timestamp: %s
spec:
  replicas: %d
  # Day %d backup content
  # This content changes daily but overwrites the same file`, 
				resource, cluster, day, time.Now().AddDate(0, 0, -30+day).Format(time.RFC3339), day%5+1, day)

			_, err := client.PutObject(
				ctx,
				bucket,
				objectPath,
				strings.NewReader(content),
				int64(len(content)),
				minio.PutObjectOptions{
					ContentType: "application/x-yaml",
				},
			)
			
			if err != nil {
				log.Printf("❌ Day %d backup failed for app-%d: %v", day, resource, err)
			}
		}
		
		// Check storage state every 10 days
		if day%10 == 0 {
			analyzeClusterStorage(ctx, client, bucket, cluster, fmt.Sprintf("Day %d", day))
		}
	}
	
	log.Println("📊 Final analysis after 30 days:")
	analyzeClusterStorage(ctx, client, bucket, cluster, "Final")
}

func testStorageWithCleanup(ctx context.Context, client *minio.Client, bucket string) {
	cluster := "with-cleanup-cluster"
	retentionDays := 7 // Keep only last 7 days
	
	log.Printf("📈 Simulating 30 days of backups WITH %d-day retention...", retentionDays)
	
	for day := 1; day <= 30; day++ {
		currentTime := time.Now().AddDate(0, 0, -30+day)
		
		// Each day, backup 10 different resources with timestamp in metadata
		for resource := 1; resource <= 10; resource++ {
			objectPath := fmt.Sprintf("clusterbackup/%s/default/deployments/app-%d.yaml", cluster, resource)
			content := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-%d
  namespace: default
  cluster: %s
  backup-day: %d
  timestamp: %s
  backup-date: "%s"
spec:
  replicas: %d`, 
				resource, cluster, day, currentTime.Format(time.RFC3339), currentTime.Format("2006-01-02"), day%5+1)

			_, err := client.PutObject(
				ctx,
				bucket,
				objectPath,
				strings.NewReader(content),
				int64(len(content)),
				minio.PutObjectOptions{
					ContentType: "application/x-yaml",
				},
			)
			
			if err != nil {
				log.Printf("❌ Day %d backup failed for app-%d: %v", day, resource, err)
				continue
			}
		}
		
		// Simulate cleanup mechanism (keep only last 7 days)
		if day > retentionDays {
			performCleanup(ctx, client, bucket, cluster, currentTime, retentionDays)
		}
		
		// Check storage state every 10 days
		if day%10 == 0 {
			analyzeClusterStorage(ctx, client, bucket, cluster, fmt.Sprintf("Day %d (with cleanup)", day))
		}
	}
	
	log.Println("📊 Final analysis after 30 days with cleanup:")
	analyzeClusterStorage(ctx, client, bucket, cluster, "Final with cleanup")
}

func performCleanup(ctx context.Context, client *minio.Client, bucket, cluster string, currentTime time.Time, retentionDays int) {
	// In a real implementation, this would need to track backup timestamps
	// For this demo, we simulate cleanup by checking object metadata or timestamps
	
	cutoffTime := currentTime.AddDate(0, 0, -retentionDays)
	log.Printf("🧹 Performing cleanup for %s (keeping files newer than %s)", cluster, cutoffTime.Format("2006-01-02"))
	
	// List objects for this cluster
	objects := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix: "clusterbackup/" + cluster,
	})
	
	cleanedCount := 0
	for object := range objects {
		if object.Err != nil {
			continue
		}
		
		// In real implementation, you'd parse backup timestamp from object metadata
		// For demo, we simulate by checking object modification time
		if object.LastModified.Before(cutoffTime) {
			err := client.RemoveObject(ctx, bucket, object.Key, minio.RemoveObjectOptions{})
			if err != nil {
				log.Printf("❌ Failed to remove old object %s: %v", object.Key, err)
			} else {
				cleanedCount++
				log.Printf("🗑️  Removed old backup: %s", object.Key)
			}
		}
	}
	
	if cleanedCount > 0 {
		log.Printf("✅ Cleaned up %d old backup files", cleanedCount)
	}
}

func testCleanupPerformance(ctx context.Context, client *minio.Client, bucket string) {
	cluster := "performance-test-cluster"
	
	log.Println("⚡ Testing cleanup performance with large number of files...")
	
	// Create 1000 objects to test cleanup performance
	log.Println("📝 Creating 1000 test objects...")
	start := time.Now()
	
	for i := 0; i < 1000; i++ {
		objectPath := fmt.Sprintf("clusterbackup/%s/test-namespace/deployments/test-app-%d.yaml", cluster, i)
		content := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-%d
  namespace: test-namespace
spec:
  replicas: 1`, i)

		_, err := client.PutObject(
			ctx,
			bucket,
			objectPath,
			strings.NewReader(content),
			int64(len(content)),
			minio.PutObjectOptions{
				ContentType: "application/x-yaml",
			},
		)
		
		if err != nil {
			log.Printf("❌ Failed to create test object %d: %v", i, err)
		}
	}
	
	createDuration := time.Since(start)
	log.Printf("✅ Created 1000 objects in %v", createDuration)
	
	// Test cleanup performance
	log.Println("🧹 Testing cleanup performance...")
	cleanupStart := time.Now()
	
	objects := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix: "clusterbackup/" + cluster,
	})
	
	cleanedCount := 0
	for object := range objects {
		if object.Err != nil {
			continue
		}
		
		err := client.RemoveObject(ctx, bucket, object.Key, minio.RemoveObjectOptions{})
		if err != nil {
			log.Printf("❌ Failed to remove object %s: %v", object.Key, err)
		} else {
			cleanedCount++
		}
	}
	
	cleanupDuration := time.Since(cleanupStart)
	log.Printf("✅ Cleaned up %d objects in %v", cleanedCount, cleanupDuration)
	log.Printf("📊 Performance: %.2f objects/second", float64(cleanedCount)/cleanupDuration.Seconds())
}

func analyzeClusterStorage(ctx context.Context, client *minio.Client, bucket, cluster, phase string) {
	objects := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix: "clusterbackup/" + cluster,
	})
	
	var objectCount int
	var totalSize int64
	var oldestTime, newestTime time.Time
	
	for object := range objects {
		if object.Err != nil {
			continue
		}
		
		objectCount++
		totalSize += object.Size
		
		if oldestTime.IsZero() || object.LastModified.Before(oldestTime) {
			oldestTime = object.LastModified
		}
		if newestTime.IsZero() || object.LastModified.After(newestTime) {
			newestTime = object.LastModified
		}
	}
	
	log.Printf("📊 %s Analysis for %s:", phase, cluster)
	log.Printf("  Objects: %d", objectCount)
	log.Printf("  Total size: %d bytes (%.2f KB)", totalSize, float64(totalSize)/1024)
	if !oldestTime.IsZero() {
		log.Printf("  Date range: %s to %s", oldestTime.Format("2006-01-02"), newestTime.Format("2006-01-02"))
		log.Printf("  Retention span: %.1f days", newestTime.Sub(oldestTime).Hours()/24)
	}
}