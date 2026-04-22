# HEIC Converter

A fast, self-contained desktop app that converts HEIC / HEIF photos to
**JPG, PNG, TIFF, or WebP** — with EXIF metadata preserved and a parallel
worker pool that uses every core on your machine.

Built with [Wails v2](https://wails.io) (Go backend + vanilla HTML/JS/CSS
frontend). No ImageMagick, no libheif install, no Node toolchain — the
Windows binary runs on a fresh machine out of the box.

## Features

- **Batch convert** HEIC / HEIF to JPG, PNG, TIFF, or WebP
- **Parallel pipeline** — uses up to 16 CPU workers at once
- **Pure-Go HEIC decoder** via [`Rosca75/heic`](https://github.com/Rosca75/heic):
  loads a dynamic `libheif` via `purego` if available, otherwise falls back
  to a bundled WASM decoder (wazero). Works on a vanilla Windows install
  with zero external dependencies.
- **192 KB header fast-path** for listing: dimensions, embedded thumbnail,
  and EXIF all come from one small read — listing 100 iPhone HEICs takes
  ~1–2 s instead of the 8–12 s the ImageMagick-based version took.
- **EXIF preserved** via [`bep/imagemeta`](https://github.com/bep/imagemeta)
  (~2.6× faster than `rwcarlsen/goexif`, handles HEIF natively).
- **Native file & folder dialogs**, plus OS-level drag-and-drop of HEIC files.
- **Include subfolders** checkbox for recursive directory scanning.
- **Format-aware quality slider** (0–100):
  - JPG / WebP → true visual quality
  - PNG → lossless; slider picks compression effort
  - TIFF → lossless; >33 switches on Deflate + Predictor
- **Live progress bars** for both file listing and conversion.
- **Cross-platform** — Windows (primary), macOS, Linux.

## Screenshots

The app is a single-window layout with five stacked zones:
header → input / drop zone → file bundle table → output & options → convert.

## Installation

### End users — pre-built binaries

Download the latest release from
[GitHub Releases](https://github.com/Rosca75/HEIC_converter/releases):

- **Windows**: `heic-converter.exe` — double-click to run, no install.
- **Linux**: `heic-converter-linux` — requires `libgtk-3-0` and
  `libwebkit2gtk-4.1-0` (standard on most desktop distros including
  Ubuntu 24.04).

### Developers — build from source

Prerequisites:

- **Go 1.25+** (<https://go.dev/dl/>)
- **Wails v2.12.0** — `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **Linux only**: `sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config`

Then:

```bash
git clone https://github.com/Rosca75/HEIC_converter.git
cd HEIC_converter
go mod download

# Run in dev mode with hot-reload:
wails dev

# Or build a release binary:
wails build                                  # Windows / macOS
wails build -tags webkit2_41                 # Linux (Ubuntu 24.04)
# Binary is written to build/bin/
```

On Windows you can also run the legacy environment probe:

```powershell
.\test_environment.ps1
```

## Usage

1. Click **Select files** or **Select folder** (optionally tick
   **Include subfolders**) — or drag HEIC files onto the drop zone.
2. Watch the file bundle table populate with thumbnails, dimensions,
   capture date, and camera model as the streaming progress bar advances.
3. Click **Select output folder** to pick where the converted files go.
4. Choose the output **Format** (JPG / PNG / TIFF / WebP) and adjust the
   **Quality** slider. The hint under the slider explains what the slider
   actually does for the currently-selected format.
5. Click **Convert**. The progress bar in the action zone animates as each
   file completes. A summary (`N converted, M failed, K skipped`) appears
   when the batch finishes.

## Architecture overview

- `main.go` + `app.go` — Wails entry point and Go↔JS bridge.
- `heic_init.go` + `heic_version_*.go` — picks between dynamic libheif and
  the bundled WASM decoder at startup, with a purego-based version probe on
  Linux and macOS (libheif < 1.18 forces WASM mode because older versions
  mis-decode iPhone HDR "tmap"-brand HEICs).
- `converter/` — pure-Go package:
  - `converter.go` / `convert_one.go` — parallel worker pool, single-file
    decode→encode, path expansion / HEIC detection.
  - `encode.go` — per-format encoder dispatch (JPG, PNG, TIFF, WebP).
  - `meta.go` / `thumb.go` / `exif.go` / `walk.go` — metadata fast-path:
    192 KB header read → dimensions + EXIF + embedded thumbnail.
- `static/` — `index.html`, `app.js`, `table.js`, `styles.css` (no build
  step, no frontend dependencies).
- `.github/workflows/` — `ci.yml` (vet + fmt + test on push/PR) and
  `release.yml` (Windows + Linux binaries on `v*` tag push).

Full design notes and project rules are in [`CLAUDE.md`](CLAUDE.md).

## Troubleshooting

- **Thumbnails blank for a file** → the 192 KB fast-path couldn't parse the
  container (common on some Samsung / Huawei HEICs). The decoder falls back
  to a full-file read automatically — the file will still convert, just
  slightly slower.
- **Conversion fails with "decode" error** → the HEIC file is likely
  damaged or uses a codec the bundled decoder doesn't support. Try
  re-saving from the source device.
- **Linux build fails with `webkit2gtk-4.0 not found`** → pass
  `-tags webkit2_41` to `wails build` (Ubuntu 24.04 dropped webkit2gtk-4.0).

## Releases

A new GitHub Release with Windows + Linux binaries is cut automatically
whenever a `v*` tag is pushed:

```bash
git tag v0.2.0 -m "Pure-Go HEIC pipeline; parallel conversion; subfolder scan"
git push origin v0.2.0
```

## License

MIT
