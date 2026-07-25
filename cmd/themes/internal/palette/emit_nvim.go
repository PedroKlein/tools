package palette

import (
	"fmt"
	"io"
)

// v4 nvim emitter — reads theme.json, writes nvim.lua.
//
// Pipeline (via EmitStandard):
//   baseline  — safe fallback: termguicolors + `dark` background
//   semantic  — pcall the colorscheme named in hints.nvim.colorscheme;
//               on failure fall back to hints.nvim.fallback; on double
//               failure warn but keep sourcing
//   hints     — expose hints.nvim.* via vim.g when applicable
//   overrides — verbatim overrides.nvim + overrides.nvim_path sidecar
//
// Overrides for nvim are `--` line-comment safe (Lua), so EmitStandard's
// `#`-prefixed markers would produce invalid Lua. We use `--` prefix.

type nvimEmitter struct{}

func (nvimEmitter) App() string      { return "nvim" }
func (nvimEmitter) Filename() string { return "nvim.lua" }

func (e nvimEmitter) Emit(t *Theme, w io.Writer) error {
	return EmitStandard(t, w, e.App(), "--", emitNvimSemantic, emitNvimHints)
}

func emitNvimSemantic(t *Theme, w io.Writer) error {
	// The user's LazyVim config sources this file. It must be a valid
	// Lua chunk. Colorscheme name and fallback come from hints.
	name, fallback := "default", "carbonfox"
	if h := t.Hint("nvim"); h != nil {
		if v, ok := h["colorscheme"].(string); ok && v != "" {
			name = v
		}
		if v, ok := h["fallback"].(string); ok && v != "" {
			fallback = v
		}
	}
	fmt.Fprintf(w, `local ok, _ = pcall(vim.cmd.colorscheme, %q)
if not ok then
  local ok2 = pcall(vim.cmd.colorscheme, %q)
  if not ok2 then
    vim.notify("theme: could not load %s or %s", vim.log.levels.WARN)
  end
end
`, name, fallback, name, fallback)

	// Expose semantic slots on vim.g so any user config can pull them.
	s := t.Palette.Semantic
	fmt.Fprintf(w, `
vim.g.theme_bg = %q
vim.g.theme_fg = %q
vim.g.theme_accent = %q
vim.g.theme_error = %q
vim.g.theme_warning = %q
vim.g.theme_ok = %q
`, s.Bg, s.Fg, s.Accent, s.Error, s.Warning, s.Ok)
	return nil
}

func emitNvimHints(t *Theme, w io.Writer) error {
	// Anything under hints.nvim.options.<name> becomes vim.g.<name>.
	h := t.Hint("nvim")
	if h == nil {
		return nil
	}
	opts, ok := h["options"].(map[string]any)
	if !ok {
		return nil
	}
	for k, v := range opts {
		switch val := v.(type) {
		case string:
			fmt.Fprintf(w, "vim.g.%s = %q\n", k, val)
		case bool:
			fmt.Fprintf(w, "vim.g.%s = %t\n", k, val)
		case float64:
			// JSON numbers unmarshal as float64
			fmt.Fprintf(w, "vim.g.%s = %g\n", k, val)
		}
	}
	return nil
}
