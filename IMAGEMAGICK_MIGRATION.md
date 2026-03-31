# HEIC Converter – Migration vers ImageMagick

## Contexte

Le projet utilisait initialement `go-libheif` (bindings CGO) pour décoder les fichiers HEIC.
Cette approche a été abandonnée en raison d'incompatibilités CGO sur Windows (TDM-GCC, wailsbindings.exe invalide).

**Nouvelle approche : déléguer toute la conversion à ImageMagick via `exec.Command`.**

---

## Prérequis utilisateur

ImageMagick doit être installé sur la machine et accessible dans le PATH :
- Téléchargement : https://imagemagick.org/script/download.php#windows
- Cocher **"Add application directory to your system path"** à l'installation
- Vérifier : `magick --version` dans un terminal

Le projet **ne doit pas embarquer ImageMagick** — c'est une dépendance externe assumée.

---

## Changements à effectuer

### 1. Supprimer toutes les dépendances CGO

- Supprimer les imports et références à `go-libheif` ou tout autre binding C
- Supprimer les directives `#cgo` dans le code Go
- S'assurer que `CGO_ENABLED=0` est compatible (le projet doit compiler sans CGO)

### 2. Réécrire le moteur de conversion

Remplacer la logique de décodage HEIC par des appels à ImageMagick :

```go
import (
    "fmt"
    "os/exec"
    "path/filepath"
    "strings"
)

// ConvertHEIC convertit un fichier HEIC vers le format cible (jpeg, png, webp…)
func ConvertHEIC(inputPath string, outputDir string, format string) error {
    baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
    outputPath := filepath.Join(outputDir, baseName+"."+format)

    cmd := exec.Command("magick", "convert", inputPath, outputPath)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("ImageMagick error: %w\n%s", err, string(output))
    }
    return nil
}
```

Pour une conversion batch :

```go
cmd := exec.Command("magick", "mogrify", "-format", format, "-path", outputDir, "*.heic")
```

### 3. Vérification de la disponibilité d'ImageMagick au démarrage

Ajouter une vérification au lancement de l'app (dans `main.go` ou au démarrage Wails) :

```go
func checkImageMagick() error {
    cmd := exec.Command("magick", "--version")
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("ImageMagick n'est pas installé ou absent du PATH.\nTélécharger : https://imagemagick.org/script/download.php#windows")
    }
    return nil
}
```

Si ImageMagick est absent, afficher un message d'erreur clair dans l'UI Wails (pas un panic).

### 4. Mettre à jour go.mod

- Supprimer les dépendances liées à libheif / go-libheif
- Exécuter `go mod tidy`

### 5. Mettre à jour l'UI (frontend Wails)

- Ajouter un indicateur d'état au démarrage : "ImageMagick détecté ✓" ou message d'erreur avec lien de téléchargement
- Le reste de l'UI (sélection de fichiers, format de sortie, dossier de destination) reste inchangé

---

## Formats supportés via ImageMagick

ImageMagick avec support HEIC permet de convertir vers :
- `jpg` / `jpeg`
- `png`
- `webp`
- `tiff`

Exposer ces options dans l'UI via un select/dropdown.

---

## Ce qu'il ne faut PAS faire

- Ne pas tenter d'embarquer ImageMagick dans le binaire
- Ne pas réintroduire CGO sous quelque forme que ce soit
- Ne pas utiliser `go-libheif`, `govips`, ou tout autre wrapper C/C++

---

## Objectif final

Un binaire Wails v2 compilable avec `CGO_ENABLED=0`, qui :
1. Vérifie la présence d'ImageMagick au démarrage
2. Expose une UI simple de conversion HEIC → format choisi
3. Délègue 100% de la conversion à `magick convert` ou `magick mogrify`
