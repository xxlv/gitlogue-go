# gitlogue-go

[English](README.md) | [简体中文](README.zh.md)

Cinematic Git replay in the terminal. Run `gitlogue` in a repo and watch a commit type itself at 60 FPS.

![gitlogue demo](docs/demo.gif)

Install onto `$PATH`:

```bash
go install github.com/xxlv/gitlogue-go/cmd/gitlogue@latest
# or locally
go install ./cmd/gitlogue
```

## Usage

```bash
# Replay the last commit in the current repo
gitlogue

# A specific commit
gitlogue 7a8b9c0

# First-parent walk from main to feature (oldest first)
gitlogue main..feature

# One file, 2× speed
gitlogue --file main.go --speed 2.0

# Debug: print the patch / Action stream, skip the TUI
gitlogue --inspect --script --seed 1
```

- `--repo` walks up to `.git`
- `--commits N` caps how many commits to play (single revision defaults to 1; `A..B` defaults to the whole range)
- `--file` filters by path, basename, or glob
- `--speed` initial multiplier (any value `> 0`); during playback `j` / `-` / `←` halve, `k` / `+` / `→` double, with no cap
- `--theme` syntax highlighting (default `dracula`)

## Keys

| Key | Action |
|---|---|
| `Space` | Pause / resume. If focus is on **another file**, replay that file from the start |
| `Enter` | Replay the focused (or playhead) file |
| `j` / `-` / `←` | Half speed |
| `k` / `+` / `→` | Double speed |
| `Tab` / `↑` / `↓` | Move focus in the file tree (pauses) |
| `PgUp` / `PgDn` | Scroll the editor. Mouse wheel over the file pane does the same |
| `Home` / `End` | Jump to the top / bottom of the open file |
| `n` / `p` | Next / previous changed file |
| `]` / `[` | Next / previous commit |
| `r` | Restart the current commit (whole track, not one file) |
| `q` / `Ctrl+C` | Quit |

Below the status bar is an **attribution strip**: the commit subject, then author / committer / co-authors / reviewers / Generated-by, and so on. Identities whose name or email looks like Copilot or Cursor are tagged `[agent]`, so a human can spot AI-written diffs. Trailers include `Co-authored-by`, `Reviewed-by`, `Signed-off-by`, `Generated-by`, and `Assisted-by`. `--inspect` dumps the same credits.
