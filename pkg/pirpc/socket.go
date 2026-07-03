package pirpc

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// SocketPath returns the default q daemon socket path for the current user.
func SocketPath() string {
	dir := socketDir()
	return filepath.Join(dir, "q.sock")
}

// PIDPath returns the default PID file path.
func PIDPath() string {
	dir := socketDir()
	return filepath.Join(dir, "q.pid")
}

func socketDir() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir
	}

	if runtime.GOOS == "darwin" {
		return filepath.Join(os.TempDir(), fmt.Sprintf("q-%d", os.Getuid()))
	}

	return fmt.Sprintf("/tmp/q-%d", os.Getuid())
}
