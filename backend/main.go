package main

import (
	"database/sql"
	"log"

	"github.com/amandeep2102/image-processor/backend/worker"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

var (
	db         *sql.DB
	workerPool *worker.Pool
)

func main() {
	var err error
	db, err = sql.Open("postgres",
		"host=localhost port=5432 user=imageuser password=imagepass dbname=imagedb sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize worker pool with 10 workers
	workerPool = worker.NewPool(10, db)
	workerPool.Start()
	defer workerPool.Stop()

	r := gin.Default()

	r.GET("/health", handleHealth)
	r.POST("/process/resize", handleResize)
	r.POST("/process/thumbnail", handleThumbnail)
	r.POST("/process/filter", handleFilter)
	r.POST("/process/convert", handleConvert)
	r.GET("/job/:job_id", handleGetJobResult)
	r.GET("/workers/stats", handleWorkerStats)

	log.Println("Backend server starting on :8081")
	r.Run(":8081")
}

func handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":         "ok",
		"queue_size":     workerPool.GetQueueSize(),
		"queue_capacity": workerPool.GetQueueCapacity(),
	})
}

// Async processing (returns immediately with job ID)
func handleResize(c *gin.Context) {
	var req struct {
		ImageID string `json:"image_id"`
		Width   int    `json:"width"`
		Height  int    `json:"height"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Create job with unique ID
	job := worker.Job{
		JobID:     uuid.New().String(),
		ImageID:   req.ImageID,
		Operation: "resize",
		Parameters: map[string]interface{}{
			"width":  req.Width,
			"height": req.Height,
		},
	}

	// Submit to worker pool (non-blocking)
	if err := workerPool.Submit(job); err != nil {
		c.JSON(503, gin.H{
			"error":   "Worker pool is busy",
			"message": err.Error(),
		})
		return
	}

	// Return immediately with job ID
	c.JSON(202, gin.H{
		"job_id":     job.JobID,
		"message":    "Job submitted successfully",
		"queue_size": workerPool.GetQueueSize(),
	})
}

func handleThumbnail(c *gin.Context) {
	var req struct {
		ImageID string `json:"image_id"`
		Size    int    `json:"size"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	job := worker.Job{
		JobID:     uuid.New().String(),
		ImageID:   req.ImageID,
		Operation: "thumbnail",
		Parameters: map[string]interface{}{
			"size": req.Size,
		},
	}

	if err := workerPool.Submit(job); err != nil {
		c.JSON(503, gin.H{"error": err.Error()})
		return
	}

	c.JSON(202, gin.H{
		"job_id":  job.JobID,
		"message": "Job submitted successfully",
	})
}

func handleFilter(c *gin.Context) {
	var req struct {
		ImageID    string  `json:"image_id"`
		FilterType string  `json:"filter_type"`
		Intensity  float64 `json:"intensity"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	job := worker.Job{
		JobID:     uuid.New().String(),
		ImageID:   req.ImageID,
		Operation: "filter",
		Parameters: map[string]interface{}{
			"filter_type": req.FilterType,
			"intensity":   req.Intensity,
		},
	}

	if err := workerPool.Submit(job); err != nil {
		c.JSON(503, gin.H{"error": err.Error()})
		return
	}

	c.JSON(202, gin.H{
		"job_id":  job.JobID,
		"message": "Job submitted successfully",
	})
}

func handleConvert(c *gin.Context) {
	var req struct {
		ImageID string `json:"image_id"`
		Format  string `json:"format"`
		Quality int    `json:"quality"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	job := worker.Job{
		JobID:     uuid.New().String(),
		ImageID:   req.ImageID,
		Operation: "convert",
		Parameters: map[string]interface{}{
			"format":  req.Format,
			"quality": req.Quality,
		},
	}

	if err := workerPool.Submit(job); err != nil {
		c.JSON(503, gin.H{"error": err.Error()})
		return
	}

	c.JSON(202, gin.H{
		"job_id":  job.JobID,
		"message": "Job submitted successfully",
	})
}

// NEW: Get job result
func handleGetJobResult(c *gin.Context) {
	jobID := c.Param("job_id")

	result, found := workerPool.GetResult(jobID)
	if !found {
		c.JSON(404, gin.H{
			"job_id":  jobID,
			"status":  "pending",
			"message": "Job is still processing or not found",
		})
		return
	}

	c.JSON(200, result)
}

// NEW: Worker statistics
func handleWorkerStats(c *gin.Context) {
	c.JSON(200, gin.H{
		"queue_size":     workerPool.GetQueueSize(),
		"queue_capacity": workerPool.GetQueueCapacity(),
		"queue_usage":    float64(workerPool.GetQueueSize()) / float64(workerPool.GetQueueCapacity()) * 100,
	})
}
