package processor

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

func Thumbnail(db *sql.DB, imageID string, params map[string]interface{}) (string, error) {
	var originalPath string
	err := db.QueryRow("SELECT original_path FROM images WHERE id = $1", imageID).Scan(&originalPath)
	if err != nil {
		return "", fmt.Errorf("image not found: %v", err)
	}

	img, err := imaging.Open(originalPath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %v", err)
	}

	size := int(params["size"].(float64))

	// Create thumbnail (CPU-intensive)
	thumb := imaging.Thumbnail(img, size, size, imaging.Lanczos)

	outputPath := filepath.Join(storageBasePath, fmt.Sprintf("%s_thumb_%d.jpg", imageID, size))
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	err = imaging.Save(thumb, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to save thumbnail: %v", err)
	}

	return outputPath, nil
}
