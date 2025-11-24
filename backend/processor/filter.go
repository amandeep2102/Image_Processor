package processor

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

func Filter(imageData []byte, imageID string, params map[string]interface{}) (string, error) {

	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return "", fmt.Errorf("failed to decode image from bytes: %v", err)
	}

	filterType := params["filter_type"].(string)
	intensity := params["intensity"].(float64)

	var filtered image.Image

	// applying filters
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
