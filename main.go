package main

import (
    "github.com/wailsapp/wails/v2"
    "github.com/wailsapp/wails/v2/pkg/options"
)

func main() {
    app := NewApp()
    if err := wails.Run(&options.App{
        Title:  "HEIC Converter",
        Width:  400,
        Height: 300,
        Bind: []interface{}{
            app,
        },
    }); err != nil {
        panic(err)
    }
}