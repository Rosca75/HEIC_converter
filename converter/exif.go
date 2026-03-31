package converter

import (
	"fmt"
	"os"

	"github.com/adrium/goheif"
	"github.com/rwcarlsen/goexif/exif"
)

// ExtractExif extracts raw EXIF bytes from a HEIC/HEIF file.
func ExtractExif(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	rawExif, err := goheif.ExtractExif(file)
	if err != nil || len(rawExif) == 0 {
		return nil, nil
	}

	return rawExif, nil
}

// GetEXIF parses EXIF metadata from a JPEG file.
func GetEXIF(srcPath string) (*exif.Exif, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return exif.Decode(f)
}

func InjectExif(jpegPath string, rawExif []byte) error {
	if len(rawExif) == 0 {
		return nil
	}

	jpegBytes, err := os.ReadFile(jpegPath)
	if err != nil {
		return err
	}
	if len(jpegBytes) < 2 || jpegBytes[0] != 0xFF || jpegBytes[1] != 0xD8 {
		return fmt.Errorf("invalid jpeg file")
	}

	app1Length := len(rawExif) + 2
	if app1Length > 0xFFFF {
		return fmt.Errorf("exif metadata too large")
	}

	updated := make([]byte, 0, len(jpegBytes)+len(rawExif)+4)
	updated = append(updated, 0xFF, 0xD8)
	updated = append(updated, 0xFF, 0xE1, byte(app1Length>>8), byte(app1Length))
	updated = append(updated, rawExif...)
	updated = append(updated, jpegBytes[2:]...)

	return os.WriteFile(jpegPath, updated, 0o644)
}
