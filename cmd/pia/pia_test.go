package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
