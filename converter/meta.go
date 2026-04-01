package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// FileMeta holds display metadata and a thumbnail for one HEIC file.
type FileMeta struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	CreatedAt   string `json:"createdAt"`
	Camera      string `json:"camera"`
	ThumbBase64 string `json:"thumbBase64"`
}

// GetFileMeta returns FileMeta for each path, processing files in parallel.
// Directories are expanded to their HEIC contents. onProgress is called after each file.
func GetFileMeta(paths []string, onProgress func(done, total int)) ([]FileMeta, error) {
	expanded, err := expandPaths(paths)
	if err != nil {
		return nil, err
	}
	total := len(expanded)
	metas := make([]FileMeta, total)
	errs := make([]error, total)
	var wg sync.WaitGroup
	var cnt atomic.Int32
	workers := runtime.NumCPU()
	if workers > 16 {
		workers = 16
	}
	sem := make(chan struct{}, workers)
	for i, p := range expanded {
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m, e := getOneFileMeta(p)
			metas[i] = m
			errs[i] = e
			n := int(cnt.Add(1))
			if onProgress != nil {
				onProgress(n, total)
			}
		}(i, p)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			return nil, fmt.Errorf("meta for %s: %w", expanded[i], e)
		}
	}
	return metas, nil
}

// ExpandPaths resolves directory paths to their contained HEIC files (exported for app.go).
func ExpandPaths(paths []string) ([]string, error) {
	return expandPaths(paths)
}

// expandPaths resolves any directory paths to the HEIC files they contain.
func expandPaths(paths []string) ([]string, error) {
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", p, err)
		}
		if info.IsDir() {
			heics, err := listHEICFiles(p)
			if err != nil {
				return nil, err
			}
			out = append(out, heics...)
		} else {
			out = append(out, p)
		}
	}
	return out, nil
}

// listHEICFiles returns all HEIC/HEIF file paths found in a directory tree.
func listHEICFiles(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && isHEIC(path) {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

// GetOneFileMeta extracts metadata for a single HEIC file (exported for streaming use).
func GetOneFileMeta(p string) (FileMeta, error) {
	return getOneFileMeta(p)
}

// getOneFileMeta tries the fast pure-Go path first, falling back to ImageMagick.
func getOneFileMeta(p string) (FileMeta, error) {
	m := FileMeta{Path: p, Name: filepath.Base(p)}

	w, h, cam, date, thumb, err := extractMetaFast(p)
	if err == nil && w > 0 {
		m.Width, m.Height, m.Camera, m.CreatedAt = w, h, cam, date
		m.ThumbBase64 = thumb
		if m.ThumbBase64 == "" {
			m.ThumbBase64 = generateThumb(p)
		}
		if m.CreatedAt == "" {
			if info, e := os.Stat(p); e == nil {
				m.CreatedAt = info.ModTime().UTC().Format(time.RFC3339)
			} else {
				m.CreatedAt = "unknown"
			}
		}
		return m, nil
	}

	// Full ImageMagick fallback for non-standard HEIC files.
	m.Width, m.Height, m.Camera, m.CreatedAt = parseVerboseInfo(p)
	if m.CreatedAt == "" {
		if info, e := os.Stat(p); e == nil {
			m.CreatedAt = info.ModTime().UTC().Format(time.RFC3339)
		} else {
			m.CreatedAt = "unknown"
		}
	}
	m.ThumbBase64 = generateThumb(p)
	return m, nil
}
