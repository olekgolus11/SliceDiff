package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/olekgolus11/SliceDiff/internal/config"
	"github.com/olekgolus11/SliceDiff/internal/github"
	"github.com/olekgolus11/SliceDiff/internal/tui"
)

const version = "0.1.0-dev"

func main() {
	var runner string
	var noAI bool
	var regen bool

	flag.StringVar(&runner, "ai-runner", "", "AI runner override: codex or opencode")
	flag.BoolVar(&noAI, "no-ai", false, "disable AI grouping and show raw diff navigation only")
	flag.BoolVar(&regen, "regen-slices", false, "ignore cached semantic slices and regenerate")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: slicediff [--ai-runner codex|opencode] [--no-ai] [--regen-slices] <pr-url|owner/repo#number|number>")
		os.Exit(2)
	}

	target, err := github.ParseTarget(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid PR target: %v\n", err)
		os.Exit(2)
	}

	store, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not load config: %v\n", err)
		os.Exit(1)
	}

	model := tui.New(tui.Options{
		Target:         target,
		Config:         store,
		RunnerOverride: runner,
		NoAI:           noAI,
		RegenSlices:    regen,
		Version:        version,
	})

	if _, err := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "slicediff failed: %v\n", err)
		os.Exit(1)
	}
}
