package processor

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

const storageBasePath = "/home/polarbeer/Documents/Image-Processor/storage/processed"

func Resize(imageData []byte, imageID string, params map[string]interface{}) (string, error) {

	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return "", fmt.Errorf("failed to decode image from bytes: %v", err)
	}

	// height and width ???
	width, height, err := getResizeDimensions(params)
	if err != nil {
		return "", err
	}

	// Resize and save
	return resizeAndSave(img, imageID, width, height)
}

func getResizeDimensions(params map[string]interface{}) (int, int, error) {
	var width, height int

	switch v := params["width"].(type) {
	case float64:
		width = int(v)
	case int:
		width = v
	default:
		return 0, 0, fmt.Errorf("invalid width type %T", v)
	}

	switch v := params["height"].(type) {
	case float64:
		height = int(v)
	case int:
		height = v
	default:
		return 0, 0, fmt.Errorf("invalid height type %T", v)
	}

	return width, height, nil
}

func resizeAndSave(img image.Image, imageID string, width, height int) (string, error) {

	resized := imaging.Resize(img, width, height, imaging.Lanczos)

	// Save processed image
	outputPath := filepath.Join(storageBasePath, fmt.Sprintf("%s_resized_%dx%d.jpg", imageID, width, height))
	os.MkdirAll(filepath.Dir(outputPath), 0755)

	err := imaging.Save(resized, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to save image: %v", err)
	}

	return outputPath, nil
}
