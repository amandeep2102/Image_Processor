package processor

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

func Thumbnail(imageData []byte, imageID string, params map[string]interface{}) (string, error) {

	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return "", fmt.Errorf("failed to decode image from bytes: %v", err)
	}

	// size ???
	var size int
	switch v := params["size"].(type) {
	case float64:
		size = int(v)
	case int:
		size = v
	default:
		return "", fmt.Errorf("invalid size type: %T", v)
	}

	thumb := imaging.Thumbnail(img, size, size, imaging.Lanczos)

	outputPath := filepath.Join(storageBasePath, fmt.Sprintf("%s_thumb_%d.jpg", imageID, size))
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	err = imaging.Save(thumb, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to save thumbnail: %v", err)
	}

	return outputPath, nil
}
