package converter

import (
    "fmt"
    "os"
    "github.com/strukturag/libheif/go-libheif"
    "github.com/dsoprea/go-exif/v3"
)

// ConvertHEICtoJPG convertit un fichier HEIC en JPG en conservant les métadonnées EXIF.
func ConvertHEICtoJPG(inputPath, outputPath string, quality int) (string, error) {
    heifFile, err := os.Open(inputPath)
    if err != nil {
        return "", fmt.Errorf("failed to open HEIC file: %v", err)
    }
    defer heifFile.Close()

    heifImage, err := libheif.Decode(heifFile)
    if err != nil {
        return "", fmt.Errorf("failed to decode HEIC: %v", err)
    }

    exifData, err := exif.SearchAndExtractExif(heifFile)
    if err != nil && err != exif.ErrNoExif {
        return "", fmt.Errorf("failed to extract EXIF: %v", err)
    }

    outFile, err := os.Create(outputPath)
    if err != nil {
        return "", fmt.Errorf("failed to create output file: %v", err)
    }
    defer outFile.Close()

    if err := libheif.EncodeJpeg(outFile, heifImage, quality); err != nil {
        return "", fmt.Errorf("failed to encode JPG: %v", err)
    }

    if exifData != nil {
        if err := exif.WriteExif(outFile, exifData); err != nil {
            return "", fmt.Errorf("failed to write EXIF: %v", err)
        }
    }

    return fmt.Sprintf("Conversion réussie: %s -> %s", inputPath, outputPath), nil
}