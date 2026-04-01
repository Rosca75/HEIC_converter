'use strict';

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
        imStatus.textContent = 'ImageMagick not found \u2014 ' + err;
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

  // --- Hide quality for lossless formats ---
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

  // --- Drop zone — visual feedback via HTML5 drag events ---
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

  // --- Wails file drop — provides real OS paths on all platforms ---
  if (window.runtime?.OnFileDrop) {
    window.runtime.OnFileDrop((x, y, paths) => {
      dropZone.classList.remove('drag-over');
      const heic = paths.filter(p => /\.(heic|heif)$/i.test(p));
      if (heic.length) addFilesToBundle(heic);
    }, false);
  }

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
      const result = await window.go.main.App.ConvertFiles(
        bundle.map(b => b.path), outputPath, format.value, Number(quality.value)
      );
      status.textContent = `Done. Converted ${result.converted?.length ?? 0} file(s).`;
    } catch (err) {
      status.textContent = `Conversion failed: ${err}`;
    } finally {
      convertBtn.disabled = false;
    }
  });
});
