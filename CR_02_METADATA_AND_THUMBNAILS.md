# CR_02 — Metadata and thumbnails (fast path)

**Goal**: rewrite `converter/meta.go`, `converter/thumb.go`, and
`converter/exif.go` so that listing a folder of HEIC files never forks a
subprocess. Use the 192 KB header-read pattern from `dedup-photos/heic_support.go`
and route all EXIF through `bep/imagemeta`.

**Depends on**: phase 1 complete.

---

## 2.1 Performance thesis (why 192 KB)

iPhone HEIC files store their thumbnail tile (typically a 320×240 JPEG image
item) inside the `meta` → `iloc` box, whose absolute file offset is recorded in
the ISOBMFF header. The `Rosca75/heic` WASM decoder walks the `iloc` box to
find the tile; if the offset points inside the buffer we handed it, the decoder
reads directly from that buffer and never touches the rest of the file.

On every iPhone HEIC we tested, `ftyp` + `meta` + `iloc` + thumbnail tile all
live within the first 128 KB. We read 192 KB as headroom for edge cases
(portrait-mode HEICs with extra auxiliary images, HDR gain maps, and non-Apple
encoders like Samsung). If the 192 KB window is still too small, we fall back
to a full file read — rare on typical corpora.

This single pattern gives us:
- file dimensions (via `bep/imagemeta` reading the `ispe` box in the header)
- embedded thumbnail decoded to `image.Image` (via `heic.DecodeThumbnail`)
- EXIF block (via `bep/imagemeta` with `ImageFormat: imagemeta.HEIF`)

…all from **one 192 KB I/O per file**, typically parallelised across
`runtime.NumCPU()` goroutines.

---

## 2.2 Replace `converter/thumb.go` entirely

Delete the current contents of `converter/thumb.go` and replace with:

```go
// converter/thumb.go — HEIC thumbnail extraction via the 192 KB fast-path.
//
// The Rosca75/heic decoder walks the ISOBMFF `iloc` box to locate the
// embedded thumbnail tile. If the tile's absolute file offset falls inside
// the buffer we hand it, no additional I/O is required — so reading only
// the first 192 KB of the file is enough for iPhone HEICs.
package converter

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"

	"github.com/Rosca75/heic"
	"golang.org/x/image/draw"
)

// heicHeaderReadSize is the byte-range size used by the HEIC fast path.
// 192 KB comfortably covers ftyp + meta + iloc + thumbnail tile on every
// iPhone HEIC tested; if a specific file needs more, we fall back to a
// full read in decodeHEICThumbnail.
const heicHeaderReadSize = 192 * 1024

// readHEICHeader reads up to heicHeaderReadSize bytes from path. Short files
// are fine — io.ReadFull returns ErrUnexpectedEOF and we just use however
// many bytes we actually got.
func readHEICHeader(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, heicHeaderReadSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}

// decodeHEICThumbnail returns an image.Image thumbnail from a HEIC file.
// Tries, in order: header-only thumbnail decode → header-only primary decode
// (for files without an embedded thumbnail) → full-file thumbnail decode →
// full-file primary decode.
func decodeHEICThumbnail(path string) (image.Image, error) {
	header, err := readHEICHeader(path)
	if err != nil {
		return nil, err
	}
	if img, thumbErr := heic.DecodeThumbnail(bytes.NewReader(header)); thumbErr == nil {
		return img, nil
	}
	if img, decErr := heic.Decode(bytes.NewReader(header)); decErr == nil {
		return img, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if img, thumbErr := heic.DecodeThumbnail(bytes.NewReader(data)); thumbErr == nil {
		return img, nil
	}
	return heic.Decode(bytes.NewReader(data))
}

// heicThumbnailJPEG returns a JPEG-encoded, 48×48-max thumbnail for the
// gallery row. 48 px is enough for the table; we use ApproxBiLinear because
// it is ~10× faster than a manual img.At/Set loop and produces visibly
// better output at this size.
func heicThumbnailJPEG(path string) ([]byte, error) {
	img, err := decodeHEICThumbnail(path)
	if err != nil {
		return nil, err
	}
	out := resizeImageToJPEG(img, 48, 80)
	if out == nil {
		return nil, fmt.Errorf("jpeg encode failed")
	}
	return out, nil
}

// resizeImageToJPEG resizes img to fit inside maxDim×maxDim (preserving
// aspect ratio) and JPEG-encodes it at the given quality. Returns nil on
// encoder failure.
func resizeImageToJPEG(img image.Image, maxDim, quality int) []byte {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	newW, newH := srcW, srcH
	if srcW > maxDim || srcH > maxDim {
		if srcW >= srcH {
			newW = maxDim
			newH = srcH * maxDim / srcW
		} else {
			newH = maxDim
			newW = srcW * maxDim / srcH
		}
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}
	dstRect := image.Rect(0, 0, newW, newH)
	thumb := image.NewRGBA(dstRect)
	draw.ApproxBiLinear.Scale(thumb, dstRect, img, b, draw.Src, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: quality}); err != nil {
		return nil
	}
	return buf.Bytes()
}
```

> If the file ends up over 150 lines after you format it, split
> `resizeImageToJPEG` into a new file `converter/resize.go`. The budget is
> tight but the current draft sits at ~110 source lines once gofmt'd.

---

## 2.3 Replace `converter/exif.go` entirely

Delete the stub and replace with:

```go
// converter/exif.go — unified EXIF extraction via bep/imagemeta.
//
// bep/imagemeta handles JPEG, TIFF, WebP, PNG, HEIF/HEIC, AVIF, DNG, CR2,
// NEF, ARW — a superset of rwcarlsen/goexif's JPEG/TIFF-only coverage, and
// runs ~2.6× faster with ~40× fewer allocations. It is the only EXIF reader
// we use in this project.
package converter

import (
	"io"
	"strings"
	"time"

	"github.com/bep/imagemeta"
)

// extractExifInto populates the EXIF fields on meta from the HEIF stream in r.
// r must implement io.ReadSeeker (os.File and bytes.NewReader both do). Fields
// already set on meta are not overwritten — this lets callers pre-seed values
// from the ISOBMFF container before invoking this helper.
func extractExifInto(r io.ReadSeeker, meta *FileMeta) {
	var make_, model string

	_, _ = imagemeta.Decode(imagemeta.Options{
		R:           r,
		ImageFormat: imagemeta.HEIF,
		HandleTag: func(ti imagemeta.TagInfo) error {
			switch ti.Tag {
			case "DateTimeOriginal":
				if meta.CreatedAt == "" {
					switch v := ti.Value.(type) {
					case string:
						if t, err := time.Parse("2006:01:02 15:04:05", v); err == nil {
							meta.CreatedAt = t.UTC().Format(time.RFC3339)
						}
					case time.Time:
						if !v.IsZero() {
							meta.CreatedAt = v.UTC().Format(time.RFC3339)
						}
					}
				}
			case "Make":
				if s, ok := ti.Value.(string); ok {
					make_ = strings.TrimSpace(s)
				}
			case "Model":
				if s, ok := ti.Value.(string); ok {
					model = strings.TrimSpace(s)
				}
			}
			return nil
		},
	})

	if meta.Camera == "" || meta.Camera == "unknown" {
		combined := strings.TrimSpace(make_ + " " + model)
		if combined != "" {
			meta.Camera = combined
		}
	}
}
```

---

## 2.4 Rewrite `converter/meta.go`

This is the orchestrator. Replace the file entirely:

```go
// converter/meta.go — metadata extraction for the file-bundle table.
//
// Single-I/O strategy: read 192 KB, then pull dimensions from bep/imagemeta's
// CONFIG pass, decode the embedded thumbnail from the same buffer via
// Rosca75/heic, and extract EXIF from the same buffer via bep/imagemeta's
// tag callback. Files that do not yield to this path fall back to a full
// file read — same degradation pattern as dedup-photos.
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
		// Dimensions from the same buffer — no extra I/O.
		if res, metaErr := imagemeta.Decode(imagemeta.Options{
			R:           bytes.NewReader(header),
			ImageFormat: imagemeta.HEIF,
			Sources:     imagemeta.CONFIG,
		}); metaErr == nil && res.ImageConfig.Width > 0 {
			m.Width, m.Height = res.ImageConfig.Width, res.ImageConfig.Height
		}
		// EXIF from the same buffer.
		extractExifInto(bytes.NewReader(header), &m)
	}

	// Thumbnail — decodeHEICThumbnail re-reads the header internally and
	// falls back to a full file read only if needed. We accept the tiny
	// redundancy in exchange for a cleaner API.
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
```

> Line count is right at the 150-line limit. If after gofmt you are above,
> split `expandPaths` + `listHEICFiles` into a new file
> `converter/walk.go` — that is the cleanest cut.

---

## 2.5 Update `app.go`

Two surgical changes:

1. **Remove** the `CheckImageMagick` binding (method, docstring, and the
   matching line in the JS-Go contract comment block at the top of the file).
2. **Add a `recursive` parameter** to `GetFileMeta` and `GetFileMetaStreaming`.

Replace the top-of-file contract comment with:

```go
// JS↔Go API surface:
// window.go.main.App.OpenFileDialog()                    → []string
// window.go.main.App.OpenFolderDialog()                  → string
// window.go.main.App.OpenOutputFolderDialog()            → string
// window.go.main.App.GetFileMeta(paths, recursive)       → []FileMeta
// window.go.main.App.GetFileMetaStreaming(paths, r)      → error (emits meta:start/meta:file/meta:done)
// window.go.main.App.ConvertFiles(paths, out, fmt, q)    → ConversionSummary
//                                                          (emits convert:start/convert:file/convert:done — see phase 3)
```

Remove the `CheckImageMagick` method block entirely.

Change `GetFileMeta`:

```go
// GetFileMeta returns metadata and thumbnails for the given files or
// directories. If recursive is true, directories are walked into subfolders.
func (a *App) GetFileMeta(paths []string, recursive bool) ([]converter.FileMeta, error) {
	return converter.GetFileMeta(paths, recursive, func(done, total int) {
		runtime.EventsEmit(a.ctx, "meta:progress", map[string]interface{}{
			"done": done, "total": total,
		})
	})
}
```

Change `GetFileMetaStreaming`:

```go
// GetFileMetaStreaming emits meta:start with the total count, one meta:file
// event per file as soon as it is ready, then meta:done. If recursive is
// true, directories are walked into subfolders.
func (a *App) GetFileMetaStreaming(paths []string, recursive bool) error {
	expanded, err := converter.ExpandPaths(paths, recursive)
	if err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "meta:start", len(expanded))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for _, p := range expanded {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m, _ := converter.GetOneFileMeta(p)
			runtime.EventsEmit(a.ctx, "meta:file", m)
		}(p)
	}
	wg.Wait()
	runtime.EventsEmit(a.ctx, "meta:done")
	return nil
}
```

---

## 2.6 Acceptance checks

1. **No remaining `exec.Command("magick", …)` calls in `converter/thumb.go`,
   `converter/meta.go`, or `converter/exif.go`**. The only file that may still
   reference `magick` is `converter/converter.go`, which is handled in phase 3.
   Verify: `grep -n magick converter/*.go` shows matches only in
   `converter.go`.
2. `go build ./...` and `go vet ./...` both pass.
3. `wails dev` launches; selecting a folder of HEICs populates the table with
   thumbnails, dimensions, dates, and camera models — all without spawning a
   subprocess (check Task Manager / `htop` — no `magick` child).
4. Listing a folder of 100 iPhone HEICs takes **under 3 seconds** on a modern
   SSD (previously ~8–12 s). This is the primary success criterion for phase 2.
5. Conversion still works via the old ImageMagick path — that is fine, phase 3
   replaces it.

If thumbnails come up blank for a specific file, check whether it is a Samsung
or Huawei HEIC (non-Apple encoders sometimes write unusual tile layouts).
Those files will fall back to the full-file read path automatically — slow but
correct.
