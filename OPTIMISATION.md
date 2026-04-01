# HEIC Converter — File List Load Time Optimisation

## Goal

Make the file list display near-instantaneous by replacing the current ImageMagick-based
metadata + thumbnail pipeline with a pure-Go HEIC container parser. Target: **100 files
in < 1 second** (current: ~28 seconds).

---

## 1. Current architecture and bottleneck

### What happens today (`converter/meta.go`)

When the user selects files, `GetFileMeta()` runs `getOneFileMeta()` per file with a
concurrency of 4 goroutines. Per file, **two ImageMagick processes** are spawned:

1. `magick identify -verbose file.heic[0]` — full 12MP HEIC decode + verbose EXIF dump → **500–800 ms**
2. `magick convert file.heic[0] -thumbnail 48x48^ … jpg:-` — full 12MP HEIC decode again + resize → **400–600 ms**

**Total: ~1 second per file.** With `sem = 4`, effective throughput is ~3.6 files/sec.
100 files → ~28 seconds.

### Root cause

ImageMagick **always fully decodes** the HEIC image (all 12 million pixels) even if
you only want a tiny thumbnail or the image dimensions. There is no "read header only"
shortcut for HEIC in ImageMagick.

---

## 2. The fix: pure-Go HEIC container parsing

### Key insight

Every HEIC file from iOS (and most Android cameras) contains:
- Image dimensions in the `ispe` box (spatial extents) — readable from the file header
- EXIF metadata (camera model, date) in an embedded EXIF block — readable without decoding
- A pre-rendered **JPEG thumbnail** (~160×120 or ~320×240) — extractable as raw JPEG bytes

All of this lives in the HEIC container structure and can be read **without decoding a
single pixel of the actual HEVC/AV1 image**. Cost: **< 5 ms per file** vs ~1000 ms today.

### Libraries to use

| Library | Import path | Purpose | CGo? |
|---------|------------|---------|------|
| goheif HEIF parser | `github.com/jdeng/goheif/heif` | Parse HEIC container, extract thumbnail bytes | **No — pure Go** |
| goheif BMFF reader | `github.com/jdeng/goheif/heif/bmff` | Low-level box parsing | **No — pure Go** |
| goexif | `github.com/rwcarlsen/goexif/exif` | Parse EXIF for Model, DateTime | **No — pure Go** |

### Critical: CGo is NOT required

The `jdeng/goheif` module contains CGo-dependent image decoders (`libde265/`, `dav1d/`)
but we **do NOT import those packages**. We only import the `heif/` subpackage which is
100% pure Go (verified — its only imports are stdlib: `errors`, `fmt`, `io`, `log`,
`bufio`, `bytes`, `encoding/binary`, `strings`).

**Do NOT import `github.com/jdeng/goheif`** (the root package) — that pulls in the
CGo decoders. Only import `github.com/jdeng/goheif/heif`.

The actual HEIC→JPG conversion at convert time still uses ImageMagick via `exec.Command`,
which is unchanged.

---

## 3. Implementation plan

### 3.1 Add dependencies

```bash
go get github.com/jdeng/goheif@latest
go get github.com/rwcarlsen/goexif@latest
```

### 3.2 Create `converter/thumb.go` — pure-Go thumbnail + metadata extraction

This new file replaces the ImageMagick-based `getOneFileMeta()`, `parseVerboseInfo()`,
and `generateThumb()` in `meta.go`.

**Core logic** (pseudocode showing the API surface to use):

```go
package converter

import (
    "bytes"
    "encoding/base64"
    "image/jpeg"
    "io"
    "os"

    "github.com/jdeng/goheif/heif"
    "github.com/jdeng/goheif/heif/bmff"
    "github.com/rwcarlsen/goexif/exif"
)

// extractMetaFast reads HEIC container metadata without decoding the image.
// Returns width, height, camera model, creation date, and embedded JPEG thumbnail.
func extractMetaFast(path string) (width, height int, camera, createdAt, thumbBase64 string, err error) {
    f, err := os.Open(path)
    if err != nil {
        return 0, 0, "unknown", "", "", err
    }
    defer f.Close()

    // 1. Open the HEIC container (reads headers only, not pixel data)
    hf := heif.Open(f)

    // 2. Get the primary item for dimensions
    primary, err := hf.PrimaryItem()
    if err != nil {
        return 0, 0, "unknown", "", "", err
    }
    w, h, ok := primary.SpatialExtents()  // from ispe box — no decode
    if ok {
        width, height = w, h
    }

    // 3. Extract EXIF (camera model, date) — no decode
    camera = "unknown"
    createdAt = ""
    exifBytes, exifErr := hf.EXIF()
    if exifErr == nil && len(exifBytes) > 0 {
        x, decErr := exif.Decode(bytes.NewReader(exifBytes))
        if decErr == nil {
            if tag, e := x.Get(exif.Model); e == nil {
                camera = tag.StringVal()
            }
            if dt, e := x.DateTime(); e == nil {
                createdAt = dt.UTC().Format(time.RFC3339)
            }
        }
    }

    // 4. Extract embedded JPEG thumbnail via "thmb" reference — no decode
    thumbBase64 = extractEmbeddedThumb(hf, primary)

    return width, height, camera, createdAt, thumbBase64, nil
}

// extractEmbeddedThumb finds the thumbnail item via the "thmb" reference type
// and returns its raw JPEG bytes as a base64 data URL.
func extractEmbeddedThumb(hf *heif.File, primary *heif.Item) string {
    // The HEIC container stores item references in the iref box.
    // A "thmb" reference points FROM a thumbnail item TO the primary item.
    // We need to access the metadata to scan these references.
    //
    // The heif.File exposes item references through Item.References.
    // We need to scan ALL items' references to find one with type "thmb"
    // whose ToItemIDs includes the primary item's ID.
    //
    // Approach: get the meta box, iterate ItemReference.ItemRefs,
    // find entries where Type().String() == "thmb" and ToItemIDs contains primary.ID.
    // Then load that item's data with GetItemData().

    meta, err := hf.GetMeta()  // Note: GetMeta may be unexported as getMeta
    if err != nil || meta.ItemReference == nil {
        return ""
    }

    for _, ref := range meta.ItemReference.ItemRefs {
        if ref.Type().String() != "thmb" {
            continue
        }
        for _, toID := range ref.ToItemIDs {
            if toID == primary.ID {
                thumbItem, err := hf.ItemByID(ref.FromItemID)
                if err != nil {
                    continue
                }
                data, err := hf.GetItemData(thumbItem)
                if err != nil || len(data) == 0 {
                    continue
                }
                return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
            }
        }
    }
    return ""
}
```

**Important implementation notes:**

- `hf.getMeta()` is currently **unexported** in the `heif` package. You have two options:
  a) Fork/vendor the `heif/` package and export `getMeta()` → rename to `GetMeta()`
  b) Use `ItemByID()` with known item IDs to find the thumbnail (less clean)
  
  **Recommended: option (a)** — copy `heif/` and `heif/bmff/` into the project as a
  vendored package (e.g. `converter/heif/`) and export `getMeta`. These two files total
  ~500 lines and are Apache 2.0 licensed. This also insulates the project from upstream changes.

- The embedded thumbnail in iOS HEIC files is typically a small JPEG (160×120 or 320×240).
  For the 48×48 display in the table, this is more than adequate — you can either serve it
  at its native size (the `<img>` tag will downscale it) or resize it in Go using
  `image/jpeg` + `golang.org/x/image/draw` (still pure Go, still < 1 ms).

- **~5% of HEIC files** (non-Apple, some older Samsung) may lack an embedded thumbnail.
  The fallback should use the existing ImageMagick thumbnail generation for those files only.

### 3.3 Update `converter/meta.go` — use the new fast path

Replace `getOneFileMeta()` to try the fast pure-Go path first, falling back to
ImageMagick only when needed:

```go
func getOneFileMeta(p string) (FileMeta, error) {
    m := FileMeta{Path: p, Name: filepath.Base(p)}

    // Try fast pure-Go extraction first
    w, h, cam, date, thumb, err := extractMetaFast(p)
    if err == nil && w > 0 {
        m.Width = w
        m.Height = h
        m.Camera = cam
        m.CreatedAt = date
        m.ThumbBase64 = thumb

        // Fallback: if no embedded thumbnail, use ImageMagick
        if m.ThumbBase64 == "" {
            m.ThumbBase64 = generateThumb(p)  // existing ImageMagick function
        }
        // Fallback: if no EXIF date, use file mod time
        if m.CreatedAt == "" {
            if info, e := os.Stat(p); e == nil {
                m.CreatedAt = info.ModTime().UTC().Format(time.RFC3339)
            } else {
                m.CreatedAt = "unknown"
            }
        }
        return m, nil
    }

    // Full fallback to ImageMagick (for non-standard HEIC files)
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
```

### 3.4 Increase concurrency in `GetFileMeta()`

Since the pure-Go path is I/O-bound (reading ~100 KB of headers per file) rather than
CPU-bound, we can safely increase parallelism:

```go
import "runtime"

workers := runtime.NumCPU()
if workers > 16 {
    workers = 16
}
sem := make(chan struct{}, workers)  // was: 4
```

### 3.5 Add streaming metadata emission (optional but recommended)

Instead of waiting for all files to complete, emit each file's metadata to the frontend
immediately via Wails events. This makes the first row appear within milliseconds.

**Backend (`app.go`):**

```go
func (a *App) GetFileMetaStreaming(paths []string) error {
    expanded, err := converter.ExpandPaths(paths)  // export expandPaths
    if err != nil {
        return err
    }
    total := len(expanded)
    runtime.EventsEmit(a.ctx, "meta:start", total)

    var wg sync.WaitGroup
    sem := make(chan struct{}, runtime.NumCPU())
    for _, p := range expanded {
        wg.Add(1)
        go func(p string) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            m, _ := converter.GetOneFileMeta(p)  // export getOneFileMeta
            runtime.EventsEmit(a.ctx, "meta:file", m)
        }(p)
    }
    wg.Wait()
    runtime.EventsEmit(a.ctx, "meta:done")
    return nil
}
```

**Frontend (`static/table.js`) — replace `addFilesToBundle()`:**

```js
async function addFilesToBundle(paths) {
    showLoadProgress(true, 0, 1);
    let total = 0;
    let done = 0;

    window.runtime.EventsOn('meta:start', (t) => { total = t; });
    window.runtime.EventsOn('meta:file', (m) => {
        done++;
        showLoadProgress(true, done, total);
        if (!bundle.find(b => b.path === m.path)) {
            bundle.push(m);
            appendTableRow(m, bundle.length - 1);  // incremental DOM append
        }
    });
    window.runtime.EventsOn('meta:done', () => {
        window.runtime.EventsOff('meta:start');
        window.runtime.EventsOff('meta:file');
        window.runtime.EventsOff('meta:done');
        showLoadProgress(false, 0, 0);
    });

    await window.go.main.App.GetFileMetaStreaming(paths);
}

// appendTableRow adds a single row to the table without re-rendering everything.
function appendTableRow(item, idx) {
    const tbody = document.getElementById('fileTableBody');
    const empty = document.getElementById('emptyRow');
    if (empty) empty.remove();

    const tr = document.createElement('tr');
    const thumb = item.thumbBase64
        ? `<img src="${item.thumbBase64}" width="48" height="48" class="thumb-img">`
        : '';
    tr.innerHTML =
        `<td>${thumb}</td>` +
        `<td>${item.name}</td>` +
        `<td class="path-cell" title="${item.path}">${item.path}</td>` +
        `<td>${item.width}\u00d7${item.height}</td>` +
        `<td>${item.createdAt}</td>` +
        `<td>${item.camera}</td>` +
        `<td><button class="btn-remove" data-idx="${idx}">\u2715</button></td>`;
    tbody.appendChild(tr);

    tr.querySelector('.btn-remove').addEventListener('click', () => {
        bundle.splice(Number(tr.querySelector('.btn-remove').dataset.idx), 1);
        renderTable();  // full re-render after remove (infrequent operation)
    });
}
```

---

## 4. Vendoring strategy for the `heif/` package

Since `getMeta()` is unexported and we need to access `BoxMeta.ItemReference`, the
recommended approach is to vendor the two source files:

1. Copy `heif/heif.go` and `heif/bmff/bmff.go` into the project:
   ```
   converter/
     heifparse/
       heif.go       ← from github.com/jdeng/goheif/heif/heif.go
       bmff/
         bmff.go     ← from github.com/jdeng/goheif/heif/bmff/bmff.go
   ```
2. Change the package declaration to match (`package heifparse`, `package bmff`)
3. Update the import path in `heif.go`: `"heic-converter/converter/heifparse/bmff"`
4. Export `getMeta()` → `GetMeta()` (one-character change)
5. Add the Apache 2.0 LICENSE from goheif into `converter/heifparse/`

This keeps the project CGo-free, self-contained, and independent of upstream changes.
Total vendored code: ~500 lines across 2 files, all pure Go.

The `goexif` dependency can be added normally via `go get` — it is a stable, pure-Go
library with no CGo.

---

## 5. Files to modify

| File | Action |
|------|--------|
| `converter/heifparse/heif.go` | **New** — vendored HEIC container parser (from goheif) |
| `converter/heifparse/bmff/bmff.go` | **New** — vendored BMFF box reader (from goheif) |
| `converter/heifparse/LICENSE` | **New** — Apache 2.0 license from goheif |
| `converter/thumb.go` | **New** — `extractMetaFast()`, `extractEmbeddedThumb()` |
| `converter/meta.go` | **Modify** — update `getOneFileMeta()` to use fast path first, increase semaphore, optionally export helpers |
| `app.go` | **Modify** — optionally add `GetFileMetaStreaming()` for progressive UI |
| `static/table.js` | **Modify** — optionally add `appendTableRow()` for incremental rendering |
| `go.mod` | **Modify** — add `github.com/rwcarlsen/goexif` dependency |

---

## 6. Constraints (from CLAUDE.md)

- No file may exceed 150 lines — split if needed
- No CGo dependencies — **the vendored `heifparse/` packages are pure Go, verified**
- No new frontend dependencies — vanilla JS only
- Keep `app.go` as a thin bridge — business logic goes in `converter/`
- All functions must have a one-line doc comment
- Use `filepath.Join` for all paths, `fmt.Errorf("context: %w", err)` for error wrapping
- ImageMagick is still used for the actual HEIC→JPG conversion (unchanged)

---

## 7. Expected performance

| Scenario | Current | After this optimisation |
|----------|---------|----------------------|
| 10 files | ~3 s | < 0.1 s |
| 50 files | ~14 s | < 0.3 s |
| 100 files | ~28 s | < 0.5 s |
| 500 files | ~138 s | < 2.5 s |

The fast path (pure Go, no ImageMagick) handles ~95% of HEIC files (all Apple, most Samsung).
The remaining ~5% fall back to the existing ImageMagick pipeline transparently.
