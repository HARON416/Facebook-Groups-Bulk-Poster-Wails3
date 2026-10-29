package main

import (
	"embed"
	"log"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	if !developmentBuild {
		handlePlatformUpdateHelper()
		updater.HandleHelperMode()
	}
	if err := prepareDevelopmentStorage(); err != nil {
		log.Fatal(err)
	}
	profiles, err := newProfileService("")
	if err != nil {
		log.Fatal(err)
	}
	options := application.Options{
		Name:        appDisplayName,
		Description: "Postika desktop application",
		Icon:        appIcon,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: appInstanceID,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if window, ok := application.Get().Window.GetByName("postika-main"); ok {
					if window.IsMinimised() {
						window.UnMinimise()
					}
					window.Show().Focus()
				}
			},
		},
		Linux: application.LinuxOptions{
			// Production matches its installed launcher; dev has a distinct
			// identity so GNOME does not group it with the installed app.
			ApplicationID: appLinuxID,
			ProgramName:   appLinuxID,
		},
		Services: []application.Service{
			application.NewService(profiles),
			application.NewService(&ContactService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	}
	if developmentBuild {
		options.Windows.WebviewUserDataPath = filepath.Join(profiles.root, "webview")
	}
	app := application.New(options)
	profiles.app = app
	profiles.monitor = true
	updates, err := newUpdateService(app, profiles)
	if err != nil {
		log.Fatal(err)
	}
	app.RegisterService(application.NewService(updates))

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "postika-main",
		Title:            appDisplayName,
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
