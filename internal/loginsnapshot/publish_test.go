package loginsnapshot_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/PedroKlein/tools/internal/loginsnapshot"
)

func requestBytes(t *testing.T, revision time.Time, wallpaper []byte) []byte {
	t.Helper()

	data, err := json.Marshal(loginsnapshot.Request{
		Revision:   revision.UTC().Format(time.RFC3339Nano),
		Background: "#101010", Foreground: "#ffffff", Muted: "#aaaaaa",
		Accent: "#abcdef", Error: "#ff0000", Font: "JetBrainsMono Nerd Font", Image: wallpaper,
	})
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func snapshotRoot(t *testing.T) *os.Root {
	t.Helper()

	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = root.Close() })

	return root
}

func currentConfig(t *testing.T, root *os.Root) string {
	t.Helper()

	file, err := root.Open("current.conf")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func TestPublishCompleteSnapshot(t *testing.T) {
	root := snapshotRoot(t)

	var imageData bytes.Buffer

	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)

	if err := png.Encode(&imageData, img); err != nil {
		t.Fatal(err)
	}

	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), imageData.Bytes())), decodeTestRaster); err != nil {
		t.Fatal(err)
	}

	config := currentConfig(t, root)
	if !strings.Contains(config, "Background=/var/lib/dotfiles-sddm/snapshot-") || !strings.Contains(config, "HighlightBackgroundColor=#abcdef") {
		t.Fatal(config)
	}

	info, err := root.Stat("current.conf")
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("greeter permissions: %v, %v", info, err)
	}
}

func TestRejectedInputRetainsPreviousSnapshot(t *testing.T) {
	root := snapshotRoot(t)

	good := requestBytes(t, time.Now(), nil)
	if err := loginsnapshot.Publish(root, bytes.NewReader(good), decodeTestRaster); err != nil {
		t.Fatal(err)
	}

	before := currentConfig(t, root)
	for name, data := range map[string][]byte{
		"null":            []byte("null"),
		"unknown-path":    bytes.Replace(good, []byte(`"font":`), []byte(`"output":"/etc/passwd","font":`), 1),
		"font-injection":  bytes.Replace(good, []byte("JetBrainsMono Nerd Font"), []byte(`bad\n[General]`), 1),
		"color-injection": bytes.Replace(good, []byte("#abcdef"), []byte("red;command"), 1),
		"trailing":        append(append([]byte{}, good...), []byte(" {}")...),
		"oversized":       bytes.Repeat([]byte("x"), loginsnapshot.MaxRequestBytes+1),
		"invalid-image":   requestBytes(t, time.Now(), []byte("not raster")),
		"stale":           requestBytes(t, time.Now().Add(-time.Hour), nil),
		"future":          requestBytes(t, time.Now().Add(time.Hour), nil),
	} {
		t.Run(name, func(t *testing.T) {
			if err := loginsnapshot.Publish(root, bytes.NewReader(data), decodeTestRaster); err == nil {
				t.Fatal("unsafe request accepted")
			}

			if after := currentConfig(t, root); before != after {
				t.Fatal("rejection replaced the previous snapshot")
			}
		})
	}
}

func TestSolidFallbackPublishesNonemptyVersionedImage(t *testing.T) {
	root := snapshotRoot(t)
	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), nil)), decodeTestRaster); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(currentConfig(t, root), "Background=/var/lib/dotfiles-sddm/snapshot-") {
		t.Fatal("empty image overlay would leave Astronaut's previous/default wallpaper")
	}
}

func TestIdenticalApplyDoesNotAccumulateSnapshots(t *testing.T) {
	root := snapshotRoot(t)

	data := requestBytes(t, time.Now(), nil)
	for range 40 {
		if err := loginsnapshot.Publish(root, bytes.NewReader(data), decodeTestRaster); err != nil {
			t.Fatal(err)
		}
	}

	file, err := root.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	entries, err := file.ReadDir(-1)
	if err != nil {
		t.Fatal(err)
	}

	count := 0

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "snapshot-") {
			count++
		}
	}

	if count != 1 {
		t.Fatalf("repeated apply retained %d snapshots, want 1", count)
	}
}

func TestImageDimensionBombLeavesPreviousSnapshot(t *testing.T) {
	root := snapshotRoot(t)
	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), nil)), decodeTestRaster); err != nil {
		t.Fatal(err)
	}

	before := currentConfig(t, root)

	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewGray(image.Rect(0, 0, 8193, 1))); err != nil {
		t.Fatal(err)
	}

	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), imageData.Bytes())), decodeTestRaster); err == nil {
		t.Fatal("oversized raster accepted")
	}

	if currentConfig(t, root) != before {
		t.Fatal("invalid image replaced current snapshot")
	}
}

func TestRetentionIsBoundedAndPreservesCurrentSnapshot(t *testing.T) {
	root := snapshotRoot(t)

	now := time.Now().Add(-time.Minute)
	for i := range 32 {
		if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, now.Add(time.Duration(i)*time.Second), nil)), decodeTestRaster); err != nil {
			t.Fatal(err)
		}
	}

	before := currentConfig(t, root)
	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), nil)), decodeTestRaster); err == nil {
		t.Fatal("unbounded login snapshot storage")
	}

	if currentConfig(t, root) != before {
		t.Fatal("retention failure replaced current config")
	}
}

func TestEqualRevisionCannotReplaceDifferentSnapshot(t *testing.T) {
	root := snapshotRoot(t)

	data := requestBytes(t, time.Now(), nil)
	if err := loginsnapshot.Publish(root, bytes.NewReader(data), decodeTestRaster); err != nil {
		t.Fatal(err)
	}

	before := currentConfig(t, root)

	changed := bytes.Replace(data, []byte("#abcdef"), []byte("#fedcba"), 1)
	if err := loginsnapshot.Publish(root, bytes.NewReader(changed), decodeTestRaster); err == nil {
		t.Fatal("conflicting same-revision data accepted")
	}

	if currentConfig(t, root) != before {
		t.Fatal("same-revision request replaced committed data")
	}
}

func TestUnsafeLockLinkCannotWriteOutsideDestination(t *testing.T) {
	root := snapshotRoot(t)
	outside := t.TempDir()

	path := filepath.Join(outside, "keep")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(path, filepath.Join(root.Name(), "publish.lock")); err != nil {
		t.Fatal(err)
	}

	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), nil)), decodeTestRaster); err == nil {
		t.Fatal("unsafe lock link accepted")
	}

	file, err := os.OpenRoot(outside)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	data, err := fs.ReadFile(file.FS(), "keep")
	if err != nil || string(data) != "original" {
		t.Fatal("outside destination modified")
	}
}

func TestWriteFailureLeavesCurrentSnapshotIntact(t *testing.T) {
	if dir := os.Getenv("LOGIN_TEST_FILE_LIMIT"); dir != "" {
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer root.Close()

		if err := syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 64, Max: 64}); err != nil {
			t.Fatal(err)
		}

		if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), nil)), decodeTestRaster); err == nil {
			t.Fatal("partial write was published")
		}

		return
	}

	root := snapshotRoot(t)
	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now().Add(-time.Minute), nil)), decodeTestRaster); err != nil {
		t.Fatal(err)
	}

	before := currentConfig(t, root)

	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, "-test.run=^TestWriteFailureLeavesCurrentSnapshotIntact$")

	cmd.Env = append(os.Environ(), "LOGIN_TEST_FILE_LIMIT="+root.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("write-failure subprocess: %v: %s", err, out)
	}

	if currentConfig(t, root) != before {
		t.Fatal("failed staged write replaced current snapshot")
	}
}

func TestFailurePublishingPointerLeavesOldSnapshotUsable(t *testing.T) {
	root := snapshotRoot(t)
	if err := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), nil)), decodeTestRaster); err != nil {
		t.Fatal(err)
	}

	file, err := root.Open("current.conf")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if removeErr := root.Remove("current.conf"); removeErr != nil {
		t.Fatal(removeErr)
	}

	if mkdirErr := root.Mkdir("current.conf", 0o755); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}

	if publishErr := loginsnapshot.Publish(root, bytes.NewReader(requestBytes(t, time.Now(), nil)), decodeTestRaster); publishErr == nil {
		t.Fatal("blocked pointer update accepted")
	}

	data, err := io.ReadAll(file)
	if err != nil || !strings.Contains(string(data), "BackgroundColor=#101010") {
		t.Fatalf("old snapshot unusable: %s, %v", data, err)
	}
}

func TestConcurrentPublicationCannotOverwriteNewerRevision(t *testing.T) {
	root := snapshotRoot(t)
	now := time.Now().Add(-time.Minute)

	var wg sync.WaitGroup

	for i := range 8 {
		payload := requestBytes(t, now.Add(time.Duration(i)*time.Second), nil)

		wg.Add(1)
		go func() {
			defer wg.Done()

			_ = loginsnapshot.Publish(root, bytes.NewReader(payload), decodeTestRaster)
		}()
	}

	wg.Wait()

	if !strings.HasPrefix(currentConfig(t, root), "# revision="+now.Add(7*time.Second).UTC().Format(time.RFC3339Nano)+"\n") {
		t.Fatal("newer committed state overwritten")
	}
}
