package main

import (
	"database/sql"
	"log"

	"github.com/amandeep2102/image-processor/backend/worker"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var (
	db         *sql.DB
	workerPool *worker.Pool
)

func main() {
	// Connect to database
	var err error
	db, err = sql.Open("postgres",
		"host=localhost port=5432 user=imageuser password=imagepass dbname=imagedb sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize worker pool
	workerPool = worker.NewPool(10, db) // 10 workers
	workerPool.Start()
	defer workerPool.Stop()

	// Setup router
	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Process endpoints
	r.POST("/process/resize", handleResize)
	r.POST("/process/thumbnail", handleThumbnail)
	r.POST("/process/filter", handleFilter)
	r.POST("/process/convert", handleConvert)
	r.POST("/process/batch", handleBatch)

	log.Println("Backend server starting on :8081")
	r.Run(":8081")
}

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

	// Submit job to worker pool
	job := worker.Job{
		ImageID:   req.ImageID,
		Operation: "resize",
		Parameters: map[string]interface{}{
			"width":  req.Width,
			"height": req.Height,
		},
	}

	result := workerPool.Submit(job)
	c.JSON(200, result)
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
		ImageID:   req.ImageID,
		Operation: "thumbnail",
		Parameters: map[string]interface{}{
			"size": req.Size,
		},
	}

	result := workerPool.Submit(job)
	c.JSON(200, result)
}

func handleFilter(c *gin.Context) {
	var req struct {
		ImageID    string  `json:"image_id"`
		FilterType string  `json:"filter_type"` // blur, sharpen, grayscale
		Intensity  float64 `json:"intensity"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	job := worker.Job{
		ImageID:   req.ImageID,
		Operation: "filter",
		Parameters: map[string]interface{}{
			"filter_type": req.FilterType,
			"intensity":   req.Intensity,
		},
	}

	result := workerPool.Submit(job)
	c.JSON(200, result)
}

func handleConvert(c *gin.Context) {
	var req struct {
		ImageID string `json:"image_id"`
		Format  string `json:"format"` // jpeg, png, webp
		Quality int    `json:"quality"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	job := worker.Job{
		ImageID:   req.ImageID,
		Operation: "convert",
		Parameters: map[string]interface{}{
			"format":  req.Format,
			"quality": req.Quality,
		},
	}

	result := workerPool.Submit(job)
	c.JSON(200, result)
}

func handleBatch(c *gin.Context) {
	var req struct {
		ImageIDs   []string               `json:"image_ids"`
		Operation  string                 `json:"operation"`
		Parameters map[string]interface{} `json:"parameters"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	results := make([]interface{}, 0)
	for _, imageID := range req.ImageIDs {
		job := worker.Job{
			ImageID:    imageID,
			Operation:  req.Operation,
			Parameters: req.Parameters,
		}
		result := workerPool.Submit(job)
		results = append(results, result)
	}

	c.JSON(200, gin.H{"results": results})
}
