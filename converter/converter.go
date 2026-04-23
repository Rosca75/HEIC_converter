// converter/converter.go — conversion orchestration.
//
// Parallel worker pool decodes each HEIC once via Rosca75/heic (WASM or
// dynamic libheif), then dispatches to encode.go for the chosen output
// format. No subprocesses are forked; no temp files are written.
package converter

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
)

// FileResult records one successful conversion.
type FileResult struct {
	Input  string `json:"input"`
	Output string `json:"output"`
}

// ConversionSummary is returned to the frontend after ConvertFiles finishes.
type ConversionSummary struct {
	Converted []FileResult `json:"converted"`
	Skipped   []string     `json:"skipped"`
	Failed    []string     `json:"failed"`
}

// ProgressFn is invoked after each file finishes (successfully or not).
// done is the running count, total is the number of files being processed.
type ProgressFn func(done, total int, currentPath string, ok bool)

// ConvertFiles converts paths to the target format in parallel and returns
// a summary. The quality argument is clamped to 1..100.
func ConvertFiles(paths []string, outputDir, format string, quality int, progress ProgressFn) (ConversionSummary, error) {
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}
	encode, ext, err := encoderFor(format)
	if err != nil {
		return ConversionSummary{}, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ConversionSummary{}, fmt.Errorf("cannot create output directory: %w", err)
	}

	total := len(paths)
	results := make([]FileResult, total)
	statuses := make([]int, total) // 0 = failed, 1 = converted, 2 = skipped

	workers := runtime.NumCPU()
	if workers > 16 {
		workers = 16
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	var cnt atomic.Int32

	for i, p := range paths {
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if !isHEIC(p) {
				statuses[i] = 2
				n := int(cnt.Add(1))
				if progress != nil {
					progress(n, total, p, false)
				}
				return
			}
			out, cerr := convertOne(p, outputDir, ext, quality, encode)
			if cerr == nil {
				results[i] = FileResult{Input: p, Output: out}
				statuses[i] = 1
			}
			n := int(cnt.Add(1))
			if progress != nil {
				progress(n, total, p, cerr == nil)
			}
		}(i, p)
	}
	wg.Wait()

	summary := ConversionSummary{}
	for i, s := range statuses {
		switch s {
		case 1:
			summary.Converted = append(summary.Converted, results[i])
		case 2:
			summary.Skipped = append(summary.Skipped, paths[i])
		default:
			summary.Failed = append(summary.Failed, paths[i])
		}
	}
	return summary, nil
}
