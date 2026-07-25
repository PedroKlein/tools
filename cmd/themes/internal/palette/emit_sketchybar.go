package palette

import (
	"fmt"
	"io"
	"strings"
)

// v4 sketchybar emitter — reads theme.json, writes sketchybar.sh.
//
// The sketchybarrc + plugin scripts expect env vars in the sketchybar
// `0x<AA><RRGGBB>` color format (AA = alpha hex). Bar bg is
// semi-transparent (0xd0) by default; overridable via hints.sketchybar.

type sketchybarEmitter struct{}

func (sketchybarEmitter) App() string      { return "sketchybar" }
func (sketchybarEmitter) Filename() string { return "sketchybar.sh" }

func (e sketchybarEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitSketchybarSemantic, emitSketchybarHints)
}

// sbColor formats a hex + alpha as sketchybar's `0xAARRGGBB`.
func sbColor(hex, alpha string) string {
	return "0x" + alpha + strings.ToLower(strings.TrimPrefix(hex, "#"))
}
func sbFull(hex string) string { return sbColor(hex, "ff") }

func emitSketchybarSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic

	// Bar surface with slight transparency.
	fmt.Fprintf(w, "export BAR_BG=%s\n", sbColor(s.Bg, "d0"))
	fmt.Fprintf(w, "export BAR_BORDER=%s\n", sbFull(s.Border))

	// Foreground family.
	fmt.Fprintf(w, "export FG=%s\n", sbFull(s.Fg))
	fmt.Fprintf(w, "export FG_MUTED=%s\n", sbFull(s.Muted))
	fmt.Fprintf(w, "export FG_BRIGHT=%s\n", sbFull(t.Palette.Ansi[15]))

	// Accents.
	fmt.Fprintf(w, "export ACCENT=%s\n", sbFull(s.Accent))
	// ACCENT_BRIGHT is a same-hue brighter shade of ACCENT (matches v3
	// osaka-jade UX where the workspace border used a lighter green,
	// not accent2 which can be a different hue entirely). Default to
	// s.Ok which is semantically the 'bright positive' color; themes
	// with a red accent should override via hints.sketchybar.accentBright.
	accentBright := s.Ok
	if h := t.Hint("sketchybar"); h != nil {
		if v, ok := h["accentBright"].(string); ok && v != "" {
			accentBright = v
		}
	}
	fmt.Fprintf(w, "export ACCENT_BRIGHT=%s\n", sbFull(accentBright))
	fmt.Fprintf(w, "export ACCENT_ON=%s\n", sbFull(YIQContrast(s.Accent)))
	fmt.Fprintf(w, "export HIGHLIGHT=%s\n", sbFull(s.Accent))

	// Semantic status.
	fmt.Fprintf(w, "export RED=%s\n", sbFull(s.Error))
	fmt.Fprintf(w, "export YELLOW=%s\n", sbFull(s.Warning))
	fmt.Fprintf(w, "export GREEN=%s\n", sbFull(s.Ok))
	fmt.Fprintf(w, "export CYAN=%s\n", sbFull(t.Palette.Ansi[6]))
	fmt.Fprintf(w, "export MAGENTA=%s\n", sbFull(t.Palette.Ansi[5]))
	fmt.Fprintf(w, "export INFO=%s\n", sbFull(s.Info))
	// V3 compat: sketchybarrc + plugins reference $TEAL, $JADE.
	// TEAL = the cool-water color (ANSI blue slot in most palettes).
	// JADE = a green accent alternative (Accent by default — for themes
	// where Accent isn't green, override via hints.sketchybar.jade).
	teal := t.Palette.Ansi[4]
	jade := s.Accent
	volumeC := t.Palette.Ansi[6]
	percentageC := s.Fg
	if h := t.Hint("sketchybar"); h != nil {
		if v, ok := h["teal"].(string); ok && v != "" {
			teal = v
		}
		if v, ok := h["jade"].(string); ok && v != "" {
			jade = v
		}
		if v, ok := h["volume"].(string); ok && v != "" {
			volumeC = v
		}
		if v, ok := h["percentage"].(string); ok && v != "" {
			percentageC = v
		}
	}
	fmt.Fprintf(w, "export TEAL=%s\n", sbFull(teal))
	fmt.Fprintf(w, "export JADE=%s\n", sbFull(jade))
	fmt.Fprintf(w, "export VOLUME=%s\n", sbFull(volumeC))
	fmt.Fprintf(w, "export PERCENTAGE=%s\n", sbFull(percentageC))

	// Surfaces.
	fmt.Fprintf(w, "export SURFACE=%s\n", sbFull(s.BgAlt))
	fmt.Fprintf(w, "export SURFACE_LIGHT=%s\n", sbColor(s.Border, "44"))

	// Status-plugin vars.
	fmt.Fprintf(w, "export ICON=%s\n", sbFull(s.Fg))
	fmt.Fprintf(w, "export CHARGING=%s\n", sbFull(s.Warning))
	fmt.Fprintf(w, "export FOCUSED=%s\n", sbFull(s.Accent2))
	// V3 compat: aerospace.sh + sketchybarrc use $FOCUSED_WORKSPACE
	// (no _COLOR suffix). Emit both to smooth the transition.
	fmt.Fprintf(w, "export FOCUSED_WORKSPACE=%s\n", sbFull(s.Accent))
	fmt.Fprintf(w, "export FOCUSED_WORKSPACE_COLOR=%s\n", sbFull(s.Accent))
	fmt.Fprintf(w, "export NON_EMPTY=%s\n", sbFull(s.Fg))
	fmt.Fprintf(w, "export BADGE=%s\n", sbFull(s.Warning))
	return nil
}

// emitSketchybarHints reads hints.sketchybar.{barHeight,cornerRadius,margin}
// and re-exports them (overriding the baseline defaults). Unknown hint
// keys are ignored.
func emitSketchybarHints(t *Theme, w io.Writer) error {
	h := t.Hint("sketchybar")
	if h == nil {
		return nil
	}
	if v, ok := numericHint(h, "barHeight"); ok {
		fmt.Fprintf(w, "export BAR_HEIGHT=%d\n", int(v))
	}
	if v, ok := numericHint(h, "cornerRadius"); ok {
		fmt.Fprintf(w, "export BAR_CORNER_RADIUS=%d\n", int(v))
	}
	if v, ok := numericHint(h, "margin"); ok {
		fmt.Fprintf(w, "export BAR_MARGIN=%d\n", int(v))
	}
	if v, ok := numericHint(h, "yOffset"); ok {
		fmt.Fprintf(w, "export BAR_Y_OFFSET=%d\n", int(v))
	}
	return nil
}

// numericHint reads a JSON number from a free-form hint map. JSON numbers
// unmarshal into any as float64, so we accept float and int shape both.
func numericHint(h map[string]any, key string) (float64, bool) {
	raw, ok := h[key]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}
