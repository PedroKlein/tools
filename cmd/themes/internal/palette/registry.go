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
	)
}
