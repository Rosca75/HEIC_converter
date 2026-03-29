# HEIC Converter

Convertissez vos fichiers HEIC en JPG en conservant les métadonnées EXIF.

## Fonctionnalités
- Conversion de HEIC vers JPG
- Conservation des métadonnées EXIF (date, lieu, etc.)
- Interface graphique moderne (Wails + Go)
- Multiplateforme (Windows, macOS, Linux)

## 🛠 Prérequis
- **Go** : 1.21+
- **Wails** : `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **Libheif** :
  - **Windows** : `choco install libheif` (ou télécharger les binaires [ici](https://github.com/strukturag/libheif/releases)).
  - **Linux** : `sudo apt install libheif-dev` ou `brew install libheif` (macOS).

## 🚨 Dépannage
- **Erreur `unknown revision`** : Exécutez `go clean -modcache` puis `go mod tidy`.
- **Libheif introuvable** : Installez les binaires ou utilisez Chocolatey (`choco install libheif`).
- **Wails ne se lance pas** : Vérifiez que `wails` est installé (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

## 📥 Installation et exécution
1. **Cloner le dépôt** :
   ```powershell
   git clone https://github.com/Rosca75/HEIC_converter.git
   cd HEIC_converter
   ```
2. **Tester l'environnement** :
   ```powershell
   .\test_environment.ps1
   ```
3. **Nettoyer et mettre à jour** :
   ```powershell
   go clean -modcache
   rm -r -fo vendor
   rm go.sum
   go mod tidy
   ```
4. **Lancer l'application** :
   ```powershell
   wails dev
   ```
5. **Build pour la production** (optionnel) :
   ```powershell
   wails build
   ```

## Utilisation
1. Cliquez sur **"Parcourir..."** pour sélectionner un fichier ou dossier HEIC.
2. Choisissez le **dossier de sortie**.
3. Ajustez la **qualité** (85 par défaut).
4. Cliquez sur **"Convertir"** pour lancer la conversion.

## Licence
MIT