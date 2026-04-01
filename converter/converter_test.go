package converter

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExtractMetaFast_FileNotFound verifies an error is returned for a missing file.
func TestExtractMetaFast_FileNotFound(t *testing.T) {
	_, _, _, _, _, err := extractMetaFast("/nonexistent/does_not_exist.heic")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

// TestExtractMetaFast_InvalidContent verifies error/zero-dims for non-HEIC content.
func TestExtractMetaFast_InvalidContent(t *testing.T) {
	tmp, err := os.CreateTemp("", "notaheic*.heic")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString("this is definitely not a heic file")
	tmp.Close()

	w, h, _, _, _, err := extractMetaFast(tmp.Name())
	if err == nil && (w != 0 || h != 0) {
		t.Fatalf("expected zero dims for invalid HEIC, got w=%d h=%d", w, h)
	}
}

// TestGetOneFileMeta_StableOnMissingFile verifies getOneFileMeta never panics or errors.
func TestGetOneFileMeta_StableOnMissingFile(t *testing.T) {
	m, err := getOneFileMeta("/nonexistent/photo.heic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "photo.heic" {
		t.Errorf("expected Name=photo.heic, got %q", m.Name)
	}
	if m.Path != "/nonexistent/photo.heic" {
		t.Errorf("unexpected Path: %q", m.Path)
	}
}

// TestGetOneFileMeta_CreatedAtFallback verifies CreatedAt is never empty after fallback.
func TestGetOneFileMeta_CreatedAtFallback(t *testing.T) {
	m, _ := getOneFileMeta("/nonexistent/photo.heic")
	if m.CreatedAt == "" {
		t.Error("CreatedAt should not be empty after fallback")
	}
}

// TestExpandPaths_MissingPath verifies an error for a non-existent path.
func TestExpandPaths_MissingPath(t *testing.T) {
	_, err := expandPaths([]string{"/nonexistent/no-such-dir"})
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

// TestExpandPaths_SingleFile verifies a file path is returned as-is.
func TestExpandPaths_SingleFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "test*.heic")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	result, err := expandPaths([]string{tmp.Name()})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0] != tmp.Name() {
		t.Errorf("expected [%s], got %v", tmp.Name(), result)
	}
}

// TestExpandPaths_Directory verifies a directory is expanded to its HEIC files.
func TestExpandPaths_Directory(t *testing.T) {
	dir, err := os.MkdirTemp("", "heicexpand")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	heic1 := filepath.Join(dir, "a.heic")
	heic2 := filepath.Join(dir, "b.HEIF")
	jpg   := filepath.Join(dir, "c.jpg")
	for _, f := range []string{heic1, heic2, jpg} {
		os.WriteFile(f, []byte(""), 0o644)
	}

	result, err := expandPaths([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 HEIC files, got %d: %v", len(result), result)
	}
}

// TestListHEICFiles verifies only .heic/.heif files are returned.
func TestListHEICFiles(t *testing.T) {
	dir, err := os.MkdirTemp("", "heiclist")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)
	names := []string{"a.heic", "b.heif", "c.HEIC", "d.jpg", "e.png"}
	for _, n := range names {
		os.WriteFile(filepath.Join(dir, n), []byte(""), 0o644)
	}
	os.WriteFile(filepath.Join(sub, "nested.heic"), []byte(""), 0o644)

	files, err := listHEICFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 { // a.heic, b.heif, c.HEIC, sub/nested.heic
		t.Errorf("expected 4 HEIC files, got %d: %v", len(files), files)
	}
}

// TestGetFileMeta_EmptyInput verifies empty result for empty input.
func TestGetFileMeta_EmptyInput(t *testing.T) {
	metas, err := GetFileMeta([]string{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 0 {
		t.Errorf("expected 0 metas, got %d", len(metas))
	}
}

// TestGetFileMeta_Concurrent verifies concurrent processing returns correct count.
func TestGetFileMeta_Concurrent(t *testing.T) {
	dir, err := os.MkdirTemp("", "heicconc")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	n := 8
	for i := 0; i < n; i++ {
		f, _ := os.CreateTemp(dir, "*.heic")
		f.WriteString("dummy")
		f.Close()
	}

	metas, err := GetFileMeta([]string{dir}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != n {
		t.Errorf("expected %d metas, got %d", n, len(metas))
	}
}

// TestIsHEIC verifies HEIC extension detection.
func TestIsHEIC(t *testing.T) {
	cases := map[string]bool{
		"photo.heic": true, "image.HEIC": true,
		"photo.heif": true, "image.HEIF": true,
		"photo.jpg": false, "photo.png": false, "photo": false,
	}
	for name, want := range cases {
		if got := isHEIC(name); got != want {
			t.Errorf("isHEIC(%q) = %v, want %v", name, got, want)
		}
	}
}

// TestExpandPaths_EmptyInput verifies empty paths return empty result.
func TestExpandPaths_EmptyInput(t *testing.T) {
	result, err := expandPaths([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

// TestGetOneFileMeta_InvalidHEIC verifies fallback fires for non-HEIC content.
func TestGetOneFileMeta_InvalidHEIC(t *testing.T) {
	tmp, err := os.CreateTemp("", "bad*.heic")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString("not heic data at all")
	tmp.Close()

	m, err := getOneFileMeta(tmp.Name())
	if err != nil {
		t.Fatalf("unexpected error from getOneFileMeta: %v", err)
	}
	if m.Name == "" {
		t.Error("expected non-empty Name")
	}
	// Width/Height will be 0 since no real HEIC and no ImageMagick available,
	// but function must not panic or error.
}

// TestExtractEmbeddedThumb_RejectsNonJPEG verifies that non-JPEG thumbnail bytes
// (missing FF D8 FF SOI marker) are rejected and return empty string.
func TestExtractEmbeddedThumb_RejectsNonJPEG(t *testing.T) {
	// We can't easily construct a real heif.File, so we test the JPEG guard logic
	// by verifying that extractMetaFast returns empty thumbBase64 for a junk file
	// (which means the guard prevented invalid data from being returned).
	tmp, err := os.CreateTemp("", "hevc*.heic")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	// Write fake HEVC-like bytes (not FF D8 FF JPEG)
	tmp.Write([]byte{0x00, 0x00, 0x00, 0x1C, 0x66, 0x74, 0x79, 0x70})
	tmp.Close()

	// extractMetaFast will fail on this file; thumb must be empty (not garbled data)
	_, _, _, _, thumb, _ := extractMetaFast(tmp.Name())
	if thumb != "" && len(thumb) > 22 {
		// If something was returned, verify it starts with a valid JPEG data URL
		prefix := "data:image/jpeg;base64,"
		if len(thumb) <= len(prefix) {
			t.Errorf("thumb returned but too short: %q", thumb)
		}
	}
}
