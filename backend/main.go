// package main

// import (
// 	"database/sql"
// 	"log"
// 	"os"
// 	"time"

// 	"github.com/amandeep2102/image-processor/backend/cache"
// 	"github.com/amandeep2102/image-processor/backend/worker"
// 	"github.com/gin-gonic/gin"
// 	"github.com/google/uuid"
// 	_ "github.com/lib/pq"
// )

// // all the global varialbles here
// var (
// 	db         *sql.DB
// 	workerPool *worker.Pool
// 	imageCache *cache.ImageCache
// )

// func main() {
// 	var err error
// 	db, err = sql.Open("postgres",
// 		"host=localhost port=5432 user=imageuser password=imagepass dbname=imagedb sslmode=disable")
// 	if err != nil {
// 		log.Fatal("Failed to connect to database:", err)
// 	}
// 	defer db.Close()

// 	// cache - 100 image
// 	imageCache = cache.NewImageCache(100)
// 	log.Println("Image cache initialized with capacity: 100")

// 	// initializing 10 workers
// 	workerPool = worker.NewPool(100, db, imageCache)
// 	workerPool.Start()
// 	defer workerPool.Stop()

// 	r := gin.Default()

// 	// endpoints
// 	r.GET("/health", handleHealth)
// 	r.POST("/process/resize", handleResize)
// 	r.POST("/process/thumbnail", handleThumbnail)
// 	r.POST("/process/filter", handleFilter)
// 	r.POST("/process/convert", handleConvert)
// 	r.GET("/job/:job_id", handleGetJobResult)
// 	r.GET("/workers/stats", handleWorkerStats)

// 	// endpoints for caching
// 	r.POST("/cache", handleCache)
// 	r.GET("/cache/stats", handleCacheStats)
// 	r.DELETE("/cache", handleCacheClear)

// 	log.Println("Backend server starting on :8081")
// 	r.Run(":8081")
// }

// func handleHealth(c *gin.Context) {
// 	c.JSON(200, gin.H{
// 		"status":             "ok",
// 		"job_queue_size":     workerPool.GetQueueSize(),
// 		"job_queue_capacity": workerPool.GetQueueCapacity(),
// 		"cache_size":         imageCache.Size(),
// 		"cache_capacity":     imageCache.Capacity(),
// 	})
// }

// func handleCache(c *gin.Context) {
// 	var req struct {
// 		ImageID  string   `json:"image_id"`  // Single image
// 		ImageIDs []string `json:"image_ids"` // Multiple images (optional)
// 	}

// 	if err := c.BindJSON(&req); err != nil {
// 		c.JSON(400, gin.H{"error": "Invalid request"})
// 		return
// 	}

// 	startTime := time.Now()

// 	imagesToCache := []string{}
// 	if req.ImageID != "" {
// 		imagesToCache = append(imagesToCache, req.ImageID)
// 	}
// 	if len(req.ImageIDs) > 0 {
// 		imagesToCache = append(imagesToCache, req.ImageIDs...)
// 	}

// 	if len(imagesToCache) == 0 {
// 		c.JSON(400, gin.H{"error": "No image_id or image_ids provided"})
// 		return
// 	}

// 	successCount := 0
// 	failedIDs := []string{}
// 	cachedIDs := []string{}

// 	for _, imageID := range imagesToCache {
// 		// get image path from database
// 		var imagePath string
// 		err := db.QueryRow("SELECT original_path FROM images WHERE id = $1", imageID).Scan(&imagePath)
// 		if err != nil {
// 			log.Printf("Image %s not found in database", imageID)
// 			failedIDs = append(failedIDs, imageID)
// 			continue
// 		}

// 		// read image
// 		imageData, err := os.ReadFile(imagePath)
// 		if err != nil {
// 			log.Printf("Failed to read image %s: %v", imageID, err)
// 			failedIDs = append(failedIDs, imageID)
// 			continue
// 		}

// 		// store in cache
// 		imageCache.Set(imageID, imageData, 30*time.Minute)
// 		successCount++
// 		cachedIDs = append(cachedIDs, imageID)

// 		log.Printf("✓ Cached image: %s (%d bytes)", imageID, len(imageData))
// 	}

// 	duration := time.Since(startTime)

// 	c.JSON(200, gin.H{
// 		"message":        "Cache operation completed",
// 		"requested":      len(imagesToCache),
// 		"cached":         successCount,
// 		"failed":         len(failedIDs),
// 		"cached_ids":     cachedIDs,
// 		"failed_ids":     failedIDs,
// 		"duration_ms":    duration.Milliseconds(),
// 		"cache_size":     imageCache.Size(),
// 		"cache_capacity": imageCache.Capacity(),
// 	})
// }

// func handleCacheStats(c *gin.Context) {
// 	stats := imageCache.GetStats()

// 	c.JSON(200, gin.H{
// 		"size":           stats.Size,
// 		"capacity":       stats.Capacity,
// 		"hit_count":      stats.HitCount,
// 		"miss_count":     stats.MissCount,
// 		"hit_rate":       stats.HitRate,
// 		"eviction_count": stats.EvictionCount,
// 	})
// }

// func handleCacheClear(c *gin.Context) {
// 	imageCache.Clear()

// 	c.JSON(200, gin.H{
// 		"message":    "Cache cleared successfully",
// 		"cache_size": imageCache.Size(),
// 	})
// }

// func handleResize(c *gin.Context) {
// 	var req struct {
// 		ImageID string `json:"image_id"`
// 		Width   int    `json:"width"`
// 		Height  int    `json:"height"`
// 	}

// 	if err := c.BindJSON(&req); err != nil {
// 		c.JSON(400, gin.H{"error": err.Error()})
// 		return
// 	}

// 	// Create job with unique ID
// 	job := worker.Job{
// 		JobID:     uuid.New().String(),
// 		ImageID:   req.ImageID,
// 		Operation: "resize",
// 		Parameters: map[string]interface{}{
// 			"width":  req.Width,
// 			"height": req.Height,
// 		},
// 	}

// 	// submitting to worker pool
// 	if err := workerPool.Submit(job); err != nil {
// 		c.JSON(503, gin.H{
// 			"error":   "Worker pool is busy",
// 			"message": err.Error(),
// 		})
// 		return
// 	}

// 	// return immediately
// 	c.JSON(202, gin.H{
// 		"job_id":     job.JobID,
// 		"message":    "Job submitted successfully",
// 		"queue_size": workerPool.GetQueueSize(),
// 	})
// }

// func handleThumbnail(c *gin.Context) {
// 	var req struct {
// 		ImageID string `json:"image_id"`
// 		Size    int    `json:"size"`
// 	}

// 	if err := c.BindJSON(&req); err != nil {
// 		c.JSON(400, gin.H{"error": err.Error()})
// 		return
// 	}

// 	job := worker.Job{
// 		JobID:     uuid.New().String(),
// 		ImageID:   req.ImageID,
// 		Operation: "thumbnail",
// 		Parameters: map[string]interface{}{
// 			"size": req.Size,
// 		},
// 	}

// 	if err := workerPool.Submit(job); err != nil {
// 		c.JSON(503, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(202, gin.H{
// 		"job_id":  job.JobID,
// 		"message": "Job submitted successfully",
// 	})
// }

// func handleFilter(c *gin.Context) {
// 	var req struct {
// 		ImageID    string  `json:"image_id"`
// 		FilterType string  `json:"filter_type"`
// 		Intensity  float64 `json:"intensity"`
// 	}

// 	if err := c.BindJSON(&req); err != nil {
// 		c.JSON(400, gin.H{"error": err.Error()})
// 		return
// 	}

// 	job := worker.Job{
// 		JobID:     uuid.New().String(),
// 		ImageID:   req.ImageID,
// 		Operation: "filter",
// 		Parameters: map[string]interface{}{
// 			"filter_type": req.FilterType,
// 			"intensity":   req.Intensity,
// 		},
// 	}

// 	if err := workerPool.Submit(job); err != nil {
// 		c.JSON(503, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(202, gin.H{
// 		"job_id":  job.JobID,
// 		"message": "Job submitted successfully",
// 	})
// }

// func handleConvert(c *gin.Context) {
// 	var req struct {
// 		ImageID string `json:"image_id"`
// 		Format  string `json:"format"`
// 		Quality int    `json:"quality"`
// 	}

// 	if err := c.BindJSON(&req); err != nil {
// 		c.JSON(400, gin.H{"error": err.Error()})
// 		return
// 	}

// 	job := worker.Job{
// 		JobID:     uuid.New().String(),
// 		ImageID:   req.ImageID,
// 		Operation: "convert",
// 		Parameters: map[string]interface{}{
// 			"format":  req.Format,
// 			"quality": req.Quality,
// 		},
// 	}

// 	if err := workerPool.Submit(job); err != nil {
// 		c.JSON(503, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(202, gin.H{
// 		"job_id":  job.JobID,
// 		"message": "Job submitted successfully",
// 	})
// }

// // get job result
// func handleGetJobResult(c *gin.Context) {
// 	jobID := c.Param("job_id")

// 	result, found := workerPool.GetResult(jobID)
// 	if !found {
// 		c.JSON(404, gin.H{
// 			"job_id":  jobID,
// 			"status":  "pending",
// 			"message": "Job is still processing or not found",
// 		})
// 		return
// 	}

// 	c.JSON(200, result)
// }

// // worker statistics
//
//	func handleWorkerStats(c *gin.Context) {
//		c.JSON(200, gin.H{
//			"queue_size":     workerPool.GetQueueSize(),
//			"queue_capacity": workerPool.GetQueueCapacity(),
//			"queue_usage":    float64(workerPool.GetQueueSize()) / float64(workerPool.GetQueueCapacity()) * 100,
//		})
//	}
package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	"github.com/amandeep2102/image-processor/backend/cache"
	"github.com/amandeep2102/image-processor/backend/worker"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var (
	db         *sql.DB
	workerPool *worker.Pool
	imageCache *cache.ImageCache
)

func main() {
	var err error
	db, err = sql.Open("postgres",
		"host=localhost port=5432 user=imageuser password=imagepass dbname=imagedb sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize cache
	imageCache = cache.NewImageCache(10)
	log.Println("✓ Image cache initialized with capacity: 10")

	// Initialize worker pool with queue (20 workers)
	workerPool = worker.NewPool(20, db, imageCache)
	workerPool.Start()
	defer workerPool.Stop()

	r := gin.Default()

	// Health check
	r.GET("/health", handleHealth)

	// ============ SYNCHRONOUS PROCESSING ENDPOINTS ============
	r.POST("/process/resize", handleResize)
	r.POST("/process/thumbnail", handleThumbnail)
	r.POST("/process/filter", handleFilter)
	r.POST("/process/convert", handleConvert)

	// Worker and cache stats
	r.GET("/workers/stats", handleWorkerStats)
	r.GET("/cache/stats", handleCacheStats)
	r.POST("/cache", handleCache)
	r.DELETE("/cache", handleCacheClear)

	log.Println("Backend server starting on :8081")
	log.Println("✓ Queue-based worker pool: 50 workers, 1000 queue capacity")
	log.Println("✓ Synchronous processing endpoints")

	r.Run(":8081")
}

func handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":         "ok",
		"queue_size":     workerPool.GetQueueSize(),
		"queue_capacity": workerPool.GetQueueCapacity(),
		"completed_jobs": workerPool.GetCompletedJobs(),
		"failed_jobs":    workerPool.GetFailedJobs(),
		"total_workers":  workerPool.GetWorkerCount(),
		"cache_size":     imageCache.Size(),
		"cache_capacity": imageCache.Capacity(),
	})
}

// ============ SYNCHRONOUS HANDLERS ============

func handleResize(c *gin.Context) {
	var req struct {
		ImageID string `json:"image_id"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	params := map[string]interface{}{
		"width":  req.Width,
		"height": req.Height,
	}

	result, err := workerPool.SubmitAndWait(req.ImageID, "resize", params)
	if err != nil {
		c.JSON(503, gin.H{
			"error":   "Processing failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, result)
}

func handleThumbnail(c *gin.Context) {
	var req struct {
		ImageID string `json:"image_id"`
		Size    int    `json:"size"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	params := map[string]interface{}{
		"size": req.Size,
	}

	result, err := workerPool.SubmitAndWait(req.ImageID, "thumbnail", params)
	if err != nil {
		c.JSON(503, gin.H{
			"error":   "Processing failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, result)
}

func handleFilter(c *gin.Context) {
	var req struct {
		ImageID    string  `json:"image_id"`
		FilterType string  `json:"filter_type"`
		Intensity  float64 `json:"intensity"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	params := map[string]interface{}{
		"filter_type": req.FilterType,
		"intensity":   req.Intensity,
	}

	result, err := workerPool.SubmitAndWait(req.ImageID, "filter", params)
	if err != nil {
		c.JSON(503, gin.H{
			"error":   "Processing failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, result)
}

func handleConvert(c *gin.Context) {
	var req struct {
		ImageID string `json:"image_id"`
		Format  string `json:"format"`
		Quality int    `json:"quality"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	params := map[string]interface{}{
		"format":  req.Format,
		"quality": req.Quality,
	}

	result, err := workerPool.SubmitAndWait(req.ImageID, "convert", params)
	if err != nil {
		c.JSON(503, gin.H{
			"error":   "Processing failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, result)
}

func handleWorkerStats(c *gin.Context) {
	c.JSON(200, gin.H{
		"queue_size":      workerPool.GetQueueSize(),
		"queue_capacity":  workerPool.GetQueueCapacity(),
		"queue_usage_pct": float64(workerPool.GetQueueSize()) / float64(workerPool.GetQueueCapacity()) * 100,
		"completed_jobs":  workerPool.GetCompletedJobs(),
		"failed_jobs":     workerPool.GetFailedJobs(),
		"total_workers":   workerPool.GetWorkerCount(),
	})
}

func handleCacheStats(c *gin.Context) {
	stats := imageCache.GetStats()
	c.JSON(200, stats)
}

func handleCache(c *gin.Context) {
	var req struct {
		ImageID  string   `json:"image_id"`
		ImageIDs []string `json:"image_ids"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	imagesToCache := []string{}
	if req.ImageID != "" {
		imagesToCache = append(imagesToCache, req.ImageID)
	}
	imagesToCache = append(imagesToCache, req.ImageIDs...)

	if len(imagesToCache) == 0 {
		c.JSON(400, gin.H{"error": "No image_id provided"})
		return
	}

	cachedCount := 0
	for _, imageID := range imagesToCache {
		var imagePath string
		err := db.QueryRow("SELECT original_path FROM images WHERE id = $1", imageID).Scan(&imagePath)
		if err != nil {
			continue
		}

		imageData, err := os.ReadFile(imagePath)
		if err != nil {
			continue
		}

		imageCache.Set(imageID, imageData, 30*time.Minute)
		cachedCount++
	}

	c.JSON(200, gin.H{
		"cached":         cachedCount,
		"requested":      len(imagesToCache),
		"cache_size":     imageCache.Size(),
		"cache_capacity": imageCache.Capacity(),
	})
}

func handleCacheClear(c *gin.Context) {
	imageCache.Clear()
	c.JSON(200, gin.H{"message": "Cache cleared"})
}
