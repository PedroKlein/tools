package main

import (
	"context"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
)

var previewSketchybarHook = func(themeRoot string) error {
	ctx, cancel := context.WithTimeout(context.Background(), sketchybarPreviewTimeout)
	defer cancel()

	return reload.PreviewSketchybar(ctx, themeRoot)
}

func PreviewSketchybar(theme string) error {
	if theme == "" {
		return nil
	}

	return previewSketchybarHook(themeDir(theme))
}
