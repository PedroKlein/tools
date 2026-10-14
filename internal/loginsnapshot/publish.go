package loginsnapshot

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const MaxRequestBytes = 24 * 1024 * 1024

var (
	colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	fontPattern  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 ._-]{0,95}$`)
)

type Request struct {
	Revision   string `json:"revision"`
	Background string `json:"background"`
	Foreground string `json:"foreground"`
	Muted      string `json:"muted"`
	Accent     string `json:"accent"`
	Error      string `json:"error"`
	Font       string `json:"font"`
	Image      []byte `json:"image,omitempty"`
}

func Publish(root *os.Root, input io.Reader, decode func([]byte) (*image.NRGBA, error)) error {
	dir, err := root.Open(".")
	if err != nil {
		return fmt.Errorf("open snapshot directory: %w", err)
	}

	defer func() { _ = dir.Close() }()

	lock, err := root.OpenFile("publish.lock", os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0o600)
	if err != nil {
		return fmt.Errorf("open publication lock: %w", err)
	}

	defer func() { _ = lock.Close() }()

	info, err := lock.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("publication lock must be a regular file")
	}

	if lockErr := lockPublication(int(lock.Fd())); lockErr != nil {
		return lockErr
	}

	request, err := parse(input)
	if err != nil {
		return err
	}

	unchanged, staleErr := rejectStale(root, request)
	if staleErr != nil {
		return staleErr
	}

	if unchanged {
		return nil
	}

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return fmt.Errorf("inspect snapshot retention: %w", err)
	}

	count := 0

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "snapshot-") {
			count++
		}
	}

	if count >= 32 {
		return errors.New("login snapshot retention limit reached; root maintenance required")
	}

	return publishLocked(root, int(dir.Fd()), request, decode)
}

func lockPublication(fd int) error {
	deadline := time.Now().Add(2 * time.Second)

	for {
		err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return nil
		}

		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EINTR) {
			return fmt.Errorf("lock publication: %w", err)
		}

		if !time.Now().Before(deadline) {
			return errors.New("lock publication: publisher or maintenance is busy")
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func parse(input io.Reader) (*Request, error) {
	data, err := io.ReadAll(io.LimitReader(input, MaxRequestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read login request: %w", err)
	}

	if len(data) > MaxRequestBytes {
		return nil, errors.New("login request exceeds size limit")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var request Request
	if err = decoder.Decode(&request); err != nil {
		return nil, fmt.Errorf("decode login request: %w", err)
	}

	if err = decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, errors.New("login request must contain exactly one object")
	}

	revision, err := time.Parse(time.RFC3339Nano, request.Revision)
	if err != nil || revision.After(time.Now().Add(time.Minute)) || revision.IsZero() {
		return nil, errors.New("invalid login revision")
	}

	for _, color := range []string{request.Background, request.Foreground, request.Muted, request.Accent, request.Error} {
		if !colorPattern.MatchString(color) {
			return nil, errors.New("invalid login color")
		}
	}

	if !fontPattern.MatchString(request.Font) {
		return nil, errors.New("invalid login font family")
	}

	return &request, nil
}

func rejectStale(root *os.Root, request *Request) (bool, error) {
	file, err := root.Open("current.conf")
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("read current login config: %w", err)
	}

	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 4097))
	if err != nil || len(data) > 4096 {
		return false, errors.New("current login config is unreadable or oversized")
	}

	first, rest, _ := strings.Cut(string(data), "\n")

	previous, err := time.Parse(time.RFC3339Nano, strings.TrimPrefix(first, "# revision="))
	if err != nil {
		return false, fmt.Errorf("parse current login revision: %w", err)
	}

	next, _ := time.Parse(time.RFC3339Nano, request.Revision)
	if next.Before(previous) {
		return false, errors.New("login request superseded by a newer revision")
	}

	if next.Equal(previous) {
		if strings.HasPrefix(rest, "# digest="+requestDigest(request)+"\n") {
			return true, nil
		}

		return false, errors.New("conflicting login snapshot for the same revision")
	}

	return false, nil
}

func requestDigest(request *Request) string {
	data, _ := json.Marshal(request)
	digest := sha256.Sum256(data)

	return hex.EncodeToString(digest[:])
}

func publishLocked(root *os.Root, fd int, request *Request, decode func([]byte) (*image.NRGBA, error)) error {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return fmt.Errorf("create snapshot identifier: %w", err)
	}

	name := "snapshot-" + hex.EncodeToString(token[:])
	if err := root.Mkdir(name, 0o755); err != nil {
		return fmt.Errorf("create login snapshot: %w", err)
	}

	complete := false
	defer func() {
		if !complete {
			_ = root.Remove(name + "/theme.conf")
			_ = root.Remove(name + "/wallpaper.png")
			_ = root.Remove(name)
		}
	}()

	if len(request.Image) > 0 {
		if err := writeImage(root, name+"/wallpaper.png", request.Image, decode); err != nil {
			return err
		}
	} else {
		value, _ := strconv.ParseUint(strings.TrimPrefix(request.Background, "#"), 16, 32)
		pixel := image.NewNRGBA(image.Rect(0, 0, 1, 1))
		pixel.Set(0, 0, color.NRGBA{R: uint8(value >> 16 & 0xff), G: uint8(value >> 8 & 0xff), B: uint8(value & 0xff), A: 255})

		if err := writeFile(root, name+"/wallpaper.png", func(writer io.Writer) error { return png.Encode(writer, pixel) }); err != nil {
			return err
		}
	}

	wallpaper := "/var/lib/dotfiles-sddm/" + name + "/wallpaper.png"

	var config strings.Builder

	_, _ = fmt.Fprintf(&config, "# revision=%s\n# digest=%s\n[General]\nBackground=%s\nFont=%s\n", request.Revision, requestDigest(request), wallpaper, request.Font)

	for _, group := range []struct {
		color string
		keys  []string
	}{
		{request.Background, []string{"BackgroundColor", "FormBackgroundColor", "DimBackgroundColor", "LoginFieldBackgroundColor", "PasswordFieldBackgroundColor", "DropdownBackgroundColor", "LoginButtonTextColor"}},
		{request.Foreground, []string{"HeaderTextColor", "DateTextColor", "TimeTextColor", "LoginFieldTextColor", "PasswordFieldTextColor", "UserIconColor", "PasswordIconColor", "SystemButtonsIconsColor", "SessionButtonTextColor", "VirtualKeyboardButtonTextColor", "DropdownTextColor", "HighlightTextColor"}},
		{request.Accent, []string{"LoginButtonBackgroundColor", "DropdownSelectedBackgroundColor", "HighlightBackgroundColor", "HighlightBorderColor", "HoverUserIconColor", "HoverPasswordIconColor", "HoverSystemButtonsIconsColor", "HoverSessionButtonTextColor", "HoverVirtualKeyboardButtonTextColor"}},
		{request.Muted, []string{"PlaceholderTextColor"}},
		{request.Error, []string{"WarningColor"}},
	} {
		for _, key := range group.keys {
			config.WriteString(key + "=" + group.color + "\n")
		}
	}

	if err := writeFile(root, name+"/theme.conf", func(writer io.Writer) error {
		if _, err := io.WriteString(writer, config.String()); err != nil {
			return fmt.Errorf("encode login config: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	if err := syncDirectory(root, name); err != nil {
		return err
	}

	link := name + ".link"
	if err := unix.Symlinkat(name+"/theme.conf", fd, link); err != nil {
		return fmt.Errorf("stage login pointer: %w", err)
	}

	defer func() { _ = root.Remove(link) }()

	if err := unix.Renameat(fd, link, fd, "current.conf"); err != nil {
		return fmt.Errorf("publish login pointer: %w", err)
	}

	complete = true

	if err := syncDirectory(root, "."); err != nil {
		return err
	}

	return nil
}

func writeImage(root *os.Root, name string, data []byte, decode func([]byte) (*image.NRGBA, error)) error {
	if decode == nil {
		return errors.New("isolated raster decoder is required")
	}

	decoded, err := decode(data)
	if err != nil {
		return fmt.Errorf("decode login wallpaper: %w", err)
	}

	return writeFile(root, name, func(writer io.Writer) error { return png.Encode(writer, decoded) })
}

func writeFile(root *os.Root, name string, write func(io.Writer) error) error {
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		return fmt.Errorf("create snapshot file: %w", err)
	}

	defer func() { _ = file.Close() }()

	if err = write(file); err != nil {
		return fmt.Errorf("write snapshot file: %w", err)
	}

	if err = file.Sync(); err != nil {
		return fmt.Errorf("sync snapshot file: %w", err)
	}

	if err = file.Close(); err != nil {
		return fmt.Errorf("close snapshot file: %w", err)
	}

	return nil
}

func syncDirectory(root *os.Root, name string) error {
	dir, err := root.Open(name)
	if err != nil {
		return fmt.Errorf("open snapshot directory for sync: %w", err)
	}

	defer func() { _ = dir.Close() }()

	if err = dir.Sync(); err != nil {
		return fmt.Errorf("sync snapshot directory: %w", err)
	}

	return nil
}
