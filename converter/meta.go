package converter

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	sem := make(chan struct{}, 4) // max 4 concurrent ImageMagick processes
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

// getOneFileMeta extracts metadata for a single HEIC file using one identify call.
func getOneFileMeta(p string) (FileMeta, error) {
	m := FileMeta{Path: p, Name: filepath.Base(p)}
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

// parseVerboseInfo extracts resolution and EXIF info using a single identify -verbose call.
func parseVerboseInfo(p string) (width, height int, camera, createdAt string) {
	out, err := exec.Command("magick", "identify", "-verbose", p+"[0]").CombinedOutput()
	if err != nil {
		return 0, 0, "unknown", ""
	}
	camera = "unknown"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "Geometry:"); ok {
			fmt.Sscanf(strings.TrimSpace(after), "%dx%d", &width, &height)
		}
		if after, ok := strings.CutPrefix(line, "exif:Model:"); ok {
			camera = strings.TrimSpace(after)
		}
		if after, ok := strings.CutPrefix(line, "date:create:"); ok {
			createdAt = strings.TrimSpace(after)
		}
	}
	return width, height, camera, createdAt
}

// generateThumb returns a base64 data URL of a 48×48 JPEG thumbnail.
func generateThumb(p string) string {
	cmd := exec.Command("magick", "convert", p+"[0]",
		"-thumbnail", "48x48^",
		"-gravity", "center",
		"-extent", "48x48",
		"jpg:-",
	)
	data, err := cmd.Output()
	if err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
}
