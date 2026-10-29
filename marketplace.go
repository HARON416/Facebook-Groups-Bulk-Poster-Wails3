package main

import (
	"errors"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var marketplaceWindowMu sync.Mutex

// OpenMarketplace reuses one blank workspace window across accounts.
func (m *ProfileService) OpenMarketplace() error {
	return m.openMarketplaceWindow("postika-marketplace", "Marketplace", "marketplace")
}

func (m *ProfileService) OpenCreateListing() error {
	return m.openMarketplaceWindow("postika-create-listing", "Create new listing", "create-listing")
}

func (m *ProfileService) openMarketplaceWindow(name, title, view string) error {
	marketplaceWindowMu.Lock()
	defer marketplaceWindowMu.Unlock()
	if m.app == nil {
		return errors.New("The desktop application is not ready")
	}
	if window, ok := m.app.Window.GetByName(name); ok {
		if window.IsMinimised() {
			window.UnMinimise()
		}
		showAuxiliaryWindow(window)
		return nil
	}
	window := m.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name: name, Title: title + " · " + appDisplayName,
		Frameless: true, Width: 1200, Height: 800, MinWidth: 800, MinHeight: 600,
		URL: "/?view=" + view, BackgroundColour: application.NewRGB(248, 249, 250),
	})
	retainAuxiliaryWindow(m.app, window)
	return nil
}
