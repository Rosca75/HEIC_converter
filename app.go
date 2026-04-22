package main

// JS↔Go API surface:
// window.go.main.App.OpenFileDialog()                    → []string
// window.go.main.App.OpenFolderDialog()                  → string
// window.go.main.App.OpenOutputFolderDialog()            → string
// window.go.main.App.GetFileMeta(paths, recursive)       → []FileMeta
// window.go.main.App.GetFileMetaStreaming(paths, r)      → error (emits meta:start/meta:file/meta:done)
// window.go.main.App.ConvertFiles(paths, out, fmt, q)    → ConversionSummary
//                                                          (emits convert:start/convert:file/convert:done)

import (
	"context"
	"sync"

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

// GetFileMeta returns metadata and thumbnails for the given files or
// directories. If recursive is true, directories are walked into subfolders.
func (a *App) GetFileMeta(paths []string, recursive bool) ([]converter.FileMeta, error) {
	return converter.GetFileMeta(paths, recursive, func(done, total int) {
		runtime.EventsEmit(a.ctx, "meta:progress", map[string]interface{}{
			"done": done, "total": total,
		})
	})
}

// ConvertFiles converts the given HEIC files to the target format in
// parallel. Emits convert:start with the total count, one convert:file
// event per file with {path, ok, done, total}, then convert:done with the
// summary.
func (a *App) ConvertFiles(paths []string, outputDir, format string, quality int) (converter.ConversionSummary, error) {
	runtime.EventsEmit(a.ctx, "convert:start", len(paths))
	summary, err := converter.ConvertFiles(paths, outputDir, format, quality,
		func(done, total int, currentPath string, ok bool) {
			runtime.EventsEmit(a.ctx, "convert:file", map[string]interface{}{
				"path":  currentPath,
				"ok":    ok,
				"done":  done,
				"total": total,
			})
		})
	runtime.EventsEmit(a.ctx, "convert:done", summary)
	return summary, err
}

// Convert is the legacy single-path/folder conversion method, kept so the
// JS binding surface does not break for any in-flight callers.
func (a *App) Convert(inputPath, outputDir, format string, quality int) (converter.ConversionSummary, error) {
	return converter.ConvertPath(inputPath, outputDir, format, quality)
}

// GetFileMetaStreaming emits meta:start with the total count, one meta:file
// event per file as soon as it is ready, then meta:done. If recursive is
// true, directories are walked into subfolders.
func (a *App) GetFileMetaStreaming(paths []string, recursive bool) error {
	expanded, err := converter.ExpandPaths(paths, recursive)
	if err != nil {
		return err
	}
	runtime.EventsEmit(a.ctx, "meta:start", len(expanded))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for _, p := range expanded {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			m, _ := converter.GetOneFileMeta(p)
			runtime.EventsEmit(a.ctx, "meta:file", m)
		}(p)
	}
	wg.Wait()
	runtime.EventsEmit(a.ctx, "meta:done")
	return nil
}
