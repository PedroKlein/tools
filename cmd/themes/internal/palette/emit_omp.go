package palette

import (
	"encoding/json"
	"fmt"
	"io"
)

// v4 omp emitter — reads theme.json, writes omp.json.
//
// OMP custom themes live under ~/.omp/agent/themes/*.json. The reload hook
// writes this payload to current.json so OMP's built-in file watcher retints
// running sessions without restarting.
type ompEmitter struct{}

func (ompEmitter) App() string      { return "omp" }
func (ompEmitter) Filename() string { return "omp.json" }

func (ompEmitter) Emit(t *Theme, w io.Writer) error {
	return emitOMPJSON(t, w)
}

type ompThemeV1 struct {
	Schema  string            `json:"$schema"`
	Name    string            `json:"name"`
	Vars    map[string]string `json:"vars"`
	Colors  map[string]string `json:"colors"`
	Export  map[string]string `json:"export,omitempty"`
	Symbols ompSymbols        `json:"symbols,omitempty"`
}

type ompSymbols struct {
	Preset string `json:"preset,omitempty"`
}

func emitOMPJSON(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	a := t.Palette.Ansi

	vars := map[string]string{
		"bg":        s.Bg,
		"mantle":    s.BgAlt,
		"surface0":  s.SelectionBg,
		"surface1":  s.Border,
		"surface2":  s.FgDim,
		"accent":    s.Accent,
		"bright":    s.Accent2,
		"muted":     s.Muted,
		"fg":        s.Fg,
		"white":     a[15],
		"red":       s.Error,
		"redBright": a[9],
		"yellow":    s.Warning,
		"green":     s.Ok,
		"cyan":      a[6],
		"cyanMuted": a[14],
		"blue":      s.Info,
		"purple":    s.Syntax.Keyword,
		"pink":      a[13],
		"jade":      s.Ok,
		"teal":      s.Info,
	}

	colors := map[string]string{
		"accent":              "accent",
		"border":              "teal",
		"borderAccent":        "bright",
		"borderMuted":         "surface1",
		"success":             "green",
		"error":               "red",
		"warning":             "yellow",
		"muted":               "muted",
		"dim":                 s.FgDim,
		"text":                "fg",
		"thinkingText":        "muted",
		"selectedBg":          "surface1",
		"userMessageBg":       "surface0",
		"userMessageText":     "fg",
		"customMessageBg":     "mantle",
		"customMessageText":   "fg",
		"customMessageLabel":  "purple",
		"toolPendingBg":       "mantle",
		"toolSuccessBg":       "surface0",
		"toolErrorBg":         "surface0",
		"toolTitle":           "fg",
		"toolOutput":          "muted",
		"mdHeading":           "yellow",
		"mdLink":              "cyan",
		"mdLinkUrl":           "muted",
		"mdCode":              "bright",
		"mdCodeBlock":         "fg",
		"mdCodeBlockBorder":   "muted",
		"mdQuote":             "muted",
		"mdQuoteBorder":       "muted",
		"mdHr":                "muted",
		"mdListBullet":        "accent",
		"toolDiffAdded":       s.Git.Added,
		"toolDiffRemoved":     s.Git.Removed,
		"toolDiffContext":     "muted",
		"syntaxComment":       s.Syntax.Comment,
		"syntaxKeyword":       s.Syntax.Keyword,
		"syntaxFunction":      s.Syntax.Function,
		"syntaxVariable":      "cyanMuted",
		"syntaxString":        s.Syntax.String,
		"syntaxNumber":        s.Syntax.Number,
		"syntaxType":          s.Syntax.Type,
		"syntaxOperator":      s.Syntax.Operator,
		"syntaxPunctuation":   "muted",
		"thinkingOff":         "surface1",
		"thinkingMinimal":     "muted",
		"thinkingLow":         "teal",
		"thinkingMedium":      "accent",
		"thinkingHigh":        "bright",
		"thinkingXhigh":       "cyan",
		"bashMode":            "yellow",
		"pythonMode":          "blue",
		"statusLineBg":        "mantle",
		"statusLineSep":       "surface2",
		"statusLineModel":     "bright",
		"statusLinePath":      "fg",
		"statusLineGitClean":  "green",
		"statusLineGitDirty":  "yellow",
		"statusLineContext":   "cyanMuted",
		"statusLineSpend":     "yellow",
		"statusLineStaged":    "bright",
		"statusLineDirty":     "yellow",
		"statusLineUntracked": "redBright",
		"statusLineOutput":    "fg",
		"statusLineCost":      "yellow",
		"statusLineSubagents": "purple",
	}

	omp := ompThemeV1{
		Schema: "https://raw.githubusercontent.com/can1357/oh-my-pi/main/packages/coding-agent/theme-schema.json",
		Name:   t.Name,
		Vars:   vars,
		Colors: colors,
		Export: map[string]string{
			"pageBg": "bg",
			"cardBg": "mantle",
			"infoBg": "surface0",
		},
		Symbols: ompSymbols{Preset: "nerd"},
	}

	if h := t.Hint("omp"); h != nil {
		if extraVars, ok := h["vars"].(map[string]any); ok {
			for k, v := range extraVars {
				if s, ok := v.(string); ok {
					omp.Vars[k] = s
				}
			}
		}
		if extraColors, ok := h["colors"].(map[string]any); ok {
			for k, v := range extraColors {
				if s, ok := v.(string); ok {
					omp.Colors[k] = s
				}
			}
		}
		if preset, ok := h["symbolPreset"].(string); ok && preset != "" {
			omp.Symbols.Preset = preset
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	if err := enc.Encode(omp); err != nil {
		return fmt.Errorf("emit omp.json: %w", err)
	}
	return nil
}
