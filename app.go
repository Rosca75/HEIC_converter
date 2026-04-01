package main

// JS↔Go API surface:
// window.go.main.App.CheckImageMagick()          → string
// window.go.main.App.OpenFileDialog()            → []string
// window.go.main.App.OpenFolderDialog()          → string
// window.go.main.App.OpenOutputFolderDialog()    → string
// window.go.main.App.GetFileMeta(paths)          → []FileMeta
// window.go.main.App.ConvertFiles(paths,…)       → ConversionSummary
// window.go.main.App.Convert(inputPath,…)        → ConversionSummary (legacy)

import (
	"context"

	"heic-converter/converter"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
}

// NewApp creates a new App instance.
func NewApp() *App {
	return &App{}
}

// startup stores the Wails context needed for dialog calls.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// CheckImageMagick verifies ImageMagick is available on PATH.
func (a *App) CheckImageMagick() string {
	if err := converter.CheckImageMagick(); err != nil {
		return err.Error()
	}
	return ""
}

// OpenFileDialog opens a native multi-file picker filtered to HEIC/HEIF.
func (a *App) OpenFileDialog() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select HEIC / HEIF files",
		Filters: []runtime.FileFilter{
			{DisplayName: "HEIC / HEIF images", Pattern: "*.heic;*.heif;*.HEIC;*.HEIF"},
		},
	})
}

// OpenFolderDialog opens a native folder picker for input HEIC files.
func (a *App) OpenFolderDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select folder containing HEIC files",
	})
}

// OpenOutputFolderDialog opens a native folder picker for the output directory.
func (a *App) OpenOutputFolderDialog() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select output folder",
	})
}

// GetFileMeta returns metadata and thumbnails for the given file or folder paths.
// Emits "meta:progress" events as each file is processed so the UI can show a progress bar.
func (a *App) GetFileMeta(paths []string) ([]converter.FileMeta, error) {
	return converter.GetFileMeta(paths, func(done, total int) {
		runtime.EventsEmit(a.ctx, "meta:progress", map[string]interface{}{
			"done": done, "total": total,
		})
	})
}

// ConvertFiles converts a specific list of HEIC file paths to the target format.
func (a *App) ConvertFiles(paths []string, outputDir, format string, quality int) (converter.ConversionSummary, error) {
	return converter.ConvertFiles(paths, outputDir, format, quality)
}

// Convert is the legacy single-path/folder conversion method.
func (a *App) Convert(inputPath, outputDir, format string, quality int) (converter.ConversionSummary, error) {
	return converter.ConvertPath(inputPath, outputDir, format, quality)
}
