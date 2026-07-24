# `themes`

Cross-app terminal theme switcher. One command swaps colors and wallpaper across 15 apps at once.

**Companion CLI to** [`pia`](../pia), [`repos`](../repos), [`q`](../q), [`todo`](../todo). Shares the same conventions:

- stdlib + charm (Bubble Tea) — no other deps
- `--json` on read commands (`list`, `current`, `doctor`)
- exit codes: 0=success, 1=error, 2=ambiguous, 3=not-found
- table-driven tests, white-box package

## Overview

The switcher is state-driven, not symlink-driven. `~/.config/themes/.state.json` is the source of truth; `~/.config/themes/.current` (symlink) is derived. Switching a theme is an atomic symlink swap (`renameat` on Unix) followed by parallel per-app reload hooks.

**Full design and per-app include recipes:** [`~/dotfiles/docs/themes.md`](https://github.com/PedroKlein/dotfiles/blob/main/docs/themes.md).

## Install

```sh
go install github.com/PedroKlein/tools/cmd/themes@latest
```

Wire the includes into your dotfiles per `docs/themes.md`, then run `setup_shared.sh` (or equivalent) to create the runtime symlinks. `themes doctor` will tell you what's not wired.

## Usage

```
theme                        Open interactive picker (default)
themes list [--json]          List installed themes
themes current [--json]       Print current theme and wallpaper
themes set <name>             Apply theme (headless)
themes back                   Revert to previous theme
themes apply                  Re-run hooks against current theme (no swap)
themes wallpaper              Interactive wallpaper picker
themes wallpaper set <path>   Set wallpaper directly
themes wallpaper random       Random wallpaper from current theme
themes doctor [--json]        Verify wiring across all apps
themes install <url>          Import an Omarchy-marketplace theme
themes derive <name>          Regenerate the 13 derived files for a theme
```

## Structure

```
cmd/themes/
├── main.go            dispatcher, flag parsing
├── paths.go           XDG paths, symlink names
├── exitcodes.go       0/1/2/3
├── json.go            --json helpers
├── state.go           .state.json read/write with fsync + rename
├── set.go             atomic symlink swap + hook dispatch
├── doctor.go          per-app wiring probes
├── wallpaper.go       wallpaper picker + setter
├── install.go         themes install / derive (calls .bin/theme-derive)
├── subcommands.go     list/current/set/back/apply/doctor/wallpaper CLI glue
├── tui.go             Bubbletea picker (P3.4)
└── theme_test.go      table-driven tests
```

## Related

- `../repos` — manage git repositories in the `~/Dev/host/owner/repo` layout
- `../pia` — Pi profile launcher and sync
- `../q` — persistent Pi daemon for shell LLM access
- `../todo` — task board with PR review integration
