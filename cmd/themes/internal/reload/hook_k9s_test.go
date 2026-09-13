package reload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHookK9sWritesCurrentSkinAsFile(t *testing.T) {
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	derivedDir := filepath.Join(themeDir, "derived")
	if err := os.MkdirAll(derivedDir, 0o750); err != nil {
		t.Fatal(err)
	}
	payload := []byte("k9s:\n  body:\n    fgColor: '#D4BE98'\n")
	if err := os.WriteFile(filepath.Join(derivedDir, "k9s.yaml"), payload, 0o600); err != nil {
		t.Fatal(err)
	}

	configDir := filepath.Join(tmp, "k9s")
	skinsDir := filepath.Join(configDir, "skins")
	if err := os.MkdirAll(skinsDir, 0o750); err != nil {
		t.Fatal(err)
	}
	oldTarget := filepath.Join(tmp, "old-current.yaml")
	if err := os.WriteFile(oldTarget, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(skinsDir, "current.yaml")
	if err := os.Symlink(oldTarget, current); err != nil {
		t.Fatal(err)
	}

	origConfigDir := k9sConfigDir
	k9sConfigDir = func() string { return configDir }
	t.Cleanup(func() { k9sConfigDir = origConfigDir })

	if err := hookK9s(context.Background(), themeDir); err != nil {
		t.Fatalf("hookK9s: %v", err)
	}
	info, err := os.Lstat(current)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("current skin is still a symlink; k9s reactive watcher will not see target swaps")
	}
	got, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("current skin = %q, want %q", got, payload)
	}
}

func TestHookK9sNoOpWithoutPayload(t *testing.T) {
	tmp := t.TempDir()
	themeDir := filepath.Join(tmp, "theme")
	if err := os.MkdirAll(filepath.Join(themeDir, "derived"), 0o750); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(tmp, "k9s")
	origConfigDir := k9sConfigDir
	k9sConfigDir = func() string { return configDir }
	t.Cleanup(func() { k9sConfigDir = origConfigDir })

	if err := hookK9s(context.Background(), themeDir); err != nil {
		t.Fatalf("hookK9s without payload: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "skins", "current.yaml")); err == nil {
		t.Fatal("wrote current skin despite missing payload")
	}
}
