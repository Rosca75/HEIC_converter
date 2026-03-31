window.addEventListener('DOMContentLoaded', async () => {
  const inputPath = document.getElementById('inputPath');
  const outputDir = document.getElementById('outputDir');
  const format = document.getElementById('format');
  const quality = document.getElementById('quality');
  const qualityValue = document.getElementById('qualityValue');
  const convertBtn = document.getElementById('convertBtn');
  const status = document.getElementById('status');
  const imageMagickStatus = document.getElementById('imageMagickStatus');

  quality.addEventListener('input', () => {
    qualityValue.textContent = quality.value;
  });

  // Show/hide quality slider based on format (only meaningful for jpg/webp)
  format.addEventListener('change', () => {
    const showQuality = format.value === 'jpg' || format.value === 'webp';
    quality.parentElement && (quality.closest('label') || quality).style;
    document.querySelector('label[for="quality"]').style.display = showQuality ? '' : 'none';
    quality.style.display = showQuality ? '' : 'none';
  });

  // Check ImageMagick availability at startup
  if (window.go?.main?.App?.CheckImageMagick) {
    try {
      const errMsg = await window.go.main.App.CheckImageMagick();
      if (errMsg) {
        imageMagickStatus.textContent = 'ImageMagick not found — ' + errMsg;
        imageMagickStatus.className = 'status-badge status-error';
        convertBtn.disabled = true;
      } else {
        imageMagickStatus.textContent = 'ImageMagick detected ✓';
        imageMagickStatus.className = 'status-badge status-ok';
      }
    } catch (e) {
      imageMagickStatus.textContent = 'Could not check ImageMagick: ' + e;
      imageMagickStatus.className = 'status-badge status-error';
    }
  }

  convertBtn.addEventListener('click', async () => {
    const source = inputPath.value.trim();
    const destination = outputDir.value.trim();
    const fmt = format.value;
    const q = Number.parseInt(quality.value, 10);

    if (!source || !destination) {
      status.textContent = 'Please provide both an input path and an output directory.';
      return;
    }

    if (!window.go?.main?.App?.Convert) {
      status.textContent =
        'Wails bindings are unavailable. Run this app with "wails dev" or "wails build".';
      return;
    }

    convertBtn.disabled = true;
    status.textContent = 'Converting...';

    try {
      const result = await window.go.main.App.Convert(source, destination, fmt, q);
      const converted = result.converted?.length ?? 0;
      const skipped = result.skipped?.length ?? 0;
      status.textContent = `Done. Converted ${converted} file(s). Skipped ${skipped} non-HEIC file(s).`;
    } catch (error) {
      status.textContent = `Conversion failed: ${error}`;
    } finally {
      convertBtn.disabled = false;
    }
  });
});
