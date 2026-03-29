package converter

import (
    "os"
    "github.com/dsoprea/go-exif/v3"
)

// ExtractExif extrait les métadonnées EXIF d'un fichier HEIC.
func ExtractExif(filePath string) ([]byte, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    exifData, err := exif.SearchAndExtractExif(file)
    if err != nil {
        return nil, err
    }
    return exifData, nil
}