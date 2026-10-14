package reload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PedroKlein/tools/internal/loginsnapshot"
	"golang.org/x/sys/unix"
)

func syncScreens(ctx context.Context, data []byte, wallpaper, revision string) error {
	_, lookupErr := exec.LookPath("sddm-theme-sync")
	if errors.Is(lookupErr, exec.ErrNotFound) {
		return nil
	}

	if lookupErr != nil {
		return fmt.Errorf("locate login publisher: %w", lookupErr)
	}

	var payload loginsnapshot.Request

	if parseErr := json.Unmarshal(data, &payload); parseErr != nil {
		return fmt.Errorf("read login sync palette: %w", parseErr)
	}

	payload.Revision = revision

	if wallpaper != "" {
		imageData, err := readSyncImage(wallpaper)
		if err != nil {
			return err
		}

		payload.Image = imageData
	}

	request, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode login sync request: %w", err)
	}

	if len(request) > loginsnapshot.MaxRequestBytes {
		return errors.New("login sync request exceeds limit")
	}

	cmd := exec.CommandContext(ctx, "sudo", "-n", "--", "/usr/local/libexec/sddm-theme-sync")
	cmd.Stdin = bytes.NewReader(request)
	cmd.Stdout = io.Discard

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sync login theme: %w: %s", err, stderr.String())
	}

	return nil
}

func readSyncImage(path string) ([]byte, error) {
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("resolve login wallpaper: %w", err)
	}

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open wallpaper directory: %w", err)
	}

	defer func() { _ = root.Close() }()

	file, err := root.OpenFile(filepath.Base(path), os.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open login wallpaper: %w", err)
	}

	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("login wallpaper must be a regular file")
	}

	data, err := io.ReadAll(io.LimitReader(file, 16*1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("read login wallpaper: %w", err)
	}

	if len(data) > 16*1024*1024 {
		return nil, errors.New("login wallpaper exceeds limit")
	}

	return data, nil
}
