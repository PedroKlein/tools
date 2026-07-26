# `themes`

macOS theme switcher for Ghostty, tmux, Neovim, Sketchybar, Aerospace, Starship, Pi, and related CLI tools.

**Companion CLI to** [`pia`](../pia), [`repos`](../repos), [`q`](../q), [`todo`](../todo). Shares the same conventions:

- stdlib + Charm (Bubble Tea) dependencies only
- `--json` on read commands (`list`, `current`)
- exit codes: 0=success, 1=error, 2=ambiguous, 3=not-found
- table-driven tests, white-box package

## Overview

Themes are source-driven. Each theme directory contains one `theme.json`; `themes derive` writes app-specific files under `<theme>/derived/`. Runtime state lives under `$XDG_STATE_HOME/themes/` (`~/.local/state/themes/` by default), where `current` points at the active theme root. Consumers read generated files through `~/.config/themes/.current/derived/<file>`.

Switching a theme derives missing/stale files, atomically swaps the XDG `current` symlink, updates `state.json`, and runs per-app reload hooks.

**Full design and per-app include recipes:** [`~/dotfiles/docs/themes.md`](https://github.com/PedroKlein/dotfiles/blob/main/docs/themes.md).

## Install

```sh
go install github.com/PedroKlein/tools/cmd/themes@latest
```

Wire the dotfiles with `setup_mac.sh`; it creates the XDG state directory, `.current` compatibility symlink, and per-app theme links.

## Usage

```
themes                        Open interactive picker (default)
themes list [--json]          List installed themes
themes current [--json]       Print current theme and wallpaper
themes set <name>             Apply the given theme (derive + hooks)
themes apply                  Re-run hooks against the active theme
themes derive <name>          Regenerate <theme>/derived/* from theme.json
themes validate <name|path>   Validate a theme.json against the v1 schema
themes wallpaper              Interactive wallpaper picker
themes wallpaper set <path>   Set wallpaper directly
themes wallpaper next         Cycle to next wallpaper for active theme
themes wallpaper random       Random wallpaper from current theme
themes import <url>           Points at Pi's /theme-import slash command
```

## Structure

```
cmd/themes/
├── main.go            dispatcher, flag parsing
├── paths.go           config paths, atomic file helper
├── set.go             derive + XDG symlink swap + hook dispatch
├── derive*.go         theme.json → derived/* pipeline
├── validate.go        JSON Schema validation
├── wallpaper*.go      wallpaper list/set/cycle flows
├── tui.go             Bubble Tea picker
├── internal/state/    XDG state.json + current symlink
├── internal/palette/  theme model, loader, emitters, baselines
└── internal/reload/   per-app reload hooks
```

## Related

- `../repos` — manage git repositories in the `~/Dev/host/owner/repo` layout
- `../pia` — Pi profile launcher and sync
- `../q` — persistent Pi daemon for shell LLM access
- `../todo` — task board with PR review integration
