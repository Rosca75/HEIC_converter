// converter/meta.go — metadata extraction for the file-bundle table.
//
// Single-I/O strategy: read 192 KB, then pull dimensions from bep/imagemeta's
// CONFIG pass, decode the embedded thumbnail from the same buffer via
// Rosca75/heic, and extract EXIF from the same buffer via bep/imagemeta's
// tag callback. Files that do not yield to this path fall back to a full
// file read.
package converter

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bep/imagemeta"
)

// FileMeta holds display metadata and a base64 JPEG thumbnail for one HEIC
// file. JSON tags are in camelCase because the frontend reads them directly.
type FileMeta struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	CreatedAt   string `json:"createdAt"`
	Camera      string `json:"camera"`
	ThumbBase64 string `json:"thumbBase64"`
}

// GetFileMeta extracts metadata for the given paths in parallel. Directory
// paths are expanded via expandPaths. onProgress fires after every file.
func GetFileMeta(paths []string, recursive bool, onProgress func(done, total int)) ([]FileMeta, error) {
	expanded, err := expandPaths(paths, recursive)
	if err != nil {
		return nil, err
	}
	total := len(expanded)
	metas := make([]FileMeta, total)
	errs := make([]error, total)
	var wg sync.WaitGroup
	var cnt atomic.Int32
	// Cap workers at 16 — beyond that we saturate disk I/O on most SSDs
	// and start losing to scheduler overhead.
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

// GetOneFileMeta is the streaming-path entry point used by app.go.
func GetOneFileMeta(p string) (FileMeta, error) {
	return getOneFileMeta(p)
}

// getOneFileMeta extracts metadata for one HEIC file. Uses the 192 KB
// fast-path for dimensions, thumbnail, and EXIF; falls back to filesystem
// mtime for CreatedAt if EXIF has no DateTimeOriginal.
func getOneFileMeta(p string) (FileMeta, error) {
	m := FileMeta{Path: p, Name: filepath.Base(p), Camera: "unknown"}

	header, hdrErr := readHEICHeader(p)
	if hdrErr == nil {
		if res, metaErr := imagemeta.Decode(imagemeta.Options{
			R:           bytes.NewReader(header),
			ImageFormat: imagemeta.HEIF,
			Sources:     imagemeta.CONFIG,
		}); metaErr == nil && res.ImageConfig.Width > 0 {
			m.Width, m.Height = res.ImageConfig.Width, res.ImageConfig.Height
		}
		extractExifInto(bytes.NewReader(header), &m)
	}

	if img, thumbErr := decodeHEICThumbnail(p); thumbErr == nil {
		if jpg := resizeImageToJPEG(img, 48, 80); jpg != nil {
			m.ThumbBase64 = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpg)
		}
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
