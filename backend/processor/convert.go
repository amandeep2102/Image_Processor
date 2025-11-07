package processor

import (
	"database/sql"
	"fmt"
	"image/png"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

func Convert(db *sql.DB, imageID string, params map[string]interface{}) (string, error) {
	var originalPath string
	err := db.QueryRow("SELECT original_path FROM images WHERE id = $1", imageID).Scan(&originalPath)
	if err != nil {
		return "", fmt.Errorf("image not found: %v", err)
	}

	img, err := imaging.Open(originalPath)
	if err != nil {
		return "", fmt.Errorf("failed to open image: %v", err)
	}

	format := params["format"].(string)
	// quality := int(params["quality"].(float64))

	var quality int
	switch v := params["quality"].(type) {
	case float64:
		quality = int(v)
	case int:
		quality = v
	default:
		return "", fmt.Errorf("invalid quality type: %T", v)
	}

	outputPath := filepath.Join(storageBasePath, fmt.Sprintf("%s_converted.%s", imageID, format))
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	// Save with specified format and quality (CPU-intensive)
	var saveErr error
	switch format {
	case "jpeg", "jpg":
		saveErr = imaging.Save(img, outputPath, imaging.JPEGQuality(quality))
	case "png":

		var level int
		if quality < 20 {
			level = -3
		} else if quality > 20 && quality < 80 {
			level = 0
		} else if quality > 80 {
			level = -2
		} else {
			level = -1
		}
		saveErr = imaging.Save(img, outputPath, imaging.PNGCompressionLevel(png.CompressionLevel(level)))

	default:
		saveErr = imaging.Save(img, outputPath)
	}

	if saveErr != nil {
		return "", fmt.Errorf("failed to save converted image: %v", saveErr)
	}

	return outputPath, nil
}
