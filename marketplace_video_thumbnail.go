package main

import (
	"context"
	"encoding/base64"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Keep the native thumbnail adapter stable in obfuscated builds as well.
func init() {
	application.RegisterBindingMethodID((*ProfileService).GenerateMarketplaceVideoThumbnail, 1437725365)
}

func (m *ProfileService) GenerateMarketplaceVideoThumbnail(ctx context.Context, profileID, encoded string) (string, error) {
	if _, err := m.session(profileID); err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	thumbnail, err := videoThumbnail(ctx, data)
	if err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(thumbnail), nil
}
