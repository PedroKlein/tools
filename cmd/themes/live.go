package main

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// liveApplyMsg fires when a debounced live-apply is due.
type liveApplyMsg struct{ name string }

// debouncer is a per-picker single-slot debounce timer.
//
// Only the most recent scheduled apply fires; earlier ones are cancelled.
// Concurrency-safe under Bubbletea's single Update loop plus timer goroutines.
type debouncer struct {
	mu    sync.Mutex
	timer *time.Timer
	last  string
}

const liveApplyDelay = 150 * time.Millisecond

// schedule cancels any pending timer and starts a fresh one that emits a
// liveApplyMsg for `name` after 150ms of quiet.
//
// The returned tea.Cmd blocks until the timer either fires or is superseded.
// Callers rely on Bubbletea's Update loop to process the resulting message.
func (d *debouncer) schedule(name string) tea.Cmd {
	return func() tea.Msg {
		d.mu.Lock()
		if d.timer != nil {
			d.timer.Stop()
		}
		d.last = name
		// Channel that receives when the timer fires OR when a newer schedule
		// supersedes us.
		ch := make(chan liveApplyMsg, 1)
		d.timer = time.AfterFunc(liveApplyDelay, func() {
			d.mu.Lock()
			currentLast := d.last
			d.mu.Unlock()
			ch <- liveApplyMsg{name: currentLast}
		})
		d.mu.Unlock()
		return <-ch
	}
}

// maybeSchedule is called from the model on cursor moves. If live-apply
// is on and the highlighted theme differs from the on-disk active theme,
// schedule a debounced Set().
func (m *pickerModel) maybeSchedule() (tea.Model, tea.Cmd) {
	if !m.liveApply {
		return m, nil
	}
	name := m.themes[m.cursor].Name
	if m.pending == nil {
		m.pending = &debouncer{}
	}
	return m, m.pending.schedule(name)
}

// doLiveApply runs Set during picker scroll. Uses a lean skip list to keep
// scroll fast: the hooks in `liveSkipHooks` are heavy (app restarts,
// osascript UI restarts) and don't matter for a preview of the terminal +
// pi UI. On commit (Enter), Commit=true triggers every hook.
//
// Kept hooks (fire on scroll):
//
//	osc-broadcast, pi, ghostty, tmux, sketchybar, nvim, bat, delta,
//	fzf, zsh-highlight, wallpaper
//
// Skipped (commit-only):
//
//	opencode, btop, k9s, television, lazygit, gh-dash, macos-system
//
// User asked for preview=commit but hit UX friction from slow scroll; this
// is the negotiated compromise.
var liveSkipHooks = []string{
	"opencode", "btop", "k9s", "television", "lazygit", "gh-dash", "macos-system",
}

func (m *pickerModel) doLiveApply(name string) (tea.Model, tea.Cmd) {
	// Only apply if selection is still on this theme (avoids stale msgs
	// after fast scroll).
	if m.cursor < 0 || m.cursor >= len(m.themes) || m.themes[m.cursor].Name != name {
		return m, nil
	}
	_ = Set(name, SetOptions{
		SkipHooks: liveSkipHooks,
		Commit:    false,
	})
	// Rebuild the TUI's own styles so the picker follows the theme as we
	// scroll. Reads .current/palette.toml which was just updated by Set().
	m.reloadStyles()
	// Refresh Current markers.
	for i := range m.themes {
		m.themes[i].Current = m.themes[i].Name == name
	}
	return m, nil
}
