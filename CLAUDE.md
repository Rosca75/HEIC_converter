# HEIC Converter — Project Rules for Claude Code

---

## 1. Overall context

HEIC Converter is a **desktop application** built with **Wails v2** (Go backend + plain
HTML/JS/CSS frontend). It lets users select HEIC/HEIF photo files, inspect their
metadata, and batch-convert them to JPG, PNG, TIFF, or WebP using a **pure-Go
pipeline** — no external binaries, no ImageMagick, no CGo dependencies for image work.

- Runtime: Go 1.25+, Wails v2.12.
- HEIC decoding: `github.com/Rosca75/heic` — loads dynamic `libheif` via
  `purego` if present, otherwise falls back to a bundled WASM decoder run
  under `wazero`. Works on a vanilla Windows machine with no install.
- EXIF: `github.com/bep/imagemeta` (handles HEIF natively, ~2.6× faster
  than `rwcarlsen/goexif` with ~40× fewer allocations).
- Encoders: stdlib `image/jpeg`, `image/png`; `golang.org/x/image/tiff`;
  `github.com/gen2brain/webp`.
- No Node.js build step. Frontend files are served directly from `static/`.
- Target OS: Windows (primary), macOS and Linux supported by Wails.
- The Wails runtime JS is injected automatically at `/wails/runtime.js` —
  never add it manually.
- Go backend methods are exposed to JS via `window.go.main.App.<Method>(args)`.
  All calls are async Promises.

---

## 2. Project folder structure

```
heic-converter/
├── CLAUDE.md                  ← this file (project rules)
├── README.md                  ← user-facing docs
├── main.go                    ← Wails entry point, window config
├── app.go                     ← Go↔JS bridge: all bound methods
├── heic_init.go               ← initHEIC() wrapper, called once from main
├── heic_version_linux.go      ← purego libheif version probe (Linux)
├── heic_version_darwin.go     ← purego libheif version probe (macOS)
├── heic_version_other.go      ← stub for Windows/BSD (no probe)
├── wails.json                 ← Wails project config
├── go.mod / go.sum
├── converter/
│   ├── converter.go           ← ConvertFiles(): parallel worker pool
│   ├── convert_one.go         ← convertOne(), ConvertPath(), isHEIC()
│   ├── encode.go              ← JPG/PNG/TIFF/WebP encoder dispatch
│   ├── meta.go                ← GetFileMeta(), getOneFileMeta()
│   ├── thumb.go               ← readHEICHeader (192 KB), decodeHEICThumbnail, resize
│   ├── exif.go                ← extractExifInto via bep/imagemeta
│   ├── walk.go                ← expandPaths, listHEICFiles (recursive flag)
│   ├── quality.go             ← RecommendedQuality()
│   └── converter_test.go
├── static/
│   ├── index.html             ← UI shell (structure only)
│   ├── app.js                 ← init + dialogs + convert trigger
│   ├── table.js               ← bundle state, streaming progress, table render
│   └── styles.css             ← design tokens + layout
└── .github/workflows/
    ├── ci.yml                 ← go vet + gofmt + go test on push/PR to main
    └── release.yml            ← Windows + Linux binaries on v* tag push
```

New Go functionality goes into the `converter/` package, not `app.go`.
`app.go` is only a thin bridge: it calls `converter/` functions and emits
Wails events for the UI.

### 2.1 Refactoring rules

- **Never move or rename** `main.go`, `app.go`, `wails.json`, or `static/`.
  Wails depends on these paths.
- When adding a new feature area, create a **new file** in `converter/`
  rather than appending to an existing file.
- If any file exceeds 150 lines, split it. Name the new file after its
  dominant responsibility (e.g. `converter/walk.go`, `converter/encode.go`).
- Do not introduce sub-packages inside `converter/`. Keep everything in
  `package converter`.
- Frontend: split `app.js` / `table.js` further if they grow past 150 lines.
  Do not introduce module bundling or `import` statements — files are served
  as plain scripts.

### 2.2 Important notes

- **No subprocess forking.** Never call `exec.Command("magick", ...)` or any
  other CLI tool. All image work goes through the Go libraries listed above.
- **No temp files.** `convertOne` decodes in memory and streams directly to
  the output file via an `encode(io.Writer, image.Image, int) error` func.
- Always call `os.MkdirAll(outputDir, 0o755)` before writing output files.
- Use `filepath.Join` for all path construction. Never hardcode separators.
- Wails dialog functions (`runtime.OpenMultipleFilesDialog`, etc.) require
  `a.ctx` — `startup()` stores it and must have fired first.
- `initHEIC()` is called once at the very top of `main()` to select between
  dynamic libheif and the bundled WASM decoder. Do not re-initialise later.

---

## 3. Frontend layout

### 3.1 Layout specification

Single-panel layout inside a centred card. Zones stacked vertically:

```
┌─────────────────────────────────────────────────┐
│  ZONE A — Header                                │
│  Title + subtitle                               │
├─────────────────────────────────────────────────┤
│  ZONE B — Input selection                       │
│  [Select files]  [Select folder]                │
│  ☐ Include subfolders                           │
│  Drop zone (dashed border)                      │
├─────────────────────────────────────────────────┤
│  ZONE C — File bundle table                     │
│  Load-progress bar (while streaming)            │
│  thumb | name | path | resolution | date | cam  │
├─────────────────────────────────────────────────┤
│  ZONE D — Output & conversion options           │
│  [Select output folder]  output path display    │
│  Format selector   Quality slider (0–100)       │
│  Format-specific hint under the slider          │
├─────────────────────────────────────────────────┤
│  ZONE E — Action & status                       │
│  [Convert] button                               │
│  Convert-progress bar (during conversion)       │
│  Status / result pre block                      │
└─────────────────────────────────────────────────┘
```

### 3.2 Zones and rules

| Zone | ID              | Rules |
|------|-----------------|-------|
| A    | `#zone-header`  | Read-only. Never put interactive controls here. |
| B    | `#zone-input`   | Buttons call Go dialogs. "Include subfolders" checkbox belongs here. Drop zone is a child. |
| C    | `#zone-table`   | `overflow-y: auto`, `max-height: 280px`. Shared `.progress` bar lives here for file listing. |
| D    | `#zone-options` | Output folder picker + format/quality controls + `#qualityHint`. |
| E    | `#zone-action`  | Convert button, `.progress` bar for conversion, `<pre id="status">`. |

- Each zone's wrapper `<section>` uses its zone ID as the element `id`.
- Do not nest zones inside each other.
- All button click handlers live in `app.js`. **No `onclick=""` attributes
  in HTML.**
- The progress bar component (`.progress` / `.progress-bar` / `.progress-text`)
  is shared by load and convert flows — see `showProgress()` in `table.js`.

### 3.3 CSS layout approach

- Outer `.app` uses `display: grid; place-items: center` to centre the card.
- `.panel` uses `display: flex; flex-direction: column; gap: var(--space-md)`.
- Zones are direct `<section>` children of `.panel`.
- Use CSS custom properties for all spacing. Never hardcode magic pixel
  values in layout rules.
- Do not use CSS Grid inside zones unless the zone explicitly needs a
  two-column layout. Use flexbox first.

### 3.4 Design tokens

All visual values live on `:root` as CSS custom properties (colour, font,
spacing, radius). When adding new visual properties, add a token to `:root`
first, then reference it. Never hardcode colours, radii, or spacing values
in rule bodies.

---

## 4. Backend architecture

### 4.1 Go files (responsibility map)

| File                          | Responsibility |
|-------------------------------|----------------|
| `main.go`                     | Wails `Run()` config. Calls `initHEIC()` before opening the window. |
| `app.go`                      | All methods bound to JS. Thin wrappers + event emission. |
| `heic_init.go`                | `initHEIC()`: picks dynamic libheif vs. WASM. |
| `heic_version_*.go`           | Platform-specific libheif version probe via `purego`. |
| `converter/converter.go`      | `ConvertFiles()` — parallel worker pool; summary aggregation. |
| `converter/convert_one.go`    | `convertOne()`, `ConvertPath()`, `isHEIC()`. |
| `converter/encode.go`         | `encoderFor()` + `encodeJPEG/PNG/TIFF/WebP`. |
| `converter/meta.go`           | `GetFileMeta()`, `getOneFileMeta()`, `FileMeta` struct. |
| `converter/thumb.go`          | `readHEICHeader()` (192 KB), `decodeHEICThumbnail()`, `resizeImageToJPEG()`. |
| `converter/exif.go`           | `extractExifInto()` via `bep/imagemeta`. |
| `converter/walk.go`           | `ExpandPaths()`, `listHEICFiles()` (honours `recursive`). |
| `converter/quality.go`        | `RecommendedQuality()`. |

### 4.2 Struct & error conventions

- Structs exposed to JS: `FileResult`, `ConversionSummary`, `FileMeta`.
  JSON tags are in **camelCase** because the frontend reads them directly.
- Every exported function returns `(T, error)`.
- Wrap errors with `fmt.Errorf("context: %w", err)`. Never discard the
  original error.
- In `app.go`, return the error directly; Wails serialises it as a JS
  rejected Promise.

### 4.3 JS ↔ Go contract

Keep the comment block at the top of `app.go` in sync. Current surface:

| JS call                                             | Go method                              | Returns               |
|-----------------------------------------------------|----------------------------------------|-----------------------|
| `window.go.main.App.OpenFileDialog()`               | `OpenFileDialog()`                     | `[]string`            |
| `window.go.main.App.OpenFolderDialog()`             | `OpenFolderDialog()`                   | `string`              |
| `window.go.main.App.OpenOutputFolderDialog()`       | `OpenOutputFolderDialog()`             | `string`              |
| `window.go.main.App.GetFileMeta(paths, recursive)`  | `GetFileMeta([]string, bool)`          | `[]FileMeta`          |
| `window.go.main.App.GetFileMetaStreaming(paths, r)` | `GetFileMetaStreaming([]string, bool)` | `error` + events      |
| `window.go.main.App.ConvertFiles(paths, …)`         | `ConvertFiles([]string, …)`            | `ConversionSummary`   |
| `window.go.main.App.Convert(inputPath, …)`          | `Convert(string, …)` (legacy)          | `ConversionSummary`   |

Emitted Wails events:

- `meta:start` (total int) → `meta:file` (FileMeta) × N → `meta:done`
- `meta:progress` ({done, total}) — batch `GetFileMeta` only
- `convert:start` (total int) → `convert:file` ({path, ok, done, total}) × N → `convert:done` (summary)

### 4.4 Parallelism & performance

- Both `GetFileMeta` and `ConvertFiles` use a goroutine worker pool sized to
  `runtime.NumCPU()`, capped at 16. Beyond 16 workers we saturate disk I/O
  on typical SSDs and start losing to scheduler overhead.
- Metadata extraction reads a **192 KB header** via `readHEICHeader()`; the
  `Rosca75/heic` decoder walks the `iloc` box inside that buffer and
  extracts the embedded thumbnail without a full-file read. EXIF comes from
  the same buffer via `bep/imagemeta`. Fallback to full-file read is
  automatic in `decodeHEICThumbnail()` if the header-only path fails.
- Conversion decodes once in memory and encodes directly to the output
  file — no temp files, no subprocesses.

---

## 5. Key constraints

### 5.1 File length — 150-line maximum

No source file (Go or JS or CSS) may exceed **150 lines**. If an edit would
push a file past 150 lines, split it first.

- HTML is exempt (structural markup is verbose) but should stay under 120
  lines.
- Comments and blank lines count toward the limit.

### 5.2 Typography — monospace everywhere

The entire UI uses the monospace font stack defined in `--font-mono`.
Headings use the same stack at a larger size (`--font-size-lg`). Do not
introduce any sans-serif or serif font.

### 5.3 Documentation — every function gets a comment

Every Go function and every JS function has a one-line comment immediately
above it describing **what it does** (not how). CSS zone blocks carry a
label comment (`/* Zone C — file bundle table */`).

Where a Go idiom is non-obvious (goroutine fan-out, `atomic.Int32`,
`io.ReadFull` short-read semantics, `sync.Pool`), add a 1–3 line prose
comment explaining **why**, not just **what**. Treat the reader as someone
comfortable with another language but still learning Go.

### 5.4 No external frontend dependencies

No `<script src="...cdn...">` tags. No npm packages. No JS framework.
Vanilla JS + CSS custom properties only.

### 5.5 State management in JS

All mutable UI state lives in explicitly declared `let` variables at the
top of `table.js` (`bundle`, `outputPath`). The DOM is always derived from
state — never the source of truth. No state in `data-*` attributes except
transient click-target indices.

### 5.6 No inline styles or scripts

- No `style="..."` attributes in HTML.
- No `<style>` blocks in HTML.
- No `onclick=""` or other inline event handlers.
- All styling goes in `styles.css`. All logic goes in `app.js` / `table.js`.

### 5.7 Wails version lock

Do not upgrade Wails (currently `v2.12.0`) without explicit instruction. Do
not add new Go dependencies without confirming they are cross-platform and
do not require CGo beyond what Wails already manages.

### 5.8 No CGo for image work

`Rosca75/heic` and `gen2brain/webp` use `purego` + bundled WASM. This is
intentional — it means the Windows binary runs on a fresh machine with no
libraries to install. Do **not** reintroduce CGo-based image libraries
(e.g. ImageMagick bindings, `goheif` with CGo libheif).

### 5.9 Quality slider semantics per format

- **JPG / WebP**: true lossy quality, passed directly (1..100).
- **PNG**: lossless; slider picks `NoCompression` (≤33) / `DefaultCompression`
  (≤66) / `BestCompression` (>66).
- **TIFF**: lossless; slider ≤33 → `Uncompressed`, >33 → `Deflate + Predictor`.
- The frontend hint text (`#qualityHint`) reflects these semantics and must
  update whenever the format selector changes.

---

## 6. CI / Release

- `.github/workflows/ci.yml` runs `go vet`, `gofmt -l .`, `go build`, and
  `go test` on every push / PR to `main`, across `ubuntu-latest`,
  `windows-latest`, `macos-latest`. Ubuntu installs
  `libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config` before building.
- `.github/workflows/release.yml` triggers on tag push matching `v*`, builds
  `heic-converter.exe` (Windows) and `heic-converter-linux` (Linux, built
  with `-tags webkit2_41` for Ubuntu 24.04), then creates a GitHub Release
  with both binaries attached.
- Both binaries are self-contained: the HEIC WASM decoder and webp encoder
  are bundled. Users do **not** need libheif or ImageMagick installed.
