window.addEventListener('load', () => {
    const browseBtn = document.getElementById('browseBtn');
    const browseOutputBtn = document.getElementById('browseOutputBtn');
    const convertBtn = document.getElementById('convertBtn');
    const qualityValue = document.getElementById('qualityValue');
    const qualityInput = document.getElementById('quality');
    const statusDiv = document.getElementById('status');

    qualityInput.addEventListener('input', () => {
        qualityValue.textContent = qualityInput.value;
    });

    browseBtn.addEventListener('click', async () => {
        const { dialog } = window.Wails;
        const result = await dialog.ShowOpenDialog({
            properties: ['openFile', 'openDirectory'],
            filters: [{ name: 'Fichiers HEIC', extensions: ['heic', 'HEIC'] }]
        });
        if (!result.canceled && result.filePaths.length > 0) {
            document.getElementById('inputPath').value = result.filePaths[0];
        }
    });

    browseOutputBtn.addEventListener('click', async () => {
        const { dialog } = window.Wails;
        const result = await dialog.ShowOpenDialog({
            properties: ['openDirectory']
        });
        if (!result.canceled && result.filePaths.length > 0) {
            document.getElementById('outputDir').value = result.filePaths[0];
        }
    });

    convertBtn.addEventListener('click', async () => {
        const inputPath = document.getElementById('inputPath').value;
        const outputDir = document.getElementById('outputDir').value;
        const quality = parseInt(qualityInput.value);

        if (!inputPath || !outputDir) {
            statusDiv.textContent = 'Veuillez sélectionner un fichier/dossier d\'entrée et un dossier de sortie.';
            statusDiv.className = 'error';
            return;
        }

        statusDiv.textContent = 'Conversion en cours...';
        statusDiv.className = '';

        try {
            const result = await window.backend.ConvertHEICtoJPG(inputPath, `${outputDir}/output.jpg`, quality);
            statusDiv.textContent = result;
            statusDiv.className = 'success';
        } catch (error) {
            statusDiv.textContent = `Erreur: ${error.message}`;
            statusDiv.className = 'error';
        }
    });
});