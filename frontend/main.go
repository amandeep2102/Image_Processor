package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"

	// "fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

var (
	db              *sql.DB
	backendURL      = "http://localhost:8081"
	storageBasePath = "/home/polarbeer/Documents/Image-Processor/frontend/storage/uploads"
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

	// Create storage directory
	os.MkdirAll(storageBasePath, 0755)

	// Setup router
	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Upload/Download endpoints (I/O-bound)
	r.POST("/upload", handleUpload)
	r.GET("/image/:id", handleDownload)
	r.GET("/image/:id/processed/:processed_id", handleDownloadProcessed)
	r.GET("/images", handleListImages)
	r.DELETE("/image/:id", handleDelete)

	// Processing endpoints (forward to backend - CPU-bound)
	r.POST("/process/resize", forwardToBackend)
	r.POST("/process/thumbnail", forwardToBackend)
	r.POST("/process/filter", forwardToBackend)
	r.POST("/process/convert", forwardToBackend)
	r.POST("/process/batch", forwardToBackend)

	// Stats endpoint
	r.GET("/stats", handleStats)

	log.Println("Frontend server starting on :8080")
	r.Run(":8080")
}

func handleUpload(c *gin.Context) {
	startTime := time.Now()

	// Parse multipart form (I/O-bound)
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(400, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// Generate unique ID
	imageID := uuid.New().String()
	uploadedBy := c.DefaultPostForm("client_id", "anonymous")

	// Save file to disk (I/O-bound)
	ext := filepath.Ext(header.Filename)
	filename := imageID + ext
	filepath := filepath.Join(storageBasePath, filename)

	out, err := os.Create(filepath)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to save file"})
		return
	}
	defer out.Close()

	size, err := io.Copy(out, file)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to save file"})
		return
	}

	// Get image dimensions (optional, CPU-bound)
	// For simplicity, we'll skip this for now

	// Save metadata to database (I/O-bound)
	_, err = db.Exec(`
        INSERT INTO images (id, filename, original_path, content_type, size_bytes, uploaded_by, status)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, imageID, header.Filename, filepath, header.Header.Get("Content-Type"), size, uploadedBy, "uploaded")

	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to save metadata"})
		return
	}

	processingTime := time.Since(startTime).Milliseconds()

	c.JSON(200, gin.H{
		"id":             imageID,
		"filename":       header.Filename,
		"size":           size,
		"uploaded_by":    uploadedBy,
		"upload_time_ms": processingTime,
	})
}

func handleDownload(c *gin.Context) {
	imageID := c.Param("id")

	// Get image path from database (I/O-bound)
	var filename, filepath, contentType string
	err := db.QueryRow("SELECT original_path, content_type, filename FROM images WHERE id = $1", imageID).
		Scan(&filepath, &contentType, &filename)

	if err != nil {
		c.JSON(404, gin.H{"error": "Image not found"})
		return
	}

	// Serve file (I/O-bound)
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	// fmt.Println(filename)
	c.File(filepath)
}

func handleDownloadProcessed(c *gin.Context) {
	imageID := c.Param("id")
	processedID := c.Param("processed_id")

	var FilePath string
	err := db.QueryRow(`
        SELECT processed_path FROM processed_images 
        WHERE id = $1 AND original_image_id = $2
    `, processedID, imageID).Scan(&FilePath)

	if err != nil {
		c.JSON(404, gin.H{"error": "Processed image not found"})
		return
	}

	filename := filepath.Base(FilePath)
	fmt.Println(filename + "this iss tthe filename ")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(FilePath)))
	c.File(FilePath)
}

func handleListImages(c *gin.Context) {
	// Query all images (I/O-bound)
	rows, err := db.Query(`
        SELECT id, filename, size_bytes, uploaded_by, uploaded_at, status
        FROM images
        ORDER BY uploaded_at DESC
    `)
	if err != nil {
		c.JSON(500, gin.H{"error": "Database query failed"})
		return
	}
	defer rows.Close()

	images := make([]map[string]interface{}, 0)
	for rows.Next() {
		var id, filename, uploadedBy, status string
		var sizeBytes int64
		var uploadedAt time.Time

		err := rows.Scan(&id, &filename, &sizeBytes, &uploadedBy, &uploadedAt, &status)
		if err != nil {
			continue
		}

		images = append(images, map[string]interface{}{
			"id":          id,
			"filename":    filename,
			"size_bytes":  sizeBytes,
			"uploaded_by": uploadedBy,
			"uploaded_at": uploadedAt,
			"status":      status,
		})
	}

	c.JSON(200, gin.H{"images": images, "count": len(images)})
}

func handleDelete(c *gin.Context) {
	imageID := c.Param("id")

	// Get file path
	var filepath string
	err := db.QueryRow("SELECT original_path FROM images WHERE id = $1", imageID).Scan(&filepath)
	if err != nil {
		c.JSON(404, gin.H{"error": "Image not found"})
		return
	}

	// Get all processed file paths before cascading delete
	rows, err := db.Query("SELECT processed_path FROM processed_images WHERE original_image_id = $1", imageID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch processed image paths"})
		return
	}
	defer rows.Close()

	processedPaths := []string{}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err == nil {
			processedPaths = append(processedPaths, path)
		}
	}

	// Delete from database (cascades to processed_images)
	_, err = db.Exec("DELETE FROM images WHERE id = $1", imageID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete from database"})
		return
	}

	// Delete all processed files
	for _, p := range processedPaths {
		if err := os.Remove(p); err != nil {
			log.Printf("Warning: failed to delete processed file %s: %v", p, err)
		}
	}

	// Delete file from disk
	os.Remove(filepath)

	c.JSON(200, gin.H{"message": "Image deleted successfully"})
}

func forwardToBackend(c *gin.Context) {
	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to read request"})
		return
	}

	// Forward to backend
	url := backendURL + c.Request.URL.Path
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(500, gin.H{"error": "Backend request failed"})
		return
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to read response"})
		return
	}

	// Forward response
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	c.JSON(resp.StatusCode, result)
}

func handleStats(c *gin.Context) {
	// Get statistics (I/O-bound database queries)
	var totalImages, totalProcessed int
	var totalSize int64

	db.QueryRow("SELECT COUNT(*), COALESCE(SUM(size_bytes), 0) FROM images").Scan(&totalImages, &totalSize)
	db.QueryRow("SELECT COUNT(*) FROM processed_images").Scan(&totalProcessed)

	// Get average processing time by operation
	rows, err := db.Query(`
        SELECT operation_type, 
               COUNT(*) as count,
               AVG(processing_time_ms) as avg_time,
               MIN(processing_time_ms) as min_time,
               MAX(processing_time_ms) as max_time
        FROM processed_images
        GROUP BY operation_type
    `)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get stats"})
		return
	}
	defer rows.Close()

	operations := make([]map[string]interface{}, 0)
	for rows.Next() {
		var opType string
		var count int
		var avgTime, minTime, maxTime float64

		rows.Scan(&opType, &count, &avgTime, &minTime, &maxTime)
		operations = append(operations, map[string]interface{}{
			"operation":   opType,
			"count":       count,
			"avg_time_ms": avgTime,
			"min_time_ms": minTime,
			"max_time_ms": maxTime,
		})
	}

	c.JSON(200, gin.H{
		"total_images":     totalImages,
		"total_processed":  totalProcessed,
		"total_size_bytes": totalSize,
		"operations":       operations,
	})
}
