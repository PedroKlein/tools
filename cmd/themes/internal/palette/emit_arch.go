package palette

import (
	"fmt"
	"io"
	"strings"
)

type hyprlandEmitter struct{}

func (hyprlandEmitter) App() string      { return "hyprland" }
func (hyprlandEmitter) Filename() string { return "hyprland.lua" }
func (e hyprlandEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "--", func(t *Theme, w io.Writer) error {
		s := t.Palette.Semantic

		_, err := fmt.Fprintf(w, "hl.config({\n    general = {\n        col = {\n            active_border = %q,\n            inactive_border = %q,\n        },\n    },\n})\n", hyprlandColor(s.Accent), hyprlandColor(s.Border))
		if err != nil {
			return fmt.Errorf("emit hyprland: %w", err)
		}

		return nil
	}, NoHints)
}

func hyprlandColor(color string) string {
	hex := strings.TrimPrefix(color, "#")
	switch len(hex) {
	case 3:
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	case 8:
		return "rgba(" + hex + ")"
	}

	return "rgb(" + hex + ")"
}

type waybarEmitter struct{}

func (waybarEmitter) App() string      { return "waybar" }
func (waybarEmitter) Filename() string { return "waybar.css" }
func (e waybarEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "", func(t *Theme, w io.Writer) error {
		s := t.Palette.Semantic

		_, err := fmt.Fprintf(w, "@define-color bg %s;\n@define-color fg %s;\n@define-color muted %s;\n@define-color accent %s;\n@define-color error %s;\n@define-color warning %s;\n", s.Bg, s.Fg, s.Muted, s.Accent, s.Error, s.Warning)
		if err != nil {
			return fmt.Errorf("emit waybar: %w", err)
		}

		return nil
	}, NoHints)
}
