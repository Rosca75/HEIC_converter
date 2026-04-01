# HEIC Converter — Project Rules for Claude Code

---

## 1. Overall context

HEIC Converter is a **desktop application** built with **Wails v2** (Go backend + plain
HTML/JS/CSS frontend). It allows users to select HEIC/HEIF photo files, inspect their
metadata, and batch-convert them to JPG, PNG, WebP, or TIFF via ImageMagick.

- Runtime: Go 1.22+, Wails v2.12, ImageMagick (`magick` CLI) must be on PATH.
- No Node.js build step. Frontend files are served directly from `static/` by Wails.
- Target OS: Windows (primary), macOS and Linux supported by Wails.
- The Wails runtime JS is injected automatically at `/wails/runtime.js` — never add it
  manually.
- Go backend methods are exposed to JS via `window.go.main.App.<Method>(args)`.
  All calls are async and return Promises.

---

## 2. Overall project folder structure

### 2.1 Project and folder structure — target architecture

```
heic-converter/
├── CLAUDE.md                  ← this file (project rules)
├── FRONTEND-IMPROVEMENT.md    ← pending UI feature spec
├── main.go                    ← Wails entry point, window config
├── app.go                     ← Go↔JS bridge: all bound methods
├── wails.json                 ← Wails project config
├── go.mod / go.sum
├── converter/
│   ├── converter.go           ← ConvertPath(), ConvertFiles(), convertOne()
│   ├── meta.go                ← GetFileMeta(), thumbnail generation
│   ├── exif.go                ← EXIF helpers (currently stub)
│   └── quality.go             ← RecommendedQuality()
└── static/
    ├── index.html             ← UI shell (structure only, no inline styles/scripts)
    ├── app.js                 ← All frontend logic (vanilla JS, no framework)
    └── styles.css             ← All styles (CSS custom properties + layout)
```

New Go functionality must go into the `converter/` package, not into `app.go`.
`app.go` is only a thin bridge: it calls `converter/` functions and returns results.

### 2.2 Refactoring rules

- **Never move or rename** `main.go`, `app.go`, `wails.json`, or the `static/` folder.
  Wails depends on these paths.
- When adding a new feature area (e.g. metadata, presets), create a **new file** inside
  `converter/` rather than appending to an existing file.
- If any file exceeds 150 lines, split it. Name the new file after its dominant
  responsibility (e.g. `converter/thumb.go` for thumbnail logic).
- Do not introduce sub-packages inside `converter/`. Keep everything in `package converter`.
- Frontend: split `app.js` into logical sections with clear section-header comments if it
  grows beyond 150 lines. Do not introduce module bundling or `import` statements — the
  files are served as plain scripts.

### 2.3 Important notes

- ImageMagick is invoked via `exec.Command("magick", ...)`. Always capture both stdout and
  stderr with `CombinedOutput()` and include the output in any returned error.
- Never hardcode paths. Use `filepath.Join` for all path construction in Go.
- Always call `os.MkdirAll(outputDir, 0o755)` before writing any output file.
- Wails dialog functions (`runtime.OpenMultipleFilesDialog`, etc.) require `a.ctx` — make
  sure `startup()` has been called before any dialog method is invoked.

---

## 3. Frontend layout

### 3.1 Layout specification

The UI is a **single-panel layout** inside a centred card. Zones are stacked vertically:

```
┌─────────────────────────────────────────────────┐
│  ZONE A — Header                                │
│  Title + subtitle + ImageMagick status badge    │
├─────────────────────────────────────────────────┤
│  ZONE B — Input selection                       │
│  [Select files]  [Select folder]                │
│  Drop zone (dashed border)                      │
├─────────────────────────────────────────────────┤
│  ZONE C — File bundle table                     │
│  thumb | name | path | resolution | date | cam  │
│  (scrollable, max-height capped)                │
├─────────────────────────────────────────────────┤
│  ZONE D — Output & conversion options           │
│  [Select output folder]  output path display    │
│  Format selector   Quality slider               │
├─────────────────────────────────────────────────┤
│  ZONE E — Action & status                       │
│  [Convert] button                               │
│  Status / result pre block                      │
└─────────────────────────────────────────────────┘
```

### 3.2 Zones description and associated rules

| Zone | ID in HTML      | Rules |
|------|-----------------|-------|
| A    | `#zone-header`  | Read-only. Never put interactive controls here. |
| B    | `#zone-input`   | Buttons call Go dialogs. Drop zone is child of this zone. |
| C    | `#zone-table`   | `overflow-y: auto`, `max-height: 280px`. Empty state shows placeholder text. |
| D    | `#zone-options` | Output folder picker + format/quality controls. |
| E    | `#zone-action`  | Single Convert button + `<pre id="status">` result area. |

- Each zone must have its zone ID as the wrapper `<section>` element's `id`.
- Do not nest zones inside each other.
- All button click handlers live in `app.js`. No `onclick=""` attributes in HTML.

### 3.3 CSS layout approach

- The outer `.app` uses `display: grid; place-items: center` to centre the card.
- The `.panel` card uses `display: flex; flex-direction: column; gap: var(--space-md)`.
- Zones are direct `<section>` children of `.panel`.
- Use CSS custom properties for all spacing (see §3.4). Never use magic pixel values
  directly in layout rules.
- Do **not** use CSS Grid inside zones unless a zone explicitly needs two-column layout
  (e.g. Zone D options row). Use flexbox first.
- The drop zone (`#dropZone`) uses `border: 2px dashed var(--color-accent)` and transitions
  background on `.drag-over` class toggle.
- The file table (`#fileTable`) uses `border-collapse: collapse`. Rows alternate with
  `--color-row-alt` background. Long path cells use `text-overflow: ellipsis`.

### 3.4 Design token approach

All visual values must be defined as CSS custom properties on `:root`. Never hardcode
colours, radii, or spacing values in rule bodies.

```css
:root {
  /* Colour palette */
  --color-bg:        #f5f7ff;
  --color-bg-alt:    #eef1ff;
  --color-surface:   #ffffff;
  --color-border:    #d6def5;
  --color-accent:    #3c67ff;
  --color-accent-dim:#a9b8ef;
  --color-text:      #1a1f36;
  --color-text-muted:#4b587c;
  --color-danger:    #c0392b;
  --color-ok:        #1d7a3a;
  --color-row-alt:   #f9faff;

  /* Typography */
  --font-mono: "JetBrains Mono", "Fira Mono", "Consolas", monospace;
  --font-size-base: 0.9rem;
  --font-size-sm:   0.82rem;
  --font-size-lg:   1.6rem;

  /* Spacing scale */
  --space-xs:  4px;
  --space-sm:  8px;
  --space-md:  14px;
  --space-lg:  24px;

  /* Shape */
  --radius-sm: 6px;
  --radius-md: 10px;
  --radius-lg: 14px;
}
```

When adding new visual properties, always add a token to `:root` first, then reference it.

---

## 4. Backend architecture

### 4.1 Go files

| File                    | Responsibility |
|-------------------------|----------------|
| `main.go`               | Wails `Run()` config only. Window size, asset binding. |
| `app.go`                | All methods bound to JS. Thin wrappers — no business logic. |
| `converter/converter.go`| `ConvertPath()`, `ConvertFiles()`, `convertOne()`, `isHEIC()`, `CheckImageMagick()` |
| `converter/meta.go`     | `GetFileMeta()`, `generateThumb()`, `parseResolution()`, `parseCamera()` |
| `converter/exif.go`     | EXIF helpers if/when expanded beyond ImageMagick identify. |
| `converter/quality.go`  | `RecommendedQuality()` |

**Struct naming:**
- Result structs live in `converter/` and are tagged with `json:"camelCase"`.
- Structs exposed to JS: `FileResult`, `ConversionSummary`, `FileMeta`.
- Never expose unexported structs across the `app.go` / `converter` boundary.

**Error handling:**
- All exported functions return `(T, error)`.
- Wrap errors with `fmt.Errorf("context: %w", err)` — never discard the original error.
- In `app.go`, return the error directly; Wails serialises it as a JS rejected Promise.

### 4.2 API endpoints (JS↔Go contract)

Wails does not use HTTP. The full JS↔Go surface is:

| JS call                                       | Go method                    | Returns               |
|-----------------------------------------------|------------------------------|-----------------------|
| `window.go.main.App.CheckImageMagick()`       | `CheckImageMagick()`         | `string` (empty = OK) |
| `window.go.main.App.OpenFileDialog()`         | `OpenFileDialog()`           | `[]string`            |
| `window.go.main.App.OpenFolderDialog()`       | `OpenFolderDialog()`         | `string`              |
| `window.go.main.App.OpenOutputFolderDialog()` | `OpenOutputFolderDialog()`   | `string`              |
| `window.go.main.App.GetFileMeta(paths)`       | `GetFileMeta([]string)`      | `[]FileMeta`          |
| `window.go.main.App.ConvertFiles(paths,…)`    | `ConvertFiles([]string,…)`   | `ConversionSummary`   |

When adding a new bound method, add a row to this table in a comment block at the top of
`app.go` so the contract stays self-documented.

---

## 5. Key constraints

### 5.1 File length — 150-line maximum

No source file (Go or JS or CSS) may exceed **150 lines**. This limit exists for token
efficiency when Claude Code reads files into context.

- If an edit would push a file past 150 lines, split it first, then make the edit.
- HTML is exempt (structural markup is verbose) but should stay under 120 lines.
- Comments and blank lines count toward the limit.

### 5.2 Typography — monospace everywhere

The entire UI uses the monospace font stack defined in `--font-mono`. Apply it globally:

```css
body {
  font-family: var(--font-mono);
  font-size: var(--font-size-base);
}
```

This gives the app a consistent, technical aesthetic. Headings use the same stack at a
larger size (`--font-size-lg`). Do not introduce any sans-serif or serif font.

### 5.3 Documentation — every function gets a comment

Every Go function and every JS function must have a one-line comment immediately above it
describing **what it does** (not how).

Go:
```go
// convertOne converts a single HEIC file to the target format using ImageMagick.
func convertOne(inputPath, outputDir, format string, quality int) (string, error) {
```

JS:
```js
// renderTable rebuilds the file bundle table from the current bundle state.
function renderTable() {
```

CSS: each zone block must have a label comment:
```css
/* Zone C — file bundle table */
#zone-table { … }
```

### 5.4 No external frontend dependencies

Do not add `<script src="...cdn...">` tags, npm packages, or any JS framework.
The frontend is intentionally dependency-free. Vanilla JS and CSS custom properties suffice.

### 5.5 State management in JS

All mutable UI state lives in explicitly declared `let` variables at the top of `app.js`,
grouped under a `// --- State ---` comment block:

```js
// --- State ---
let bundle     = [];   // array of FileMeta objects currently in the table
let outputPath = '';   // user-selected output directory
```

No state is stored in DOM attributes or `data-*` properties. The DOM is always derived from
state — never the source of truth.

### 5.6 No inline styles or scripts

- No `style="..."` attributes in HTML.
- No `<style>` blocks in HTML.
- No `onclick=""` or other inline event handlers.
- All styling goes in `styles.css`. All logic goes in `app.js`.

### 5.7 Wails version lock

Do not upgrade Wails (currently `v2.12.0`) without explicit instruction. Do not add new Go
dependencies without confirming they are cross-platform and do not require CGo beyond what
Wails already manages.
