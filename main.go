package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	profiles, err := newProfileService("")
	if err != nil {
		log.Fatal(err)
	}
	app := application.New(application.Options{
		Name:        "Postika",
		Description: "Postika desktop application",
		Icon:        appIcon,
		Linux: application.LinuxOptions{
			// Match the launcher basename so GNOME can associate every window
			// with Postika's installed icon on Wayland and X11.
			ApplicationID: "app.postika",
			ProgramName:   "app.postika",
		},
		Services: []application.Service{
			application.NewService(profiles),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})
	profiles.app = app
	profiles.monitor = true

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "postika-main",
		Title:            "Postika",
		Width:            1200,
		Height:           800,
		MinWidth:         800,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(248, 249, 250),
		URL:              "/",
		EnableFileDrop:   true,
		Frameless:        true,
	})
	registerImageFileDrop(app, window)
	// Auxiliary windows are retained while hidden. They must not keep the
	// application alive after the user closes its main window.
	window.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		app.Quit()
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
