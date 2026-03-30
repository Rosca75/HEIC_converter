package converter

import (
	"fmt"
	"os"

	exif "github.com/dsoprea/go-exif/v3"
)

func ExtractExif(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	rawExif, err := exif.SearchAndExtractExif(file)
	if err != nil {
		if err == exif.ErrNoExif {
			return nil, nil
		}
		return nil, err
	}

	return rawExif, nil
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
