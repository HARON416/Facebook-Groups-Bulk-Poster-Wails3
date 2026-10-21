package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Retain auxiliary webviews so an in-flight binding response still has a valid
// window ID. Closing the UI hides it; reopening resumes its polling session.
func retainAuxiliaryWindow(app *application.App, window *application.WebviewWindow) {
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if app.Context().Err() != nil {
			return // Real application shutdown must destroy retained windows.
		}
		event.Cancel()
		window.ExecJS(`window.dispatchEvent(new Event('postika:window-hidden'))`)
		window.Hide()
	})
}

func showAuxiliaryWindow(window application.Window) {
	window.Show()
	window.ExecJS(`window.dispatchEvent(new Event('postika:window-shown'))`)
	window.Focus()
}
