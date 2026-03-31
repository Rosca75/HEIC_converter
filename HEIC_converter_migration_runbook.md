# HEIC Converter — Migration Runbook
## Switch to `goheif` + `goexif` (no system DLL required)

**Repo:** https://github.com/Rosca75/HEIC_converter  
**Goal:** Replace `go-libheif` (requires external DLL) with `goheif` (bundles C++ source, only needs MinGW/GCC which Wails already requires).

---

## Prerequisites (on your personal machine)

- Go 1.21+
- Git
- GCC / MinGW (already required by Wails)

---

## Step 1 — Clone the repo

```bash
git clone https://github.com/Rosca75/HEIC_converter.git
cd HEIC_converter
```

---

## Step 2 — Update `go.mod`

Replace the contents of `go.mod` with:

```go
module heic-converter

go 1.21

require (
	github.com/adrium/goheif v0.0.0-20230113133812-2cc00592af48
	github.com/rwcarlsen/goexif v0.0.0-20190401172101-9e8deecbddbd
	github.com/wailsapp/wails/v2 v2.9.2
)
```

---

## Step 3 — Tidy dependencies

```bash
go mod tidy
```

This will regenerate `go.sum` automatically. If you get a CGO error, make sure GCC is on your PATH:

```bash
gcc --version  # should return a version
```

---

## Step 4 — Update the HEIC conversion logic

Find the file that currently handles HEIC conversion (likely `internal/converter/converter.go` or similar). Replace the import and decode logic with the following pattern:

```go
package converter

import (
	"image/jpeg"
	"os"

	"github.com/adrium/goheif"
)

// ConvertHEICToJPEG converts a HEIC file to JPEG and saves it to destPath.
func ConvertHEICToJPEG(srcPath, destPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	img, err := goheif.Decode(srcFile)
	if err != nil {
		return err
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	return jpeg.Encode(destFile, img, &jpeg.Options{Quality: 90})
}
```

---

## Step 5 — Update EXIF extraction (if applicable)

If the project extracts EXIF metadata, replace `go-exif/v3` usage with `goexif`:

```go
import (
	"os"

	"github.com/rwcarlsen/goexif/exif"
)

func GetEXIF(srcPath string) (*exif.Exif, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return exif.Decode(f)
}
```

---

## Step 6 — Build and verify

```bash
# Standard build
go build ./...

# Wails build (if applicable)
wails build
```

Fix any import path mismatches that surface — the old package names may differ slightly.

---

## Step 7 — Commit and push

```bash
git add .
git commit -m "feat: replace go-libheif with goheif (no system DLL required)"
git push
```

---

## Notes

- `goheif` bundles `libde265` as C++ source files — no `.dll` or Chocolatey needed. GCC (MinGW) handles compilation automatically.
- `goexif` is the same library used in `dedup-photos`, so the API is already familiar.
- Wails version aligned to `v2.9.2` matching `dedup-photos`.
- If the actual file structure differs from what's assumed in Steps 4–5, adapt the package paths accordingly. The conversion and EXIF patterns above are drop-in templates.
