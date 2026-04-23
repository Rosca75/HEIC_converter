package converter

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGetOneFileMeta_StableOnMissingFile verifies no panic/error on missing file.
func TestGetOneFileMeta_StableOnMissingFile(t *testing.T) {
	m, err := getOneFileMeta("/nonexistent/photo.heic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "photo.heic" {
		t.Errorf("expected Name=photo.heic, got %q", m.Name)
	}
	if m.CreatedAt == "" {
		t.Error("CreatedAt should not be empty after fallback")
	}
}

// TestExpandPaths_MissingPath verifies an error for a non-existent path.
func TestExpandPaths_MissingPath(t *testing.T) {
	if _, err := expandPaths([]string{"/nonexistent/no-such-dir"}, false); err == nil {
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

	result, err := expandPaths([]string{tmp.Name()}, false)
	if err != nil || len(result) != 1 || result[0] != tmp.Name() {
		t.Errorf("expected [%s], got %v (err=%v)", tmp.Name(), result, err)
	}
}

// TestExpandPaths_Recursion verifies subfolders are walked only when requested.
func TestExpandPaths_Recursion(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.MkdirAll(sub, 0o755)
	os.WriteFile(filepath.Join(dir, "a.heic"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "b.HEIF"), nil, 0o644)
	os.WriteFile(filepath.Join(dir, "c.jpg"), nil, 0o644)
	os.WriteFile(filepath.Join(sub, "nested.heic"), nil, 0o644)

	flat, err := expandPaths([]string{dir}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(flat) != 2 {
		t.Errorf("non-recursive: expected 2, got %d: %v", len(flat), flat)
	}
	deep, err := expandPaths([]string{dir}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(deep) != 3 {
		t.Errorf("recursive: expected 3, got %d: %v", len(deep), deep)
	}
}

// TestGetFileMeta_EmptyInput verifies empty result for empty input.
func TestGetFileMeta_EmptyInput(t *testing.T) {
	metas, err := GetFileMeta([]string{}, false, nil)
	if err != nil || len(metas) != 0 {
		t.Errorf("expected empty, got %d metas err=%v", len(metas), err)
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

// TestEncoderFor verifies every supported format resolves and unknown errors.
func TestEncoderFor(t *testing.T) {
	for _, f := range []string{"jpg", "jpeg", "png", "tiff", "tif", "webp"} {
		enc, ext, err := encoderFor(f)
		if err != nil || enc == nil || ext == "" {
			t.Errorf("encoderFor(%q) failed: ext=%q err=%v", f, ext, err)
		}
	}
	if _, _, err := encoderFor("bmp"); err == nil {
		t.Error("expected error for unsupported format bmp")
	}
}

// TestConvertAllFormats exercises the decode→encode pipeline on every format
// if a sample HEIC is available in testdata/.
func TestConvertAllFormats(t *testing.T) {
	in := filepath.Join("testdata", "sample.heic")
	if _, err := os.Stat(in); err != nil {
		t.Skip("no sample.heic in converter/testdata/")
	}
	for _, f := range []string{"jpg", "png", "tiff", "webp"} {
		out := t.TempDir()
		if _, err := ConvertFiles([]string{in}, out, f, 80, nil); err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		entries, _ := os.ReadDir(out)
		if len(entries) == 0 {
			t.Errorf("%s: no output file produced", f)
		}
	}
}
