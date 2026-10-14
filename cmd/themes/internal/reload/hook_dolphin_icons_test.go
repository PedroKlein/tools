package reload_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PedroKlein/tools/cmd/themes/internal/reload"
)

const folderArtwork = `<svg><path fill="#5294e2"/><path fill="#4877b1"/><path fill="#1d344f"/><path fill="#ffffff"/></svg>`

func TestDolphinFolderColors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux desktop")
	}

	root, theme := dolphinFixture(t, "lifecycle")
	installFolderFixture(t, root)

	manifest, err := fs.ReadFile(root.FS(), "config/themes/sample/theme.json")
	if err != nil {
		t.Fatal(err)
	}

	skip := dolphinSkipList(t)

	var names []string

	for i, color := range []string{"#abcdef", "#fedcba", "#abcdef"} {
		data := strings.Replace(string(manifest), `"accent":"#111111"`, `"accent":"#111111","accent2":"`+color+`"`, 1)
		if err = writeDolphinFixtureFile(root, "config/themes/sample/theme.json", []byte(data)); err != nil {
			t.Fatal(err)
		}

		preview := i != 0

		results := reload.RunAll(context.Background(), theme, reload.Options{SkipHooks: skip, SkipUserHooks: true, LiveApply: &preview})
		if len(results) != 1 || results[0].Err != nil {
			t.Fatalf("reload: %+v", results)
		}

		names = append(names, checkFolderOverlay(t, root, color))
	}

	if names[0] == names[1] || names[0] != names[2] {
		t.Fatalf("preview/restore names: %v", names)
	}

	original, err := fs.ReadFile(root.FS(), "data/icons/Papirus/64x64/places/folder-blue.svg")
	if err != nil {
		t.Fatal(err)
	}

	if string(original) != folderArtwork {
		t.Fatal("source artwork modified")
	}
}

func installFolderFixture(t *testing.T, root *os.Root) {
	t.Helper()

	source := filepath.Join(os.Getenv("XDG_DATA_HOME"), "icons", "Papirus")

	for _, path := range []string{"data", "data/icons", "data/icons/Papirus", "data/icons/Papirus/64x64", "data/icons/Papirus/64x64/places"} {
		if err := root.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	index := "[Icon Theme]\nName=Papirus\nDirectories=64x64/places\n\n[64x64/places]\nSize=64\nType=Fixed\nContext=Places\n"
	for path, data := range map[string]string{"index.theme": index, "64x64/places/folder-blue.svg": folderArtwork} {
		if err := writeDolphinFixtureFile(root, "data/icons/Papirus/"+path, []byte(data)); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Symlink("folder-blue.svg", filepath.Join(source, "64x64/places/folder.svg")); err != nil {
		t.Fatal(err)
	}
}

func checkFolderOverlay(t *testing.T, root *os.Root, color string) string {
	t.Helper()

	globals, err := fs.ReadFile(root.FS(), "config/kdeglobals")
	if err != nil {
		t.Fatal(err)
	}

	name := ""

	for line := range strings.SplitSeq(string(globals), "\n") {
		if value, ok := strings.CutPrefix(line, "Theme="); ok {
			name = value
		}
	}

	overlay := "data/icons/" + name

	icon, err := fs.ReadFile(root.FS(), overlay+"/64x64/places/folder.svg")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(icon), color) || strings.Contains(string(icon), "#5294e2") || !strings.Contains(string(icon), "#ffffff") {
		t.Fatalf("incorrect secondary folder color: %s", icon)
	}

	inherited, err := fs.ReadFile(root.FS(), overlay+"/index.theme")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(inherited), "Inherits=Papirus-Dark") {
		t.Fatalf("missing icon fallback: %s", inherited)
	}

	return name
}
