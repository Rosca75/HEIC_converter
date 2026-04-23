// converter/convert_one.go — single-file decode→encode helper + legacy
// ConvertPath wrapper. Split out of converter.go to keep both files under
// the 150-line project limit.
package converter

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Rosca75/heic"
)

// ConvertPath expands a single file or directory and delegates to
// ConvertFiles. Subfolder recursion is OFF; the new frontend uses
// ConvertFiles directly with the fully expanded list.
func ConvertPath(inputPath, outputDir, format string, quality int) (ConversionSummary, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return ConversionSummary{}, fmt.Errorf("cannot access input path: %w", err)
	}
	var paths []string
	if info.IsDir() {
		paths, err = listHEICFiles(inputPath, false)
		if err != nil {
			return ConversionSummary{}, err
		}
		if len(paths) == 0 {
			return ConversionSummary{}, errors.New("no HEIC/HEIF files found in the selected directory")
		}
	} else {
		paths = []string{inputPath}
	}
	return ConvertFiles(paths, outputDir, format, quality, nil)
}

// convertOne decodes one HEIC file and writes it through the provided
// encoder function. Returns the output path on success.
func convertOne(inputPath, outputDir, ext string, quality int, encode func(io.Writer, image.Image, int) error) (string, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", inputPath, err)
	}
	img, err := heic.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", inputPath, err)
	}
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, baseName+"."+ext)
	f, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", outputPath, err)
	}
	defer f.Close()
	if err := encode(f, img, quality); err != nil {
		return "", fmt.Errorf("encode %s: %w", outputPath, err)
	}
	return outputPath, nil
}

// isHEIC reports whether path has a .heic or .heif extension.
func isHEIC(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".heic" || ext == ".heif"
}
