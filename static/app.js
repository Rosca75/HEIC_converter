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
