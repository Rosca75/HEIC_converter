package converter

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// GetFileMeta returns FileMeta for each path. Directories are expanded to their HEIC contents.
func GetFileMeta(paths []string) ([]FileMeta, error) {
	var expanded []string
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
			expanded = append(expanded, heics...)
		} else {
			expanded = append(expanded, p)
		}
	}
	metas := make([]FileMeta, 0, len(expanded))
	for _, p := range expanded {
		m, err := getOneFileMeta(p)
		if err != nil {
			return nil, fmt.Errorf("meta for %s: %w", p, err)
		}
		metas = append(metas, m)
	}
	return metas, nil
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

// getOneFileMeta extracts metadata for a single HEIC file.
func getOneFileMeta(p string) (FileMeta, error) {
	m := FileMeta{Path: p, Name: filepath.Base(p)}
	if w, h, err := parseResolution(p); err == nil {
		m.Width = w
		m.Height = h
	}
	m.Camera, m.CreatedAt = parseCamera(p)
	if m.CreatedAt == "" {
		if info, err := os.Stat(p); err == nil {
			m.CreatedAt = info.ModTime().UTC().Format(time.RFC3339)
		} else {
			m.CreatedAt = "unknown"
		}
	}
	m.ThumbBase64 = generateThumb(p)
	return m, nil
}

// parseResolution returns width and height of an image via magick identify.
func parseResolution(p string) (int, int, error) {
	out, err := exec.Command("magick", "identify", "-format", "%wx%h", p).CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("identify: %w\n%s", err, string(out))
	}
	var w, h int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%dx%d", &w, &h); err != nil {
		return 0, 0, fmt.Errorf("parse resolution %q: %w", string(out), err)
	}
	return w, h, nil
}

// parseCamera extracts EXIF camera model and creation date from a HEIC file.
func parseCamera(p string) (camera, createdAt string) {
	out, err := exec.Command("magick", "identify", "-verbose", p).CombinedOutput()
	if err != nil {
		return "unknown", ""
	}
	camera = "unknown"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "exif:Model:"); ok {
			camera = strings.TrimSpace(after)
		}
		if after, ok := strings.CutPrefix(line, "date:create:"); ok {
			createdAt = strings.TrimSpace(after)
		}
	}
	return camera, createdAt
}

// generateThumb returns a base64 data URL of a 12×12 JPEG thumbnail.
func generateThumb(p string) string {
	cmd := exec.Command("magick", "convert", p,
		"-thumbnail", "12x12^",
		"-gravity", "center",
		"-extent", "12x12",
		"jpg:-",
	)
	data, err := cmd.Output()
	if err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
}
