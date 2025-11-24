package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	FRONTEND_URL = "http://localhost:8080"
	BACKEND_URL  = "http://localhost:8081"
)

type Metrics struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalResponseTime  int64
	ResponseTimes      []int64
	mu                 sync.Mutex
}

type TestConfig struct {
	CachedImages      int
	ConcurrentClients int
	OperationsPerSec  int
	TestDuration      time.Duration
	ImageSize         int
	Operations        []string
	NumImages         int
	imageSize         int
}

var metrics Metrics

func main() {
	testType := flag.String("test", "cpu", "Test type: upload, cpu")
	cachedImages := flag.Int("cached-images", 5, "Number of images to cache (max 10)")
	OpPerSec := flag.Int("OpPerSec", 10, "Number of operations per sec")
	concurrent := flag.Int("concurrent", 20, "Number of concurrent clients")
	duration := flag.Duration("duration", 1*time.Minute, "Test duration")
	imageSize := flag.Int("image-size", 1024, "Image dimension (WxH)")
	numImages := flag.Int("images", 10, "Number of images to upload")

	flag.Parse()

	switch *testType {
	case "upload":
		config := TestConfig{
			NumImages:         *numImages,
			ConcurrentClients: *concurrent,
			ImageSize:         *imageSize,
			TestDuration:      *duration,
			OperationsPerSec:  *OpPerSec,
		}

		fmt.Printf("Configuration:\n")
		fmt.Printf("Concurrent clients: %d\n", config.ConcurrentClients)
		fmt.Printf("Test duration: %v\n", config.TestDuration)
		fmt.Printf("Image size: %dx%d\n\n", config.ImageSize, config.ImageSize)
		testUploadBottleneck(config)
	case "cpu":
		config := TestConfig{
			CachedImages:      *cachedImages,
			ConcurrentClients: *concurrent,
			TestDuration:      *duration,
			ImageSize:         *imageSize,
			Operations:        []string{"resize", "thumbnail", "filter", "convert"},
			OperationsPerSec:  *OpPerSec,
		}

		if config.CachedImages > 100 {
			config.CachedImages = 100
		}

		fmt.Printf("Configuration:\n")
		fmt.Printf("Cached images: %d\n", config.CachedImages)
		fmt.Printf("Concurrent clients: %d\n", config.ConcurrentClients)
		fmt.Printf("Operations/sec per client: %d\n", config.OperationsPerSec)
		fmt.Printf("Test duration: %v\n", config.TestDuration)
		fmt.Printf("Image size: %dx%d\n\n", config.ImageSize, config.ImageSize)

		testCPUBottleneck(config)
	default:
		log.Fatal("Unknown test type. Use: upload or cpu")
	}
}

// ============ HELPER FUNCTIONS ============

func createTestImage(size int) []byte {
	filename := fmt.Sprintf("test_image_%d.jpg", size)

	// If image already exists, reuse it
	if data, err := os.ReadFile(filename); err == nil {
		return data
	}

	// Generate new image
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r := uint8((x * 255) / size)
			g := uint8((y * 255) / size)
			b := uint8(((x + y) * 255) / (size * 2))
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}

	buf := new(bytes.Buffer)
	jpeg.Encode(buf, img, &jpeg.Options{Quality: 80})
	data := buf.Bytes()

	// Save for reuse
	os.WriteFile(filename, data, 0644)

	return data
}

func showImageStats(data []byte) {
	reader := bytes.NewReader(data)
	config, format, err := image.DecodeConfig(reader)
	if err != nil {
		log.Printf("Error reading image config: %v", err)
		return
	}

	fmt.Printf("Image Format: %s\n", format)
	fmt.Printf("Image Size: %dx%d\n", config.Width, config.Height)
	fmt.Printf("Image File Size: %d bytes\n\n", len(data))
}

func uploadAndCacheImages(count int, imageData []byte) []string {
	imageIDs := make([]string, 0, count)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Limit concurrent uploads

	fmt.Printf("Uploading %d images...\n", count)

	for i := 0; i < count; i++ {
		wg.Add(1)
		sem <- struct{}{} // Acquire slot

		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }() // Release slot

			resp, err := uploadImage(imageData, fmt.Sprintf("cpu-test-%d", idx))

			if err == nil {
				mu.Lock()
				imageIDs = append(imageIDs, resp["id"].(string))
				mu.Unlock()

				if (idx+1)%10 == 0 {
					fmt.Printf("- Uploaded %d images\n", idx+1)
				}
			} else {
				log.Printf("Upload failed for image %d: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	// Cache all images
	fmt.Printf("\nCaching %d images into backend cache...\n", len(imageIDs))
	cacheImages(imageIDs)

	return imageIDs
}

func uploadImage(imageData []byte, clientID string) (map[string]interface{}, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	if clientID != "" {
		part, err := writer.CreateFormField("client_id")
		if err != nil {
			return nil, fmt.Errorf("failed to create client_id field: %v", err)
		}
		part.Write([]byte(clientID))
	}

	filePart, err := writer.CreateFormFile("image", "test.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create image field: %v", err)
	}
	_, err = filePart.Write(imageData)
	if err != nil {
		return nil, fmt.Errorf("failed to write image data: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %v", err)
	}

	req, err := http.NewRequest("POST", FRONTEND_URL+"/upload", body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return result, nil
}

func cacheImages(imageIDs []string) error {
	payload := map[string]interface{}{
		"image_ids": imageIDs,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", BACKEND_URL+"/cache", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("✓ Cached: %v/%v images\n", result["cached"], result["requested"])

	return nil
}

func getCacheStats() map[string]interface{} {
	req, _ := http.NewRequest("GET", BACKEND_URL+"/cache/stats", nil)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return result
}

func deleteImage(imageID string) error {
	req, _ := http.NewRequest("DELETE", FRONTEND_URL+"/image/"+imageID, nil)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// Upload Bottleneck
// func testUploadBottleneck(config TestConfig) {
// 	fmt.Printf("Configuration:\n")
// 	fmt.Printf("Duration of test: %v\n", config.TestDuration)
// 	fmt.Printf("Images to upload: %d\n", config.NumImages)
// 	fmt.Printf("Concurrent clients: %d\n", config.ConcurrentClients)
// 	fmt.Printf("Image size: %dx%d\n\n", config.ImageSize, config.ImageSize)

// 	metrics = Metrics{ResponseTimes: []int64{}}

// 	fmt.Printf("Creating images...")
// 	testImage := createTestImage(config.ImageSize)
// 	showImageStats(testImage)

// 	// Upload Images
// 	fmt.Println("PHASE 1: Uploading images...")

// 	uploadedIDs := []string{}
// 	var idMu sync.Mutex
// 	var wg sync.WaitGroup
// 	sem := make(chan struct{}, config.ConcurrentClients)

// 	uploadStart := time.Now()
// 	idx := int64(0)

// 	for time.Since(uploadStart) < config.TestDuration {
// 		wg.Add(1)
// 		sem <- struct{}{} // Acquire slot

// 		go func(idx int64) {
// 			defer wg.Done()
// 			defer func() { <-sem }() // Release slot

// 			startTime := time.Now()
// 			resp, err := uploadImage(testImage, fmt.Sprintf("upload-test-%d", idx))

// 			if err != nil {
// 				atomic.AddInt64(&metrics.FailedRequests, 1)
// 				log.Printf("Upload failed: %v", err)
// 				return
// 			}

// 			respTime := time.Since(startTime).Milliseconds()
// 			metrics.mu.Lock()
// 			metrics.ResponseTimes = append(metrics.ResponseTimes, respTime)
// 			metrics.mu.Unlock()

// 			atomic.AddInt64(&metrics.TotalRequests, 1)
// 			atomic.AddInt64(&metrics.SuccessfulRequests, 1)
// 			atomic.AddInt64(&metrics.TotalResponseTime, respTime)

// 			idMu.Lock()
// 			uploadedIDs = append(uploadedIDs, resp["id"].(string))
// 			idMu.Unlock()

// 			if (idx+1)%1000 == 0 {
// 				fmt.Printf("✓ Uploaded %d images\n", idx+1)
// 			}
// 		}(idx)

// 		idx++
// 	}

// 	wg.Wait()
// 	uploadDuration := time.Since(uploadStart)

// 	fmt.Printf("\n✓ Upload phase complete\n")
// 	printMetrics("Upload", uploadDuration, config)

// 	// PHASE 2: Delete Images
// 	fmt.Println("PHASE 2: Deleting images...")

// 	wg = sync.WaitGroup{}                       // reset wg
// 	metrics = Metrics{ResponseTimes: []int64{}} // Reset metrics
// 	deleteStart := time.Now()

// 	for _, id := range uploadedIDs {
// 		wg.Add(1)
// 		sem <- struct{}{} // reuse semaphore

// 		go func(imageID string) {
// 			defer wg.Done()
// 			defer func() { <-sem }()

// 			startTime := time.Now()
// 			if err := deleteImage(imageID); err != nil {
// 				atomic.AddInt64(&metrics.FailedRequests, 1)
// 				log.Printf("Delete failed: %v", err)
// 				return
// 			}

// 			respTime := time.Since(startTime).Milliseconds()
// 			metrics.mu.Lock()
// 			metrics.ResponseTimes = append(metrics.ResponseTimes, respTime)
// 			metrics.mu.Unlock()

// 			atomic.AddInt64(&metrics.TotalRequests, 1)
// 			atomic.AddInt64(&metrics.SuccessfulRequests, 1)
// 			atomic.AddInt64(&metrics.TotalResponseTime, respTime)
// 		}(id)
// 	}

// 	wg.Wait()
// 	deleteDuration := time.Since(deleteStart)
// 	fmt.Printf("\n✓ Delete phase complete\n")
// 	printMetrics("Delete", deleteDuration, config)
// }

// ============ DISK BOTTLENECK TEST ============

func testUploadBottleneck(config TestConfig) {
	fmt.Printf("Configuration:\n")
	fmt.Printf("Duration of test: %v\n", config.TestDuration)
	fmt.Printf("Images to upload: %d\n", config.NumImages)
	fmt.Printf("Concurrent clients: %d\n", config.ConcurrentClients)
	fmt.Printf("Image size: %dx%d\n\n", config.ImageSize, config.ImageSize)
	fmt.Printf("OpPerSec: %d\n", config.OperationsPerSec)

	metrics = Metrics{ResponseTimes: []int64{}}

	fmt.Printf("Creating images...")
	testImage := createTestImage(config.ImageSize)
	showImageStats(testImage)

	// ============ PHASE 1: Uploading images ============
	fmt.Println("\nPHASE 1: Uploading images...")
	uploadedIDs := []string{}
	var idMu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, config.ConcurrentClients)

	uploadStart := time.Now()
	endTime := uploadStart.Add(config.TestDuration)

	for ClientID := 0; ClientID < config.ConcurrentClients; ClientID++ {
		wg.Add(1)

		go func(ClientID int) {
			defer wg.Done()
			// Rate limiter: wait before uploading
			clientRateLimiter := time.NewTicker(time.Second / time.Duration(config.OperationsPerSec))
			defer clientRateLimiter.Stop()

			for time.Now().Before(endTime) {
				// wait
				<-clientRateLimiter.C

				startTime := time.Now()
				resp, err := uploadImage(testImage, fmt.Sprintf("upload-test-%d", ClientID))

				if err != nil {
					atomic.AddInt64(&metrics.FailedRequests, 1)
					log.Printf("Upload failed: %v", err)
					return
				}

				respTime := time.Since(startTime).Milliseconds()
				metrics.mu.Lock()
				metrics.ResponseTimes = append(metrics.ResponseTimes, respTime)
				metrics.mu.Unlock()

				atomic.AddInt64(&metrics.TotalRequests, 1)
				atomic.AddInt64(&metrics.SuccessfulRequests, 1)
				atomic.AddInt64(&metrics.TotalResponseTime, respTime)

				idMu.Lock()
				uploadedIDs = append(uploadedIDs, resp["id"].(string))
				idMu.Unlock()
			}
		}(ClientID)
	}

	wg.Wait()
	uploadDuration := time.Since(uploadStart)

	fmt.Printf("\n✓ Upload phase complete\n")
	fmt.Printf("Total images uploaded: %d\n", len(uploadedIDs))
	printMetrics("Upload", uploadDuration, config)

	// ============ PHASE 2: Deleting images ============
	fmt.Println("\n========== PHASE 2: Deleting images ==========")

	wg = sync.WaitGroup{}                               // reset wg
	metrics = Metrics{ResponseTimes: []int64{}}         // Reset metrics
	sem = make(chan struct{}, config.ConcurrentClients) // reset semaphore

	deleteStart := time.Now()
	deleteIdx := int64(0)

	for _, id := range uploadedIDs {
		wg.Add(1)
		sem <- struct{}{} // Acquire slot

		go func(imageID string) {
			defer wg.Done()
			defer func() { <-sem }() // Release slot

			startTime := time.Now()
			if err := deleteImage(imageID); err != nil {
				atomic.AddInt64(&metrics.FailedRequests, 1)
				log.Printf("Delete failed: %v", err)
				return
			}

			respTime := time.Since(startTime).Milliseconds()
			metrics.mu.Lock()
			metrics.ResponseTimes = append(metrics.ResponseTimes, respTime)
			metrics.mu.Unlock()

			atomic.AddInt64(&metrics.TotalRequests, 1)
			atomic.AddInt64(&metrics.SuccessfulRequests, 1)
			atomic.AddInt64(&metrics.TotalResponseTime, respTime)

			if (atomic.AddInt64(&deleteIdx, 1))%100 == 0 {
				fmt.Printf("✓ Deleted %d images\n", deleteIdx)
			}
		}(id)
	}

	wg.Wait()
	deleteDuration := time.Since(deleteStart)

	fmt.Printf("\n✓ Delete phase complete\n")
	fmt.Printf("Total images deleted: %d\n", atomic.LoadInt64(&deleteIdx))
	printMetrics("Delete", deleteDuration, config)
}

func printMetrics(testName string, duration time.Duration, config TestConfig) {
	// Path for performance CSV
	filePath := fmt.Sprintf("../%s.csv", testName)

	// Open (or create) CSV file for appending
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("failed to open metrics file: %v", err)
	}
	defer file.Close()

	// CHECK FILE SIZE ON THE SAME FILE (CSV)
	info, err := file.Stat()
	if err != nil {
		log.Fatalf("failed to stat metrics file: %v", err)
	}

	// WRITE HEADER ONLY IF FILE IS EMPTY
	if info.Size() == 0 {
		_, _ = file.WriteString("Duration,NumClients,TotalRequests,SuccessfulRequests,Failed,MinResTime,MaxResTime,AvgResTime,P50,P95,P99,StdDev\n")
	}

	fmt.Printf("\n--- %s Results ---\n", testName)
	fmt.Printf("Duration: %v\n", duration)
	totalReq := atomic.LoadInt64(&metrics.TotalRequests)
	successReq := atomic.LoadInt64(&metrics.SuccessfulRequests)
	failedReq := atomic.LoadInt64(&metrics.FailedRequests)

	fmt.Printf("Total Requests: %d\n", totalReq)
	fmt.Printf("Successful: %d\n", successReq)
	fmt.Printf("Failed: %d\n", failedReq)

	// write basic metrics to CSV (prefix)
	_, _ = fmt.Fprintf(file, "%v,%d,%d,%d,%d,",
		duration.Milliseconds(),
		config.ConcurrentClients,
		totalReq,
		successReq,
		failedReq,
	)

	// Avoid divide-by-zero
	if totalReq == 0 {
		fmt.Println("No requests recorded, skipping latency stats.")
		// still close CSV row cleanly
		_, _ = file.WriteString("0,0,0,0,0,0\n")
		fmt.Println(strings.Repeat("=", 70))
		return
	}

	successRate := float64(successReq) / float64(totalReq) * 100
	fmt.Printf("Success Rate: %.2f%%\n", successRate)

	throughput := float64(totalReq) / duration.Seconds()
	fmt.Printf("Throughput: %.2f requests/sec\n", throughput)

	if len(metrics.ResponseTimes) > 0 {
		// Sort for percentile calculation
		sort.Slice(metrics.ResponseTimes, func(i, j int) bool {
			return metrics.ResponseTimes[i] < metrics.ResponseTimes[j]
		})

		n := len(metrics.ResponseTimes)
		avgTime := atomic.LoadInt64(&metrics.TotalResponseTime) / int64(n)
		minTime := metrics.ResponseTimes[0]
		maxTime := metrics.ResponseTimes[n-1]

		// safer percentile indexing: use (n-1)*q to stay in range
		p50 := metrics.ResponseTimes[int(float64(n-1)*0.50)]
		p95 := metrics.ResponseTimes[int(float64(n-1)*0.95)]
		p99 := metrics.ResponseTimes[int(float64(n-1)*0.99)]

		fmt.Printf("Response Times (ms):\n")
		fmt.Printf("  Min: %d, Max: %d, Avg: %d\n", minTime, maxTime, avgTime)
		fmt.Printf("  P50: %d, P95: %d, P99: %d\n", p50, p95, p99)

		// Standard deviation
		var sumSquares int64
		for _, rt := range metrics.ResponseTimes {
			diff := rt - avgTime
			sumSquares += diff * diff
		}
		variance := float64(sumSquares) / float64(n)
		stdDev := math.Sqrt(variance)
		fmt.Printf("  StdDev: %.2f\n", stdDev)

		// Write latency stats to CSV
		_, _ = fmt.Fprintf(file, "%d,%d,%d,%d,%d,%d,%.2f\n",
			minTime,
			maxTime,
			avgTime,
			p50,
			p95,
			p99,
			stdDev,
		)
	}

	// Simple separator line
	fmt.Println(strings.Repeat("=", 70))
}

// ============ REFACTORED CPU BOTTLENECK TEST ============

func testCPUBottleneck(config TestConfig) {
	fmt.Println("===== CPU BOTTLENECK TEST =====")
	fmt.Printf("Configuration:\n")
	fmt.Printf("Concurrent clients: %d\n", config.ConcurrentClients)
	fmt.Printf("Test duration: %v\n", config.TestDuration)
	fmt.Printf("Image size: %dx%d\n\n", config.ImageSize, config.ImageSize)
	fmt.Printf("Operations per second: %d\n", config.OperationsPerSec)

	// PHASE 1: Check if images exist, if not generate them
	fmt.Println("========== PHASE 1: Generate/Check Images ==========")
	testImage := createTestImage(config.ImageSize)
	showImageStats(testImage)

	// PHASE 2: Upload images if they don't exist
	fmt.Println("\n========== PHASE 2: Upload Images ==========")
	fmt.Printf("Uploading %d images...\n", config.CachedImages)
	uploadStart := time.Now()
	imageIDs := uploadAndCacheImages(config.CachedImages, testImage)
	uploadDuration := time.Since(uploadStart)

	if len(imageIDs) == 0 {
		log.Fatal("Failed to upload images")
	}

	fmt.Printf("✓ Successfully uploaded %d images in %.2fs\n\n", len(imageIDs), uploadDuration.Seconds())

	cacheStats := getCacheStats()
	fmt.Printf("✓ Cache Size: %v/%v\n", int(cacheStats["size"].(float64)), int(cacheStats["capacity"].(float64)))
	fmt.Printf("✓ Cache Hit Rate: %.2f%%\n", cacheStats["hit_rate"].(float64))
	fmt.Printf("✓ Cache Hits: %v, Misses: %v\n\n", int(cacheStats["hit_count"].(float64)), int(cacheStats["miss_count"].(float64)))

	// PHASE 4: CPU Load Test
	fmt.Printf("========== PHASE 3: CPU Load Test ==========\n")
	fmt.Printf("Running operations for %v with %d concurrent clients...\n", config.TestDuration, config.ConcurrentClients)

	metrics = Metrics{ResponseTimes: []int64{}}

	var wg sync.WaitGroup
	testStart := time.Now()
	endTime := testStart.Add(config.TestDuration)
	operationIndex := int64(0)

	// Spawn concurrent clients
	for clientID := 0; clientID < config.ConcurrentClients; clientID++ {
		wg.Add(1)
		go func(cID int) {
			defer wg.Done()
			imageIdx := cID % len(imageIDs)
			clientRateLimiter := time.NewTicker(time.Second / time.Duration(config.OperationsPerSec))
			defer clientRateLimiter.Stop()

			for time.Now().Before(endTime) {
				// Wait for rate limit token
				<-clientRateLimiter.C

				imageID := imageIDs[imageIdx%len(imageIDs)]
				operation := config.Operations[int(atomic.AddInt64(&operationIndex, 1)-1)%len(config.Operations)]

				startTime := time.Now()
				success := performOperation(imageID, operation)
				respTime := time.Since(startTime).Milliseconds()

				metrics.mu.Lock()
				metrics.ResponseTimes = append(metrics.ResponseTimes, respTime)
				metrics.mu.Unlock()

				atomic.AddInt64(&metrics.TotalRequests, 1)

				if success {
					atomic.AddInt64(&metrics.SuccessfulRequests, 1)
					atomic.AddInt64(&metrics.TotalResponseTime, respTime)
				} else {
					atomic.AddInt64(&metrics.FailedRequests, 1)
				}

				imageIdx++
			}
		}(clientID)
	}

	wg.Wait()
	testDuration := time.Since(testStart)

	// PHASE 5: Print Results
	fmt.Println("\n========== PHASE 4: Results & Analysis ==========")
	printMetrics("cpu", testDuration, config)

	// PHASE 6: Final Cache Stats
	// fmt.Println("========== PHASE 6: Final Cache Statistics ==========")
	// finalCacheStats := getCacheStats()
	// fmt.Printf("Cache Size: %v/%v\n", int(finalCacheStats["size"].(float64)), int(finalCacheStats["capacity"].(float64)))
	// fmt.Printf("Cache Hit Rate: %.2f%%\n", finalCacheStats["hit_rate"].(float64))
	// fmt.Printf("Total Cache Hits: %v\n", int(finalCacheStats["hit_count"].(float64)))
	// fmt.Printf("Total Cache Misses: %v\n", int(finalCacheStats["miss_count"].(float64)))
	// fmt.Printf("Evictions: %v\n\n", int(finalCacheStats["eviction_count"].(float64)))
}

// ============ HELPER FUNCTIONS ============

func performOperation(imageID, operation string) bool {
	var payload map[string]interface{}

	switch operation {
	case "resize":
		payload = map[string]interface{}{
			"image_id": imageID,
			"width":    512,
			"height":   512,
		}
	case "thumbnail":
		payload = map[string]interface{}{
			"image_id": imageID,
			"size":     128,
		}
	case "filter":
		payload = map[string]interface{}{
			"image_id":    imageID,
			"filter_type": "blur",
			"intensity":   5.0,
		}
	case "convert":
		payload = map[string]interface{}{
			"image_id": imageID,
			"format":   "png",
			"quality":  80,
		}
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", FRONTEND_URL+"/process/"+operation, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	success, ok := result["success"].(bool)
	return ok && success
}
