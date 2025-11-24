package processor

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/disintegration/imaging"
)

func Convert(imageData []byte, imageID string, params map[string]interface{}) (string, error) {

	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return "", fmt.Errorf("failed to decode image from bytes: %v", err)
	}

	format := params["format"].(string)

	// quality??
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

	// Saving
	var saveErr error
	switch format {
	case "jpeg", "jpg":
		saveErr = imaging.Save(img, outputPath, imaging.JPEGQuality(quality))
	case "png":
		// defining levels for png
		// 0 - default
		// -1 - noCompression
		// -2 - bestSpeed
		// -3 - bestCompression

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
