// Package ui renders atop's table, both as the interactive Bubble Tea
// program and as a one-shot plain-text snapshot.
package ui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/sperano/atop/internal/source"
)

// Column widths; DETAIL takes what is left of the terminal.
const (
	markWidth      = 1
	sourceWidth    = 13
	nameWidth      = 34
	stateWidth     = 18
	ageWidth       = 5
	columnGap      = " "
	columnCount    = 6
	minDetailWidth = 20
	ellipsis       = "…"
	needsYouMark   = "●"
	blankMark      = " "
	unknownAge     = "-"
	hoursPerDay    = 24
)

// fixedWidth is the width of every column but DETAIL, gaps included.
const fixedWidth = markWidth + sourceWidth + nameWidth + stateWidth + ageWidth + (columnCount-1)*len(columnGap)

var ageUnits = []struct {
	suffix string
	size   time.Duration
}{
	{"d", hoursPerDay * time.Hour},
	{"h", time.Hour},
	{"m", time.Minute},
	{"s", time.Second},
}

// FormatAge renders the time since `since` in its largest whole unit.
func FormatAge(since, now time.Time) string {
	if since.IsZero() {
		return unknownAge
	}
	elapsed := max(now.Sub(since), 0)
	for _, u := range ageUnits {
		if elapsed >= u.size {
			return fmt.Sprintf("%d%s", elapsed/u.size, u.suffix)
		}
	}
	return "0s"
}

// fit truncates text to width display cells, ending with an ellipsis.
func fit(text string, width int) string {
	return ansi.Truncate(text, width, ellipsis)
}

// padRight fits text to exactly width cells.
func padRight(text string, width int) string {
	text = fit(text, width)
	return text + strings.Repeat(" ", width-ansi.StringWidth(text))
}

func padLeft(text string, width int) string {
	text = fit(text, width)
	return strings.Repeat(" ", width-ansi.StringWidth(text)) + text
}

// group orders rows: needs you, then busy, then the rest.
func group(r source.Row) int {
	switch {
	case r.NeedsYou:
		return 0
	case r.Busy:
		return 1
	default:
		return 2
	}
}

// SortRows orders rows by group, each group most recent first, undated last.
func SortRows(rows []source.Row) {
	slices.SortStableFunc(rows, func(a, b source.Row) int {
		if ga, gb := group(a), group(b); ga != gb {
			return ga - gb
		}
		return b.Since.Compare(a.Since)
	})
}

// Summary counts the rows that need you, are busy, and are idle.
func Summary(rows []source.Row) string {
	var needs, busy, idle int
	for _, r := range rows {
		switch {
		case r.NeedsYou:
			needs++
		case r.Busy:
			busy++
		}
		if r.State == source.StateIdle {
			idle++
		}
	}
	return fmt.Sprintf("%d need you · %d busy · %d idle", needs, busy, idle)
}

// formatColumns lays out one table line of exactly width cells, or of
// fixedWidth+minDetailWidth when the terminal is narrower.
func formatColumns(mark, src, name, state, age, detail string, width int) string {
	detailWidth := max(minDetailWidth, width-fixedWidth)
	return strings.Join([]string{
		padRight(mark, markWidth),
		padRight(src, sourceWidth),
		padRight(name, nameWidth),
		padRight(state, stateWidth),
		padLeft(age, ageWidth),
		padRight(detail, detailWidth),
	}, columnGap)
}

func headerLine(width int) string {
	return formatColumns(blankMark, "SOURCE", "NAME", "STATE", "AGE", "DETAIL", width)
}

func rowLine(r source.Row, now time.Time, width int) string {
	mark := blankMark
	if r.NeedsYou {
		mark = needsYouMark
	}
	return formatColumns(mark, r.Source, r.Name, r.State, FormatAge(r.Since, now), oneLine(r.Detail), width)
}

// oneLine flattens newlines so that a detail cannot break the table.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
