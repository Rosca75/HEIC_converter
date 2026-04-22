# CR_04 — Frontend: subfolders, quality slider, progress

**Goal**: update the UI to match the new backend. Drop the ImageMagick status
badge, add an **Include subfolders** checkbox in Zone B, stretch the quality
slider to 0–100 with format-aware labelling, and show a conversion progress
bar so the user sees the parallel pool finish each file.

**Depends on**: phases 1–3 complete.

**Project rule reminder (`CLAUDE.md` §5.6)**: no inline styles, no inline
event handlers, no `<style>` blocks in HTML. Every JS function needs a
one-line `// what it does` comment above it.

---

## 4.1 Rewrite `static/index.html`

Replace the current file with:

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>HEIC Converter</title>
    <link rel="stylesheet" href="styles.css" />
  </head>
  <body>
    <main class="app">
      <div class="panel">

        <!-- Zone A — header -->
        <section id="zone-header">
          <h1>HEIC Converter</h1>
          <p class="subtitle">Convert HEIC / HEIF photos to JPG, PNG, TIFF, or WebP.</p>
        </section>

        <!-- Zone B — input selection -->
        <section id="zone-input">
          <div class="btn-row">
            <button id="btnPickFiles" type="button">Select files</button>
            <button id="btnPickFolder" type="button">Select folder</button>
            <label class="checkbox-inline" for="recursiveCheckbox">
              <input type="checkbox" id="recursiveCheckbox" />
              <span>Include subfolders</span>
            </label>
          </div>
          <div id="dropZone">
            <span>Drop HEIC / HEIF files here</span>
          </div>
        </section>

        <!-- Zone C — file bundle table -->
        <section id="zone-table">
          <div id="loadProgress" class="progress hidden">
            <div id="loadProgressBar" class="progress-bar"></div>
            <span id="loadProgressText" class="progress-text"></span>
          </div>
          <table id="fileTable">
            <thead>
              <tr>
                <th></th>
                <th>File name</th>
                <th>Path</th>
                <th>Resolution</th>
                <th>Created</th>
                <th>Camera</th>
                <th></th>
              </tr>
            </thead>
            <tbody id="fileTableBody">
              <tr id="emptyRow"><td colspan="7" class="empty-msg">No files selected.</td></tr>
            </tbody>
          </table>
        </section>

        <!-- Zone D — output & conversion options -->
        <section id="zone-options">
          <div class="options-row">
            <button id="btnPickOutput" type="button" class="btn-secondary">Select output folder</button>
            <div id="outputPathDisplay" class="path-badge">No output folder selected</div>
          </div>
          <div class="options-row">
            <label for="format">Format</label>
            <select id="format">
              <option value="jpg">JPG</option>
              <option value="png">PNG</option>
              <option value="tiff">TIFF</option>
              <option value="webp">WebP</option>
            </select>
            <label for="quality" id="qualityLabel">Quality: <strong id="qualityValue">85</strong></label>
            <input type="range" id="quality" min="0" max="100" value="85" />
            <small id="qualityHint" class="hint"></small>
          </div>
        </section>

        <!-- Zone E — action & status -->
        <section id="zone-action">
          <button id="convertBtn" type="button">Convert</button>
          <div id="convertProgress" class="progress hidden">
            <div id="convertProgressBar" class="progress-bar"></div>
            <span id="convertProgressText" class="progress-text"></span>
          </div>
          <pre id="status" aria-live="polite"></pre>
        </section>

      </div>
    </main>
    <script src="/wails/runtime.js"></script>
    <script src="table.js"></script>
    <script src="app.js"></script>
  </body>
</html>
```

Key diffs from the current file:
- **Removed**: `#imageMagickStatus` badge in Zone A.
- **Added**: `#recursiveCheckbox` inside `.btn-row` in Zone B.
- **Added**: `<option value="tiff">` moved up; order is now JPG/PNG/TIFF/WebP.
- **Changed**: quality slider `min="0"` (was 60).
- **Added**: `#qualityHint` element under the slider for format-specific text.
- **Added**: `#convertProgress` bar in Zone E.
- **Renamed**: load-progress CSS classes from `load-progress*` → `progress*`
  because the same visual component is now used twice.

---

## 4.2 Rewrite `static/app.js`

The file must stay under 150 lines (project rule §5.1). If you exceed the
budget, split the quality-hint logic into a separate `static/quality.js` and
add it as an additional `<script>` tag in `index.html`.

```js
'use strict';

window.addEventListener('DOMContentLoaded', async () => {
  // --- DOM refs ---
  const format       = document.getElementById('format');
  const quality      = document.getElementById('quality');
  const qualityValue = document.getElementById('qualityValue');
  const qualityHint  = document.getElementById('qualityHint');
  const convertBtn   = document.getElementById('convertBtn');
  const status       = document.getElementById('status');
  const recursive    = document.getElementById('recursiveCheckbox');

  // --- State ---
  // outputPath and bundle live in table.js (shared globals).

  // updateQualityUI relabels the slider hint based on the selected format.
  // JPG/WebP are truly lossy; PNG/TIFF use the slider to trade CPU for size.
  function updateQualityUI() {
    qualityValue.textContent = quality.value;
    switch (format.value) {
      case 'jpg':
      case 'webp':
        qualityHint.textContent = 'Visual quality — lower = smaller file, more artefacts.';
        break;
      case 'png':
        qualityHint.textContent = 'Lossless. Slider controls compression effort.';
        break;
      case 'tiff':
        qualityHint.textContent = 'Lossless. Above 33 switches on Deflate+Predictor compression.';
        break;
    }
  }
  format.addEventListener('change', updateQualityUI);
  quality.addEventListener('input', updateQualityUI);
  updateQualityUI();

  // --- Dialog buttons ---
  document.getElementById('btnPickFiles').addEventListener('click', async () => {
    const paths = await window.go.main.App.OpenFileDialog();
    if (paths && paths.length) addFilesToBundle(paths, recursive.checked);
  });
  document.getElementById('btnPickFolder').addEventListener('click', async () => {
    const dir = await window.go.main.App.OpenFolderDialog();
    if (dir) addFilesToBundle([dir], recursive.checked);
  });
  document.getElementById('btnPickOutput').addEventListener('click', async () => {
    const dir = await window.go.main.App.OpenOutputFolderDialog();
    if (dir) {
      outputPath = dir;
      document.getElementById('outputPathDisplay').textContent = dir;
    }
  });

  // --- Drop zone visual feedback ---
  const dropZone = document.getElementById('dropZone');
  dropZone.addEventListener('dragover', e => {
    e.preventDefault();
    dropZone.classList.add('drag-over');
  });
  dropZone.addEventListener('dragleave', () => dropZone.classList.remove('drag-over'));
  dropZone.addEventListener('drop', e => {
    e.preventDefault();
    dropZone.classList.remove('drag-over');
  });

  // --- Wails OS-level file drop — handles real filesystem paths ---
  if (window.runtime?.OnFileDrop) {
    window.runtime.OnFileDrop((x, y, paths) => {
      dropZone.classList.remove('drag-over');
      const heic = paths.filter(p => /\.(heic|heif)$/i.test(p));
      if (heic.length) addFilesToBundle(heic, recursive.checked);
    }, false);
  }

  // --- Convert button with streaming progress ---
  convertBtn.addEventListener('click', async () => {
    if (!bundle.length) { status.textContent = 'Add files first.'; return; }
    if (!outputPath)    { status.textContent = 'Select an output folder first.'; return; }
    convertBtn.disabled = true;
    status.textContent = '';
    bindConvertProgress();
    try {
      const result = await window.go.main.App.ConvertFiles(
        bundle.map(b => b.path), outputPath, format.value, Number(quality.value)
      );
      const conv = result.converted?.length ?? 0;
      const fail = result.failed?.length ?? 0;
      const skip = result.skipped?.length ?? 0;
      status.textContent = `Done. ${conv} converted, ${fail} failed, ${skip} skipped.`;
    } catch (err) {
      status.textContent = `Conversion failed: ${err}`;
    } finally {
      convertBtn.disabled = false;
      showConvertProgress(false, 0, 0);
    }
  });
});
```

---

## 4.3 Update `static/table.js`

Two changes only:
1. Pass the `recursive` flag through to the streaming call.
2. Add the `bindConvertProgress` + `showConvertProgress` functions used above.

Replace the file with:

```js
'use strict';

// --- State (module-level globals shared with app.js) ---
let bundle     = [];  // array of FileMeta objects currently in the table
let outputPath = '';  // user-selected output directory

// showProgress toggles a progress bar by element prefix ('load' or 'convert').
function showProgress(prefix, show, done, total) {
  const el  = document.getElementById(prefix + 'Progress');
  const bar = document.getElementById(prefix + 'ProgressBar');
  const txt = document.getElementById(prefix + 'ProgressText');
  if (!el) return;
  if (!show) { el.classList.add('hidden'); return; }
  el.classList.remove('hidden');
  bar.style.width = (total > 0 ? (done / total) * 100 : 0) + '%';
  txt.textContent = prefix === 'load'
    ? `Loading ${done} / ${total}`
    : `Converting ${done} / ${total}`;
}
const showLoadProgress    = (show, d, t) => showProgress('load', show, d, t);
const showConvertProgress = (show, d, t) => showProgress('convert', show, d, t);

// bindConvertProgress wires the Wails event stream for the conversion run.
// Called right before window.go.main.App.ConvertFiles(...) is invoked.
function bindConvertProgress() {
  if (!window.runtime?.EventsOn) return;
  let total = 0;
  window.runtime.EventsOn('convert:start', t => { total = t; showConvertProgress(true, 0, total); });
  window.runtime.EventsOn('convert:file',  m => { showConvertProgress(true, m.done, m.total); });
  window.runtime.EventsOn('convert:done',  () => {
    window.runtime.EventsOff('convert:start');
    window.runtime.EventsOff('convert:file');
    window.runtime.EventsOff('convert:done');
  });
}

// addFilesToBundle calls GetFileMetaStreaming and appends rows as events arrive.
async function addFilesToBundle(paths, recursive) {
  showLoadProgress(true, 0, 1);
  let total = 0;
  let done  = 0;
  if (window.runtime?.EventsOn) {
    window.runtime.EventsOn('meta:start', (t) => { total = t; });
    window.runtime.EventsOn('meta:file', (m) => {
      done++;
      showLoadProgress(true, done, total || 1);
      if (!bundle.find(b => b.path === m.path)) {
        bundle.push(m);
        appendTableRow(m, bundle.length - 1);
      }
    });
    window.runtime.EventsOn('meta:done', () => {
      window.runtime.EventsOff('meta:start');
      window.runtime.EventsOff('meta:file');
      window.runtime.EventsOff('meta:done');
      showLoadProgress(false, 0, 0);
    });
  }
  try {
    await window.go.main.App.GetFileMetaStreaming(paths, !!recursive);
  } catch {
    if (window.runtime?.EventsOff) {
      window.runtime.EventsOff('meta:start');
      window.runtime.EventsOff('meta:file');
      window.runtime.EventsOff('meta:done');
    }
    showLoadProgress(false, 0, 0);
  }
}

// appendTableRow adds one FileMeta row to the table body without a full rerender.
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
    renderTable();
  });
}

// renderTable rebuilds the entire table body from the current bundle state.
function renderTable() {
  const tbody = document.getElementById('fileTableBody');
  if (!bundle.length) {
    tbody.innerHTML = '<tr id="emptyRow"><td colspan="7" class="empty-msg">No files selected.</td></tr>';
    return;
  }
  tbody.innerHTML = '';
  bundle.forEach((item, idx) => appendTableRow(item, idx));
}
```

---

## 4.4 Extend `static/styles.css`

Add these blocks (append; do not remove anything unrelated). If the
current `load-progress*` rules are still in the file, rename them to
`progress*` to match the new class naming.

```css
/* Inline checkbox next to pick-file / pick-folder buttons */
.checkbox-inline {
  display: inline-flex;
  align-items: center;
  gap: var(--space-xs);
  font-size: var(--font-size-sm);
  color: var(--color-text);
  cursor: pointer;
}
.checkbox-inline input[type="checkbox"] {
  accent-color: var(--color-accent);
}

/* Shared progress bar — used for both load and convert */
.progress {
  position: relative;
  width: 100%;
  height: 18px;
  background: var(--color-bg-alt);
  border-radius: var(--radius-sm);
  overflow: hidden;
  margin: var(--space-xs) 0;
}
.progress.hidden { display: none; }
.progress-bar {
  height: 100%;
  background: var(--color-accent);
  transition: width 120ms linear;
}
.progress-text {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: var(--font-size-sm);
  color: var(--color-text);
  pointer-events: none;
}

/* Small italic hint under the quality slider */
.hint {
  color: var(--color-text-muted);
  font-size: var(--font-size-sm);
  font-style: italic;
}
```

Also **remove** any CSS block that styled `#imageMagickStatus` (the badge no
longer exists). Search for `.status-badge` and delete the whole rule if it
is not used elsewhere.

---

## 4.5 Acceptance checks

1. The window no longer shows an ImageMagick badge at the top.
2. "Include subfolders" checkbox appears next to the Select folder button.
   Ticking it and selecting a folder walks into subdirectories; unticking
   it ignores them. Verify by pointing at a nested structure with HEICs in
   child folders.
3. Quality slider starts at 85, ranges 0–100, and the hint text under it
   changes when the format dropdown changes.
4. During a conversion of 20+ files, the progress bar in Zone E animates
   smoothly from 0 to 100% as each file completes — proving the events
   fire per-file (not just at start and end).
5. No `console.error` entries. No `onclick=""` attributes in the rendered
   DOM. No `<style>` blocks or `style="..."` attributes in `index.html`.
