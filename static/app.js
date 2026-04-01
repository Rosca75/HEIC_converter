'use strict';

// --- State ---
let bundle     = [];  // array of FileMeta objects currently in the table
let outputPath = '';  // user-selected output directory

window.addEventListener('DOMContentLoaded', async () => {
  // --- DOM refs ---
  const format       = document.getElementById('format');
  const quality      = document.getElementById('quality');
  const qualityValue = document.getElementById('qualityValue');
  const qualityLabel = document.getElementById('qualityLabel');
  const convertBtn   = document.getElementById('convertBtn');
  const status       = document.getElementById('status');
  const imStatus     = document.getElementById('imageMagickStatus');

  // --- ImageMagick availability check ---
  if (window.go?.main?.App?.CheckImageMagick) {
    try {
      const err = await window.go.main.App.CheckImageMagick();
      if (err) {
        imStatus.textContent = 'ImageMagick not found — ' + err;
        imStatus.className = 'status-badge status-error';
        convertBtn.disabled = true;
      } else {
        imStatus.textContent = 'ImageMagick detected \u2713';
        imStatus.className = 'status-badge status-ok';
      }
    } catch (e) {
      imStatus.textContent = 'Could not check ImageMagick: ' + e;
      imStatus.className = 'status-badge status-error';
    }
  }

  // --- Quality slider ---
  quality.addEventListener('input', () => { qualityValue.textContent = quality.value; });

  // --- Hide quality controls for lossless formats ---
  format.addEventListener('change', () => {
    const show = format.value === 'jpg' || format.value === 'webp';
    qualityLabel.style.display = show ? '' : 'none';
    quality.style.display = show ? '' : 'none';
  });

  // --- File and folder picker buttons ---
  document.getElementById('btnPickFiles').addEventListener('click', async () => {
    const paths = await window.go.main.App.OpenFileDialog();
    if (paths && paths.length) addFilesToBundle(paths);
  });

  document.getElementById('btnPickFolder').addEventListener('click', async () => {
    const dir = await window.go.main.App.OpenFolderDialog();
    if (dir) addFilesToBundle([dir]);
  });

  document.getElementById('btnPickOutput').addEventListener('click', async () => {
    const dir = await window.go.main.App.OpenOutputFolderDialog();
    if (dir) {
      outputPath = dir;
      document.getElementById('outputPathDisplay').textContent = dir;
    }
  });

  // --- Drag-and-drop zone ---
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
      .map(f => f.path)
      .filter(p => /\.(heic|heif)$/i.test(p));
    if (paths.length) addFilesToBundle(paths);
  });

  // --- Convert button ---
  convertBtn.addEventListener('click', async () => {
    if (!bundle.length) { status.textContent = 'Add files first.'; return; }
    if (!outputPath)    { status.textContent = 'Select an output folder first.'; return; }
    if (!window.go?.main?.App?.ConvertFiles) {
      status.textContent = 'Wails bindings unavailable. Run with "wails dev" or "wails build".';
      return;
    }
    convertBtn.disabled = true;
    status.textContent = 'Converting...';
    try {
      const paths  = bundle.map(b => b.path);
      const result = await window.go.main.App.ConvertFiles(
        paths, outputPath, format.value, Number(quality.value)
      );
      const n = result.converted?.length ?? 0;
      status.textContent = `Done. Converted ${n} file(s).`;
    } catch (err) {
      status.textContent = `Conversion failed: ${err}`;
    } finally {
      convertBtn.disabled = false;
    }
  });
});

// addFilesToBundle fetches metadata for paths and merges new entries into the bundle.
async function addFilesToBundle(paths) {
  const metas = await window.go.main.App.GetFileMeta(paths);
  metas.forEach(m => {
    if (!bundle.find(b => b.path === m.path)) bundle.push(m);
  });
  renderTable();
}

// renderTable rebuilds the file bundle table from the current bundle state.
function renderTable() {
  const tbody = document.getElementById('fileTableBody');
  if (!bundle.length) {
    tbody.innerHTML = '<tr id="emptyRow"><td colspan="7" class="empty-msg">No files selected.</td></tr>';
    return;
  }
  tbody.innerHTML = '';
  bundle.forEach((item, idx) => {
    const tr = document.createElement('tr');
    const thumb = item.thumbBase64
      ? `<img src="${item.thumbBase64}" width="12" height="12" class="thumb-img">`
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
  });
  document.querySelectorAll('.btn-remove').forEach(btn => {
    btn.addEventListener('click', () => {
      bundle.splice(Number(btn.dataset.idx), 1);
      renderTable();
    });
  });
}
