// converter/walk.go — filesystem traversal for HEIC input lists.
//
// Split out of meta.go to keep both files under the 150-line project limit.
package converter

import (
	"fmt"
	"os"
	"path/filepath"
)

// ExpandPaths resolves any directory paths to HEIC files they contain.
// If recursive is true, subdirectories are walked; otherwise only the
// immediate directory contents are returned.
func ExpandPaths(paths []string, recursive bool) ([]string, error) {
	return expandPaths(paths, recursive)
}

// expandPaths is the internal lower-case helper.
func expandPaths(paths []string, recursive bool) ([]string, error) {
	var out []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", p, err)
		}
		if info.IsDir() {
			heics, err := listHEICFiles(p, recursive)
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

// listHEICFiles returns HEIC paths in dir. When recursive is false, only the
// immediate directory is scanned (filepath.WalkDir is still used, but we
// SkipDir on any subdirectory).
func listHEICFiles(dir string, recursive bool) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if isHEIC(path) {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}
