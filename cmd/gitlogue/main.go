// Command gitlogue is the entrypoint for gitlogue-go.
//
// Default: play the last commit of the current repo as a 60 FPS TUI.
// Pass a hash, ref, or A..B range as the first argument.
package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/xxlv/gitlogue-go/internal/animator"
	"github.com/xxlv/gitlogue-go/internal/gitengine"
	"github.com/xxlv/gitlogue-go/internal/highlighter"
	"github.com/xxlv/gitlogue-go/internal/scheduler"
	"github.com/xxlv/gitlogue-go/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gitlogue: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("gitlogue", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	repo := fs.String("repo", ".", "path to a Git repository (walks up to .git)")
	commit := fs.String("commit", "HEAD", "revision if no positional argument is given")
	commits := fs.Int("commits", -1, "max commits (-1 = 1 for a single rev, all for A..B)")
	file := fs.String("file", "", "only replay paths matching this name, basename, or glob")
	inspect := fs.Bool("inspect", false, "dump the commit patch instead of playing")
	script := fs.Bool("script", false, "dump the compiled Action stream instead of playing")
	seed := fs.Int64("seed", 0, "RNG seed for typing delays (0 = random)")
	_ = fs.Bool("play", true, "run the TUI (default; --inspect/--script dump instead)")
	speed := fs.Float64("speed", 1, "playback speed multiplier")
	fps := fs.Int("fps", scheduler.DefaultFPS, "frames per second")
	theme := fs.String("theme", highlighter.DefaultTheme, "Chroma colour theme (e.g. dracula, monokai, nord)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	spec := *commit
	if rest := fs.Args(); len(rest) > 0 {
		spec = rest[0]
	}

	_, _, dots := gitengine.SplitRev(spec)
	n := *commits
	if n < 0 {
		if dots == 0 {
			n = 1
		} else {
			n = 0
		}
	}

	engine, err := gitengine.Open(*repo)
	if err != nil {
		return err
	}

	diffs, err := engine.RevSpec(spec, n)
	if err != nil {
		return err
	}
	diffs = gitengine.FilterDiffs(diffs, *file)
	if len(diffs) == 0 {
		if *file != "" {
			return fmt.Errorf("no commits touch %q in %s", *file, spec)
		}
		return fmt.Errorf("no commits to replay for %s", spec)
	}

	tracks := make([]ui.Track, 0, len(diffs))
	for i, d := range diffs {
		tracks = append(tracks, ui.Track{
			Diff:   d,
			Script: animator.Compile(d, animator.Options{Seed: compileSeed(*seed, i)}),
		})
	}

	if *inspect || *script {
		return dump(tracks, *inspect, *script)
	}

	model := ui.NewPlaylist(tracks, scheduler.Options{
		FPS:   *fps,
		Speed: *speed,
	}, *theme)
	model.InterGap = ui.DefaultInterGap
	_, err = tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func dump(tracks []ui.Track, inspect, script bool) error {
	for i, t := range tracks {
		if i > 0 {
			fmt.Println()
		}
		if inspect {
			fmt.Print(t.Diff.Format())
		}
		if script {
			if inspect {
				fmt.Println()
			}
			fmt.Print(t.Script.Format())
		}
	}
	return nil
}

func compileSeed(base int64, i int) int64 {
	if base == 0 {
		return 0
	}
	return base + int64(i)
}
