# HEIC Converter — Change Request for Claude Code

This change request replaces the ImageMagick-based HEIC pipeline with a pure-Go
architecture mirroring `Rosca75/dedup-photos` (branch `preview`), adds subfolder
scanning, wires the quality slider into real encoder knobs across all output
formats, and ships a CI + release pipeline that cuts Windows + Linux binaries on
every `v*` tag push.

---

## How to work through this CR

The work is split into **five numbered phase files** in this same folder. Each
file is self-contained and lists explicit file paths, exact code edits, and
acceptance checks. Implement them **in order**; do not skip ahead — phase 2 and
3 depend on the library swap being complete before the old ImageMagick paths are
removed.

| # | File | Focus | Key deliverable |
|---|------|-------|-----------------|
| 1 | `CR_01_DEPENDENCIES.md` | Swap `go.mod` to `Rosca75/heic` + `bep/imagemeta` + `gen2brain/webp`; add platform-specific libheif version probes. | New `go.mod` / `go.sum`; `heic_init.go` + three `heic_version_*.go` files; successful `go build ./...`. |
| 2 | `CR_02_METADATA_AND_THUMBNAILS.md` | Replace `extractMetaFast` + `generateThumb` + `parseVerboseInfo` with the 192 KB fast-path used by dedup-photos. Use `bep/imagemeta` for EXIF. | `converter/meta.go` + `converter/thumb.go` rewritten; zero `exec.Command("magick", ...)` calls in that code path. |
| 3 | `CR_03_CONVERSION_PIPELINE.md` | Rewrite `convertOne` / `ConvertFiles` / `ConvertPath` to decode with `Rosca75/heic` and encode with `image/jpeg`, `image/png`, `x/image/tiff`, `gen2brain/webp`. Parallelise conversion with a worker pool. Emit progress events. | New `converter/encode.go`; updated `converter/converter.go`; conversion no longer touches the `magick` binary. |
| 4 | `CR_04_FRONTEND_UI.md` | Add **"Include subfolders"** checkbox next to Select folder. Drop the ImageMagick status badge. Rework the quality slider (0–100, format-aware labelling, disabled for formats where it is inert). Wire a conversion progress bar. | Updated `static/index.html`, `static/app.js`, `static/table.js`, `static/styles.css`; new bound method `OpenFolderDialogRecursive`. |
| 5 | `CR_05_CI_RELEASE.md` | Add `.github/workflows/ci.yml` and `.github/workflows/release.yml`. Windows + Ubuntu 24.04 builds on tag push; Linux uses `-tags webkit2_41`. Create GitHub Releases with both binaries attached. | Two YAML files, tag-triggered, producing `heic-converter.exe` and `heic-converter-linux`. |

When in doubt, consult the reference implementation:
<https://github.com/Rosca75/dedup-photos/tree/preview>. Files particularly worth
mirroring: `heic_support.go`, `exif_extract.go`, `heic_version_*.go`, and
`.github/workflows/`.

---

## Target architecture (post-CR)

```
heic-converter/
├── CLAUDE.md                     ← unchanged (existing project rules)
├── main.go                       ← unchanged
├── app.go                        ← one new bound method + CheckImageMagick removed
├── wails.json                    ← unchanged
├── go.mod / go.sum               ← rewritten: ImageMagick deps out, Rosca75/heic in
├── heic_init.go                  ← NEW: initHEIC() wrapper
├── heic_version_linux.go         ← NEW: purego libheif version probe
├── heic_version_darwin.go        ← NEW: purego libheif version probe
├── heic_version_other.go         ← NEW: non-probe stub for Windows/BSD
├── converter/
│   ├── converter.go              ← rewritten: pure-Go decode + parallel pool
│   ├── encode.go                 ← NEW: format-specific encoders (jpg/png/tiff/webp)
│   ├── meta.go                   ← rewritten: 192 KB fast-path, bep/imagemeta
│   ├── thumb.go                  ← rewritten: heic.DecodeThumbnail + x/image/draw
│   ├── exif.go                   ← rewritten: extractExifInto using bep/imagemeta
│   ├── quality.go                ← unchanged
│   └── converter_test.go         ← updated assertions (no more ImageMagick expectation)
├── static/
│   ├── index.html                ← "Include subfolders" checkbox, quality slider 0–100
│   ├── app.js                    ← wired to new APIs, conversion progress
│   ├── table.js                  ← unchanged structurally, minor field rename if needed
│   └── styles.css                ← checkbox styling, progress bar for conversion
└── .github/
    └── workflows/
        ├── ci.yml                ← NEW
        └── release.yml           ← NEW
```

The 150-line-per-file rule from `CLAUDE.md` §5.1 is preserved. Two files
(`converter/converter.go` and `converter/meta.go`) are close to the limit
post-rewrite; if any ends up over, split the encoder dispatch into
`converter/encode.go` as already planned.

---

## Global constraints (apply across all phases)

1. **No CGo**, no external binaries, no ImageMagick. `Rosca75/heic` and
   `gen2brain/webp` both load a dynamic library via purego if available and
   otherwise run a WASM decoder via wazero — this is intentional and is the only
   acceptable pattern.
2. **Keep Wails at v2.12.0** and Go at 1.22+. Do not bump either gratuitously.
3. **Preserve the JS ↔ Go contract** listed at the top of `app.go` — when adding
   or removing a bound method, update that comment block in the same commit.
4. **Every exported Go function and every JS function must have a one-line
   comment** above it (project rule §5.3).
5. **Error handling**: every exported Go function returns `(T, error)`; wrap with
   `fmt.Errorf("context: %w", err)`, never discard the underlying error.
6. **Heavily commented Go code** — Oscar does not consider himself a Go expert.
   When the idiom is non-obvious (e.g. `sync.Pool`, `atomic.Int32`,
   `io.ReadFull` short-read semantics, goroutine fan-out), write a 1–3 line
   prose comment explaining *why*, not just *what*.
7. **No new frontend dependencies**. Vanilla JS + CSS custom properties only.
   No npm, no CDN `<script>` tags.

---

## Why this rewrite (summary of the performance thesis)

The current app's two hot paths both fork `magick` subprocesses:

- **Listing a folder of N HEIC files** spawns up to 2N `magick identify -verbose`
  + `magick convert … -thumbnail 48x48` processes. Process creation on Windows
  costs ~30–50 ms each; 100 files = 3–5 s just in fork overhead before any
  decoding happens.
- **Converting N files** spawns N `magick convert` processes serially, each of
  which re-reads the entire HEIC file and re-parses the container from scratch.

The dedup-photos pattern eliminates both costs:

- Thumbnails and dimensions are extracted from a **192 KB header read** — on
  iPhone HEICs the `ftyp` + `meta` + `iloc` + thumbnail tile all fit inside this
  window, so one I/O gives us everything the gallery needs.
- Conversion runs in a **goroutine worker pool** sized to `runtime.NumCPU()`,
  decodes once via the WASM libheif, and encodes through Go's stdlib — no
  process fork, no intermediate file, no re-parse.
- `bep/imagemeta` replaces `rwcarlsen/goexif`: ~2.6× faster with ~40× fewer
  allocations, and it handles HEIF natively (goexif cannot parse HEIF containers
  directly, which is why the current code uses a manual `heif.Open` first).

Expected improvement on a 100-file iPhone HEIC folder: listing drops from
~8–12 s to ~1–2 s; conversion from ~45 s (serial `magick`) to ~6–10 s (parallel
WASM) on a modern 8-core CPU.
