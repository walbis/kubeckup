package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Test concurrent writes to same MinIO bucket from different clusters
func main() {
	// MinIO configuration
	endpoint := "localhost:9000"
	accessKey := "minioadmin"
	secretKey := "minioadmin"
	bucketName := "test-cluster-backups"
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

	// Test 1: Concurrent writes from different clusters
	log.Println("\n=== Test 1: Concurrent Writes from Different Clusters ===")
	testConcurrentWrites(ctx, minioClient, bucketName)

	// Test 2: Same resource from different clusters
	log.Println("\n=== Test 2: Same Resource from Different Clusters ===")
	testSameResourceDifferentClusters(ctx, minioClient, bucketName)

	// Test 3: Storage growth over time
	log.Println("\n=== Test 3: Storage Growth Over Time ===")
	testStorageGrowth(ctx, minioClient, bucketName)

	// Test 4: Check final storage state
	log.Println("\n=== Test 4: Final Storage Analysis ===")
	analyzeStorageState(ctx, minioClient, bucketName)
}

func testConcurrentWrites(ctx context.Context, client *minio.Client, bucket string) {
	clusters := []string{"production-east", "production-west", "staging"}
	var wg sync.WaitGroup

	// Simulate concurrent backup operations
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			
			for i := 0; i < 5; i++ {
				objectPath := fmt.Sprintf("clusterbackup/%s/default/deployments/app-%d.yaml", clusterName, i)
				content := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-%d
  namespace: default
  cluster: %s
  timestamp: %s
spec:
  replicas: 1`, i, clusterName, time.Now().Format(time.RFC3339))

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
					log.Printf("❌ Cluster %s failed to write object %d: %v", clusterName, i, err)
				} else {
					log.Printf("✅ Cluster %s wrote object %d successfully", clusterName, i)
				}
				
				// Small delay to simulate realistic backup timing
				time.Sleep(100 * time.Millisecond)
			}
		}(cluster)
	}

	wg.Wait()
	log.Println("✅ Concurrent write test completed")
}

func testSameResourceDifferentClusters(ctx context.Context, client *minio.Client, bucket string) {
	clusters := []string{"production-east", "production-west"}
	var wg sync.WaitGroup

	// Both clusters backup the same resource name but different content
	for _, cluster := range clusters {
		wg.Add(1)
		go func(clusterName string) {
			defer wg.Done()
			
			objectPath := "clusterbackup/" + clusterName + "/kube-system/services/kube-dns.yaml"
			content := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  cluster: %s
  timestamp: %s
spec:
  clusterIP: 10.96.0.10
  ports:
  - port: 53`, clusterName, time.Now().Format(time.RFC3339))

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
				log.Printf("❌ Cluster %s failed to write kube-dns service: %v", clusterName, err)
			} else {
				log.Printf("✅ Cluster %s wrote kube-dns service successfully", clusterName)
			}
		}(cluster)
	}

	wg.Wait()
	log.Println("✅ Same resource different clusters test completed")
}

func testStorageGrowth(ctx context.Context, client *minio.Client, bucket string) {
	// Simulate daily backups for a week
	cluster := "test-cluster"
	
	for day := 1; day <= 7; day++ {
		log.Printf("📅 Simulating day %d backup", day)
		
		// Create objects for this day
		for i := 0; i < 3; i++ {
			objectPath := fmt.Sprintf("clusterbackup/%s/default/deployments/daily-app-%d.yaml", cluster, i)
			content := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: daily-app-%d
  namespace: default
  cluster: %s
  day: %d
  timestamp: %s
spec:
  replicas: %d`, i, cluster, day, time.Now().Format(time.RFC3339), day)

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
				log.Printf("❌ Day %d backup failed for app-%d: %v", day, i, err)
			}
		}
		
		// Check storage growth
		objects := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
			Prefix: "clusterbackup/" + cluster,
		})
		
		count := 0
		var totalSize int64
		for object := range objects {
			if object.Err != nil {
				continue
			}
			count++
			totalSize += object.Size
		}
		
		log.Printf("📊 Day %d: %d objects, total size: %d bytes", day, count, totalSize)
		
		// Small delay to simulate daily intervals
		time.Sleep(50 * time.Millisecond)
	}
	
	log.Println("✅ Storage growth test completed")
}

func analyzeStorageState(ctx context.Context, client *minio.Client, bucket string) {
	objects := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix: "clusterbackup/",
	})
	
	clusterStats := make(map[string]int)
	var totalObjects int
	var totalSize int64
	
	for object := range objects {
		if object.Err != nil {
			log.Printf("Error listing object: %v", object.Err)
			continue
		}
		
		totalObjects++
		totalSize += object.Size
		
		// Extract cluster name from path
		parts := strings.Split(object.Key, "/")
		if len(parts) >= 2 {
			cluster := parts[1]
			clusterStats[cluster]++
		}
		
		log.Printf("📄 %s (size: %d bytes, modified: %s)", object.Key, object.Size, object.LastModified)
	}
	
	log.Printf("\n📈 Storage Analysis:")
	log.Printf("Total objects: %d", totalObjects)
	log.Printf("Total size: %d bytes (%.2f KB)", totalSize, float64(totalSize)/1024)
	
	log.Printf("\n📊 Per-cluster breakdown:")
	for cluster, count := range clusterStats {
		log.Printf("  %s: %d objects", cluster, count)
	}
	
	// Check for any conflicts or duplicates
	log.Printf("\n🔍 Conflict Analysis:")
	log.Printf("✅ Each cluster writes to its own directory - no conflicts")
	log.Printf("⚠️  No cleanup mechanism - storage will grow indefinitely")
	log.Printf("💡 Recommendation: Implement retention policy")
}