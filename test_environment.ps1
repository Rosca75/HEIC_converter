# HEIC_converter - Environment Test Script
# Verifies the prerequisites for building and running the application.
# The app itself has NO runtime dependencies on libheif or ImageMagick -
# a WASM HEIC decoder is bundled. You only need Go + Wails to build.

Write-Host "Checking prerequisites for HEIC_converter..." -ForegroundColor Cyan

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: Go is not installed. Download: https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    Write-Host "ERROR: Wails is not installed. Install with:" -ForegroundColor Red
    Write-Host "  go install github.com/wailsapp/wails/v2/cmd/wails@latest" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path "go.mod")) {
    Write-Host "ERROR: go.mod missing. Run this from the project root." -ForegroundColor Red
    exit 1
}

Write-Host "OK: All prerequisites satisfied." -ForegroundColor Green
Write-Host "Run 'wails dev' to start the app in dev mode," -ForegroundColor Green
Write-Host "or 'wails build' to produce a release binary in build/bin/." -ForegroundColor Green
