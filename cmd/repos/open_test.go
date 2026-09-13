package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReposOpenCreatesHerdrWorkspace(t *testing.T) {
	root, worktreeDir := openTestRepo(t)
	fakeBin, commandLog := fakeHerdr(t)

	cmd := reposCommand(t, root, fakeBin, "open", "project")
	cmd.Env = append(cmd.Env, "HERDR_SCENARIO=create")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("repos open: %v\n%s", err, out)
	}

	wantOutput := "opened example.com/owner/project (workspace: example.com/owner/project)\n"
	if string(out) != wantOutput {
		t.Fatalf("stdout = %q, want %q", out, wantOutput)
	}

	assertHerdrCommands(t, commandLog, []string{
		worktreeDir + "|<workspace><list>",
		worktreeDir + "|<workspace><create><--cwd><" + worktreeDir + "><--label><example.com/owner/project><--focus>",
	})
}

func TestReposOpenFocusesExistingHerdrWorkspace(t *testing.T) {
	root, worktreeDir := openTestRepo(t)
	fakeBin, commandLog := fakeHerdr(t)

	cmd := reposCommand(t, root, fakeBin, "open", "project")
	cmd.Env = append(cmd.Env, "HERDR_SCENARIO=existing")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("repos open: %v\n%s", err, out)
	}

	wantOutput := "focused example.com/owner/project (workspace: example.com/owner/project)\n"
	if string(out) != wantOutput {
		t.Fatalf("stdout = %q, want %q", out, wantOutput)
	}

	assertHerdrCommands(t, commandLog, []string{
		worktreeDir + "|<workspace><list>",
		worktreeDir + "|<workspace><focus><w7>",
	})
}

func TestReposOpenJSON(t *testing.T) {
	tests := []struct {
		name          string
		scenario      string
		wantWorkspace string
		wantCreated   bool
	}{
		{name: "created", scenario: "create", wantWorkspace: "w9", wantCreated: true},
		{name: "focused", scenario: "existing", wantWorkspace: "w7", wantCreated: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, worktreeDir := openTestRepo(t)
			fakeBin, _ := fakeHerdr(t)

			cmd := reposCommand(t, root, fakeBin, "--json", "open", "project")
			cmd.Env = append(cmd.Env, "HERDR_SCENARIO="+tt.scenario)

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("repos open --json: %v\n%s", err, out)
			}

			var result OpenResult
			if err := json.Unmarshal(out, &result); err != nil {
				t.Fatalf("decoding stdout as JSON: %v\n%s", err, out)
			}

			want := OpenResult{
				Workspace: tt.wantWorkspace,
				Path:      worktreeDir,
				Created:   tt.wantCreated,
				Focused:   true,
			}
			if result != want {
				t.Fatalf("result = %+v, want %+v", result, want)
			}
		})
	}
}

func TestReposOpenReportsHerdrErrors(t *testing.T) {
	tests := []struct {
		name     string
		scenario string
		want     string
	}{
		{
			name:     "create",
			scenario: "create-error",
			want:     "error: creating Herdr workspace: exit status 12: create exploded\n",
		},
		{
			name:     "focus",
			scenario: "focus-error",
			want:     "error: focusing Herdr workspace: exit status 13: focus exploded\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := openTestRepo(t)
			fakeBin, _ := fakeHerdr(t)

			cmd := reposCommand(t, root, fakeBin, "open", "project")
			cmd.Env = append(cmd.Env, "HERDR_SCENARIO="+tt.scenario)

			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("repos open succeeded, want error\n%s", out)
			}

			if string(out) != tt.want {
				t.Fatalf("output = %q, want %q", out, tt.want)
			}
		})
	}
}

func TestReposOpenJSONError(t *testing.T) {
	root, _ := openTestRepo(t)
	fakeBin, _ := fakeHerdr(t)

	cmd := reposCommand(t, root, fakeBin, "--json", "open", "project")
	cmd.Env = append(cmd.Env, "HERDR_SCENARIO=create-error")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("repos open --json succeeded, want error\n%s", out)
	}

	var result struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("decoding stdout as JSON: %v\n%s", err, out)
	}

	want := "creating Herdr workspace: exit status 12: create exploded"
	if result.Error != want {
		t.Fatalf("error = %q, want %q", result.Error, want)
	}
}

func TestReposOpenRequiresHerdr(t *testing.T) {
	root, _ := openTestRepo(t)
	emptyBin := t.TempDir()

	cmd := reposCommand(t, root, emptyBin, "open", "project")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("repos open succeeded, want error\n%s", out)
	}

	if string(out) != "error: Herdr not found in PATH\n" {
		t.Fatalf("output = %q, want missing Herdr error", out)
	}
}

func TestReposHelpDescribesHerdr(t *testing.T) {
	cmd := reposCommand(t, t.TempDir(), t.TempDir(), "help")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("repos help: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "Open or focus a Herdr workspace") {
		t.Fatalf("help does not describe Herdr:\n%s", out)
	}

	if strings.Contains(strings.ToLower(string(out)), "t"+"mux") {
		t.Fatalf("help still describes the removed multiplexer:\n%s", out)
	}
}

func TestReposOpenHelper(_ *testing.T) {
	if os.Getenv("REPOS_OPEN_HELPER") != "1" {
		return
	}

	separator := 0

	for i, arg := range os.Args {
		if arg == "--" {
			separator = i + 1
			break
		}
	}

	os.Args = os.Args[separator:]

	main()
	os.Exit(0)
}

func openTestRepo(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	repoPath := filepath.Join(root, "example.com", "owner", "project")
	gitDir := filepath.Join(repoPath, ".git")
	worktreeDir := filepath.Join(repoPath, "main")

	if err := os.MkdirAll(worktreeDir, 0o750); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatal(err)
	}

	config := "[core]\n\tbare = true\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	return root, worktreeDir
}

func fakeHerdr(t *testing.T) (string, string) {
	t.Helper()

	binDir := t.TempDir()
	commandLog := filepath.Join(t.TempDir(), "herdr.log")

	script := `#!/bin/sh
printf '%s|' "$PWD" >> "$HERDR_COMMAND_LOG"
printf '<%s>' "$@" >> "$HERDR_COMMAND_LOG"
printf '\n' >> "$HERDR_COMMAND_LOG"

case "$1:$2:$HERDR_SCENARIO" in
  workspace:list:existing|workspace:list:focus-error)
    printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w6","label":"project"},{"workspace_id":"w7","label":"example.com/owner/project"}]}}'
    ;;
  workspace:list:*)
    printf '%s\n' '{"result":{"workspaces":[]}}'
    ;;
  workspace:create:create-error)
    printf '%s\n' 'create exploded' >&2
    exit 12
    ;;
  workspace:create:*)
    printf '%s\n' '{"result":{"workspace":{"workspace_id":"w9","label":"example.com/owner/project"}}}'
    ;;
  workspace:focus:focus-error)
    printf '%s\n' 'focus exploded' >&2
    exit 13
    ;;
  workspace:focus:*)
    printf '%s\n' '{"result":{"workspace":{"workspace_id":"w7","label":"example.com/owner/project"}}}'
    ;;
esac
`

	herdrPath := filepath.Join(binDir, "herdr")
	if err := os.WriteFile(herdrPath, []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(herdrPath, 0o700); err != nil { //nolint:gosec // the controlled test fixture must be executable
		t.Fatal(err)
	}

	t.Setenv("HERDR_COMMAND_LOG", commandLog)

	return binDir, commandLog
}

func reposCommand(t *testing.T, root, binDir string, args ...string) *exec.Cmd {
	t.Helper()

	cmdArgs := append([]string{"-test.run=^TestReposOpenHelper$", "--", "repos"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...) //nolint:gosec // subprocess reruns the current test binary with fixed helper arguments

	cmd.Env = append(os.Environ(),
		"REPOS_OPEN_HELPER=1",
		"REPOS_ROOT="+root,
		"PATH="+binDir,
	)

	return cmd
}

func assertHerdrCommands(t *testing.T, path string, want []string) {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // path is created by this test under t.TempDir
	if err != nil {
		t.Fatal(err)
	}

	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(got) != len(want) {
		t.Fatalf("Herdr commands = %q, want %q", got, want)
	}

	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Herdr command %d = %q, want %q", i, got[i], want[i])
		}
	}
}
