# HEIC Converter

Convertissez vos fichiers HEIC en JPG en conservant les métadonnées EXIF.

## Fonctionnalités
- Conversion de HEIC vers JPG
- Conservation des métadonnées EXIF (date, lieu, etc.)
- Interface graphique moderne (Wails + Go)
- Multiplateforme (Windows, macOS, Linux)

## Prérequis
- Go 1.21+
- Wails v2
- Libheif (avec codecs libde265 et x265)

## Installation
```bash
wails init -n HEIC_converter -t https://github.com/Rosca75/HEIC_converter
cd HEIC_converter
go mod tidy
wails dev
```

## Utilisation
1. Lancez l'application avec `wails dev`
2. Sélectionnez un fichier ou un dossier contenant des HEIC
3. Choisissez la qualité de sortie (80-95)
4. Cliquez sur "Convertir"

## Licence
MIT