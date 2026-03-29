# HEIC_converter - Environment Test Script
# Vérifie que tous les prérequis sont installés avant de lancer l'application.

Write-Host "Vérification des prérequis pour HEIC_converter..." -ForegroundColor Cyan

# Vérification de Go
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Go n'est pas installé ! Téléchargez-le depuis : https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

# Vérification de Wails
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Wails n'est pas installé ! Installez-le avec : go install github.com/wailsapp/wails/v2/cmd/wails@latest" -ForegroundColor Red
    exit 1
}

# Vérification de go.mod
if (-not (Test-Path "go.mod")) {
    Write-Host "❌ go.mod manquant ! Vérifiez que vous êtes dans le bon dossier." -ForegroundColor Red
    exit 1
}

# Vérification de la présence des binaires libheif (Windows)
if ($IsWindows) {
    $libheifPath = (Get-Command libheif -ErrorAction SilentlyContinue)
    if (-not $libheifPath) {
        Write-Host "ℹ️ Libheif n'est pas dans le PATH. Si l'application échoue, installez-le via : choco install libheif" -ForegroundColor Yellow
    }
}

Write-Host "✅ Tous les prérequis sont satisfaits !" -ForegroundColor Green
Write-Host "Vous pouvez maintenant lancer : wails dev" -ForegroundColor Green