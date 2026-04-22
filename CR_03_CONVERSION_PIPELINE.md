# CR_03 — Conversion pipeline (pure-Go, parallel)

**Goal**: remove every remaining call to ImageMagick. Decode HEIC via
`Rosca75/heic`, encode via `image/jpeg` / `image/png` / `x/image/tiff` /
`gen2brain/webp`. Run conversions in parallel across `runtime.NumCPU()`
workers. Emit per-file progress events so the UI can show a real conversion
progress bar.

**Depends on**: phases 1 and 2 complete.

---

## 3.1 Quality semantics per format (read before coding)

The user-facing quality slider is 0–100, but only two of the four formats
actually apply a lossy quality factor. Map the slider to each format as
follows:

| Format | Library | Slider 0–100 → library parameter |
|--------|---------|-----------------------------------|
| **JPG** | `image/jpeg` | Passed directly as `jpeg.Options{Quality: q}`. Range 1–100. |
| **PNG** | `image/png` | PNG is **lossless**; quality controls compression effort. Map: `0–33 → png.NoCompression`, `34–66 → png.DefaultCompression`, `67–100 → png.BestCompression`. |
| **TIFF** | `x/image/tiff` | TIFF is **lossless**; `x/image/tiff.Encode` only supports `Uncompressed` and `Deflate`. Map: `0–33 → Uncompressed`, `34–100 → Deflate` (with `Predictor: true`, which is a free ~20% size reduction on photographic data). |
| **WebP** | `gen2brain/webp` | Passed directly as `webp.Options{Quality: float32(q), Method: 4}`. Method 4 is a good speed/size tradeoff (0 = fast/large, 6 = slow/small). |

For **JPG and WebP**, the slider genuinely trades off visual quality vs file
size. For **PNG and TIFF**, the slider trades off *encoder CPU time* vs file
size with no impact on visual fidelity — the frontend (phase 4) relabels the
slider to say so.

Clamp incoming values to 1–100 in `ConvertFiles` defensively.

---

## 3.2 Create `converter/encode.go`

```go
// converter/encode.go — format-specific encoders.
//
// One Go function per output format. Each takes an already-decoded
// image.Image and a 0–100 quality value, writes the encoded bytes to w, and
// returns an error. The dispatch table lives in converter.go.
package converter

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/gen2brain/webp"
	"golang.org/x/image/tiff"
)

// encodeJPEG writes img as JPEG with the given quality (1..100).
func encodeJPEG(w io.Writer, img image.Image, quality int) error {
	return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
}

// encodePNG writes img as PNG. PNG is lossless, so "quality" controls only
// the compression effort level.
func encodePNG(w io.Writer, img image.Image, quality int) error {
	var level png.CompressionLevel
	switch {
	case quality <= 33:
		level = png.NoCompression
	case quality <= 66:
		level = png.DefaultCompression
	default:
		level = png.BestCompression
	}
	enc := &png.Encoder{CompressionLevel: level}
	return enc.Encode(w, img)
}

// encodeTIFF writes img as TIFF. x/image/tiff supports only Uncompressed
// and Deflate; we toggle on the Deflate+Predictor combo for anything above
// the lowest third of the slider (Predictor gives a ~20% size reduction on
// photographic data for free).
func encodeTIFF(w io.Writer, img image.Image, quality int) error {
	opts := &tiff.Options{}
	if quality > 33 {
		opts.Compression = tiff.Deflate
		opts.Predictor = true
	}
	return tiff.Encode(w, img, opts)
}

// encodeWebP writes img as WebP. Quality is a true 0–100 lossy knob; Method
// 4 is the library's "balanced" speed/quality setting.
func encodeWebP(w io.Writer, img image.Image, quality int) error {
	return webp.Encode(w, img, webp.Options{
		Quality: float32(quality),
		Method:  4,
	})
}

// encoderFor returns the encoder function and output file extension for the
// given format string. Returns an error for unknown formats.
func encoderFor(format string) (func(io.Writer, image.Image, int) error, string, error) {
	switch format {
	case "jpg", "jpeg":
		return encodeJPEG, "jpg", nil
	case "png":
		return encodePNG, "png", nil
	case "tiff", "tif":
		return encodeTIFF, "tiff", nil
	case "webp":
		return encodeWebP, "webp", nil
	}
	return nil, "", fmt.Errorf("unsupported output format: %s", format)
}
```

> If Oscar decides to drop WebP later (the original item 7 only lists JPG,
> PNG, TIFF), delete `encodeWebP`, remove the `webp` case from `encoderFor`,
> drop `github.com/gen2brain/webp` from `go.mod`, and remove the `<option>`
> from `index.html`. Everything else keeps working.

---

## 3.3 Rewrite `converter/converter.go`

Replace the file entirely:

```go
// converter/converter.go — conversion orchestration.
//
// Parallel worker pool decodes each HEIC once via Rosca75/heic (WASM or
// dynamic libheif), then dispatches to encode.go for the chosen output
// format. No subprocesses are forked; no temp files are written.
package converter

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Rosca75/heic"
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

// ConvertPath is kept as a thin wrapper so the legacy binding in app.go
// still compiles. It expands a single file or directory path and delegates
// to ConvertFiles. Subfolder recursion is OFF (the new frontend uses
// ConvertFiles directly with the full expanded list).
func ConvertPath(inputPath, outputDir, format string, quality int) (ConversionSummary, error) {
	info, err := os.Stat(inputPath)
	if err != nil {
		return ConversionSummary{}, fmt.Errorf("cannot access input path: %w", err)
	}
	var paths []string
	if info.IsDir() {
		paths, err = listHEICFiles(inputPath, false)
		if err != nil {
			return ConversionSummary{}, err
		}
		if len(paths) == 0 {
			return ConversionSummary{}, errors.New("no HEIC/HEIF files found in the selected directory")
		}
	} else {
		paths = []string{inputPath}
	}
	return ConvertFiles(paths, outputDir, format, quality, nil)
}

// convertOne decodes one HEIC file and writes it through the provided
// encoder function. Returns the output path on success.
func convertOne(inputPath, outputDir, ext string, quality int, encode func(io.Writer, image.Image, int) error) (string, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", inputPath, err)
	}
	img, err := heic.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode %s: %w", inputPath, err)
	}
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, baseName+"."+ext)
	f, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", outputPath, err)
	}
	defer f.Close()
	if err := encode(f, img, quality); err != nil {
		return "", fmt.Errorf("encode %s: %w", outputPath, err)
	}
	return outputPath, nil
}

// isHEIC reports whether path has a .heic or .heif extension.
func isHEIC(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".heic" || ext == ".heif"
}
```

> ⚠ The `convertOne` signature references `io.Writer` and `image.Image`. Add
> `"image"` and `"io"` to the import list — I left them out of the example
> to keep it skimmable. Also remove the `CheckImageMagick` function
> entirely; nothing calls it anymore.

---

## 3.4 Update `app.go` — new binding with progress events

Replace `ConvertFiles` in `app.go` with a version that emits streaming
progress events:

```go
// ConvertFiles converts the given HEIC files to the target format in
// parallel. Emits convert:start with the total count, one convert:file
// event per file with {path, ok, done, total}, then convert:done with the
// summary.
func (a *App) ConvertFiles(paths []string, outputDir, format string, quality int) (converter.ConversionSummary, error) {
	runtime.EventsEmit(a.ctx, "convert:start", len(paths))
	summary, err := converter.ConvertFiles(paths, outputDir, format, quality,
		func(done, total int, currentPath string, ok bool) {
			runtime.EventsEmit(a.ctx, "convert:file", map[string]interface{}{
				"path":  currentPath,
				"ok":    ok,
				"done":  done,
				"total": total,
			})
		})
	runtime.EventsEmit(a.ctx, "convert:done", summary)
	return summary, err
}
```

The legacy `Convert(inputPath, ...)` binding can keep its current body
(`return converter.ConvertPath(...)`) — it is untouched by this change.

---

## 3.5 Update / rewrite `converter/converter_test.go`

The existing test asserts against the ImageMagick binary. Replace the body
so it only checks that the pure-Go pipeline produces a valid output file for
each format against one of Oscar's sample HEICs (copy one into
`converter/testdata/sample.heic` if there isn't one already, or skip the
test gracefully when `testdata/` is empty).

Minimum test:

```go
func TestConvertAllFormats(t *testing.T) {
	in := filepath.Join("testdata", "sample.heic")
	if _, err := os.Stat(in); err != nil {
		t.Skip("no sample.heic in converter/testdata/")
	}
	out := t.TempDir()
	for _, fmt := range []string{"jpg", "png", "tiff", "webp"} {
		_, err := ConvertFiles([]string{in}, out, fmt, 80, nil)
		if err != nil {
			t.Errorf("%s: %v", fmt, err)
		}
		got, _ := os.ReadDir(out)
		if len(got) == 0 {
			t.Errorf("%s: no output file", fmt)
		}
		_ = os.RemoveAll(out)
	}
}
```

Drop the existing `TestCheckImageMagick` test entirely — the function no
longer exists.

---

## 3.6 Acceptance checks

1. **Zero ImageMagick references**: `grep -rn "magick\|ImageMagick\|exec.Command" converter/ *.go`
   returns nothing. `CheckImageMagick` is gone from both Go and JS.
2. `go build ./...`, `go vet ./...`, and `go test ./...` all pass.
3. On `wails dev`, converting a folder of 50 HEICs to JPG finishes in
   roughly `(serial_time / NumCPU)` seconds. On an 8-core machine that is
   typically 6–10 s vs ~45 s before.
4. Quality slider at extremes produces visibly different file sizes for JPG
   and WebP (e.g. 10 vs 95 for a 4000×3000 HEIC should give files that
   differ by ~5× in size).
5. Quality slider on PNG produces different file sizes (larger at low
   quality, smaller at high quality) but pixel-identical images — verify with
   a hash.
6. Quality slider on TIFF: output at ≤33 is Uncompressed (bigger); output at
   > 33 is Deflate-compressed (smaller).
7. Cancelling / failing on one file does not abort the whole batch —
   `ConversionSummary.Failed` contains that one path; the rest complete.
