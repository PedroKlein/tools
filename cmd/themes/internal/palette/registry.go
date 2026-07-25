package palette

// registry.go — one central place to build the EmittersV4 slice.
// Populated additively as waves land. Each wave's init() appends its
// emitters; the final order matches the derive pipeline in emit.go
// for v3-parity logging.

func init() {
	// Wave 1: instant hot-reload apps.
	EmittersV4 = append(EmittersV4,
		ghosttyEmitter{},
		tmuxEmitter{},
		sketchybarEmitter{},
		aerospaceEmitter{},
		starshipEmitter{},
		// Wave 2: editor + shell.
		nvimEmitter{},
		piEmitter{},
		fzfEmitter{},
		zshHighlightEmitter{},
		// Wave 3: git tools + pagers.
		deltaEmitter{},
		batEmitter{},
		lazygitEmitter{},
		ghDashEmitter{},
		// Wave 4: TUI theme apps.
		k9sEmitter{},
		televisionEmitter{},
		btopEmitter{},
		opencodeEmitter{},
		// Wave 5: macOS system (wallpaper is a resolver, not a file emit).
		macosEmitter{},
	)
}
