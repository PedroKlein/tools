package main

import (
	"encoding/json"
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
