#!/usr/bin/env bash
# sketchybar palette (derived; do not edit)
# Variable names match the ones sketchybarrc + plugins reference.

# --- bar surface ------------------------------------------------------------
export BAR_BG=0xd0111c18
export BAR_BORDER=0xff23372b

# --- foreground -------------------------------------------------------------
export FG=0xffc1c497
export FG_MUTED=0xff627a6c
export FG_BRIGHT=0xfff6f5dd

# --- accents ---------------------------------------------------------------
export ACCENT=0xff549e6a
export ACCENT_BRIGHT=0xff63b07a
export ACCENT_ON=0xff000000
export HIGHLIGHT=0xff549e6a

# --- semantic colors -------------------------------------------------------
export RED=0xffff5345
export YELLOW=0xff459451
export CYAN=0xff2dd5b7
export TEAL=0xff509475
export JADE=0xff459451
export MAGENTA=0xffd2689c

# --- surfaces ---------------------------------------------------------------
export SURFACE=0xff23372b
export SURFACE_LIGHT=0x4423372b

# --- status semantics (used by battery/charging plugins) --------------------
# FOCUSED_WORKSPACE_COLOR (not FOCUSED_WORKSPACE) — the aerospace event
# already sets FOCUSED_WORKSPACE=<id> on the subscriber, so naming a
# palette variable the same thing would clobber that env var and break
# the plugin's focus check.
export ICON=0xffc1c497
export CHARGING=0xff63b07a
export FOCUSED=0xff63b07a
export FOCUSED_WORKSPACE_COLOR=0xff549e6a
export NON_EMPTY=0xffc1c497
export BADGE=0xff459451
export INFO=0xff2dd5b7
export VOLUME=0xff2dd5b7
export PERCENTAGE=0xffc1c497
