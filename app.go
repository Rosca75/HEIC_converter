package main

import (
    "context"
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

func (a *App) ConvertHEICtoJPG(inputPath, outputPath string, quality int) (string, error) {
    result, err := converter.ConvertHEICtoJPG(inputPath, outputPath, quality)
    if err != nil {
        return "", err
    }
    return result, nil
}