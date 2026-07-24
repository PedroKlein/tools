package reload

// goos is the compile-time GOOS. Set by Go's build system.
//
// We wrap runtime.GOOS in a package-local var so unit tests can override
// osMatches's behavior without introspecting the runtime package.
import "runtime"

var goos = runtime.GOOS
