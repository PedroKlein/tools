package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveAgentDir(t *testing.T) {
	t.Run("uses PIA_AGENT_DIR when set", func(t *testing.T) {
		t.Setenv("PIA_AGENT_DIR", "/tmp/test-agent")

		got := resolveAgentDir()
		if got != "/tmp/test-agent" {
			t.Errorf("got %q, want /tmp/test-agent", got)
		}
	})

	t.Run("defaults to ~/.pi/agent", func(t *testing.T) {
		t.Setenv("PIA_AGENT_DIR", "")

		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".pi", "agent")

		got := resolveAgentDir()
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestLoadProfile(t *testing.T) {
	t.Run("valid profile", func(t *testing.T) {
		dir := t.TempDir()
		profileJSON := `{
			"description": "test profile",
			"model": "test/model",
			"thinking": "low",
			"sharedExtensions": ["ext-a", "ext-b"],
			"packages": ["npm:pkg-a"],
			"skills": [],
			"flags": ["--no-context-files"]
		}`
		path := filepath.Join(dir, "profile.json")
		os.WriteFile(path, []byte(profileJSON), 0o644) //nolint:gosec // test setup

		p, err := loadProfile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if p.Description != "test profile" {
			t.Errorf("description: got %q", p.Description)
		}

		if p.Model != "test/model" {
			t.Errorf("model: got %q", p.Model)
		}

		if len(p.SharedExtensions) != 2 {
			t.Errorf("sharedExtensions: got %d items", len(p.SharedExtensions))
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := loadProfile("/nonexistent/path/profile.json")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "profile.json")
		os.WriteFile(path, []byte("{invalid"), 0o644) //nolint:gosec // test setup

		_, err := loadProfile(path)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})
}

func TestDiscoverProfiles(t *testing.T) {
	dir := t.TempDir()
	profilesDir := filepath.Join(dir, "profiles", "quick")
	os.MkdirAll(profilesDir, 0o755)                                                                    //nolint:gosec // test setup
	os.WriteFile(filepath.Join(profilesDir, "profile.json"), []byte(`{"description":"quick"}`), 0o644) //nolint:gosec // test setup

	// Add a second profile
	researchDir := filepath.Join(dir, "profiles", "research")
	os.MkdirAll(researchDir, 0o755)                                                                       //nolint:gosec // test setup
	os.WriteFile(filepath.Join(researchDir, "profile.json"), []byte(`{"description":"research"}`), 0o644) //nolint:gosec // test setup

	profiles, err := discoverProfiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(profiles) != 2 {
		t.Fatalf("got %d profiles, want 2", len(profiles))
	}

	if profiles["quick"].Description != "quick" {
		t.Errorf("quick description: %q", profiles["quick"].Description)
	}

	if profiles["research"].Description != "research" {
		t.Errorf("research description: %q", profiles["research"].Description)
	}
}

func TestDiscoverProfilesEmpty(t *testing.T) {
	dir := t.TempDir()

	profiles, err := discoverProfiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profiles != nil {
		t.Errorf("expected nil for missing profiles dir, got %v", profiles)
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"tilde prefix", "~/.pi/agent/skills", filepath.Join(home, ".pi/agent/skills")},
		{"bare tilde", "~", home},
		{"absolute path unchanged", "/usr/local/bin", "/usr/local/bin"},
		{"relative path unchanged", "./relative/path", "./relative/path"},
		{"tilde in middle unchanged", "/some/~/path", "/some/~/path"},
		{"flag unchanged", "--no-context-files", "--no-context-files"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandTilde(tt.input)
			if got != tt.want {
				t.Errorf("expandTilde(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildArgs(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name      string
		profile   Profile
		extraArgs []string
		want      []string
	}{
		{
			name:    "all fields",
			profile: Profile{Model: "test/model", Thinking: "high", Skills: []string{"/path/skill"}, Flags: []string{"--no-context-files"}},
			want:    []string{"--model", "test/model", "--thinking", "high", "--skill", "/path/skill", "--no-context-files"},
		},
		{
			name:    "empty profile",
			profile: Profile{},
			want:    nil,
		},
		{
			name:      "extra args appended",
			profile:   Profile{Model: "m"},
			extraArgs: []string{"-c", "do something"},
			want:      []string{"--model", "m", "-c", "do something"},
		},
		{
			name: "tilde expanded in skills",
			profile: Profile{
				Skills: []string{"~/.pi/agent/profiles/research/skills"},
			},
			want: []string{"--skill", filepath.Join(home, ".pi/agent/profiles/research/skills")},
		},
		{
			name: "tilde expanded in flags",
			profile: Profile{
				Flags: []string{"--prompt-template", "~/.pi/agent/profiles/research/prompts"},
			},
			want: []string{"--prompt-template", filepath.Join(home, ".pi/agent/profiles/research/prompts")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildArgs(tt.profile, tt.extraArgs)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("arg[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestProfileLaunchSyncsBeforeExec(t *testing.T) {
	agentDir := newSyncTestAgentDir(t, map[string]Profile{
		"quick": {Packages: []string{"npm:current"}},
	})

	generatedDir := filepath.Join(filepath.Dir(agentDir), "agent-quick")
	if err := os.MkdirAll(filepath.Join(generatedDir, "sessions"), 0o750); err != nil {
		t.Fatalf("creating generated profile: %v", err)
	}

	if err := os.WriteFile(filepath.Join(generatedDir, "settings.json"), []byte(`{"packages":["npm:stale"]}`), 0o600); err != nil {
		t.Fatalf("writing stale settings: %v", err)
	}

	if err := os.WriteFile(filepath.Join(generatedDir, "sessions", "keep"), []byte("session"), 0o600); err != nil {
		t.Fatalf("writing session sentinel: %v", err)
	}

	for _, path := range []string{"npm/keep", "memory/keep", "mcp-auth.json", "runtime-state"} {
		fullPath := filepath.Join(generatedDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
			t.Fatalf("creating runtime sentinel parent: %v", err)
		}

		if err := os.WriteFile(fullPath, []byte("keep"), 0o600); err != nil {
			t.Fatalf("writing runtime sentinel: %v", err)
		}
	}

	binDir := t.TempDir()
	piPath := filepath.Join(binDir, "pi")
	script := `#!/bin/sh
cat "$PI_CODING_AGENT_DIR/settings.json"
test -f "$PI_CODING_AGENT_DIR/sessions/keep"
test -f "$PI_CODING_AGENT_DIR/npm/keep"
test -f "$PI_CODING_AGENT_DIR/memory/keep"
test -f "$PI_CODING_AGENT_DIR/mcp-auth.json"
test -f "$PI_CODING_AGENT_DIR/runtime-state"
`
	writeExecutable(t, piPath, script)

	cmd := piaCommand(t, agentDir, "profile", "quick")
	cmd.Env = append(cmd.Env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pia profile: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), `"npm:current"`) {
		t.Fatalf("profile launched with stale settings:\n%s", out)
	}
}

func TestUpdateSynchronizesAroundOrderedExtensionUpdates(t *testing.T) {
	agentDir := newSyncTestAgentDir(t, map[string]Profile{
		"zeta":  {Packages: []string{"npm:zeta-current"}},
		"alpha": {Packages: []string{"npm:alpha-current"}},
	})
	logPath := filepath.Join(t.TempDir(), "commands.log")
	binDir := t.TempDir()
	piPath := filepath.Join(binDir, "pi")
	script := `#!/bin/sh
printf '%s|%s\n' "$PI_CODING_AGENT_DIR" "$*" >> "$PIA_TEST_LOG"
if [ "$*" = "update --extensions" ] && [ "$PI_CODING_AGENT_DIR" = "$(dirname "$PIA_AGENT_DIR")/agent-alpha" ]; then
  grep -q 'npm:alpha-current' "$PI_CODING_AGENT_DIR/settings.json"
fi
if [ "$*" = "update --extensions" ] && [ "$PI_CODING_AGENT_DIR" = "$(dirname "$PIA_AGENT_DIR")/agent-zeta" ]; then
  printf '%s\n' '{"packages":["npm:zeta-after-update"]}' > "$PIA_AGENT_DIR/profiles/zeta/profile.json"
fi
`
	writeExecutable(t, piPath, script)

	cmd := piaCommand(t, agentDir, "update")
	cmd.Env = append(cmd.Env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PIA_TEST_LOG="+logPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pia update: %v\n%s", err, out)
	}

	logData, err := os.ReadFile(logPath) //nolint:gosec // test path is under t.TempDir
	if err != nil {
		t.Fatalf("reading command log: %v", err)
	}

	parentDir := filepath.Dir(agentDir)

	want := strings.Join([]string{
		agentDir + "|update",
		agentDir + "|update --extensions",
		filepath.Join(parentDir, "agent-alpha") + "|update --extensions",
		filepath.Join(parentDir, "agent-zeta") + "|update --extensions",
	}, "\n") + "\n"
	if string(logData) != want {
		t.Fatalf("command log:\n%s\nwant:\n%s", logData, want)
	}

	zetaSettings, err := os.ReadFile(filepath.Join(parentDir, "agent-zeta", "settings.json")) //nolint:gosec // test-managed profile path
	if err != nil {
		t.Fatalf("reading post-update settings: %v", err)
	}

	if !strings.Contains(string(zetaSettings), "npm:zeta-after-update") {
		t.Fatalf("profiles were not synchronized after updates:\n%s", zetaSettings)
	}
}

func TestUpdateOptions(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want func(agentDir string) string
	}{
		{
			name: "extensions only",
			args: []string{"--extensions-only"},
			want: func(agentDir string) string {
				return strings.Join([]string{
					"pi|" + agentDir + "|update --extensions",
					"pi|" + filepath.Join(filepath.Dir(agentDir), "agent-quick") + "|update --extensions",
				}, "\n") + "\n"
			},
		},
		{
			name: "exact Pi version",
			args: []string{"--pi-version", "1.0.0"},
			want: func(agentDir string) string {
				return strings.Join([]string{
					"npm||install --global --ignore-scripts @earendil-works/pi-coding-agent@1.0.0",
					"pi|" + agentDir + "|update --extensions",
					"pi|" + filepath.Join(filepath.Dir(agentDir), "agent-quick") + "|update --extensions",
				}, "\n") + "\n"
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			agentDir := newSyncTestAgentDir(t, map[string]Profile{"quick": {}})
			logPath := filepath.Join(t.TempDir(), "commands.log")
			binDir := t.TempDir()

			for _, name := range []string{"pi", "npm"} {
				script := fmt.Sprintf("#!/bin/sh\nprintf '%s|%%s|%%s\\n' \"$PI_CODING_AGENT_DIR\" \"$*\" >> \"$PIA_TEST_LOG\"\n", name)
				writeExecutable(t, filepath.Join(binDir, name), script)
			}

			args := append([]string{"update"}, tt.args...)
			cmd := piaCommand(t, agentDir, args...)
			cmd.Env = append(cmd.Env,
				"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"PIA_TEST_LOG="+logPath,
			)

			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("pia update: %v\n%s", err, out)
			}

			logData, err := os.ReadFile(logPath) //nolint:gosec // test path is under t.TempDir
			if err != nil {
				t.Fatalf("reading command log: %v", err)
			}

			if string(logData) != tt.want(agentDir) {
				t.Fatalf("command log:\n%s\nwant:\n%s", logData, tt.want(agentDir))
			}
		})
	}
}

func TestExactUpdateInstallsBeforeLookingUpPi(t *testing.T) {
	agentDir := newSyncTestAgentDir(t, map[string]Profile{"quick": {}})
	binDir := t.TempDir()
	piPath := filepath.Join(binDir, "pi")

	piScript := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(piPath, []byte(piScript), 0o600); err != nil {
		t.Fatalf("writing inactive pi: %v", err)
	}

	npmScript := "#!/bin/sh\n/bin/chmod +x \"$PIA_TEST_PI_PATH\"\n"
	writeExecutable(t, filepath.Join(binDir, "npm"), npmScript)

	cmd := piaCommand(t, agentDir, "update", "--pi-version", "1.0.0")
	cmd.Env = append(cmd.Env,
		"PATH="+binDir,
		"PIA_TEST_PI_PATH="+piPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exact update looked up Pi before installation: %v\n%s", err, out)
	}
}

func TestUpdateStopsAtFirstFailure(t *testing.T) {
	agentDir := newSyncTestAgentDir(t, map[string]Profile{
		"quick": {Packages: []string{"npm:before-failure"}},
	})
	logPath := filepath.Join(t.TempDir(), "commands.log")
	binDir := t.TempDir()
	piPath := filepath.Join(binDir, "pi")
	script := `#!/bin/sh
printf '%s|%s\n' "$PI_CODING_AGENT_DIR" "$*" >> "$PIA_TEST_LOG"
if [ "$*" = "update" ]; then
  printf '%s\n' '{"packages":["npm:after-failure"]}' > "$PIA_AGENT_DIR/profiles/quick/profile.json"
  exit 0
fi
exit 9
`
	writeExecutable(t, piPath, script)
	cmd := piaCommand(t, agentDir, "update")
	cmd.Env = append(cmd.Env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PIA_TEST_LOG="+logPath,
	)

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("pia update accepted failed extension update:\n%s", out)
	}

	logData, err := os.ReadFile(logPath) //nolint:gosec // test path is under t.TempDir
	if err != nil {
		t.Fatalf("reading command log: %v", err)
	}

	want := agentDir + "|update\n" + agentDir + "|update --extensions\n"
	if string(logData) != want {
		t.Fatalf("update continued after failure:\n%s\nwant:\n%s", logData, want)
	}

	settings, err := os.ReadFile(filepath.Join(filepath.Dir(agentDir), "agent-quick", "settings.json"))
	if err != nil {
		t.Fatalf("reading generated settings: %v", err)
	}

	if strings.Contains(string(settings), "npm:after-failure") {
		t.Fatalf("post-update sync ran after failure:\n%s", settings)
	}
}

func TestUpdateRejectsInvalidOptions(t *testing.T) {
	for _, args := range [][]string{
		{"--pi-version"},
		{"--pi-version", "latest"},
		{"--extensions-only", "--pi-version", "1.0.0"},
		{"--unknown"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := piaCommand(t, t.TempDir(), append([]string{"update"}, args...)...)

			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("pia update accepted invalid options %v:\n%s", args, out)
			}
		})
	}
}

func TestDoctorReportsPiVersionAndChecksOfflineProfiles(t *testing.T) {
	agentDir := newSyncTestAgentDir(t, map[string]Profile{
		"zeta":  {},
		"alpha": {},
	})
	if err := syncProfiles(agentDir, nil); err != nil {
		t.Fatalf("syncing profiles: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "commands.log")
	binDir := t.TempDir()
	piPath := filepath.Join(binDir, "pi")
	script := `#!/bin/sh
if [ "$*" = "--version" ]; then
  printf '%s\n' '1.0.0'
  exit 0
fi
printf '%s|%s|%s\n' "$PI_CODING_AGENT_DIR" "$PI_OFFLINE" "$*" >> "$PIA_TEST_LOG"
`
	writeExecutable(t, piPath, script)

	cmd := piaCommand(t, agentDir, "doctor")
	cmd.Env = append(cmd.Env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PIA_TEST_LOG="+logPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pia doctor: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "Pi version: 1.0.0") {
		t.Fatalf("doctor did not report Pi version:\n%s", out)
	}

	logData, err := os.ReadFile(logPath) //nolint:gosec // test path is under t.TempDir
	if err != nil {
		t.Fatalf("reading command log: %v", err)
	}

	parentDir := filepath.Dir(agentDir)

	want := strings.Join([]string{
		agentDir + "|1|list --no-approve",
		filepath.Join(parentDir, "agent-alpha") + "|1|list --no-approve",
		filepath.Join(parentDir, "agent-zeta") + "|1|list --no-approve",
	}, "\n") + "\n"
	if string(logData) != want {
		t.Fatalf("command log:\n%s\nwant:\n%s", logData, want)
	}
}

func TestDoctorRejectsInvalidProfile(t *testing.T) {
	agentDir := newSyncTestAgentDir(t, map[string]Profile{"quick": {}})
	if err := syncProfiles(agentDir, nil); err != nil {
		t.Fatalf("syncing profiles: %v", err)
	}

	brokenDir := filepath.Join(agentDir, "profiles", "broken")
	if err := os.MkdirAll(brokenDir, 0o750); err != nil {
		t.Fatalf("creating broken profile: %v", err)
	}

	if err := os.WriteFile(filepath.Join(brokenDir, "profile.json"), []byte(`{`), 0o600); err != nil {
		t.Fatalf("writing broken profile: %v", err)
	}

	binDir := t.TempDir()
	piPath := filepath.Join(binDir, "pi")
	script := "#!/bin/sh\n[ \"$1\" = \"--version\" ] && echo 1.0.0\nexit 0\n"
	writeExecutable(t, piPath, script)
	cmd := piaCommand(t, agentDir, "doctor")
	cmd.Env = append(cmd.Env, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("doctor accepted invalid profile:\n%s", out)
	}

	if !strings.Contains(string(out), `invalid profile "broken"`) {
		t.Fatalf("doctor did not identify invalid profile:\n%s", out)
	}
}

func TestDoctorRejectsGeneratedSettingsDrift(t *testing.T) {
	agentDir := newSyncTestAgentDir(t, map[string]Profile{
		"quick": {Packages: []string{"npm:current"}},
	})
	if err := syncProfiles(agentDir, nil); err != nil {
		t.Fatalf("syncing profiles: %v", err)
	}

	settingsPath := filepath.Join(filepath.Dir(agentDir), "agent-quick", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{"packages":["npm:stale"]}`), 0o600); err != nil {
		t.Fatalf("writing stale settings: %v", err)
	}

	out, err := doctorCommand(t, agentDir, "")
	if err == nil {
		t.Fatalf("doctor accepted settings drift:\n%s", out)
	}

	if !strings.Contains(out, "settings drift") {
		t.Fatalf("doctor did not identify settings drift:\n%s", out)
	}
}

func TestDoctorRejectsManagedLinkDrift(t *testing.T) {
	for _, resource := range []string{"mcp.json", "web-search.json"} {
		t.Run(resource, func(t *testing.T) {
			agentDir := newSyncTestAgentDir(t, map[string]Profile{"quick": {}})

			profileDir := filepath.Join(agentDir, "profiles", "quick")
			if err := os.WriteFile(filepath.Join(profileDir, "mcp.json"), []byte(`{}`), 0o600); err != nil {
				t.Fatalf("writing MCP config: %v", err)
			}

			if err := os.WriteFile(filepath.Join(agentDir, "web-search.json"), []byte(`{}`), 0o600); err != nil {
				t.Fatalf("writing web config: %v", err)
			}

			if err := syncProfiles(agentDir, nil); err != nil {
				t.Fatalf("syncing profiles: %v", err)
			}

			link := filepath.Join(filepath.Dir(agentDir), "agent-quick", resource)
			if err := os.Remove(link); err != nil {
				t.Fatalf("removing managed link: %v", err)
			}

			if err := os.Symlink(filepath.Join(agentDir, "foreign.json"), link); err != nil {
				t.Fatalf("writing foreign link: %v", err)
			}

			out, err := doctorCommand(t, agentDir, "")
			if err == nil {
				t.Fatalf("doctor accepted %s link drift:\n%s", resource, out)
			}

			if !strings.Contains(out, resource+" link drift") {
				t.Fatalf("doctor did not identify %s link drift:\n%s", resource, out)
			}
		})
	}
}

func TestDoctorRejectsMissingProfileAndPiWarnings(t *testing.T) {
	t.Run("missing generated profile", func(t *testing.T) {
		agentDir := newSyncTestAgentDir(t, map[string]Profile{"quick": {}})

		out, err := doctorCommand(t, agentDir, "")
		if err == nil {
			t.Fatalf("doctor accepted missing generated profile:\n%s", out)
		}

		if !strings.Contains(out, "not synced") {
			t.Fatalf("doctor did not identify missing generated profile:\n%s", out)
		}
	})

	t.Run("offline package warning", func(t *testing.T) {
		agentDir := newSyncTestAgentDir(t, map[string]Profile{"quick": {}})
		if err := syncProfiles(agentDir, nil); err != nil {
			t.Fatalf("syncing profiles: %v", err)
		}

		out, err := doctorCommand(t, agentDir, "Warning: incompatible package")
		if err == nil {
			t.Fatalf("doctor accepted package warning:\n%s", out)
		}

		if !strings.Contains(out, "pi list warnings") {
			t.Fatalf("doctor did not identify package warning:\n%s", out)
		}
	})
}

func TestDeepMerge(t *testing.T) {
	t.Run("scalar override", func(t *testing.T) {
		base := map[string]any{"key": "old"}
		overlay := map[string]any{"key": "new"}

		result := deepMerge(base, overlay)
		if result["key"] != "new" {
			t.Errorf("got %v", result["key"])
		}
	})

	t.Run("nested merge", func(t *testing.T) {
		base := map[string]any{"outer": map[string]any{"a": 1, "b": 2}}
		overlay := map[string]any{"outer": map[string]any{"b": 99}}
		result := deepMerge(base, overlay)

		outer, ok := result["outer"].(map[string]any)
		if !ok {
			t.Fatalf("outer is not map[string]any")
		}

		if outer["a"] != 1 {
			t.Errorf("outer.a: got %v", outer["a"])
		}

		if outer["b"] != 99 {
			t.Errorf("outer.b: got %v", outer["b"])
		}
	})

	t.Run("array replaces", func(t *testing.T) {
		base := map[string]any{"arr": []any{"a", "b"}}
		overlay := map[string]any{"arr": []any{"x"}}
		result := deepMerge(base, overlay)

		arr, ok := result["arr"].([]any)
		if !ok {
			t.Fatalf("arr is not []any")
		}

		if len(arr) != 1 || arr[0] != "x" {
			t.Errorf("got %v", arr)
		}
	})

	t.Run("nil overlay", func(t *testing.T) {
		base := map[string]any{"key": "val"}

		result := deepMerge(base, nil)
		if result["key"] != "val" {
			t.Errorf("got %v", result["key"])
		}
	})

	t.Run("new keys added", func(t *testing.T) {
		base := map[string]any{"a": 1}
		overlay := map[string]any{"b": 2}

		result := deepMerge(base, overlay)
		if result["a"] != 1 || result["b"] != 2 {
			t.Errorf("got %v", result)
		}
	})
}

func TestSyncProfilesSelection(t *testing.T) {
	tests := []struct {
		name       string
		profiles   map[string]Profile
		selected   []string
		wantSynced []string
		wantAbsent []string
		wantError  string
	}{
		{
			name:       "no selection syncs all",
			profiles:   map[string]Profile{"quick": {}, "research": {}},
			wantSynced: []string{"quick", "research"},
		},
		{
			name:       "one selection",
			profiles:   map[string]Profile{"quick": {}, "research": {}},
			selected:   []string{"quick"},
			wantSynced: []string{"quick"},
			wantAbsent: []string{"research"},
		},
		{
			name:       "multiple selections",
			profiles:   map[string]Profile{"personal": {}, "quick": {}, "research": {}},
			selected:   []string{"quick", "research"},
			wantSynced: []string{"quick", "research"},
			wantAbsent: []string{"personal"},
		},
		{
			name:       "unknown selection",
			profiles:   map[string]Profile{"quick": {}},
			selected:   []string{"quick", "missing"},
			wantAbsent: []string{"quick"},
			wantError:  `profile "missing" not found`,
		},
		{
			name:      "unknown selection without profiles",
			selected:  []string{"missing"},
			wantError: `profile "missing" not found`,
		},
		{
			name: "selected profile fails",
			profiles: map[string]Profile{
				"broken": {SharedExtensions: []string{"missing"}},
				"quick":  {},
			},
			selected:   []string{"broken", "quick"},
			wantAbsent: []string{"quick"},
			wantError:  `syncing "broken": syncing extensions: shared extension "missing" not found at `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentDir := newSyncTestAgentDir(t, tt.profiles)
			err := syncProfiles(agentDir, tt.selected)

			if tt.wantError == "" && err != nil {
				t.Fatalf("syncProfiles: %v", err)
			}

			if tt.wantError != "" && (err == nil || !strings.HasPrefix(err.Error(), tt.wantError)) {
				t.Fatalf("error = %v, want prefix %q", err, tt.wantError)
			}

			for _, name := range tt.wantSynced {
				settingsPath := filepath.Join(filepath.Dir(agentDir), "agent-"+name, "settings.json")
				if _, statErr := os.Stat(settingsPath); statErr != nil {
					t.Errorf("profile %q was not synced: %v", name, statErr)
				}
			}

			for _, name := range tt.wantAbsent {
				profileDir := filepath.Join(filepath.Dir(agentDir), "agent-"+name)
				if _, statErr := os.Stat(profileDir); !os.IsNotExist(statErr) {
					t.Errorf("profile %q was unexpectedly synced: %v", name, statErr)
				}
			}
		})
	}
}

func TestSyncAllProfilesUsesStableOrder(t *testing.T) {
	agentDir := newSyncTestAgentDir(t, map[string]Profile{
		"zeta":   {},
		"alpha":  {},
		"middle": {},
	})
	for range 20 {
		out, err := piaCommand(t, agentDir, "sync").CombinedOutput()
		if err != nil {
			t.Fatalf("pia sync: %v\n%s", err, out)
		}

		text := string(out)
		alpha := strings.Index(text, "agent-alpha")
		middle := strings.Index(text, "agent-middle")

		zeta := strings.Index(text, "agent-zeta")
		if alpha < 0 || middle < alpha || zeta < middle {
			t.Fatalf("profiles were not synced alphabetically:\n%s", out)
		}
	}
}

func TestSyncCommandSelectionAndErrors(t *testing.T) {
	tests := []struct {
		name      string
		selected  string
		wantError string
	}{
		{name: "selected profile", selected: "quick"},
		{name: "unknown profile", selected: "missing", wantError: "error: profile \"missing\" not found\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentDir := newSyncTestAgentDir(t, map[string]Profile{"quick": {}, "research": {}})
			cmd := piaCommand(t, agentDir, "sync", tt.selected)

			out, err := cmd.CombinedOutput()
			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("pia sync succeeded, want error\n%s", out)
				}

				if string(out) != tt.wantError {
					t.Fatalf("output = %q, want %q", out, tt.wantError)
				}

				return
			}

			if err != nil {
				t.Fatalf("pia sync: %v\n%s", err, out)
			}

			if _, statErr := os.Stat(filepath.Join(filepath.Dir(agentDir), "agent-quick", "settings.json")); statErr != nil {
				t.Fatalf("quick profile was not synced: %v", statErr)
			}

			if _, statErr := os.Stat(filepath.Join(filepath.Dir(agentDir), "agent-research")); !os.IsNotExist(statErr) {
				t.Fatalf("unselected research profile was synced: %v", statErr)
			}
		})
	}
}

func TestPIAHelpDescribesSelectiveSync(t *testing.T) {
	cmd := piaCommand(t, t.TempDir(), "help")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pia help: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "pia sync [profile...]") {
		t.Fatalf("help does not describe selective sync:\n%s", out)
	}
}

func TestPIACommandHelper(_ *testing.T) {
	if os.Getenv("PIA_COMMAND_HELPER") != "1" {
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
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing executable: %v", err)
	}

	if err := os.Chmod(path, 0o700); err != nil { //nolint:gosec // test executable is private to t.TempDir
		t.Fatalf("making executable: %v", err)
	}
}

func doctorCommand(t *testing.T, agentDir, listOutput string) (string, error) {
	t.Helper()
	binDir := t.TempDir()
	piPath := filepath.Join(binDir, "pi")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
  printf '%s\n' '1.0.0'
  exit 0
fi
printf '%s' "$PIA_TEST_LIST_OUTPUT"
exit 0
`
	writeExecutable(t, piPath, script)
	cmd := piaCommand(t, agentDir, "doctor")
	cmd.Env = append(cmd.Env,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"PIA_TEST_LIST_OUTPUT="+listOutput,
	)
	out, err := cmd.CombinedOutput()

	return string(out), err
}

func piaCommand(t *testing.T, agentDir string, args ...string) *exec.Cmd {
	t.Helper()

	cmdArgs := append([]string{"-test.run=^TestPIACommandHelper$", "--", "pia"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...) //nolint:gosec // subprocess reruns the current test binary with fixed helper arguments

	cmd.Env = append(os.Environ(), "PIA_COMMAND_HELPER=1", "PIA_AGENT_DIR="+agentDir)

	return cmd
}

func newSyncTestAgentDir(t *testing.T, profiles map[string]Profile) string {
	t.Helper()

	agentDir := filepath.Join(t.TempDir(), "agent")
	if err := os.MkdirAll(agentDir, 0o750); err != nil {
		t.Fatalf("creating agent dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{}`), 0o644); err != nil { //nolint:gosec // test setup
		t.Fatalf("writing settings: %v", err)
	}

	for name, profile := range profiles {
		profileDir := filepath.Join(agentDir, "profiles", name)
		if err := os.MkdirAll(profileDir, 0o750); err != nil {
			t.Fatalf("creating profile dir: %v", err)
		}

		data, err := json.Marshal(profile)
		if err != nil {
			t.Fatalf("marshaling profile: %v", err)
		}

		if err := os.WriteFile(filepath.Join(profileDir, "profile.json"), data, 0o644); err != nil { //nolint:gosec // test setup
			t.Fatalf("writing profile: %v", err)
		}
	}

	return agentDir
}

func TestSyncProfile(t *testing.T) {
	// Setup fake agent dir
	agentDir := t.TempDir()

	// Base settings
	baseSettings := map[string]any{
		"defaultModel":    "opus",
		"defaultProvider": "hai-proxy",
		"packages":        []any{"npm:pkg-a", "npm:pkg-b"},
		"compaction":      map[string]any{"keepRecentTokens": float64(20000)},
	}
	data, _ := json.MarshalIndent(baseSettings, "", "  ")
	os.WriteFile(filepath.Join(agentDir, "settings.json"), data, 0o644) //nolint:gosec // test setup

	// Extensions
	os.MkdirAll(filepath.Join(agentDir, "extensions", "auto-retry"), 0o755) //nolint:gosec // test setup
	os.MkdirAll(filepath.Join(agentDir, "extensions", "ask-user"), 0o755)   //nolint:gosec // test setup

	// Themes dir
	os.MkdirAll(filepath.Join(agentDir, "themes"), 0o755) //nolint:gosec // test setup

	// Profile
	profileDir := filepath.Join(agentDir, "profiles", "quick")
	os.MkdirAll(profileDir, 0o755) //nolint:gosec // test setup

	profile := Profile{
		Description:      "test",
		SharedExtensions: []string{"auto-retry", "ask-user"},
		Packages:         []string{"npm:new-pkg"},
		SettingsOverrides: map[string]any{
			"compaction": map[string]any{"keepRecentTokens": float64(12000)},
		},
	}
	profileData, _ := json.Marshal(profile)
	os.WriteFile(filepath.Join(profileDir, "profile.json"), profileData, 0o644) //nolint:gosec // test setup

	// Run sync
	err := syncProfile(agentDir, "quick", profile)
	if err != nil {
		t.Fatalf("syncProfile failed: %v", err)
	}

	// Verify output dir exists
	outDir := filepath.Join(filepath.Dir(agentDir), "agent-quick")
	if _, statErr := os.Stat(outDir); os.IsNotExist(statErr) {
		t.Fatal("output dir not created")
	}

	// Verify settings.json
	settingsData, err := os.ReadFile(filepath.Join(outDir, "settings.json")) //nolint:gosec // path constructed from t.TempDir()
	if err != nil {
		t.Fatalf("reading merged settings: %v", err)
	}

	var merged map[string]any
	json.Unmarshal(settingsData, &merged) //nolint:gosec // test setup

	// Check packages replaced
	pkgs, ok := merged["packages"].([]any)
	if !ok {
		t.Fatalf("packages is not []any")
	}

	if len(pkgs) != 1 || pkgs[0] != "npm:new-pkg" {
		t.Errorf("packages not replaced: %v", pkgs)
	}

	// Check deep merge
	compaction, ok := merged["compaction"].(map[string]any)
	if !ok {
		t.Fatalf("compaction is not map[string]any")
	}

	if compaction["keepRecentTokens"] != float64(12000) {
		t.Errorf("compaction override failed: %v", compaction)
	}

	// Check extensions symlinked
	extDir := filepath.Join(outDir, "extensions")
	for _, ext := range []string{"auto-retry", "ask-user"} {
		link := filepath.Join(extDir, ext)

		target, err := os.Readlink(link)
		if err != nil {
			t.Errorf("extension %q not symlinked: %v", ext, err)
			continue
		}

		expectedTarget := filepath.Join(agentDir, "extensions", ext)
		if target != expectedTarget {
			t.Errorf("extension %q: got target %q, want %q", ext, target, expectedTarget)
		}
	}

	// Check themes symlinked
	themesLink := filepath.Join(outDir, "themes")
	if _, err := os.Readlink(themesLink); err != nil {
		t.Errorf("themes not symlinked: %v", err)
	}

	// Check sessions dir created
	if _, err := os.Stat(filepath.Join(outDir, "sessions")); os.IsNotExist(err) {
		t.Error("sessions dir not created")
	}
}

func TestSyncIdempotent(t *testing.T) {
	agentDir := t.TempDir()
	os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{}`), 0o644) //nolint:gosec // test setup
	os.MkdirAll(filepath.Join(agentDir, "extensions", "ext-a"), 0o755)          //nolint:gosec // test setup
	profileDir := filepath.Join(agentDir, "profiles", "test")
	os.MkdirAll(profileDir, 0o755) //nolint:gosec // test setup

	profile := Profile{SharedExtensions: []string{"ext-a"}}
	profileData, _ := json.Marshal(profile)
	os.WriteFile(filepath.Join(profileDir, "profile.json"), profileData, 0o644) //nolint:gosec // test setup

	// Sync twice — should not error
	if err := syncProfile(agentDir, "test", profile); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	if err := syncProfile(agentDir, "test", profile); err != nil {
		t.Fatalf("second sync: %v", err)
	}
}

func TestSyncWebSearchUsesProfileOverrideOrBase(t *testing.T) {
	agentDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatalf("writing settings: %v", err)
	}

	baseConfig := filepath.Join(agentDir, "web-search.json")
	if err := os.WriteFile(baseConfig, []byte(`{"toolActivation":"dynamic"}`), 0o600); err != nil {
		t.Fatalf("writing base web config: %v", err)
	}

	for _, name := range []string{"quick", "personal"} {
		profileDir := filepath.Join(agentDir, "profiles", name)
		if err := os.MkdirAll(profileDir, 0o750); err != nil {
			t.Fatalf("creating profile %s: %v", name, err)
		}
	}

	personalConfig := filepath.Join(agentDir, "profiles", "personal", "web-search.json")
	if err := os.WriteFile(personalConfig, []byte(`{"provider":"tavily","toolActivation":"dynamic"}`), 0o600); err != nil {
		t.Fatalf("writing personal web config: %v", err)
	}

	for _, name := range []string{"quick", "personal"} {
		if err := syncProfile(agentDir, name, Profile{}); err != nil {
			t.Fatalf("syncing %s: %v", name, err)
		}
	}

	quickTarget, err := os.Readlink(filepath.Join(filepath.Dir(agentDir), "agent-quick", "web-search.json"))
	if err != nil {
		t.Fatalf("quick web-search.json is not a symlink: %v", err)
	}

	if quickTarget != baseConfig {
		t.Errorf("quick target = %q, want %q", quickTarget, baseConfig)
	}

	personalTarget, err := os.Readlink(filepath.Join(filepath.Dir(agentDir), "agent-personal", "web-search.json"))
	if err != nil {
		t.Fatalf("personal web-search.json is not a symlink: %v", err)
	}

	if personalTarget != personalConfig {
		t.Errorf("personal target = %q, want %q", personalTarget, personalConfig)
	}
}

func TestSyncMcpSymlinked(t *testing.T) {
	agentDir := t.TempDir()
	os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{}`), 0o644) //nolint:gosec // test setup
	profileDir := filepath.Join(agentDir, "profiles", "research")
	os.MkdirAll(profileDir, 0o755) //nolint:gosec // test setup

	mcpSrc := filepath.Join(profileDir, "mcp.json")
	os.WriteFile(mcpSrc, []byte(`{"mcpServers":{"zra-mcp":{}}}`), 0o644) //nolint:gosec // test setup

	profile := Profile{}
	profileData, _ := json.Marshal(profile)
	os.WriteFile(filepath.Join(profileDir, "profile.json"), profileData, 0o644) //nolint:gosec // test setup

	if err := syncProfile(agentDir, "research", profile); err != nil {
		t.Fatalf("syncProfile: %v", err)
	}

	outDir := filepath.Join(filepath.Dir(agentDir), "agent-research")
	mcpDst := filepath.Join(outDir, "mcp.json")

	target, err := os.Readlink(mcpDst)
	if err != nil {
		t.Fatalf("mcp.json not symlinked: %v", err)
	}

	if target != mcpSrc {
		t.Errorf("mcp.json target %q, want %q", target, mcpSrc)
	}
}

func TestSyncMcpMissingLeavesTargetUntouched(t *testing.T) {
	agentDir := t.TempDir()
	os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{}`), 0o644) //nolint:gosec // test setup
	profileDir := filepath.Join(agentDir, "profiles", "quick")
	os.MkdirAll(profileDir, 0o755) //nolint:gosec // test setup

	profile := Profile{}
	profileData, _ := json.Marshal(profile)
	os.WriteFile(filepath.Join(profileDir, "profile.json"), profileData, 0o644) //nolint:gosec // test setup

	// Pre-existing hand-written mcp.json in the output dir; the sync must not
	// clobber it when the profile has no mcp.json of its own.
	outDir := filepath.Join(filepath.Dir(agentDir), "agent-quick")
	os.MkdirAll(outDir, 0o750) //nolint:gosec // test setup

	preExisting := []byte(`{"mcpServers":{"user-added":{}}}`)
	os.WriteFile(filepath.Join(outDir, "mcp.json"), preExisting, 0o600) //nolint:gosec // test setup

	if err := syncProfile(agentDir, "quick", profile); err != nil {
		t.Fatalf("syncProfile: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(outDir, "mcp.json")) //nolint:gosec // path from t.TempDir()
	if err != nil {
		t.Fatalf("reading mcp.json: %v", err)
	}

	if string(got) != string(preExisting) {
		t.Errorf("mcp.json was rewritten: got %q, want %q", got, preExisting)
	}
}

func TestSyncMcpMissingRemovesOnlyManagedSymlink(t *testing.T) {
	for _, tt := range []struct {
		name       string
		target     func(agentDir string) string
		wantExists bool
	}{
		{
			name: "managed",
			target: func(agentDir string) string {
				return filepath.Join(agentDir, "profiles", "quick", "mcp.json")
			},
			wantExists: false,
		},
		{
			name: "foreign",
			target: func(agentDir string) string {
				return filepath.Join(agentDir, "user-mcp.json")
			},
			wantExists: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			agentDir := t.TempDir()
			if err := os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{}`), 0o600); err != nil {
				t.Fatalf("writing settings: %v", err)
			}

			profileDir := filepath.Join(agentDir, "profiles", "quick")
			if err := os.MkdirAll(profileDir, 0o750); err != nil {
				t.Fatalf("creating profile: %v", err)
			}

			outDir := filepath.Join(filepath.Dir(agentDir), "agent-quick")
			if err := os.MkdirAll(outDir, 0o750); err != nil {
				t.Fatalf("creating output: %v", err)
			}

			link := filepath.Join(outDir, "mcp.json")
			if err := os.Symlink(tt.target(agentDir), link); err != nil {
				t.Fatalf("creating MCP symlink: %v", err)
			}

			if err := syncProfile(agentDir, "quick", Profile{}); err != nil {
				t.Fatalf("syncing profile: %v", err)
			}

			_, err := os.Lstat(link)
			if tt.wantExists && err != nil {
				t.Fatalf("foreign MCP symlink was removed: %v", err)
			}

			if !tt.wantExists && !os.IsNotExist(err) {
				t.Fatalf("managed MCP symlink remains: %v", err)
			}
		})
	}
}

func TestSyncMcpReplacesRegularFile(t *testing.T) {
	// Migration case: agent-personal/mcp.json was a regular file (hand-copied
	// content), and pia now ships a profile mcp.json. The sync must replace
	// the regular file with a symlink pointing at the dotfiles source.
	agentDir := t.TempDir()
	os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{}`), 0o644) //nolint:gosec // test setup
	profileDir := filepath.Join(agentDir, "profiles", "personal")
	os.MkdirAll(profileDir, 0o755) //nolint:gosec // test setup

	mcpSrc := filepath.Join(profileDir, "mcp.json")
	os.WriteFile(mcpSrc, []byte(`{"mcpServers":{"ticktick":{}}}`), 0o644) //nolint:gosec // test setup

	profile := Profile{}
	profileData, _ := json.Marshal(profile)
	os.WriteFile(filepath.Join(profileDir, "profile.json"), profileData, 0o644) //nolint:gosec // test setup

	outDir := filepath.Join(filepath.Dir(agentDir), "agent-personal")
	os.MkdirAll(outDir, 0o750)                                                       //nolint:gosec // test setup
	os.WriteFile(filepath.Join(outDir, "mcp.json"), []byte(`{"stale":true}`), 0o600) //nolint:gosec // test setup

	if err := syncProfile(agentDir, "personal", profile); err != nil {
		t.Fatalf("syncProfile: %v", err)
	}

	target, err := os.Readlink(filepath.Join(outDir, "mcp.json"))
	if err != nil {
		t.Fatalf("mcp.json is not a symlink after sync: %v", err)
	}

	if target != mcpSrc {
		t.Errorf("mcp.json target %q, want %q", target, mcpSrc)
	}
}
