package processor

import (
	"database/sql"
	"fmt"
	"github.com/disintegration/imaging"
	"image"
	"os"
	"path/filepath"
)

func ApplyFilter(db *sql.DB, imageID string, params map[string]interface{}) (string, error) {
	var originalPath string
	err := db.QueryRow("SELECT original_path FROM images WHERE id = $1", imageID).Scan(&originalPath)
	if err != nil {
		return "", fmt.Errorf("image not found: %v", err)
	}

	img, err := imaging.Open(originalPath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %v", err)
	}

	filterType := params["filter_type"].(string)
	intensity := params["intensity"].(float64)

	var filtered image.Image

	// CPU-intensive operations
	switch filterType {
	case "blur":
		filtered = imaging.Blur(img, intensity)
	case "sharpen":
		filtered = imaging.Sharpen(img, intensity)
	case "grayscale":
		filtered = imaging.Grayscale(img)
	default:
		return "", fmt.Errorf("unknown filter type: %s", filterType)
	}

	outputPath := filepath.Join(storageBasePath, fmt.Sprintf("%s_filter_%s.jpg", imageID, filterType))
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	err = imaging.Save(filtered, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to save filtered image: %v", err)
	}

	return outputPath, nil
}
