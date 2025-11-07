package processor

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

const storageBasePath = "/home/polarbeer/Documents/Image-Processor/backend/storage/processed"

func Resize(db *sql.DB, imageID string, params map[string]interface{}) (string, error) {
	// Get original image path
	var originalPath string
	err := db.QueryRow("SELECT original_path FROM images WHERE id = $1", imageID).Scan(&originalPath)
	if err != nil {
		return "", fmt.Errorf("image not found: %v", err)
	}

	// Open image
	img, err := imaging.Open(originalPath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %v", err)
	}

	// Get resize parameters
	// width := int(params["width"].(float64))
	// height := int(params["height"].(float64))
	var width, height int

	switch v := params["width"].(type) {
	case float64:
		width = int(v)
	case int:
		width = v
	default:
		return "", fmt.Errorf("invalid width type %T", v)
	}

	switch v := params["height"].(type) {
	case float64:
		height = int(v)
	case int:
		height = v
	default:
		return "", fmt.Errorf("invalid height type %T", v)
	}

	// Resize image (CPU-intensive operation)
	resized := imaging.Resize(img, width, height, imaging.Lanczos)

	// Save processed image
	outputPath := filepath.Join(storageBasePath, fmt.Sprintf("%s_resized_%dx%d.jpg", imageID, width, height))
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	err = imaging.Save(resized, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %v", err)
	}

	return outputPath, nil
}
