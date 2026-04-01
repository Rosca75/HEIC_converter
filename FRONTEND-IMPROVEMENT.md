# HEIC Converter — UI Enhancement Spec

## Project overview

This is a **Wails v2** desktop app (Go backend + vanilla HTML/JS/CSS frontend, no build framework).

```
main.go               — Wails entry point (window 860×620)
app.go                — Go methods bound to the JS frontend
converter/
  converter.go        — ConvertPath(), convertOne(), CheckImageMagick()
  exif.go             — (stub — EXIF preserved by ImageMagick)
  quality.go          — RecommendedQuality()
static/
  index.html          — UI shell
  app.js              — All frontend logic (vanilla JS)
  styles.css          — Styles (no framework)
```

Frontend calls Go via `window.go.main.App.<MethodName>(args...)` — all async, return Promises.

---

## What to build — 5 improvements

### 1 · File / folder picker buttons

Replace the plain text inputs with proper native-dialog buttons.

**Go side — add to `app.go`:**
```go
import "github.com/wailsapp/wails/v2/pkg/runtime"

// OpenFileDialog opens a native multi-file picker filtered to HEIC/HEIF.
func (a *App) OpenFileDialog() ([]string, error) {
    return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
        Title: "Select HEIC / HEIF files",
        Filters: []runtime.FileFilter{
            {DisplayName: "HEIC / HEIF images", Pattern: "*.heic;*.heif;*.HEIC;*.HEIF"},
        },
    })
}

// OpenFolderDialog opens a native folder picker.
func (a *App) OpenFolderDialog() (string, error) {
    return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
        Title: "Select folder containing HEIC files",
    })
}

// OpenOutputFolderDialog opens a native folder picker for the output directory.
func (a *App) OpenOutputFolderDialog() (string, error) {
    return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
        Title: "Select output folder",
    })
}
```

**Frontend — `static/index.html`:**
- Remove the old `<input type="text" id="inputPath">` and `<input type="text" id="outputDir">`.
- Add two buttons: **"Select files"** and **"Select folder"** for the input side.
- Add one button: **"Select output folder"** for the destination.
- Show the resolved path(s) in a read-only `<span>` or `<div>` beneath each button.

**Frontend — `static/app.js`:**
```js
// "Select files" button
document.getElementById('btnPickFiles').addEventListener('click', async () => {
  const paths = await window.go.main.App.OpenFileDialog();
  if (paths && paths.length) addFilesToBundle(paths);
});

// "Select folder" button
document.getElementById('btnPickFolder').addEventListener('click', async () => {
  const dir = await window.go.main.App.OpenFolderDialog();
  if (dir) addFilesToBundle([dir]); // folder path fed to ConvertPath as before
});

// "Select output folder" button
document.getElementById('btnPickOutput').addEventListener('click', async () => {
  const dir = await window.go.main.App.OpenOutputFolderDialog();
  if (dir) { outputPath = dir; document.getElementById('outputPathDisplay').textContent = dir; }
});
```

---

### 2 · Output folder selector

Already covered above via `OpenOutputFolderDialog()`.

- Store the chosen output path in a JS variable `let outputPath = ''`.
- Display it in `<div id="outputPathDisplay">` (styled as a read-only path badge).
- Pass `outputPath` to `Convert()` at conversion time.
- Validate it is set before allowing conversion to start.

---

### 3 · Drag-and-drop zone

Add a visible drop area in `index.html` above the file table.

```html
<div id="dropZone">
  <span>Drop HEIC / HEIF files here</span>
</div>
```

**`app.js` drag-and-drop handling:**
```js
const dropZone = document.getElementById('dropZone');

dropZone.addEventListener('dragover', e => {
  e.preventDefault();
  dropZone.classList.add('drag-over');
});
dropZone.addEventListener('dragleave', () => dropZone.classList.remove('drag-over'));
dropZone.addEventListener('drop', e => {
  e.preventDefault();
  dropZone.classList.remove('drag-over');
  const paths = Array.from(e.dataTransfer.files)
    .map(f => f.path)                          // Wails exposes the real OS path via file.path
    .filter(p => /\.(heic|heif)$/i.test(p));
  if (paths.length) addFilesToBundle(paths);
});
```

**`styles.css` — drop zone styling:**
```css
#dropZone {
  border: 2px dashed #3c67ff;
  border-radius: 10px;
  padding: 18px;
  text-align: center;
  color: #4b587c;
  cursor: default;
  transition: background 0.15s;
  margin-bottom: 14px;
}
#dropZone.drag-over {
  background: #eef1ff;
}
```

---

### 4 · File bundle table with metadata + thumbnail

**Go side — add to `app.go`:**

Add a new method that returns metadata for a list of file paths. Use ImageMagick's `identify` command to get resolution, and Go's `os.Stat` for dates. For the camera model, parse EXIF via ImageMagick's `identify -verbose` output (look for `EXIF:Model`).

```go
type FileMeta struct {
    Path        string `json:"path"`
    Name        string `json:"name"`
    Width       int    `json:"width"`
    Height      int    `json:"height"`
    CreatedAt   string `json:"createdAt"`   // RFC3339 or "unknown"
    Camera      string `json:"camera"`       // EXIF Model or "unknown"
    ThumbBase64 string `json:"thumbBase64"`  // base64 data URL of a 12×12 thumbnail
}

func (a *App) GetFileMeta(paths []string) ([]FileMeta, error) {
    // For each path:
    //   1. Run: magick identify -format "%wx%h" <path>  → parse width/height
    //   2. Run: magick identify -verbose <path>         → grep "EXIF:Model" and "date:create"
    //   3. Run: magick convert <path> -thumbnail 12x12^ -gravity center -extent 12x12 jpg:- → pipe to base64
    //   4. Use os.Stat for fallback date (ModTime)
    // Return []FileMeta
}
```

**Frontend — `app.js`:**

```js
// Central bundle state
let bundle = []; // array of { path, meta }

async function addFilesToBundle(paths) {
  const metas = await window.go.main.App.GetFileMeta(paths);
  metas.forEach(m => {
    if (!bundle.find(b => b.path === m.path)) bundle.push(m);
  });
  renderTable();
}
```

**`index.html` — table:**
```html
<table id="fileTable">
  <thead>
    <tr>
      <th></th>           <!-- thumbnail -->
      <th>File name</th>
      <th>Path</th>
      <th>Resolution</th>
      <th>Created</th>
      <th>Camera</th>
      <th></th>           <!-- remove button -->
    </tr>
  </thead>
  <tbody id="fileTableBody"></tbody>
</table>
```

**`app.js` — render function:**
```js
function renderTable() {
  const tbody = document.getElementById('fileTableBody');
  tbody.innerHTML = '';
  bundle.forEach((item, idx) => {
    const tr = document.createElement('tr');
    tr.innerHTML = `
      <td><img src="${item.thumbBase64}" width="12" height="12" style="image-rendering:pixelated"></td>
      <td>${item.name}</td>
      <td class="path-cell" title="${item.path}">${item.path}</td>
      <td>${item.width}×${item.height}</td>
      <td>${item.createdAt}</td>
      <td>${item.camera}</td>
      <td><button class="btn-remove" data-idx="${idx}">✕</button></td>
    `;
    tbody.appendChild(tr);
  });
  document.querySelectorAll('.btn-remove').forEach(btn => {
    btn.addEventListener('click', () => {
      bundle.splice(Number(btn.dataset.idx), 1);
      renderTable();
    });
  });
}
```

**`styles.css` additions:**
```css
#fileTable {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.82rem;
  margin-top: 14px;
}
#fileTable th, #fileTable td {
  padding: 5px 8px;
  border-bottom: 1px solid #dee5ff;
  text-align: left;
  vertical-align: middle;
}
.path-cell {
  max-width: 160px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.btn-remove {
  background: none;
  border: none;
  color: #c0392b;
  cursor: pointer;
  font-size: 0.85rem;
  padding: 0 4px;
  width: auto;
  margin: 0;
}
```

---

### 5 · Remove a file from the bundle

Already implemented in the `renderTable()` function above via the **✕** button per row.

Each click calls `bundle.splice(idx, 1)` then re-renders the table.

---

## Conversion flow — updated

When the user clicks **Convert**:

1. Collect file paths: `bundle.map(b => b.path)` — these are individual HEIC paths.
2. Pass them to a **new** Go method `ConvertFiles(paths []string, outputDir, format string, quality int)` that loops through each path, calling `convertOne()`.
3. Add a new method to `app.go`:

```go
func (a *App) ConvertFiles(paths []string, outputDir, format string, quality int) (converter.ConversionSummary, error) {
    return converter.ConvertFiles(paths, outputDir, format, quality)
}
```

And in `converter/converter.go`:
```go
func ConvertFiles(paths []string, outputDir, format string, quality int) (ConversionSummary, error) {
    if err := os.MkdirAll(outputDir, 0o755); err != nil {
        return ConversionSummary{}, fmt.Errorf("cannot create output directory: %w", err)
    }
    summary := ConversionSummary{}
    for _, p := range paths {
        out, err := convertOne(p, outputDir, format, quality)
        if err != nil {
            return ConversionSummary{}, fmt.Errorf("error converting %s: %w", p, err)
        }
        summary.Converted = append(summary.Converted, FileResult{Input: p, Output: out})
    }
    return summary, nil
}
```

Keep the existing `ConvertPath()` intact (it handles folder-based input from the old flow).

---

## Window size

Increase the window height in `main.go` to accommodate the table:

```go
Width:  1000,
Height: 760,
```

---

## File checklist

| File | Action |
|---|---|
| `app.go` | Add `OpenFileDialog`, `OpenFolderDialog`, `OpenOutputFolderDialog`, `GetFileMeta`, `ConvertFiles` |
| `converter/converter.go` | Add `ConvertFiles()` function |
| `static/index.html` | Replace text inputs with picker buttons + drop zone + file table |
| `static/app.js` | Full rewrite: bundle state, drag-drop, table render, remove, new convert flow |
| `static/styles.css` | Add drop zone, table, remove-button styles |
| `main.go` | Update window dimensions |

---

## Notes for Claude Code

- Do **not** introduce any npm/node build step. The frontend is plain HTML/JS/CSS served directly by Wails from the `static/` folder.
- Wails v2 runtime is available at `/wails/runtime.js` (injected at runtime) — do not add it manually.
- Use `runtime.OpenMultipleFilesDialog` / `runtime.OpenDirectoryDialog` from `github.com/wailsapp/wails/v2/pkg/runtime` — these are already available in the module, no new dependency needed.
- For `GetFileMeta`, spawn ImageMagick subprocesses the same pattern as `convertOne` — use `exec.Command("magick", ...)`.
- The `file.path` property in the drag-drop handler works in Wails's embedded WebView (Chromium) — it exposes the real OS path, unlike a browser.
- Preserve the existing `CheckImageMagick()` call on startup and the quality/format controls.
