package converter

import (
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrium/goheif"
)

type FileResult struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type ConversionSummary struct {
	Converted []FileResult `json:"converted"`
	Skipped   []string     `json:"skipped"`
}

func ConvertPath(inputPath, outputDir string, quality int) (ConversionSummary, error) {
	if quality < 1 || quality > 100 {
		return ConversionSummary{}, fmt.Errorf("quality must be between 1 and 100")
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return ConversionSummary{}, fmt.Errorf("cannot access input path: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ConversionSummary{}, fmt.Errorf("cannot create output directory: %w", err)
	}

	summary := ConversionSummary{}
	if !info.IsDir() {
		out, convErr := convertOne(inputPath, outputDir, quality)
		if convErr != nil {
			return ConversionSummary{}, convErr
		}
		summary.Converted = append(summary.Converted, FileResult{Input: inputPath, Output: out})
		return summary, nil
	}

	walkErr := filepath.WalkDir(inputPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !isHEIC(path) {
			summary.Skipped = append(summary.Skipped, path)
			return nil
		}

		out, err := convertOne(path, outputDir, quality)
		if err != nil {
			return err
		}
		summary.Converted = append(summary.Converted, FileResult{Input: path, Output: out})
		return nil
	})
	if walkErr != nil {
		return ConversionSummary{}, walkErr
	}

	if len(summary.Converted) == 0 {
		return ConversionSummary{}, errors.New("no HEIC/HEIF files found in the selected directory")
	}

	return summary, nil
}

func convertOne(inputPath, outputDir string, quality int) (string, error) {
	if !isHEIC(inputPath) {
		return "", fmt.Errorf("unsupported file extension for %s (expected .heic or .heif)", inputPath)
	}

	in, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open input file %s: %w", inputPath, err)
	}
	defer in.Close()

	img, err := goheif.Decode(in)
	if err != nil {
		return "", fmt.Errorf("failed to decode HEIC file %s: %w", inputPath, err)
	}

	name := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath)) + ".jpg"
	outputPath := filepath.Join(outputDir, name)

	out, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file %s: %w", outputPath, err)
	}

	encodeErr := jpeg.Encode(out, img, &jpeg.Options{Quality: quality})
	closeErr := out.Close()
	if encodeErr != nil {
		return "", fmt.Errorf("failed to encode JPEG for %s: %w", inputPath, encodeErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("failed to close output file %s: %w", outputPath, closeErr)
	}

	rawExif, err := ExtractExif(inputPath)
	if err == nil && len(rawExif) > 0 {
		if err := InjectExif(outputPath, rawExif); err != nil {
			return "", fmt.Errorf("failed to preserve EXIF for %s: %w", inputPath, err)
		}
	}

	return outputPath, nil
}

func isHEIC(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".heic" || ext == ".heif"
}
