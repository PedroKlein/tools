package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BaseSystemPrompt is always set on the pi daemon at startup.
// Profiles append per-request instructions via the user message.
const BaseSystemPrompt = `You are a fast, concise shell assistant. Rules:
- Never use markdown fences, backticks, or code blocks
- Never include preambles, disclaimers, or sign-offs
- If the answer is a shell command: output the raw command only
- If the answer is code: output the raw code only
- If the answer requires explanation: 1-3 sentences maximum
- If more depth is needed, suggest a URL for further reading
- Detect the most useful output format from the question`

// RunInstruction is prepended to the user message when --run is active.
const RunInstruction = "[Output ONLY a single executable shell command. No alternatives, no explanation.]"

// Profile defines a named prompt persona.
type Profile struct {
	Instruction string   `json:"instruction,omitempty"`
	Model       string   `json:"model,omitempty"`
	PiFlags     []string `json:"piFlags,omitempty"`
}

// Config defines the q tool configuration.
type Config struct {
	Model          string             `json:"model"`
	IdleTimeout    string             `json:"idleTimeout"`
	DefaultProfile string             `json:"defaultProfile"`
	AutoCopy       bool               `json:"autoCopy"`
	PiFlags        []string           `json:"piFlags"`
	Profiles       map[string]Profile `json:"profiles"`
}

// IdleTimeoutDuration parses the IdleTimeout string as a time.Duration.
func (c Config) IdleTimeoutDuration() time.Duration {
	d, err := time.ParseDuration(c.IdleTimeout)
	if err != nil {
		return 15 * time.Minute
	}

	return d
}

// DefaultConfig returns the compiled defaults that work zero-config.
func DefaultConfig() Config {
	return Config{
		Model:          "claude-haiku-4.5",
		IdleTimeout:    "15m",
		DefaultProfile: "auto",
		AutoCopy:       false,
		PiFlags:        []string{"--no-tools", "--no-skills", "--no-extensions", "--no-context-files", "--no-session", "--session-dir", "/tmp/q-sessions"},
		Profiles: map[string]Profile{
			"auto": {
				Model: "claude-haiku-4.5",
			},
			"cmd": {
				Instruction: "[Respond with a single shell command only. Nothing else.]",
				Model:       "claude-haiku-4.5",
			},
			"explain": {
				Instruction: "[Respond with a 1-3 sentence explanation. Never output bare commands.]",
				Model:       "claude-haiku-4.5",
			},
			"code": {
				Instruction: "[Respond with code only. No explanation, no language labels.]",
				Model:       "claude-haiku-4.5",
			},
			"think": {
				Instruction: "[Think step by step before answering.]",
				Model:       "claude-sonnet-4-20250514",
				PiFlags:     []string{"--thinking", "low"},
			},
		},
	}
}

// LoadConfig reads config from disk and merges over defaults.
// If the file doesn't exist, returns defaults.
// Uses Q_CONFIG_PATH env var if set, otherwise ~/.config/q/config.json.
func LoadConfig() (Config, error) {
	path := configPath()
	return loadConfigFrom(path)
}

func loadConfigFrom(path string) (Config, error) {
	cfg := DefaultConfig()

	//nolint:gosec // path comes from Q_CONFIG_PATH env var or default ~/.config/q/config.json
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}

		return cfg, fmt.Errorf("reading config: %w", err)
	}

	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return cfg, fmt.Errorf("parsing config %s: %w", path, err)
	}

	// Merge: file values override defaults where set
	if fileCfg.Model != "" {
		cfg.Model = fileCfg.Model
	}

	if fileCfg.IdleTimeout != "" {
		cfg.IdleTimeout = fileCfg.IdleTimeout
	}

	if fileCfg.DefaultProfile != "" {
		cfg.DefaultProfile = fileCfg.DefaultProfile
	}

	if fileCfg.AutoCopy {
		cfg.AutoCopy = true
	}

	if fileCfg.PiFlags != nil {
		cfg.PiFlags = fileCfg.PiFlags
	}

	if fileCfg.Profiles != nil {
		maps.Copy(cfg.Profiles, fileCfg.Profiles)
	}

	return cfg, nil
}

// ResolveProfile returns the profile for a given name.
// Priority: explicit name > config.DefaultProfile > "auto".
func ResolveProfile(cfg Config, name string) (Profile, error) {
	if name == "" {
		name = cfg.DefaultProfile
	}

	if name == "" {
		name = "auto"
	}

	p, ok := cfg.Profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("unknown profile: %q (available: %s)", name, profileNames(cfg))
	}
	// Inherit model from config if profile doesn't set one
	if p.Model == "" {
		p.Model = cfg.Model
	}

	return p, nil
}

// GetInstruction returns the per-request instruction for a profile.
func GetInstruction(cfg Config, profileName string) string {
	if profileName == "" {
		profileName = cfg.DefaultProfile
	}

	if profileName == "" {
		profileName = "auto"
	}

	if p, ok := cfg.Profiles[profileName]; ok {
		return p.Instruction
	}

	return ""
}

func profileNames(cfg Config) string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}

	return strings.Join(names, ", ")
}

func configPath() string {
	if p := os.Getenv("Q_CONFIG_PATH"); p != "" {
		return expandTilde(p)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".config", "q", "config.json")
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}

		return filepath.Join(home, path[2:])
	}

	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}

	return path
}
