package reload

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// userHookTimeout caps every user hook. Matches the default hook timeout.
const userHookTimeout = 4 * time.Second

// RunUserHooks discovers and runs every executable .sh file under the
// user's .hooks/ directory in a final wave — after every in-repo hook
// has fired. Callers pass the theme dir; each hook receives it as
// argv[1].
//
// Discovery is stable (filename order) so parallel users see a
// deterministic run order. Non-executable files are surfaced as a
// warning on stderr and skipped. Missing .hooks/ dir is a silent
// no-op.
//
// This function replaces the pre-b7 KindExternal path. In-repo hooks
// are all Go now (b1..b6); .hooks/ is user-territory.
func RunUserHooks(ctx context.Context, themeDir string, stderr io.Writer) []Result {
	dir := hooksDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // no dir; no hooks
	}

	// Collect and sort .sh files.
	scripts := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}
		if e.Name() == "README.md" {
			continue // documentation, not a hook
		}
		scripts = append(scripts, e.Name())
	}
	sort.Strings(scripts)

	if stderr == nil {
		stderr = os.Stderr
	}

	results := make([]Result, 0, len(scripts))
	for _, name := range scripts {
		full := filepath.Join(dir, name)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		// Skip non-executable files with a warning — otherwise user
		// churn against a stashed .sh file surprises them silently.
		if info.Mode().Perm()&0o111 == 0 {
			fmt.Fprintf(stderr, "reload: user hook %s is not executable; skipping\n", name)
			continue
		}

		start := time.Now()
		hctx, cancel := context.WithTimeout(ctx, userHookTimeout)
		cmd := exec.CommandContext(hctx, full, themeDir)
		cmd.Stdout = io.Discard
		cmd.Stderr = stderr
		err = cmd.Run()
		cancel()

		results = append(results, Result{
			Name:     "user:" + name,
			Err:      err,
			Duration: time.Since(start),
		})
	}
	return results
}
