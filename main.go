package main

import (
	"context"
	"embed"
	"net/http"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"nioh2mod-js/internal/transformation"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed wails.json
var wailsJsonContent []byte

func main() {
	app := NewApp()
	ref := transformation.NewRef()

	err := wails.Run(&options.App{
		Title:  "nioh2modManager",
		Width:  1324,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: http.HandlerFunc(app.fileHandler),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			ref.SetContext(ctx)
		},
		Bind: []interface{}{
			app,
			ref,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
