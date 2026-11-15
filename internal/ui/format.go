// Package ui renders vigie's table, both as the interactive Bubble Tea
// program and as a one-shot plain-text snapshot.
package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/sperano/vigie/internal/board"
)

// Column widths; DETAIL takes what is left of the terminal.
const (
	markWidth      = 1
	typeWidth      = 13
	originWidth    = 8
	nameWidth      = 34
	stateWidth     = 20
	ageWidth       = 5
	columnGap      = " "
	columnCount    = 7
	minDetailWidth = 20
	ellipsis       = "…"
	blankMark      = " "
)

// fixedWidth is the width of every column but DETAIL, gaps included.
const fixedWidth = markWidth + typeWidth + originWidth + nameWidth + stateWidth + ageWidth + (columnCount-1)*len(columnGap)

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

// formatColumns lays out one table line of exactly width cells, or of
// fixedWidth+minDetailWidth when the terminal is narrower.
func formatColumns(mark, typ, origin, name, state, age, detail string, width int) string {
	detailWidth := max(minDetailWidth, width-fixedWidth)
	return strings.Join([]string{
		padRight(mark, markWidth),
		padRight(typ, typeWidth),
		padRight(origin, originWidth),
		padRight(name, nameWidth),
		padRight(state, stateWidth),
		padLeft(age, ageWidth),
		padRight(detail, detailWidth),
	}, columnGap)
}

func headerLine(width int) string {
	return formatColumns(blankMark, "TYPE", "ORIGIN", "NAME", "STATE", "AGE", "DETAIL", width)
}

func rowLine(e board.Entry, width int) string {
	r := e.Row
	return formatColumns(e.Mark, r.Source, e.Origin, e.Prefix+r.Name, r.State, e.Age, oneLine(r.Detail), width)
}

// oneLine flattens newlines so that a detail cannot break the table.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
