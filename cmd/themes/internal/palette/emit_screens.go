package palette

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	screenAccent  = "accent"
	screenError   = "error"
	screenMuted   = "muted"
	screenWarning = "warning"
)

type hyprlockEmitter struct{}

func (hyprlockEmitter) App() string      { return "hyprlock" }
func (hyprlockEmitter) Filename() string { return "hyprlock.conf" }
func (e hyprlockEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", func(t *Theme, w io.Writer) error {
		s := t.Palette.Semantic
		for _, value := range []struct{ name, color string }{
			{"bg", s.Bg},
			{"bg_alt", s.BgAlt},
			{"fg", s.Fg},
			{screenMuted, s.Muted},
			{screenAccent, s.Accent},
			{screenError, s.Error},
			{screenWarning, s.Warning},
			{"ok", s.Ok},
		} {
			if _, err := fmt.Fprintf(w, "$screen_%s = %s\n", value.name, hyprlandColor(value.color)); err != nil {
				return fmt.Errorf("emit hyprlock color: %w", err)
			}
		}

		if _, err := fmt.Fprintf(w, "$screen_font = %s\n", t.Typography.Family); err != nil {
			return fmt.Errorf("emit hyprlock font: %w", err)
		}

		return nil
	}, NoHints)
}

type sddmEmitter struct{}

func (sddmEmitter) App() string      { return "sddm" }
func (sddmEmitter) Filename() string { return "sddm.json" }
func (sddmEmitter) Emit(t *Theme, w io.Writer) error {
	if t == nil {
		return errors.New("emit sddm: nil theme")
	}

	// The privileged publisher accepts data, never raw theme overrides or QML.
	s := t.Palette.Semantic
	values := map[string]string{
		"background": s.Bg,
		"foreground": s.Fg,
		screenMuted:  s.Muted,
		screenAccent: s.Accent,
		screenError:  s.Error,
		"font":       t.Typography.Family,
	}

	if err := json.NewEncoder(w).Encode(values); err != nil {
		return fmt.Errorf("emit sddm palette: %w", err)
	}

	return nil
}
