package palette

import (
	"fmt"
	"io"
	"strings"
)

type fuzzelEmitter struct{}

func (fuzzelEmitter) App() string      { return "fuzzel" }
func (fuzzelEmitter) Filename() string { return "fuzzel.ini" }
func (e fuzzelEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", func(t *Theme, w io.Writer) error {
		s := t.Palette.Semantic

		if _, err := io.WriteString(w, "[colors]\n"); err != nil {
			return fmt.Errorf("emit launcher section: %w", err)
		}

		for _, value := range []struct{ key, color, alpha string }{
			{"background", s.Bg, "ee"},
			{"text", s.Fg, "ff"},
			{"match", s.Accent, "ff"},
			{"selection", s.BgAlt, "ff"},
			{"selection-text", s.Fg, "ff"},
			{"selection-match", s.Accent, "ff"},
			{"border", s.Accent, "ff"},
		} {
			if _, err := fmt.Fprintf(w, "%s=%s%s\n", value.key, strings.TrimPrefix(value.color, "#"), value.alpha); err != nil {
				return fmt.Errorf("emit launcher color: %w", err)
			}
		}

		return nil
	}, NoHints)
}

type swayosdEmitter struct{}

func (swayosdEmitter) App() string      { return "swayosd" }
func (swayosdEmitter) Filename() string { return "swayosd.css" }
func (e swayosdEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "", func(t *Theme, w io.Writer) error {
		s := t.Palette.Semantic

		_, err := fmt.Fprintf(w, "window#osd { background: %s; border-color: %s; }\nwindow#osd image, window#osd label { color: %s; }\nwindow#osd trough, window#osd segment { background: %s; }\nwindow#osd progress, window#osd segment.active { background: %s; }\n", s.Bg, s.Accent, s.Fg, s.BgAlt, s.Accent)
		if err != nil {
			return fmt.Errorf("emit audio feedback colors: %w", err)
		}

		return nil
	}, NoHints)
}

type makoEmitter struct{}

func (makoEmitter) App() string      { return "mako" }
func (makoEmitter) Filename() string { return "mako.conf" }
func (e makoEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", func(t *Theme, w io.Writer) error {
		s := t.Palette.Semantic

		_, err := fmt.Fprintf(w, "background-color=%see\ntext-color=%s\nborder-color=%s\nprogress-color=over %s\n[urgency=critical]\nborder-color=%s\n", s.Bg, s.Fg, s.Accent, s.Accent, s.Error)
		if err != nil {
			return fmt.Errorf("emit notification colors: %w", err)
		}

		return nil
	}, NoHints)
}
