package palette

import (
	"fmt"
	"io"
)

// v4 k9s emitter — writes YAML fragment consumed by k9s at
// ~/.config/k9s/skins/<theme>.yaml. The k9s reload hook symlinks
// current.yaml to point at the active theme's k9s.yaml.

type k9sEmitter struct{}

func (k9sEmitter) App() string      { return "k9s" }
func (k9sEmitter) Filename() string { return "k9s.yaml" }

func (e k9sEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "#", emitK9sSemantic, NoHints)
}

func emitK9sSemantic(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	fmt.Fprintln(w, "k9s:")
	fmt.Fprintln(w, "  body:")
	fmt.Fprintf(w, "    fgColor: %q\n", s.Fg)
	fmt.Fprintf(w, "    bgColor: default\n")
	fmt.Fprintf(w, "    logoColor: %q\n", s.Accent)
	fmt.Fprintln(w, "  prompt:")
	fmt.Fprintf(w, "    fgColor: %q\n", s.Fg)
	fmt.Fprintf(w, "    bgColor: default\n")
	fmt.Fprintf(w, "    suggestColor: %q\n", s.Muted)
	fmt.Fprintln(w, "  info:")
	fmt.Fprintf(w, "    fgColor: %q\n", s.Muted)
	fmt.Fprintf(w, "    sectionColor: %q\n", s.Accent)
	fmt.Fprintln(w, "  frame:")
	fmt.Fprintln(w, "    border:")
	fmt.Fprintf(w, "      fgColor: %q\n", s.Border)
	fmt.Fprintf(w, "      focusColor: %q\n", s.Accent)
	fmt.Fprintln(w, "    menu:")
	fmt.Fprintf(w, "      fgColor: %q\n", s.Fg)
	fmt.Fprintf(w, "      keyColor: %q\n", s.Accent2)
	fmt.Fprintf(w, "      numKeyColor: %q\n", s.Warning)
	fmt.Fprintln(w, "    crumbs:")
	fmt.Fprintf(w, "      fgColor: %q\n", s.Bg)
	fmt.Fprintf(w, "      bgColor: %q\n", s.Accent)
	fmt.Fprintf(w, "      activeColor: %q\n", s.Warning)
	fmt.Fprintln(w, "    status:")
	fmt.Fprintf(w, "      newColor: %q\n", s.Ok)
	fmt.Fprintf(w, "      modifyColor: %q\n", s.Warning)
	fmt.Fprintf(w, "      addColor: %q\n", s.Git.Added)
	fmt.Fprintf(w, "      errorColor: %q\n", s.Error)
	fmt.Fprintf(w, "      highlightColor: %q\n", s.Accent2)
	fmt.Fprintf(w, "      killColor: %q\n", s.Muted)
	fmt.Fprintf(w, "      completedColor: %q\n", s.Muted)
	fmt.Fprintln(w, "    title:")
	fmt.Fprintf(w, "      fgColor: %q\n", s.Fg)
	fmt.Fprintf(w, "      bgColor: default\n")
	fmt.Fprintf(w, "      highlightColor: %q\n", s.Accent)
	fmt.Fprintf(w, "      counterColor: %q\n", s.Accent2)
	fmt.Fprintf(w, "      filterColor: %q\n", s.Info)
	fmt.Fprintln(w, "  views:")
	fmt.Fprintln(w, "    table:")
	fmt.Fprintf(w, "      fgColor: %q\n", s.Fg)
	fmt.Fprintf(w, "      bgColor: default\n")
	fmt.Fprintf(w, "      cursorFgColor: %q\n", s.Bg)
	fmt.Fprintf(w, "      cursorBgColor: %q\n", s.Accent)
	fmt.Fprintln(w, "      header:")
	fmt.Fprintf(w, "        fgColor: %q\n", s.Accent)
	fmt.Fprintf(w, "        bgColor: default\n")
	fmt.Fprintf(w, "        sorterColor: %q\n", s.Accent2)
	fmt.Fprintln(w, "    yaml:")
	fmt.Fprintf(w, "      keyColor: %q\n", s.Accent)
	fmt.Fprintf(w, "      colonColor: %q\n", s.Muted)
	fmt.Fprintf(w, "      valueColor: %q\n", s.Fg)
	fmt.Fprintln(w, "    logs:")
	fmt.Fprintln(w, "      fgColor: default")
	fmt.Fprintln(w, "      bgColor: default")
	return nil
}
