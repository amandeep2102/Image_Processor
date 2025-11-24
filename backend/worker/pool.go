// package worker

// import (
// 	"database/sql"
// 	"fmt"
// 	"log"
// 	"os"
// 	"sync"
// 	"time"

// 	"github.com/amandeep2102/image-processor/backend/cache"
// 	"github.com/amandeep2102/image-processor/backend/processor"
// )

// type Job struct {
// 	JobID      string // unique job ID
// 	ImageID    string
// 	Operation  string
// 	Parameters map[string]interface{}
// }

// type Result struct {
// 	JobID            string `json:"job_id"`
// 	Success          bool   `json:"success"`
// 	ProcessedID      string `json:"processed_id,omitempty"`
// 	Message          string `json:"message,omitempty"`
// 	ProcessingTimeMs int64  `json:"processing_time_ms"`
// 	WorkerID         int    `json:"worker_id"` // track which worker processed it
// 	CacheHit         bool   `json:"cache_hit"` // track if image was loaded from cache
// }

// type Pool struct {
// 	workers    int
// 	jobQueue   chan Job
// 	resultMap  sync.Map // store results by job ID
// 	db         *sql.DB
// 	imageCache *cache.ImageCache // add cache reference
// 	wg         sync.WaitGroup
// 	stopChan   chan struct{}
// }

// func NewPool(workers int, db *sql.DB, imageCache *cache.ImageCache) *Pool {
// 	return &Pool{
// 		workers:    workers,
// 		jobQueue:   make(chan Job, 100), // buffer for 100 pending jobs
// 		db:         db,
// 		imageCache: imageCache,
// 		stopChan:   make(chan struct{}),
// 	}
// }

// func (p *Pool) Start() {
// 	for i := 0; i < p.workers; i++ {
// 		p.wg.Add(1)
// 		go p.worker(i)
// 	}
// 	log.Printf("Started %d workers\n", p.workers)
// }

// func (p *Pool) Stop() {
// 	close(p.jobQueue) // close queue first to let workers finish
// 	p.wg.Wait()       // wait for all workers to complete
// 	close(p.stopChan)
// 	log.Println("All workers stopped")
// }

// // this is asynchronous
// func (p *Pool) Submit(job Job) error {
// 	// check if queue is full
// 	if len(p.jobQueue) >= cap(p.jobQueue) {
// 		return fmt.Errorf("job queue is full (%d/%d)", len(p.jobQueue), cap(p.jobQueue))
// 	}

// 	// sendd job to channel (workers will take it from there)
// 	p.jobQueue <- job

// 	log.Printf("Job %s submitted to queue (queue size: %d/%d)",
// 		job.JobID, len(p.jobQueue), cap(p.jobQueue))

// 	return nil
// }

// // this is synchronous
// // func (p *Pool) SubmitAndWait(job Job, timeout time.Duration) (Result, error) {
// // 	// Submit to queue
// // 	if err := p.Submit(job); err != nil {
// // 		return Result{}, err
// // 	}

// // 	// Wait for result with timeout
// // 	deadline := time.Now().Add(timeout)
// // 	ticker := time.NewTicker(100 * time.Millisecond)
// // 	defer ticker.Stop()

// // 	for {
// // 		// Check if result is ready
// // 		if value, ok := p.resultMap.Load(job.JobID); ok {
// // 			result := value.(Result)
// // 			p.resultMap.Delete(job.JobID) // Clean up
// // 			return result, nil
// // 		}

// // 		// Check timeout
// // 		if time.Now().After(deadline) {
// // 			return Result{}, fmt.Errorf("job timeout after %v", timeout)
// // 		}

// // 		// Wait a bit before checking again
// // 		<-ticker.C
// // 	}
// // }

// func (p *Pool) GetResult(jobID string) (Result, bool) {
// 	if value, ok := p.resultMap.Load(jobID); ok {
// 		return value.(Result), true
// 	}
// 	return Result{}, false
// }

// func (p *Pool) worker(workerID int) {
// 	defer p.wg.Done()

// 	log.Printf("Worker %d started\n", workerID)

// 	for job := range p.jobQueue {
// 		log.Printf("Worker %d picked up job %s (operation: %s)",
// 			workerID, job.JobID, job.Operation)

// 		// process the job
// 		result := p.processJob(job, workerID)

// 		// store result in map for retrieval
// 		p.resultMap.Store(job.JobID, result)

// 		log.Printf("Worker %d completed job %s in %dms (cache_hit: %v)",
// 			workerID, job.JobID, result.ProcessingTimeMs, result.CacheHit)
// 	}

// 	log.Printf("Worker %d stopped\n", workerID)
// }

// // retrieves image data from cache or disk
// func (p *Pool) getImageData(imageID string) ([]byte, bool, error) {
// 	// Try cache first
// 	if imageData, found := p.imageCache.Get(imageID); found {
// 		log.Printf(" YES!! Cache HIT for image: %s", imageID)
// 		return imageData, true, nil
// 	}

// 	log.Printf(" NO!! Cache MISS for image: %s - loading from disk", imageID)

// 	// cache miss - load from disk
// 	var imagePath string
// 	err := p.db.QueryRow("SELECT original_path FROM images WHERE id = $1", imageID).Scan(&imagePath)
// 	if err != nil {
// 		return nil, false, fmt.Errorf("failed to get image path from database: %v", err)
// 	}

// 	imageData, err := os.ReadFile(imagePath)
// 	if err != nil {
// 		return nil, false, fmt.Errorf("failed to read image from disk: %v", err)
// 	}

// 	// cache it for next time (with 30 min TTL)
// 	p.imageCache.Set(imageID, imageData, 30*time.Minute)
// 	log.Printf("Cached image %s for future use (%d bytes)", imageID, len(imageData))

// 	return imageData, false, nil
// }

// func (p *Pool) processJob(job Job, workerID int) Result {
// 	startTime := time.Now()

// 	// get image data (from cache or disk)
// 	imageData, cacheHit, err := p.getImageData(job.ImageID)
// 	if err != nil {
// 		return Result{
// 			JobID:            job.JobID,
// 			Success:          false,
// 			Message:          err.Error(),
// 			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
// 			WorkerID:         workerID,
// 			CacheHit:         false,
// 		}
// 	}

// 	var outputPath string

// 	switch job.Operation {
// 	case "resize":
// 		outputPath, err = processor.Resize(imageData, job.ImageID, job.Parameters)
// 	case "thumbnail":
// 		outputPath, err = processor.Thumbnail(imageData, job.ImageID, job.Parameters)
// 	case "filter":
// 		outputPath, err = processor.Filter(imageData, job.ImageID, job.Parameters)
// 	case "convert":
// 		outputPath, err = processor.Convert(imageData, job.ImageID, job.Parameters)
// 	default:
// 		return Result{
// 			JobID:            job.JobID,
// 			Success:          false,
// 			Message:          "Unknown operation",
// 			ProcessingTimeMs: 0,
// 			WorkerID:         workerID,
// 			CacheHit:         cacheHit,
// 		}
// 	}

// 	if err != nil {
// 		return Result{
// 			JobID:            job.JobID,
// 			Success:          false,
// 			Message:          err.Error(),
// 			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
// 			WorkerID:         workerID,
// 			CacheHit:         cacheHit,
// 		}
// 	}

// 	// Save processed image record
// 	processedID, err := p.saveProcessedImage(job, outputPath, time.Since(startTime).Milliseconds())
// 	if err != nil {
// 		log.Printf("Worker %d: Error saving processed image: %v\n", workerID, err)
// 	}

// 	return Result{
// 		JobID:            job.JobID,
// 		Success:          true,
// 		ProcessedID:      processedID,
// 		Message:          "Processing completed",
// 		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
// 		WorkerID:         workerID,
// 		CacheHit:         cacheHit,
// 	}
// }

// func (p *Pool) saveProcessedImage(job Job, outputPath string, processingTime int64) (string, error) {
// 	var id string
// 	err := p.db.QueryRow(`
//         INSERT INTO processed_images (original_image_id, operation_type, processed_path, parameters, processing_time_ms)
//         VALUES ($1, $2, $3, $4, $5)
//         RETURNING id
//     `, job.ImageID, job.Operation, outputPath, nil, processingTime).Scan(&id)

// 	return id, err
// }

// func (p *Pool) GetQueueSize() int {
// 	return len(p.jobQueue)
// }

// func (p *Pool) GetQueueCapacity() int {
// 	return cap(p.jobQueue)
// }
// ============ REFACTORED pool.go (No Queue) ============

package worker

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amandeep2102/image-processor/backend/cache"
	"github.com/amandeep2102/image-processor/backend/processor"
)

type Job struct {
	ImageID    string
	Operation  string
	Parameters map[string]interface{}
	resultChan chan Result // Channel for client to receive result
}

type Result struct {
	Success          bool   `json:"success"`
	ProcessedID      string `json:"processed_id,omitempty"`
	Message          string `json:"message,omitempty"`
	ProcessingTimeMs int64  `json:"processing_time_ms"`
	CacheHit         bool   `json:"cache_hit"`
	CompletedAt      int64  `json:"completed_at"`
}

type Pool struct {
	workers    int
	db         *sql.DB
	imageCache *cache.ImageCache
	jobQueue   chan Job
	wg         sync.WaitGroup
	stopChan   chan struct{}

	// Statistics
	activeJobs    int64
	completedJobs int64
	failedJobs    int64
	queuedJobs    int64
}

func NewPool(workers int, db *sql.DB, imageCache *cache.ImageCache) *Pool {
	return &Pool{
		workers:    workers,
		db:         db,
		imageCache: imageCache,
		jobQueue:   make(chan Job, 1000), // Queue capacity of 1000 jobs
		stopChan:   make(chan struct{}),
	}
}

// Start spawns worker goroutines
func (p *Pool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	log.Printf("Worker pool started with %d workers, queue capacity: 1000\n", p.workers)
}

// Stop gracefully shuts down the worker pool
func (p *Pool) Stop() {
	close(p.jobQueue)
	p.wg.Wait()
	close(p.stopChan)
	log.Println("Worker pool stopped")
}

// SubmitAndWait submits a job and waits for result (blocking call)
func (p *Pool) SubmitAndWait(imageID string, operation string, params map[string]interface{}) (Result, error) {
	// Create result channel for this job
	resultChan := make(chan Result, 1)

	job := Job{
		ImageID:    imageID,
		Operation:  operation,
		Parameters: params,
		resultChan: resultChan,
	}

	// Try to queue the job
	select {
	case p.jobQueue <- job:
		atomic.AddInt64(&p.queuedJobs, 1)
		log.Printf("Job queued: operation=%s, image=%s (queue size: %d/%d)",
			operation, imageID, len(p.jobQueue), cap(p.jobQueue))
	case <-p.stopChan:
		return Result{}, fmt.Errorf("worker pool is shutting down")
	}

	// Wait for result with timeout
	select {
	case result := <-resultChan:
		return result, nil
	case <-time.After(120 * time.Second):
		return Result{}, fmt.Errorf("job processing timeout (120 seconds)")
	case <-p.stopChan:
		return Result{}, fmt.Errorf("worker pool shutdown while processing")
	}
}

// Worker goroutine processes jobs from queue
func (p *Pool) worker(workerID int) {
	defer p.wg.Done()
	log.Printf("Worker %d started\n", workerID)

	for job := range p.jobQueue {
		log.Printf("Worker %d processing: operation=%s, image=%s",
			workerID, job.Operation, job.ImageID)

		// Process the job
		result := p.processJob(job)

		// Send result to client
		select {
		case job.resultChan <- result:
			log.Printf("Worker %d: Result sent to client", workerID)
		case <-time.After(5 * time.Second):
			log.Printf("Worker %d: Client no longer waiting for result", workerID)
		}

		// Close the result channel
		close(job.resultChan)
	}

	log.Printf("Worker %d stopped\n", workerID)
}

// ============ Job Processing ============
func (p *Pool) processJob(job Job) Result {
	startTime := time.Now()

	// Retrieve image data
	imageData, cacheHit, err := p.getImageData(job.ImageID)
	if err != nil {
		atomic.AddInt64(&p.failedJobs, 1)
		return Result{
			Success:          false,
			Message:          err.Error(),
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			CacheHit:         false,
			CompletedAt:      time.Now().Unix(),
		}
	}

	var outputPath string

	// Process based on operation type
	switch job.Operation {
	case "resize":
		outputPath, err = processor.Resize(imageData, job.ImageID, job.Parameters)
	case "thumbnail":
		outputPath, err = processor.Thumbnail(imageData, job.ImageID, job.Parameters)
	case "filter":
		outputPath, err = processor.Filter(imageData, job.ImageID, job.Parameters)
	case "convert":
		outputPath, err = processor.Convert(imageData, job.ImageID, job.Parameters)
	default:
		atomic.AddInt64(&p.failedJobs, 1)
		return Result{
			Success:          false,
			Message:          "Unknown operation: " + job.Operation,
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			CacheHit:         cacheHit,
			CompletedAt:      time.Now().Unix(),
		}
	}

	if err != nil {
		atomic.AddInt64(&p.failedJobs, 1)
		return Result{
			Success:          false,
			Message:          err.Error(),
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			CacheHit:         cacheHit,
			CompletedAt:      time.Now().Unix(),
		}
	}

	// Save metadata
	processedID, err := p.saveProcessedImage(job, outputPath, time.Since(startTime).Milliseconds())
	if err != nil {
		log.Printf("Error saving processed image metadata: %v\n", err)
	}

	atomic.AddInt64(&p.completedJobs, 1)

	return Result{
		Success:          true,
		ProcessedID:      processedID,
		Message:          "Processing completed",
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		CacheHit:         cacheHit,
		CompletedAt:      time.Now().Unix(),
	}
}

// Retrieve image from cache or disk
func (p *Pool) getImageData(imageID string) ([]byte, bool, error) {
	// Try cache first
	if imageData, found := p.imageCache.Get(imageID); found {
		log.Printf("Cache HIT for image: %s", imageID)
		return imageData, true, nil
	}

	log.Printf("Cache MISS for image: %s - loading from disk", imageID)

	// Query database for image path
	var imagePath string
	err := p.db.QueryRow("SELECT original_path FROM images WHERE id = $1", imageID).Scan(&imagePath)
	if err != nil {
		return nil, false, fmt.Errorf("image not found in database: %v", err)
	}

	// Read from disk
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read image from disk: %v", err)
	}

	// Cache for future use
	p.imageCache.Set(imageID, imageData, 30*time.Minute)
	log.Printf("Cached image %s for future use (%d bytes)", imageID, len(imageData))

	return imageData, false, nil
}

// Save processed image metadata to database
func (p *Pool) saveProcessedImage(job Job, outputPath string, processingTime int64) (string, error) {
	var id string
	err := p.db.QueryRow(`
        INSERT INTO processed_images (original_image_id, operation_type, processed_path, parameters, processing_time_ms)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
    `, job.ImageID, job.Operation, outputPath, nil, processingTime).Scan(&id)

	return id, err
}

// ============ Statistics ============
func (p *Pool) GetActiveJobs() int64 {
	return atomic.LoadInt64(&p.activeJobs)
}

func (p *Pool) GetCompletedJobs() int64 {
	return atomic.LoadInt64(&p.completedJobs)
}

func (p *Pool) GetFailedJobs() int64 {
	return atomic.LoadInt64(&p.failedJobs)
}

func (p *Pool) GetQueuedJobs() int64 {
	return atomic.LoadInt64(&p.queuedJobs)
}

func (p *Pool) GetWorkerCount() int {
	return p.workers
}

func (p *Pool) GetQueueSize() int {
	return len(p.jobQueue)
}

func (p *Pool) GetQueueCapacity() int {
	return cap(p.jobQueue)
}

// Deprecated methods (kept for backward compatibility)
func (p *Pool) Submit(job Job) error {
	return fmt.Errorf("use SubmitAndWait instead")
}

func (p *Pool) GetResult(jobID string) (Result, bool) {
	return Result{}, false
}

func (p *Pool) GetAvailableWorkers() int {
	return 0
}
