package converter

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FileResult struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

type ConversionSummary struct {
	Converted []FileResult `json:"converted"`
	Skipped   []string     `json:"skipped"`
}

func ConvertPath(inputPath, outputDir, format string, quality int) (ConversionSummary, error) {
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
		out, convErr := convertOne(inputPath, outputDir, format, quality)
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

		out, err := convertOne(path, outputDir, format, quality)
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

func convertOne(inputPath, outputDir, format string, quality int) (string, error) {
	if !isHEIC(inputPath) {
		return "", fmt.Errorf("unsupported file extension for %s (expected .heic or .heif)", inputPath)
	}

	ext := format
	if ext == "jpeg" {
		ext = "jpg"
	}

	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, baseName+"."+ext)

	cmd := exec.Command("magick", "convert", inputPath, "-quality", fmt.Sprintf("%d", quality), outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ImageMagick error: %w\n%s", err, string(output))
	}

	return outputPath, nil
}

func isHEIC(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".heic" || ext == ".heif"
}

func CheckImageMagick() error {
	cmd := exec.Command("magick", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ImageMagick is not installed or not in PATH.\nDownload: https://imagemagick.org/script/download.php#windows")
	}
	return nil
}
