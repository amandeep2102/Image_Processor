package worker

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/amandeep2102/image-processor/backend/processor"
)

type Job struct {
	JobID      string // Add unique job ID
	ImageID    string
	Operation  string
	Parameters map[string]interface{}
}

type Result struct {
	JobID            string `json:"job_id"`
	Success          bool   `json:"success"`
	ProcessedID      string `json:"processed_id,omitempty"`
	Message          string `json:"message,omitempty"`
	ProcessingTimeMs int64  `json:"processing_time_ms"`
	WorkerID         int    `json:"worker_id"` // Track which worker processed it
}

type Pool struct {
	workers   int
	jobQueue  chan Job
	resultMap sync.Map // Store results by job ID
	db        *sql.DB
	wg        sync.WaitGroup
	stopChan  chan struct{}
}

func NewPool(workers int, db *sql.DB) *Pool {
	return &Pool{
		workers:  workers,
		jobQueue: make(chan Job, 100), // Buffer for 100 pending jobs
		db:       db,
		stopChan: make(chan struct{}),
	}
}

func (p *Pool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	log.Printf("Started %d workers\n", p.workers)
}

func (p *Pool) Stop() {
	close(p.jobQueue) // Close queue first to let workers finish
	p.wg.Wait()       // Wait for all workers to complete
	close(p.stopChan)
	log.Println("All workers stopped")
}

// Submit adds job to queue (non-blocking, async)
func (p *Pool) Submit(job Job) error {
	// Check if queue is full
	if len(p.jobQueue) >= cap(p.jobQueue) {
		return fmt.Errorf("job queue is full (%d/%d)", len(p.jobQueue), cap(p.jobQueue))
	}

	// Send job to channel (workers will pick it up)
	p.jobQueue <- job

	log.Printf("Job %s submitted to queue (queue size: %d/%d)",
		job.JobID, len(p.jobQueue), cap(p.jobQueue))

	return nil
}

// SubmitAndWait submits job and waits for result (blocking, sync)
func (p *Pool) SubmitAndWait(job Job, timeout time.Duration) (Result, error) {
	// Submit to queue
	if err := p.Submit(job); err != nil {
		return Result{}, err
	}

	// Wait for result with timeout
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Check if result is ready
		if value, ok := p.resultMap.Load(job.JobID); ok {
			result := value.(Result)
			p.resultMap.Delete(job.JobID) // Clean up
			return result, nil
		}

		// Check timeout
		if time.Now().After(deadline) {
			return Result{}, fmt.Errorf("job timeout after %v", timeout)
		}

		// Wait a bit before checking again
		<-ticker.C
	}
}

// GetResult retrieves result for a job ID (non-blocking)
func (p *Pool) GetResult(jobID string) (Result, bool) {
	if value, ok := p.resultMap.Load(jobID); ok {
		return value.(Result), true
	}
	return Result{}, false
}

func (p *Pool) worker(workerID int) {
	defer p.wg.Done()

	log.Printf("Worker %d started\n", workerID)

	for job := range p.jobQueue {
		log.Printf("Worker %d picked up job %s (operation: %s)",
			workerID, job.JobID, job.Operation)

		// Process the job
		result := p.processJob(job, workerID)

		// Store result in map for retrieval
		p.resultMap.Store(job.JobID, result)

		log.Printf("Worker %d completed job %s in %dms",
			workerID, job.JobID, result.ProcessingTimeMs)
	}

	log.Printf("Worker %d stopped\n", workerID)
}

func (p *Pool) processJob(job Job, workerID int) Result {
	startTime := time.Now()

	var err error
	var outputPath string

	switch job.Operation {
	case "resize":
		outputPath, err = processor.Resize(p.db, job.ImageID, job.Parameters)
	case "thumbnail":
		outputPath, err = processor.Thumbnail(p.db, job.ImageID, job.Parameters)
	case "filter":
		outputPath, err = processor.ApplyFilter(p.db, job.ImageID, job.Parameters)
	case "convert":
		outputPath, err = processor.Convert(p.db, job.ImageID, job.Parameters)
	default:
		return Result{
			JobID:            job.JobID,
			Success:          false,
			Message:          "Unknown operation",
			ProcessingTimeMs: 0,
			WorkerID:         workerID,
		}
	}

	if err != nil {
		return Result{
			JobID:            job.JobID,
			Success:          false,
			Message:          err.Error(),
			ProcessingTimeMs: time.Since(startTime).Milliseconds(),
			WorkerID:         workerID,
		}
	}

	// Save processed image record
	processedID, err := p.saveProcessedImage(job, outputPath, time.Since(startTime).Milliseconds())
	if err != nil {
		log.Printf("Worker %d: Error saving processed image: %v\n", workerID, err)
	}

	return Result{
		JobID:            job.JobID,
		Success:          true,
		ProcessedID:      processedID,
		Message:          "Processing completed",
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		WorkerID:         workerID,
	}
}

func (p *Pool) saveProcessedImage(job Job, outputPath string, processingTime int64) (string, error) {
	var id string
	err := p.db.QueryRow(`
        INSERT INTO processed_images (original_image_id, operation_type, processed_path, parameters, processing_time_ms)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id
    `, job.ImageID, job.Operation, outputPath, nil, processingTime).Scan(&id)

	return id, err
}

// GetQueueSize returns current queue size
func (p *Pool) GetQueueSize() int {
	return len(p.jobQueue)
}

// GetQueueCapacity returns queue capacity
func (p *Pool) GetQueueCapacity() int {
	return cap(p.jobQueue)
}
