package main

import (
	"context"

	"heic-converter/converter"
)

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Convert(inputPath, outputDir, format string, quality int) (converter.ConversionSummary, error) {
	return converter.ConvertPath(inputPath, outputDir, format, quality)
}

func (a *App) CheckImageMagick() string {
	if err := converter.CheckImageMagick(); err != nil {
		return err.Error()
	}
	return ""
}
