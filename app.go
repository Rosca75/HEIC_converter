package main

import (
	"context"

	"github.com/Rosca75/HEIC_converter/converter"
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

func (a *App) Convert(inputPath, outputDir string, quality int) (converter.ConversionSummary, error) {
	return converter.ConvertPath(inputPath, outputDir, quality)
}
