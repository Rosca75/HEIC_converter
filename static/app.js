window.addEventListener('DOMContentLoaded', () => {
  const inputPath = document.getElementById('inputPath');
  const outputDir = document.getElementById('outputDir');
  const quality = document.getElementById('quality');
  const qualityValue = document.getElementById('qualityValue');
  const convertBtn = document.getElementById('convertBtn');
  const status = document.getElementById('status');

  quality.addEventListener('input', () => {
    qualityValue.textContent = quality.value;
  });

  convertBtn.addEventListener('click', async () => {
    const source = inputPath.value.trim();
    const destination = outputDir.value.trim();
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
      const result = await window.go.main.App.Convert(source, destination, q);
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
